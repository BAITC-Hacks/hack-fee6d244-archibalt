package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/ai"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/store"
)

// Визуальный концепт первого результата — необязательная фича: репозиторий без этих методов
// (тестовый fake) её просто не поддерживает; обязательный путь диалога от неё не зависит.
type visualRepo interface {
	SaveVisual(ctx context.Context, taskID int, v store.Visual) error
	GetVisual(ctx context.Context, taskID int) (store.Visual, error)
	HasVisual(ctx context.Context, taskID int) (bool, error)
}

var (
	imageGenOnce sync.Once
	visualMu     sync.Mutex
	visualBusy   = map[int]bool{}
	visualCount  = map[int]int{}
	imageGen     *ai.ImageGen
)

func getImageGen() *ai.ImageGen {
	imageGenOnce.Do(func() { imageGen = ai.NewImageGen() })
	return imageGen
}

const visualMaxPerTask = 5

func (s *server) registerVisual(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tasks/{id}/visual", s.createVisual)
	mux.HandleFunc("GET /api/tasks/{id}/visual.png", s.getVisual)
}

// hasVisual — для has_visual в ответе задачи; ошибки ответу не мешают.
func (s *server) hasVisual(ctx context.Context, id int) bool {
	vr, ok := s.repo.(visualRepo)
	if !ok {
		return false
	}
	has, err := vr.HasVisual(ctx, id)
	if err != nil {
		slog.Warn("has visual", "task", id, "err", err)
	}
	return has
}

// createVisual — POST /api/tasks/{id}/visual: только заявитель; генерирует (до 60 с) и сохраняет, повтор перезаписывает.
// Поля задачи не меняются.
func (s *server) createVisual(w http.ResponseWriter, r *http.Request) {
	vr, ok := s.repo.(visualRepo)
	if !ok {
		writeError(w, http.StatusNotImplemented, "визуальный концепт недоступен на этом сервере")
		return
	}
	t, ok := s.loadTask(w, r)
	if !ok || !s.requireOwner(w, r, t, notEditorMsg) {
		return
	}
	// ponytail: лимит в памяти — не больше visualMaxPerTask генераций на задачу и одна одновременно; общий бюджет — после сдачи.
	visualMu.Lock()
	if visualBusy[t.ID] || visualCount[t.ID] >= visualMaxPerTask {
		busy := visualBusy[t.ID]
		visualMu.Unlock()
		if busy {
			writeError(w, http.StatusTooManyRequests, "концепт уже рисуется, подождите")
		} else {
			writeError(w, http.StatusTooManyRequests, "лимит генераций для этой задачи исчерпан")
		}
		return
	}
	visualBusy[t.ID] = true
	visualCount[t.ID]++
	visualMu.Unlock()
	defer func() { visualMu.Lock(); delete(visualBusy, t.ID); visualMu.Unlock() }()
	ctx, cancel := context.WithTimeout(r.Context(), 65*time.Second)
	defer cancel()
	start := time.Now()
	img, err := getImageGen().Generate(ctx, t.Fields)
	ms := time.Since(start).Milliseconds()
	if err != nil {
		slog.Warn("visual generate", "task", t.ID, "err", err, "ms", ms)
		writeError(w, http.StatusBadGateway, "не получилось нарисовать концепт: "+err.Error()+". Карточка не изменена, можно повторить.")
		return
	}
	slog.Info("visual generated", "task", t.ID, "model", img.Model, "mode", img.Mode, "bytes", len(img.Data), "ms", ms)
	if err := vr.SaveVisual(r.Context(), t.ID, store.Visual{Mime: img.Mime, Data: img.Data, Prompt: img.Prompt, AIMode: string(img.Mode)}); err != nil {
		fail(w, err, "задача не найдена")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":     fmt.Sprintf("/api/tasks/%d/visual.png?ts=%d", t.ID, time.Now().UnixMilli()),
		"ai_mode": img.Mode, "model": img.Model, "prompt": img.Prompt, "latency_ms": ms,
	})
}

// getVisual — GET /api/tasks/{id}/visual.png: заявителю всегда, остальным — только у опубликованной задачи, иначе 404.
func (s *server) getVisual(w http.ResponseWriter, r *http.Request) {
	vr, ok := s.repo.(visualRepo)
	if !ok {
		writeError(w, http.StatusNotFound, "концепта нет")
		return
	}
	t, ok := s.loadTask(w, r)
	if !ok {
		return
	}
	if t.Status != model.StatusPublished {
		c, logged := s.businessFromRequest(r)
		if !logged || t.OwnerContact == "" || !strings.EqualFold(c, t.OwnerContact) {
			writeError(w, http.StatusNotFound, "концепта нет")
			return
		}
	}
	v, err := vr.GetVisual(r.Context(), t.ID)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			slog.Warn("get visual", "task", t.ID, "err", err)
		}
		writeError(w, http.StatusNotFound, "концепта нет")
		return
	}
	w.Header().Set("Content-Type", v.Mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(v.Data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-AI-Mode", v.AIMode)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	_, _ = w.Write(v.Data)
}
