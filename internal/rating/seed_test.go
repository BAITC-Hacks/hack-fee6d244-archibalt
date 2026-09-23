package rating

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// Регрессия на реальные демо-данные: баллы и все четыре уровня готовности.
// При правке seed/tasks.json или формулы обновить таблицу осознанно.
func TestRealSeedScoresAndLevels(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "seed", "tasks.json"))
	if err != nil {
		t.Skip("seed/tasks.json недоступен:", err)
	}
	var tasks []struct {
		ID     int          `json:"id"`
		Fields model.Fields `json:"fields"`
	}
	if err := json.Unmarshal(raw, &tasks); err != nil {
		t.Fatal(err)
	}
	want := map[int]struct {
		score int
		level model.Level
	}{
		1: {30, model.LevelDraft},
		2: {60, model.LevelWorking},
		3: {80, model.LevelReady},
		4: {90, model.LevelPriority},
		5: {42, model.LevelWorking},
	}
	seen := map[model.Level]bool{}
	for _, tk := range tasks {
		r := Compute(tk.Fields, true)
		w, ok := want[tk.ID]
		if !ok {
			t.Errorf("задача %d не в таблице ожиданий (score=%d)", tk.ID, r.Score)
			continue
		}
		if r.Score != w.score || r.Level != w.level {
			t.Errorf("задача %d: score=%d level=%s, ожидалось %d %s", tk.ID, r.Score, r.Level, w.score, w.level)
		}
		seen[r.Level] = true
	}
	for _, l := range []model.Level{model.LevelDraft, model.LevelWorking, model.LevelReady, model.LevelPriority} {
		if !seen[l] {
			t.Errorf("в демо-данных нет уровня %s", l)
		}
	}
}
