package rating

import (
	"strings"

	"github.com/BAITC-Hacks/hack-fee6d244-archibalt/internal/model"
)

// greetings — реплики, которые не несут сведений о задаче ни в каком поле.
var greetings = map[string]bool{"здравствуйте": true, "привет": true, "добрый день": true, "добрый вечер": true, "доброе утро": true,
	"здравствуй": true, "hi": true, "hello": true, "спасибо": true, "пока не знаю": true, "не знаю пока": true}

// vague — «всё», «по-разному», «пока ничего»: на вопрос о данных или ограничениях это факт («материалов нет»),
// на «все» в multi — выбор всех вариантов; шумом считается только в context/need/users, где фактом не является.
var vague = map[string]bool{"пока ничего": true, "пока нет": true, "ничего": true, "по-разному": true, "по разному": true, "всё": true, "все": true}

// vagueNoiseFields — поля, где расплывчатый ответ не описывает задачу и тему надо уточнить.
var vagueNoiseFields = map[model.FieldKey]bool{model.FieldContext: true, model.FieldNeed: true, model.FieldUsers: true}

// IsNoise — ответ не содержит сведений: приветствие, заглушка («не знаю», «x»), отказ («нет данных»);
// для context/need/users — ещё и расплывчатое «всё», «по-разному», «пока ничего».
// Такой ответ не должен попадать в поля карточки как факт.
func IsNoise(k model.FieldKey, text string) bool {
	v := strings.ToLower(strings.Join(strings.Fields(text), " "))
	v = strings.TrimRight(strings.ReplaceAll(v, "ё", "е"), " .!?,")
	if vague[v] || vague[strings.ReplaceAll(v, "е", "ё")] {
		return vagueNoiseFields[k]
	}
	return v == "" || greetings[v] || isStub(k, text) || isRefusal(text)
}

// IsGibberish — текст не описывает ситуацию: пусто, заглушка или набор букв («фвфовфыов о»).
func IsGibberish(text string) bool {
	switch evalField(model.FieldContext, text).st {
	case stEmpty, stStub, stJunk, stGibberish:
		return true
	}
	return false
}
