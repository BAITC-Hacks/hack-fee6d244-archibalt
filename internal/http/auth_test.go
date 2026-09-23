package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

func TestNormalizeContact(t *testing.T) {
	for in, want := range map[string]string{
		"  Team@Example.KZ ":  "team@example.kz",
		"+7 (701) 123-45-67":  "+77011234567",
		"8 701 123 45 67":     "+77011234567",
		"77011234567":         "+77011234567",
		"+44 20 7946 0958":    "+442079460958",
		"7011234567":          "7011234567",
		"":                    "",
		"hello":               "",
		"a@b":                 "",
		"@example.com":        "",
		"12345":               "",
		"+7 701 abc 45 67":    "",
		"1234567890123456789": "",
	} {
		got, ok := normalizeContact(in)
		if got != want || ok != (want != "") {
			t.Errorf("normalizeContact(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestAuth(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}

	// request-code
	var rc struct {
		Sent, Demo bool
		Hint       string
	}
	c.do("POST", "/api/auth/request-code", map[string]string{"contact": "Team@Example.com"}, 200, &rc)
	if !rc.Sent || !rc.Demo || !strings.Contains(rc.Hint, "000000") {
		t.Fatalf("request-code: %+v", rc)
	}
	c.do("POST", "/api/auth/request-code", map[string]string{"contact": "+7 (701) 123-45-67"}, 200, nil)
	var errResp struct{ Error string }
	c.do("POST", "/api/auth/request-code", map[string]string{"contact": "мусор"}, 400, &errResp)
	if errResp.Error != badContactMsg {
		t.Fatalf("ожидалась ошибка про контакт: %q", errResp.Error)
	}

	// verify
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "team@example.com", "code": "123456"}, 401, &errResp)
	if errResp.Error != "неверный код" {
		t.Fatalf("401: %q", errResp.Error)
	}
	type verifyResp struct {
		Token   string
		Team    model.Team
		Created bool
	}
	var v1, v2 verifyResp
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "Team@Example.com", "code": "000000"}, 200, &v1)
	if v1.Token == "" || !v1.Created || v1.Team.ID == 0 || v1.Team.Contact != "team@example.com" || v1.Team.Name != "Команда team@example.com" {
		t.Fatalf("verify (создание): %+v", v1)
	}
	c.do("POST", "/api/auth/verify", map[string]string{"contact": " TEAM@example.com ", "code": "000000", "team_name": "Другое имя"}, 200, &v2)
	if v2.Created || v2.Team.ID != v1.Team.ID || v2.Token == v1.Token {
		t.Fatalf("повторный verify должен вернуть ту же команду с новым токеном: %+v vs %+v", v2, v1)
	}

	// team_name = имя seed-команды без контакта → привязка
	var v3 verifyResp
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "8 701 123 45 67", "code": "000000", "team_name": " байты "}, 200, &v3)
	if v3.Created || v3.Team.ID != 1 || v3.Team.Contact != "+77011234567" || repo.teams[1].Contact != "+77011234567" {
		t.Fatalf("привязка seed-команды: %+v", v3)
	}
	// seed-команда уже с контактом: другой контакт с тем же именем создаёт новую команду
	var v4 verifyResp
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "other@example.com", "code": "000000", "team_name": "Байты"}, 200, &v4)
	if !v4.Created || v4.Team.ID == 1 || v4.Team.Name != "Байты" {
		t.Fatalf("занятая seed-команда не должна перехватываться: %+v", v4)
	}

	// /api/me
	c.do("GET", "/api/me", nil, 401, &errResp)
	if errResp.Error != "нужен вход команды" {
		t.Fatalf("me 401: %q", errResp.Error)
	}
	c.doAuth("bogus", "GET", "/api/me", nil, 401, nil)
	type meResp struct {
		Team      model.Team
		Proposals []MyProposal
	}
	var me meResp
	c.doAuth(v1.Token, "GET", "/api/me", nil, 200, &me)
	if me.Team.ID != v1.Team.ID || me.Proposals == nil || len(me.Proposals) != 0 {
		t.Fatalf("me до отклика: %+v", me)
	}

	// опубликованная задача
	fields := model.Fields{model.FieldTitle: "Учёт заявок"}.Full()
	task := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: fields, Rating: model.Rating{Score: 55, Level: model.LevelWorking, LevelLabel: "рабочая"}}
	if err := repo.CreateTask(t.Context(), &task); err != nil {
		t.Fatal(err)
	}
	ppath := fmt.Sprintf("/api/tasks/%d/proposals", task.ID)
	body := map[string]any{"team_id": 1, "idea": "Бот", "plan": "2 спринта", "deadline": "3 недели", "link": "https://example.com"}
	c.do("POST", ppath, body, 401, nil)
	var prop model.Proposal
	c.doAuth(v1.Token, "POST", ppath, body, 201, &prop)
	if prop.Team.ID != v1.Team.ID || prop.Team.Name != v1.Team.Name {
		t.Fatalf("команда отклика должна браться из токена, а не из team_id: %+v", prop.Team)
	}

	c.doAuth(v1.Token, "GET", "/api/me", nil, 200, &me)
	if len(me.Proposals) != 1 {
		t.Fatalf("me после отклика: %+v", me.Proposals)
	}
	got := me.Proposals[0]
	if got.ID != prop.ID || got.Task.ID != task.ID || got.Task.Title != "Учёт заявок" || got.Task.Industry != "IT" ||
		got.Task.Score != 55 || got.Task.Level != model.LevelWorking || got.Task.LevelLabel != "рабочая" || got.Task.Status != model.StatusPublished {
		t.Fatalf("MyProposal: %+v", got)
	}

	// logout
	var lo struct{ OK bool }
	c.doAuth(v1.Token, "POST", "/api/auth/logout", nil, 200, &lo)
	if !lo.OK {
		t.Fatal("logout: ok=false")
	}
	c.doAuth(v1.Token, "GET", "/api/me", nil, 401, nil)
	c.doAuth(v2.Token, "GET", "/api/me", nil, 200, nil) // другие сессии той же команды живы
	c.do("POST", "/api/auth/logout", nil, 200, nil)     // без токена тоже 200
}
