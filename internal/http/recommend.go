package httpapi

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// minRecommendScore — ТЗ §4: рекомендовать можно задачи уровня «рабочая» (40+) и выше.
const minRecommendScore = 40

// stemLen — грубый стемминг для русского: сравниваем первые 5 букв ("данные" ~ "данных").
const stemLen = 5

var recommendationStopWords = map[string]bool{
	"и": true, "в": true, "во": true, "на": true, "по": true, "с": true, "со": true,
	"к": true, "ко": true, "у": true, "из": true, "от": true, "до": true, "за": true,
	"для": true, "о": true, "об": true, "не": true, "ни": true, "а": true, "но": true,
	"или": true, "как": true, "что": true, "это": true, "при": true, "про": true, "под": true,
	"над": true, "the": true, "and": true, "for": true, "with": true, "to": true, "of": true,
	"in": true, "on": true, "is": true, "a": true, "an": true,
}

func stems(texts ...string) map[string]bool {
	out := map[string]bool{}
	for _, text := range texts {
		for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '+' && r != '#'
		}) {
			if recommendationStopWords[w] {
				continue
			}
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

// recommend — простое детерминированное пересечение контекста команды с карточкой задачи.
// Рекомендация не ограничивает каталог и ничего не назначает (ТЗ §5).
func recommend(team model.Team, tasks []model.Task, limit int) []model.Task {
	teamWords := stems(profileTexts(team)...)
	type scored struct {
		t       model.Task
		match   int
		reasons []string
	}
	var list []scored
	for _, t := range tasks {
		if t.Status != model.StatusPublished || t.Score < minRecommendScore {
			continue
		}
		texts := []string{t.Industry}
		for _, v := range t.Fields {
			texts = append(texts, v)
		}
		taskWords := stems(texts...)
		reasons := profileMatchReasons(team, taskWords)
		n := len(reasons)
		// Keep the old score semantics: one matching stem is enough to recommend;
		// several words in the same profile item don't inflate the result.
		if n == 0 && len(teamWords) > 0 {
			continue
		}
		if n > 0 {
			t.MatchReasons = reasons
			list = append(list, scored{t, n, reasons})
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

func profileTexts(team model.Team) []string {
	texts := append([]string{}, team.Skills...)
	texts = append(texts, team.Interests...)
	texts = append(texts, team.Tech...)
	if strings.TrimSpace(team.Experience) != "" {
		texts = append(texts, team.Experience)
	}
	if strings.TrimSpace(team.Achievements) != "" {
		texts = append(texts, team.Achievements)
	}
	return texts
}

// profileMatchReasons возвращает только факты из профиля команды, которые
// пересеклись с текстом задачи. Никаких сгенерированных «подходит потому что».
func profileMatchReasons(team model.Team, taskWords map[string]bool) []string {
	groups := []struct {
		label string
		items []string
	}{
		{"Навык", team.Skills},
		{"Интерес", team.Interests},
		{"Технология", team.Tech},
		{"Опыт", []string{team.Experience}},
		{"Достижение", []string{team.Achievements}},
	}
	seen := map[string]bool{}
	var reasons []string
	for _, group := range groups {
		for _, item := range group.items {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			matched := false
			for word := range stems(item) {
				if taskWords[word] {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			reason := fmt.Sprintf("%s: %s", group.label, item)
			if !seen[reason] {
				seen[reason] = true
				reasons = append(reasons, reason)
			}
		}
	}
	return reasons
}
