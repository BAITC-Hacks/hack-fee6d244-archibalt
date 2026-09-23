package rating

import (
	"strings"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// greetings — реплики, которые не несут сведений о задаче.
var greetings = map[string]bool{"здравствуйте": true, "привет": true, "добрый день": true, "добрый вечер": true, "доброе утро": true,
	"здравствуй": true, "hi": true, "hello": true, "спасибо": true, "пока ничего": true, "пока не знаю": true, "не знаю пока": true,
	"пока нет": true, "ничего": true, "по-разному": true, "по разному": true, "всё": true, "все": true}

// IsNoise — ответ не содержит сведений: приветствие, заглушка («не знаю», «x»), отказ («нет данных»).
// Такой ответ не должен попадать в поля карточки как факт.
func IsNoise(k model.FieldKey, text string) bool {
	v := strings.ToLower(strings.Join(strings.Fields(text), " "))
	v = strings.TrimRight(strings.ReplaceAll(v, "ё", "е"), " .!?,")
	return v == "" || greetings[v] || isStub(k, text) || isRefusal(text)
}
