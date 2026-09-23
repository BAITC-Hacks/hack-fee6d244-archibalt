package httpapi

import (
	"fmt"
	"testing"
)

// Задачу с контактом заявителя правит и подтверждает только заявитель; открытую — любой.
func TestOwnedTaskEditRequiresOwner(t *testing.T) {
	f := newChatFixture(t)
	path := fmt.Sprintf("/api/tasks/%d/fields", f.task.ID)
	body := map[string]any{"fields": map[string]string{"need": "Перестать терять заявки и видеть статусы"}}
	f.c.do("PUT", path, body, 403, nil)
	f.c.doAuth(f.other, "PUT", path, body, 403, nil)
	f.c.doAuth(f.owner, "PUT", path, body, 200, nil)
	f.c.do("POST", fmt.Sprintf("/api/tasks/%d/confirm", f.task.ID), nil, 403, nil)
	f.c.doAuth(f.owner, "POST", fmt.Sprintf("/api/tasks/%d/confirm", f.task.ID), nil, 200, nil)
}
