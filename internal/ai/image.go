package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Image — сгенерированный визуальный концепт.
type Image struct {
	Mime   string
	Data   []byte
	Prompt string
	Mode   model.AIMode
	Model  string
}

// ImageGen — генерация визуального концепта первого результата (OpenAI GPT Image API). Отдельно от Client:
// фича необязательная; без ключа — детерминированная SVG-заглушка (Mode = mock).
type ImageGen struct {
	apiKey   string
	model    string
	quality  string
	endpoint string
	http     *http.Client
}

// NewImageGen читает OPENAI_API_KEY, OPENAI_IMAGE_MODEL (по умолчанию gpt-image-1), OPENAI_IMAGE_QUALITY (low).
func NewImageGen() *ImageGen {
	g := &ImageGen{apiKey: os.Getenv("OPENAI_API_KEY"), model: os.Getenv("OPENAI_IMAGE_MODEL"), quality: os.Getenv("OPENAI_IMAGE_QUALITY"),
		endpoint: "https://api.openai.com/v1/images/generations", http: &http.Client{Timeout: 60 * time.Second}}
	if g.model == "" {
		g.model = "gpt-image-1"
	}
	if g.quality == "" {
		g.quality = "low"
	}
	return g
}

// ImagePrompt — промпт картинки по полям задачи; тип изображения выбирается по содержанию результата.
func ImagePrompt(f model.Fields) string {
	low := strings.ToLower(f[model.FieldExpectedResult] + " " + f[model.FieldNeed])
	kind := "the key screen of the product (web or mobile interface mockup)"
	switch {
	case containsAny(low, "процесс", "маршрут", "схем", "интеграц", "автоматиз", "workflow", "pipeline"):
		kind = "a clean process diagram showing how the solution works step by step (icons and arrows)"
	case containsAny(low, "отчёт", "отчет", "аналит", "прогноз", "исследован", "презентац"):
		kind = "an example of the delivered result: a report or dashboard page with charts"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Visual concept for discussion: %s.\n", kind)
	for _, k := range []model.FieldKey{model.FieldTitle, model.FieldNeed, model.FieldExpectedResult, model.FieldUsers} {
		if v := strings.TrimSpace(f[k]); v != "" {
			fmt.Fprintf(&b, "%s: %s\n", model.FieldLabels[k], clipRunes(v, 400))
		}
	}
	b.WriteString("Style: clean modern product concept, flat UI, soft neutral colors, generous whitespace. " +
		"No small text, no readable words (use placeholder bars instead of text), no logos, no brand names, no watermarks.")
	return b.String()
}

func containsAny(s string, subs ...string) bool {
	for _, x := range subs {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}

func clipRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// Generate — картинка по полям задачи. Без ключа — mock. Ошибка API возвращается как есть: подменять её
// заглушкой нельзя, иначе mock выдавался бы за концепт.
func (g *ImageGen) Generate(ctx context.Context, f model.Fields) (Image, error) {
	prompt := ImagePrompt(f)
	if g.apiKey == "" {
		return Image{Mime: "image/svg+xml", Data: mockSVG(f), Prompt: prompt, Mode: model.AIModeMock, Model: "mock"}, nil
	}
	body, _ := json.Marshal(map[string]any{"model": g.model, "prompt": prompt, "size": "1024x1024", "quality": g.quality, "n": 1})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(body))
	if err != nil {
		return Image{}, err
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.http.Do(req)
	if err != nil {
		var ne interface{ Timeout() bool }
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			return Image{}, errors.New("генерация не уложилась в 60 секунд")
		}
		return Image{}, errors.New("нет связи с OpenAI")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return Image{}, err
	}
	var out struct {
		Data []struct {
			B64 string `json:"b64_json"`
		} `json:"data"`
		OutputFormat string `json:"output_format"`
		Error        *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &out)
	if resp.StatusCode != http.StatusOK {
		msg := resp.Status
		if out.Error != nil && out.Error.Message != "" {
			msg = out.Error.Message
		}
		return Image{}, fmt.Errorf("OpenAI Images %d: %s", resp.StatusCode, clipRunes(msg, 200))
	}
	if len(out.Data) == 0 || out.Data[0].B64 == "" {
		return Image{}, errors.New("OpenAI Images: пустой ответ")
	}
	data, err := base64.StdEncoding.DecodeString(out.Data[0].B64)
	if err != nil {
		return Image{}, fmt.Errorf("OpenAI Images: %w", err)
	}
	mime := "image/png"
	if out.OutputFormat == "jpeg" || out.OutputFormat == "webp" {
		mime = "image/" + out.OutputFormat
	}
	return Image{Mime: mime, Data: data, Prompt: prompt, Mode: model.AIModeOpenAI, Model: g.model}, nil
}

// mockSVG — детерминированная заглушка: оттенок от названия, схема «экрана» и подпись mock.
func mockSVG(f model.Fields) []byte {
	h := fnv.New32a()
	_, _ = h.Write([]byte(f[model.FieldTitle] + f[model.FieldExpectedResult]))
	hue := h.Sum32() % 360
	title := html.EscapeString(clipRunes(strings.TrimSpace(f[model.FieldTitle]), 48))
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024" viewBox="0 0 1024 1024">
<rect width="1024" height="1024" fill="hsl(%[1]d,40%%,95%%)"/>
<rect x="112" y="152" width="800" height="600" rx="28" fill="#fff" stroke="hsl(%[1]d,35%%,80%%)" stroke-width="4"/>
<rect x="112" y="152" width="800" height="72" rx="28" fill="hsl(%[1]d,55%%,55%%)"/>
<rect x="152" y="264" width="220" height="440" rx="16" fill="hsl(%[1]d,40%%,90%%)"/>
<rect x="412" y="264" width="460" height="200" rx="16" fill="hsl(%[1]d,45%%,85%%)"/>
<rect x="412" y="504" width="220" height="200" rx="16" fill="hsl(%[1]d,40%%,90%%)"/>
<rect x="652" y="504" width="220" height="200" rx="16" fill="hsl(%[1]d,40%%,90%%)"/>
<text x="512" y="830" font-family="sans-serif" font-size="40" text-anchor="middle" fill="#334">Пример концепта (mock)</text>
<text x="512" y="886" font-family="sans-serif" font-size="28" text-anchor="middle" fill="#667">%[2]s</text>
</svg>`, hue, title))
}
