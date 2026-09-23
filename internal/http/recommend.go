package httpapi

import (
	"sort"
	"strings"
	"unicode"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// minRecommendScore — ТЗ §4: рекомендовать можно задачи уровня «рабочая» (40+) и выше.
const minRecommendScore = 40

// stemLen — грубый стемминг для русского: сравниваем первые 5 букв ("данные" ~ "данных").
const stemLen = 5

func stems(texts ...string) map[string]bool {
	out := map[string]bool{}
	for _, text := range texts {
		for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+' && r != '#'
		}) {
			rs := []rune(w)
			if len(rs) < 2 {
				continue
			}
			if len(rs) > stemLen {
				rs = rs[:stemLen]
			}
			out[string(rs)] = true
		}
	}
	return out
}

// recommend — простое пересечение слов навыков/интересов/технологий команды со словами
// карточки задачи. Рекомендация не ограничивает каталог и ничего не назначает (ТЗ §5).
func recommend(team model.Team, tasks []model.Task, limit int) []model.Task {
	teamWords := stems(append(append(append([]string{}, team.Skills...), team.Interests...), team.Tech...)...)
	type scored struct {
		t     model.Task
		match int
	}
	var list []scored
	for _, t := range tasks {
		if t.Score < minRecommendScore {
			continue
		}
		texts := []string{t.Industry}
		for _, v := range t.Fields {
			texts = append(texts, v)
		}
		taskWords := stems(texts...)
		n := 0
		for w := range teamWords {
			if taskWords[w] {
				n++
			}
		}
		if n > 0 {
			list = append(list, scored{t, n})
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].match != list[j].match {
			return list[i].match > list[j].match
		}
		return list[i].t.Score > list[j].t.Score
	})
	out := make([]model.Task, 0, limit)
	for i := 0; i < len(list) && i < limit; i++ {
		out = append(out, list[i].t)
	}
	return out
}
