package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

func TestStudentProfileQuickProposalAndRecommendation(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}

	var login struct {
		Token string
		Team  model.Team
	}
	c.do("POST", "/api/auth/verify", map[string]string{
		"contact": "student@example.com", "code": "000000", "team_name": "Команда студента",
	}, 200, &login)
	if login.Token == "" {
		t.Fatal("не получили токен студента")
	}

	var profile struct{ Team model.Team }
	c.doAuth(login.Token, "PUT", "/api/me/profile", map[string]any{
		"skills":       []string{"аналитика"},
		"interests":    []string{"образование"},
		"tech":         []string{"Python"},
		"experience":   "Делали внутренние дашборды",
		"achievements": "Запустили MVP за месяц",
	}, 200, &profile)
	if profile.Team.ID != login.Team.ID || profile.Team.Contact != login.Team.Contact ||
		profile.Team.Experience != "Делали внутренние дашборды" || profile.Team.Achievements != "Запустили MVP за месяц" ||
		len(profile.Team.Tech) != 1 || profile.Team.Tech[0] != "Python" {
		t.Fatalf("профиль команды: %+v", profile.Team)
	}

	fields := model.Fields{}.Full()
	fields[model.FieldTitle] = "Автоматизация отчётов"
	fields[model.FieldNeed] = "Python-скрипт для отчётов"
	task := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: fields, Rating: model.Rating{Score: 70}}
	if err := repo.CreateTask(t.Context(), &task); err != nil {
		t.Fatal(err)
	}
	low := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: model.Fields{model.FieldNeed: "Python"}, Rating: model.Rating{Score: 30}}
	if err := repo.CreateTask(t.Context(), &low); err != nil {
		t.Fatal(err)
	}

	var recommended []model.Task
	c.do("GET", fmt.Sprintf("/api/teams/%d/recommended", login.Team.ID), nil, 200, &recommended)
	if len(recommended) != 1 || recommended[0].ID != task.ID || len(recommended[0].MatchReasons) == 0 ||
		!strings.Contains(strings.Join(recommended[0].MatchReasons, " "), "Технология: Python") {
		t.Fatalf("рекомендации: %+v", recommended)
	}

	path := fmt.Sprintf("/api/tasks/%d/proposals", task.ID)
	c.do("POST", path, map[string]any{"quick": true}, 401, nil)
	var quick model.Proposal
	c.doAuth(login.Token, "POST", path, map[string]any{"quick": true}, 201, &quick)
	if !quick.Quick || quick.ID == 0 || quick.ProfileSnapshot == nil || quick.ProfileSnapshot.Name != "Команда студента" ||
		quick.ProfileSnapshot.Experience == "" || quick.ProfileSnapshot.Achievements == "" {
		t.Fatalf("быстрый отклик: %+v", quick)
	}
	// Повторный клик не создаёт второй быстрый отклик.
	var repeated model.Proposal
	c.doAuth(login.Token, "POST", path, map[string]any{"quick": true}, 201, &repeated)
	if repeated.ID != quick.ID {
		t.Fatalf("дубликат быстрого отклика: first=%d repeat=%d", quick.ID, repeated.ID)
	}

	// Обычный отклик остаётся независимым и по-прежнему разрешён.
	var full model.Proposal
	c.doAuth(login.Token, "POST", path, map[string]any{
		"idea": "Сделаем отчёт", "plan": "Соберём данные и прототип", "deadline": "2 недели", "link": "https://example.com/demo",
	}, 201, &full)
	if full.ID == quick.ID || full.Quick {
		t.Fatalf("обычный отклик был дедуплицирован с quick: %+v", full)
	}

	var other struct{ Token string }
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "other@example.com", "code": "000000"}, 200, &other)
	c.doAuth(other.Token, "PUT", fmt.Sprintf("/api/proposals/%d", quick.ID), map[string]string{"idea": "чужой"}, 403, nil)
	c.doAuth(login.Token, "PUT", fmt.Sprintf("/api/proposals/%d", quick.ID), map[string]string{"idea": "Подробная идея", "link": "https://example.com/spec"}, 200, &quick)
	if quick.Idea != "Подробная идея" || quick.Link != "https://example.com/spec" || quick.Status != model.ProposalNew || quick.Quick != true {
		t.Fatalf("дополнение отклика: %+v", quick)
	}
	c.doAuth(login.Token, "PUT", fmt.Sprintf("/api/proposals/%d", quick.ID), map[string]string{"link": "ftp://example.com"}, 400, nil)
}
