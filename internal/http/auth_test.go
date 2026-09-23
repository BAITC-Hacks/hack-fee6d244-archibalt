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
	if v1.Token == "" || !v1.Created || v1.Team.ID == 0 || v1.Team.Contact != "team@example.com" || !strings.HasPrefix(v1.Team.Name, "Команда ") || strings.Contains(v1.Team.Name, "example") {
		t.Fatalf("verify (создание): %+v", v1)
	}
	c.do("POST", "/api/auth/verify", map[string]string{"contact": " TEAM@example.com ", "code": "000000", "team_name": "Другое имя"}, 200, &v2)
	if v2.Created || v2.Team.ID != v1.Team.ID || v2.Token == v1.Token {
		t.Fatalf("повторный verify должен вернуть ту же команду с новым токеном: %+v vs %+v", v2, v1)
	}

	// team_name = имя seed-команды: чужую команду по имени перехватить нельзя — создаётся своя с этим именем
	var v3 verifyResp
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "8 701 123 45 67", "code": "000000", "team_name": " байты "}, 200, &v3)
	if !v3.Created || v3.Team.ID == 1 || v3.Team.Contact != "+77011234567" || repo.teams[1].Contact != "" {
		t.Fatalf("вход в чужую команду по имени должен быть невозможен: %+v", v3)
	}
	var v4 verifyResp
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "other@example.com", "code": "000000", "team_name": "Байты"}, 200, &v4)
	if !v4.Created || v4.Team.ID == 1 || v4.Team.ID == v3.Team.ID || v4.Team.Name != "Байты" {
		t.Fatalf("у каждого контакта своя команда: %+v", v4)
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

func TestMaskContact(t *testing.T) {
	for in, want := range map[string]string{
		"owner@mail.kz": "o***@mail.kz",
		"+77010000099":  "+7701***0099",
		"7011234567":    "70***4567",
		"":              "",
	} {
		if got := maskContact(in); got != want {
			t.Errorf("maskContact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBusinessOwner(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	var errResp struct{ Error string }

	// вход бизнеса
	c.do("POST", "/api/auth/business/request-code", map[string]string{"contact": "мусор"}, 400, nil)
	var rc struct{ Hint string }
	c.do("POST", "/api/auth/business/request-code", map[string]string{"contact": "owner@mail.kz"}, 200, &rc)
	if !strings.Contains(rc.Hint, "000000") {
		t.Fatalf("hint: %q", rc.Hint)
	}
	c.do("POST", "/api/auth/business/verify", map[string]string{"contact": "owner@mail.kz", "code": "111111"}, 401, nil)
	teamsBefore := len(repo.teams)
	type bizLogin struct{ Token, Contact string }
	var own, other bizLogin
	c.do("POST", "/api/auth/business/verify", map[string]string{"contact": "OWNER@mail.kz", "code": "000000"}, 200, &own)
	c.do("POST", "/api/auth/business/verify", map[string]string{"contact": "+7 701 000 00 99", "code": "000000"}, 200, &other)
	if own.Token == "" || own.Contact != "owner@mail.kz" || other.Contact != "+77010000099" || len(repo.teams) != teamsBefore {
		t.Fatalf("business verify: %+v %+v, команд %d→%d", own, other, teamsBefore, len(repo.teams))
	}

	// создание без входа — анонимный черновик (owner_contact пустой, поле в теле игнорируется); собрать карточку без входа нельзя
	var anon model.Task
	c.do("POST", "/api/tasks", map[string]string{"draft_text": "Нужен бот", "industry": "IT", "owner_contact": "owner@mail.kz"}, 201, &anon)
	if anon.OwnerContact != "" {
		t.Fatalf("анонимная задача с владельцем: %q", anon.OwnerContact)
	}
	c.do("POST", fmt.Sprintf("/api/tasks/%d/answers", anon.ID), map[string]any{"answers": map[string]string{}}, 401, nil)
	// вошедший заявитель присваивает анонимную задачу себе, после чего сборка доступна ему и закрыта чужим
	c.doAuth(own.Token, "POST", fmt.Sprintf("/api/tasks/%d/owner", anon.ID), map[string]any{}, 200, &anon)
	if anon.OwnerContact == "" {
		t.Fatal("присвоение не записало заявителя")
	}
	c.doAuth(other.Token, "POST", fmt.Sprintf("/api/tasks/%d/answers", anon.ID), map[string]any{"answers": map[string]string{}}, 403, nil)
	c.doAuth(own.Token, "POST", fmt.Sprintf("/api/tasks/%d/answers", anon.ID), map[string]any{"answers": map[string]string{}}, 200, nil)
	var team struct{ Token string }
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "team@example.com", "code": "000000"}, 200, &team)
	c.doAuth(team.Token, "POST", "/api/tasks", map[string]string{"draft_text": "Нужен бот", "industry": "IT"}, 201, nil) // токен команды — не заявитель: анонимный черновик
	var task model.Task
	c.doAuth(own.Token, "POST", "/api/tasks", map[string]string{"draft_text": "Нужен бот для заявок", "industry": "IT", "owner_contact": "hijack@mail.kz"}, 201, &task)
	if task.OwnerContact != "o***@mail.kz" || repo.tasks[task.ID].OwnerContact != "owner@mail.kz" {
		t.Fatalf("owner_contact: ответ %q, хранение %q", task.OwnerContact, repo.tasks[task.ID].OwnerContact)
	}
	path := fmt.Sprintf("/api/tasks/%d", task.ID)
	title := map[string]any{"fields": map[string]string{"title": "Бот"}}
	c.do("PUT", path+"/fields", title, 401, nil)
	c.doAuth(other.Token, "PUT", path+"/fields", title, 403, nil)
	c.doAuth(own.Token, "PUT", path+"/fields", title, 200, &task)
	c.doAuth(other.Token, "POST", path+"/confirm", nil, 403, nil)
	c.doAuth(own.Token, "POST", path+"/confirm", nil, 200, &task)
	if task.OwnerContact != "o***@mail.kz" || repo.tasks[task.ID].OwnerContact != "owner@mail.kz" {
		t.Fatalf("после правки контакт должен сохраниться маскированным: %q", task.OwnerContact)
	}
	var cat struct{ Tasks []model.Task }
	c.do("GET", "/api/tasks", nil, 200, &cat)
	if len(cat.Tasks) != 1 || cat.Tasks[0].OwnerContact != "o***@mail.kz" {
		t.Fatalf("каталог должен маскировать контакт: %+v", cat.Tasks)
	}

	// отклик команды
	var prop model.Proposal
	c.doAuth(team.Token, "POST", path+"/proposals", map[string]any{"idea": "Бот", "plan": "2 спринта", "deadline": "3 недели", "link": "https://example.com"}, 201, &prop)
	ppath := fmt.Sprintf("/api/proposals/%d", prop.ID)

	// /api/business/me: полный контакт и отклики
	c.do("GET", "/api/business/me", nil, 401, nil)
	c.doAuth(team.Token, "GET", "/api/business/me", nil, 401, nil) // токен команды не даёт вход бизнеса
	var me struct {
		Contact string
		Tasks   []model.Task
	}
	c.doAuth(own.Token, "GET", "/api/business/me", nil, 200, &me)
	if me.Contact != "owner@mail.kz" || len(me.Tasks) != 1 || me.Tasks[0].ID != task.ID || me.Tasks[0].OwnerContact != "owner@mail.kz" ||
		len(me.Tasks[0].Proposals) != 1 || me.Tasks[0].Proposals[0].ID != prop.ID {
		t.Fatalf("business me: %+v", me)
	}
	var raw struct{ Tasks []map[string]any }
	c.doAuth(own.Token, "POST", "/api/tasks", map[string]string{"draft_text": "Без откликов", "industry": "IT"}, 201, nil)
	c.doAuth(own.Token, "GET", "/api/business/me", nil, 200, &raw)
	if len(raw.Tasks) != 2 || raw.Tasks[0]["proposals"] == nil { // у задачи без откликов — [], а не отсутствие поля
		t.Fatalf("proposals должен быть массивом: %+v", raw.Tasks)
	}
	c.doAuth(other.Token, "GET", "/api/business/me", nil, 200, &me)
	if len(me.Tasks) != 0 {
		t.Fatalf("чужой бизнес не должен видеть задачу: %+v", me.Tasks)
	}
	c.doAuth(own.Token, "GET", "/api/me", nil, 401, nil) // бизнес-токен не даёт вход команды

	// решения по откликам — только заявитель: без токена 401, чужой 403
	c.do("POST", ppath+"/select", nil, 401, nil)
	c.doAuth(team.Token, "POST", ppath+"/select", nil, 401, nil)
	c.doAuth(other.Token, "POST", ppath+"/select", nil, 403, &errResp)
	if errResp.Error != notOwnerMsg {
		t.Fatalf("403: %q", errResp.Error)
	}
	c.do("POST", ppath+"/reject", nil, 401, nil)
	c.doAuth(own.Token, "POST", ppath+"/select", nil, 200, &prop)
	if prop.Status != model.ProposalSelected {
		t.Fatalf("select владельцем: %+v", prop)
	}
	c.doAuth(team.Token, "POST", ppath+"/accept", nil, 200, &prop)
	c.do("POST", ppath+"/confirm-stage", nil, 401, nil)
	c.doAuth(other.Token, "POST", ppath+"/confirm-stage", nil, 403, nil)
	c.doAuth(own.Token, "POST", ppath+"/confirm-stage", nil, 200, &prop)

	// POST /owner: только заявитель
	opath := path + "/owner"
	c.do("POST", opath, map[string]string{"owner_contact": "hijack@mail.kz"}, 401, nil)
	c.doAuth(other.Token, "POST", opath, map[string]string{"owner_contact": "hijack@mail.kz"}, 403, nil)
	c.doAuth(own.Token, "POST", opath, map[string]string{"owner_contact": "bad"}, 400, nil)
	var owned model.Task
	c.doAuth(own.Token, "POST", opath, map[string]string{"owner_contact": "8 701 000 00 99"}, 200, &owned)
	if owned.OwnerContact != "+7701***0099" || repo.tasks[task.ID].OwnerContact != "+77010000099" {
		t.Fatalf("owner: ответ %q, хранение %q", owned.OwnerContact, repo.tasks[task.ID].OwnerContact)
	}
	c.doAuth(own.Token, "PUT", path+"/fields", title, 403, nil) // передал задачу — больше не заявитель
	c.doAuth(other.Token, "PUT", path+"/fields", title, 200, nil)
	c.do("POST", "/api/tasks/999/owner", map[string]string{"owner_contact": "a@b.kz"}, 404, nil)

	// business logout
	c.doAuth(other.Token, "POST", "/api/auth/business/logout", nil, 200, nil)
	c.doAuth(other.Token, "GET", "/api/business/me", nil, 401, nil)
	c.doAuth(other.Token, "POST", ppath+"/reject", nil, 401, nil)

	// seed-задача без заявителя: смотреть и откликаться можно, править и решать — никому (403)
	seed := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: model.Fields{}.Full()}
	if err := repo.CreateTask(t.Context(), &seed); err != nil {
		t.Fatal(err)
	}
	spath := fmt.Sprintf("/api/tasks/%d", seed.ID)
	c.do("GET", spath, nil, 200, nil)
	var sp model.Proposal
	c.doAuth(team.Token, "POST", spath+"/proposals", map[string]any{"idea": "i", "plan": "p", "deadline": "d", "link": "https://x.kz"}, 201, &sp)
	for _, tok := range []string{"", own.Token} {
		c.doAuth(tok, "PUT", spath+"/fields", title, 403, &errResp)
		if errResp.Error != noOwnerMsg {
			t.Fatalf("403 задачи без заявителя: %q", errResp.Error)
		}
		c.doAuth(tok, "POST", spath+"/answers", map[string]any{"answers": map[string]string{}}, 403, nil)
		c.doAuth(tok, "POST", spath+"/confirm", nil, 403, nil)
		c.doAuth(tok, "POST", spath+"/owner", map[string]string{"owner_contact": "owner@mail.kz"}, 403, nil)
		c.doAuth(tok, "POST", fmt.Sprintf("/api/proposals/%d/select", sp.ID), nil, 403, nil)
	}
	if repo.tasks[seed.ID].OwnerContact != "" || repo.proposals[sp.ID].Status != model.ProposalNew {
		t.Fatalf("seed-задача не должна меняться: %+v %+v", repo.tasks[seed.ID].OwnerContact, repo.proposals[sp.ID])
	}
	c.do("GET", "/api/tasks", nil, 200, &cat)
	if len(cat.Tasks) != 1 { // первая задача снята правкой, seed — в каталоге
		t.Fatalf("каталог: %+v", cat.Tasks)
	}
	c.do("GET", "/api/teams/1/recommended", nil, 200, nil)
}

// business/me — «кто откликнулся, какие решения»: у каждого отклика команда, статус, даты и счётчик сообщений.
func TestBusinessMeDecisions(t *testing.T) {
	repo := newMemRepo()
	c := testClient{t, NewHandler(repo, fakeAI{}, fakeRating, t.TempDir(), "")}
	biz := c.bizLogin("decide@mail.kz")
	var task model.Task
	c.doAuth(biz, "POST", "/api/tasks", map[string]string{"draft_text": "Нужен учёт склада", "industry": "Логистика"}, 201, &task)
	path := fmt.Sprintf("/api/tasks/%d", task.ID)
	c.doAuth(biz, "PUT", path+"/fields", map[string]any{"fields": map[string]string{"title": "Склад"}}, 200, nil)
	c.doAuth(biz, "POST", path+"/confirm", nil, 200, nil)

	var a, b struct {
		Token string
		Team  model.Team
	}
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "a@team.kz", "code": "000000", "team_name": "Альфа"}, 200, &a)
	c.do("POST", "/api/auth/verify", map[string]string{"contact": "b@team.kz", "code": "000000", "team_name": "Бета"}, 200, &b)
	body := map[string]any{"idea": "Бот", "plan": "2 спринта", "deadline": "3 недели", "link": "https://example.com"}
	var pa, pb model.Proposal
	c.doAuth(a.Token, "POST", path+"/proposals", body, 201, &pa)
	c.doAuth(b.Token, "POST", path+"/proposals", body, 201, &pb)
	c.doAuth(a.Token, "POST", fmt.Sprintf("/api/proposals/%d/messages", pa.ID), map[string]string{"text": "Готовы начать"}, 201, nil)
	c.doAuth(biz, "POST", fmt.Sprintf("/api/proposals/%d/select", pa.ID), nil, 200, nil)
	c.doAuth(biz, "POST", fmt.Sprintf("/api/proposals/%d/reject", pb.ID), nil, 200, nil)
	c.doAuth(a.Token, "POST", fmt.Sprintf("/api/proposals/%d/accept", pa.ID), nil, 200, nil)

	var me struct {
		Contact string
		Tasks   []model.Task
	}
	c.doAuth(biz, "GET", "/api/business/me", nil, 200, &me)
	if len(me.Tasks) != 1 || me.Tasks[0].ID != task.ID || len(me.Tasks[0].Proposals) != 2 {
		t.Fatalf("business me: %+v", me)
	}
	got := map[int]model.Proposal{}
	for _, p := range me.Tasks[0].Proposals {
		got[p.ID] = p
	}
	ga, gb := got[pa.ID], got[pb.ID]
	if ga.Team.ID != a.Team.ID || ga.Team.Name != "Альфа" || ga.Status != model.ProposalAccepted || ga.AcceptedAt == nil ||
		ga.MessagesCount != 1 || ga.CreatedAt.IsZero() {
		t.Fatalf("отклик Альфы: %+v", ga)
	}
	if gb.Team.ID != b.Team.ID || gb.Team.Name != "Бета" || gb.Status != model.ProposalRejected || gb.AcceptedAt != nil ||
		gb.MessagesCount != 0 || gb.CreatedAt.IsZero() {
		t.Fatalf("отклик Беты: %+v", gb)
	}
}
