package httpapi

import (
	"fmt"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Задачу правит и подтверждает только заявитель: без входа 401, чужой бизнес 403;
// задачу без заявителя (seed) не правит никто — 403.
func TestOwnedTaskEditRequiresOwner(t *testing.T) {
	f := newChatFixture(t)
	path := fmt.Sprintf("/api/tasks/%d", f.task.ID)
	body := map[string]any{"fields": map[string]string{"need": "Перестать терять заявки и видеть статусы"}}
	f.c.do("PUT", path+"/fields", body, 401, nil)
	f.c.doAuth(f.teamA, "PUT", path+"/fields", body, 401, nil)
	f.c.doAuth(f.other, "PUT", path+"/fields", body, 403, nil)
	f.c.doAuth(f.owner, "PUT", path+"/fields", body, 200, nil)
	f.c.do("POST", path+"/confirm", nil, 401, nil)
	f.c.doAuth(f.other, "POST", path+"/confirm", nil, 403, nil)
	f.c.doAuth(f.owner, "POST", path+"/confirm", nil, 200, nil)

	seed := model.Task{Industry: "IT", Status: model.StatusPublished, Fields: model.Fields{}.Full()}
	if err := f.repo.CreateTask(t.Context(), &seed); err != nil {
		t.Fatal(err)
	}
	spath := fmt.Sprintf("/api/tasks/%d", seed.ID)
	for _, tok := range []string{"", f.owner, f.other} {
		f.c.doAuth(tok, "PUT", spath+"/fields", body, 403, nil)
		f.c.doAuth(tok, "POST", spath+"/confirm", nil, 403, nil)
	}
}
