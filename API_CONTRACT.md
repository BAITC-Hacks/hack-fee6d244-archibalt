# Контракт API: Go backend ⇄ Vite frontend (v3, 13:30)

Владелец контракта: Абылай (backend). Изменения — только через этот файл + уведомление в inbox/outbox.
Фронт: **Vite + React + TypeScript** в `web/`, владелец Ильяс. Бэкенд отдаёт JSON под `/api/*` и раздаёт собранный `web/dist` на `/` с SPA-fallback (любой не-`/api` путь → `index.html`).

Dev: бэкенд `go run ./cmd/server` на `:8080`; фронт `npm run dev` на `:5173` с proxy `/api → http://localhost:8080` (в `vite.config.ts`). Prod: `docker compose up` собирает фронт и Go в одном образе.

## Общие правила

- JSON, UTF-8. Даты ISO-8601 строкой. Ошибка: `{ "error": "текст" }` с 400 (валидация) / 404 / 502 (AI недоступен и mock не сработал — не должно случаться).
- Режим «Я бизнес / Я команда» — на фронте (localStorage `mode`); вход обеих ролей по одноразовому коду (разделы ниже). **Любое изменение требует входа:** задачу создаёт, дополняет, подтверждает и решает по её откликам только заявитель (Bearer business с `owner_contact` задачи), откликается — команда (Bearer team). Без токена → 401, чужой токен → 403. Смотреть каталог и карточки можно без входа.
- Все запросы синхронные; чат — по WebSocket (см. раздел «WebSocket чата»). AI-вызовы могут занимать до ~10 с — фронт показывает спиннер.

## Объекты

```ts
type Level = "draft" | "working" | "ready" | "priority";
type FieldKey = "title" | "context" | "need" | "users" | "data" | "constraints"
              | "expected_result" | "success_criteria" | "contact" | "interaction_format";

interface Task {
  id: number; industry: string;
  status: "draft" | "clarifying" | "editing" | "published";
  draft_text: string;
  fields: Record<FieldKey, string>;        // 10 полей ТЗ §3, "" если пусто
  confirmed: boolean;
  score: number;                           // 0–100
  level: Level; level_label: string;       // черновик | рабочая | готовая | приоритетная
  breakdown: { key: string; label: string; weight: number; earned: number; reason: string }[]; // 7 показателей ТЗ §4
  missing:   { key: string; label: string; gain: number; hint: string }[];                    // что добавить, чтобы +gain
  questions: Question[];                   // батч-режим: ≥3 от AI сразу; пошаговый (mode=dynamic): растёт по одному, 3–5
  ai_mode: "openai" | "mock";
  created_at: string; published_at: string | null;
  proposals?: Proposal[];                  // только в GET /api/tasks/{id}
  owner_contact: string;                   // контакт заявителя: у новых задач есть всегда (из бизнес-сессии); в публичных ответах маскирован, полный — в /api/business/me; "" — у seed-задач без заявителя
  // лестница мест — считает бэкенд на каждый ответ, в БД не хранится
  rank: number;                            // место в каталоге (1 = первое) по score DESC, published_at ASC; 0 — не опубликована
  rank_if_confirmed: number;               // место при текущем score, если подтвердить сейчас (равный балл — после уже опубликованных); у опубликованной = rank
  catalog_size: number;                    // сколько задач опубликовано сейчас (сама неопубликованная не входит)
  previous_score: number | null;           // балл до изменения — только в ответах answers / fields / confirm, иначе null
  next_level_gain: number;                 // баллов до следующего уровня (40/70/90); 0 при 90+
}
interface Question {
  id: number; text: string; field_key: FieldKey; answer: string;   // answer "" — нет ответа или пропущен
  input_type?: "text" | "choice" | "multi" | "yes_no";             // только пошаговый режим
  suggestions?: string[];                                          // 2–4 варианта-подсказки (нет у yes_no и в батч-режиме); ответ всё равно строка, multi — через «; »
}
interface Proposal {
  id: number; task_id: number; team: { id: number; name: string };
  idea: string; plan: string; deadline: string; link: string;
  status: "new" | "selected" | "accepted" | "declined" | "rejected" | "on_hold"; stage_confirmed: boolean; created_at: string;
  accepted_at: string | null; messages_count: number;
}
interface Team { id: number; name: string; skills: string[]; interests: string[]; tech: string[]; points: number; }
```

## Предварительный и официальный балл (ТЗ §4: баллы только за заполненные и подтверждённые поля)

Формула одна (`internal/rating.Compute`), но статус балла определяется полем `confirmed`:
- `confirmed=false` → `score` — **предварительный** (preview). Фронт подписывает его «предварительно» и не показывает задачу в каталоге. Любой `PUT /api/tasks/{id}/fields` сбрасывает `confirmed=false` и `status` в `editing`, даже у опубликованной задачи, до повторного `POST /confirm`.
- `confirmed=true` → `score` — **официальный**, задача в каталоге на позиции по нему. Каталог (`GET /api/tasks`) отдаёт только подтверждённые.
Отдельного поля `score_official` нет: официальный балл существует только у подтверждённой задачи. Тесты: `internal/http/handler_test.go` (сброс подтверждения при правке; задача не в каталоге до confirm).

## Лестница мест («55 → 100 · #4 → #1 из 6»)

- Ранги отдают `POST /api/tasks`, `GET /api/tasks/{id}`, `answers`, `fields`, `confirm`, `owner`; в `GET /api/tasks` — `rank`, `catalog_size`, `next_level_gain` у каждой (ранг по всему каталогу, даже при фильтре).
- Балл: `previous_score → score` (если `previous_score` null или равен `score` — показывать только `score`).
- Место: `#{rank_if_confirmed}`; прежнее место — `rank_if_confirmed` из предыдущего ответа (фронт хранит последний `Task` в состоянии, не пересчитывает).
- «из N»: у опубликованной `N = catalog_size`; у неопубликованной `catalog_size` её не включает — писать «из {catalog_size} в каталоге» или `catalog_size + 1` после подтверждения (ответ `confirm` уже вернёт новое `catalog_size`).
- Подсказка уровня: «+{next_level_gain} до {следующий уровень}», при `next_level_gain = 0` — не показывать. После `confirm` `rank == rank_if_confirmed` из ответа до него.

## Эндпоинты

| Метод и путь | Тело запроса | Ответ | Заметки |
|---|---|---|---|
| GET `/api/tasks?industry=&level=` | — | `{ tasks: Task[], industries: string[], levels: {key,label}[], stats: { tasks, proposals, teams } }` | только published; сортировка score DESC, published_at ASC; фильтры опциональны. `stats` — «пульс площадки»: опубликованных задач, всего откликов, всего команд (фильтры на него не влияют) |
| POST `/api/tasks` | `{ draft_text: string, industry: string, mode?: "dynamic" }` | `Task` (status `clarifying`, `questions` заполнены ≥3; при `mode:"dynamic"` или `?mode=dynamic` — ровно 1 первый вопрос, см. «Динамические вопросы»; `owner_contact` маскирован) | **бизнес-токен обязателен** (Bearer business): без него 401 `{error:"чтобы создать задачу, войдите по email или телефону"}`. `owner_contact` берётся из сессии, поле в теле игнорируется. draft_text пустой → 400. AI вызывается здесь |
| GET `/api/tasks/{id}` | — | `Task` с `proposals` | любой статус |
| POST `/api/tasks/{id}/answers` | `{ answers?: { [question_id: string]: string } }` | `Task` (status `editing`, `fields` собраны AI из черновика+ответов, score посчитан, confirmed=false) | AI вызывается здесь; поля без данных остаются "". Переданные ответы перекрывают сохранённые; `{}` или `{answers:{}}` — карточка из ответов, уже сохранённых в `questions` (после пошагового режима) |
| POST `/api/tasks/{id}/next-question` | `{ answer?: string, more?: boolean }` | `NextQuestion` | пошаговый режим, см. «Динамические вопросы». Права — как у `answers` |
| PUT `/api/tasks/{id}/fields` | `{ fields: Partial<Record<FieldKey,string>> }` | `Task` (score/breakdown/missing пересчитаны, confirmed=false) | ручное редактирование, можно вызывать много раз. Только заявитель (Bearer business с `owner_contact` задачи): без токена 401, чужой — 403; у задачи без `owner_contact` (seed) — 403 «у задачи нет заявителя» всем. То же для `answers`, `confirm`, `owner` |
| POST `/api/tasks/{id}/confirm` | — | `Task` (confirmed=true, status `published`, published_at) | ручное подтверждение = публикация. Баллы начисляются только подтверждённым полям, поэтому score до confirm — «предварительный» (фронт так и подписывает) |
| POST `/api/tasks/{id}/proposals` | `{ team_id, idea, plan, deadline, link }` | `Proposal` | все поля обязательны → иначе 400; лимита нет |
| POST `/api/proposals/{id}/select` | — | `Proposal` | заявитель выбрал (права — как у `fields`) |
| POST `/api/proposals/{id}/reject` | — | `Proposal` | заявитель отклонил |
| POST `/api/proposals/{id}/confirm-stage` | — | `Proposal` (+ команде начислены баллы) | заявитель подтвердил этап |
| GET `/api/teams` | — | `Team[]` | для select в форме отклика |
| GET `/api/teams/{id}/recommended` | — | `Task[]` | **вне критического пути** |
| GET `/api/ai` | — | `{ mode, prompt_questions, prompt_card, prompt_next_question, schema_example, last_error: string\|null, last_call: {...}\|null }` | страница «как работает AI» для ТЗ §5; в `last_call` email и телефоны маскированы |
| GET `/api/health` | — | `{ ok: true, db: true, ai_mode }` | для README и проверки экспертом |

## Динамические вопросы (пошаговый режим)

Альтернатива батч-режиму (все вопросы сразу — остаётся как есть, фронт выбирает любой). AI задаёт по одному вопросу и каждый следующий выбирает по черновику и уже данным ответам; останавливается сам, когда сведений достаточно, но **не раньше 3 и не позже 5 вопросов**. До `done` обязательно спрошены ключевые поля (данные, ожидаемый результат, критерии успеха, контакт), если их нет в черновике. Поле с содержательным ответом больше не спрашивается; поле, на которое ответили без сведений (приветствие, «не знаю», «пока ничего» — `rating.IsNoise`) или пропуском, **не закрыто**: остаётся в `missing_fields` и переспрашивается другой формулировкой, но не больше 2 раз на поле. По явному запросу «спросить ещё» диалог продолжается после `done` — **до 8 вопросов всего**.

1. `POST /api/tasks` `{ draft_text, industry, mode: "dynamic" }` (или `?mode=dynamic`) → `Task`, в `questions` — один вопрос (id 1).
2. `POST /api/tasks/{id}/next-question` `{ answer: "..." }` — ответ записывается в последний вопрос, если он без ответа (`answer: ""` = «пропустить», сохраняется как ""; без поля `answer` ничего не записывается). Запрос **без `answer`** (или с `more: true`) — явное «спросить ещё»: `done` модели игнорируется, следующий вопрос по оставшимся пробелам, потолок 8. Затем AI выбирает следующий:

```ts
interface NextQuestion {
  done: boolean;
  question?: Question;          // при done=false: новый вопрос уже добавлен в task.questions (id = max+1)
  reason?: string;              // при done=true: почему сведений достаточно
  asked: number;                // сколько вопросов задано (длина task.questions)
  missing_fields: FieldKey[];   // чего ещё не хватает (без полей с содержательным ответом; шум/пропуск поле не закрывает)
  card_preview: Record<FieldKey, string>; // живая карточка без вызова AI: context = черновик, ответ кладётся в поле своего вопроса
  score_preview: number;        // rating.Compute(card_preview, false) — растущий балл в диалоге
}
```

3. При `done: true` вопросы не добавляются; вызов с ответом снова вернёт done, а вызов без `answer` / с `more: true` («Спросить ещё») даст ещё вопрос, пока всего задано меньше 8; на 8 — `done` с `reason` «задано максимальное число вопросов (8)…». Карточку собирает `POST /api/tasks/{id}/answers` с телом `{}` — из сохранённых ответов, через AI + Guard, как в батч-режиме.

Права: только заявитель (без токена 401, чужой — 403, у задачи без заявителя — 403). Ошибки: неверный JSON → 400, нет задачи → 404. AI: строгая JSON-схема `{done, reason, question: {text, field_key, input_type, suggestions} | null, missing_fields}`, промпт — `GET /api/ai` → `prompt_next_question`; правила 3..5 (по `more` — до 8), «без повторов закрытых полей» и «не больше 2 раз на поле» применяются поверх ответа модели; при ошибке — повтор и детерминированный mock. Замер живого прогона (gpt-6-sol): ~6–9 с на шаг.

```bash
curl -s -X POST localhost:8080/api/tasks -H "Authorization: Bearer $BIZ" -H 'Content-Type: application/json' \
  -d '{"mode":"dynamic","draft_text":"Языковой центр вырос из Excel, заявки теряются в WhatsApp","industry":"Образование"}'
curl -s -X POST localhost:8080/api/tasks/$ID/next-question -H "Authorization: Bearer $BIZ" -H 'Content-Type: application/json' \
  -d '{"answer":"Выгрузка заявок из Excel за 2025 год"}'   # повторять до done:true
curl -s -X POST localhost:8080/api/tasks/$ID/answers -H "Authorization: Bearer $BIZ" -H 'Content-Type: application/json' -d '{}'
```

## Варианты первого результата

После `done: true` в диалоге AI предлагает 2–3 варианта первого проверяемого результата для студенческой команды — от малого к среднему. Это предложения AI, а не факты о бизнесе: в карточку попадает только выбранный (и при желании исправленный) человеком вариант. Варианты в БД не хранятся: считаются по запросу из черновика и ответов, последние показанные запоминаются в памяти сервера для `apply-result`.

```ts
interface ResultOption {
  title: string;   // 3–6 слов
  result: string;  // что команда передаст, 1–2 предложения → fields.expected_result
  check: string;   // как бизнес проверит готовность (наблюдаемо/измеримо) → fields.success_criteria
  needs: string;   // что нужно от бизнеса: данные, доступы, время; неизвестное помечено «нужно уточнить»
  weeks: number;   // оценка срока, 2–8
}
```

| Метод и путь | Тело | Ответ | Примечание |
|---|---|---|---|
| POST `/api/tasks/{id}/result-options` | — | `{ options: ResultOption[], ai_mode }` | 2–3 варианта, отсортированы по `weeks`. Задачу не меняет |
| POST `/api/tasks/{id}/apply-result` | `{ index: number, edits?: { result?: string, check?: string } }` | `Task` (с `previous_score`) | `index` — номер варианта (с 0) из последнего ответа `result-options`. `expected_result` = `edits.result` или `result`, `success_criteria` = `edits.check` или `check`; остальные поля не трогаются; рейтинг пересчитывается, `confirmed=false`, статус `editing` — как `PUT /fields` |

Права — как у `answers`: только заявитель (без токена 401, чужой — 403, у задачи без заявителя — 403). Ошибки: нет `index` или он вне диапазона, неверный JSON → 400; нет задачи → 404; AI недоступен → 502 (на практике не бывает: mock).

Что делает фронт: после `done` в диалоге вызвать `result-options` и показать 2–3 карточки: крупно «Результат» (`result`), ниже «Как проверим» (`check`), «Что нужно от вас» (`needs`), «~N недель» (`weeks`), с заголовком `title`. Карточку можно выбрать и поправить текст результата/проверки → `apply-result` → переход в редактор карточки (`/task/:id/edit`). Можно пропустить варианты и сразу собрать карточку через `answers {}` — `apply-result` имеет смысл вызывать после `answers`, иначе сборка карточки перезапишет эти два поля.

AI: быстрая модель (`OPENAI_MODEL_FAST`), строгая JSON-схема `{options: [{title, result, check, needs, weeks}]}`, промпт — `GET /api/ai` → `prompt_result_options`. Защита от выдумок поверх ответа модели: вариант удаляется, если в `title/result/check/needs` есть число, которого нет в черновике/ответах (в паре с тем же словом), или имя собственное/бренд не из них; `weeks` приводится к 2–8; больше 3 — обрезается; меньше 2 — дополняется детерминированными вариантами mock («Обзор одного процесса» по данным из ответа, «Прототип формы для <пользователи>»). При ошибке модели — повтор и mock. Замер живого прогона: `result-options` ~3.7–4.3 с, `apply-result` ~0.04 с.

```bash
curl -s -X POST localhost:8080/api/tasks/$ID/result-options -H "Authorization: Bearer $BIZ"
curl -s -X POST localhost:8080/api/tasks/$ID/apply-result -H "Authorization: Bearer $BIZ" -H 'Content-Type: application/json' \
  -d '{"index":0,"edits":{"check":"Администратор видит оплаты группы без сверки с WhatsApp"}}'
```

## Экраны фронта (маршруты SPA, на усмотрение Ильяса по дизайну)

`/` каталог · `/task/new` черновик · `/task/:id/clarify` вопросы · `/task/:id/edit` карточка + рейтинг · `/task/:id` публичная карточка + отклики + решения бизнеса · `/ai` как работает AI · `/teams` (после 16:00).

## Что Ильяс может делать прямо сейчас

1. `npm create vite@latest web -- --template react-ts`, react-router, `vite.config.ts` с proxy `/api`. Типы выше — в `web/src/api.ts`. Пока бэкенда нет — mock-данные в `web/src/mock.ts` того же формата.
2. `seed/drafts.json`, `seed/tasks.json`, `seed/teams.json`, `seed/proposals.json` по объектам выше (для Task достаточно `industry, draft_text, fields, confirmed, status`; score считает бэкенд).
3. `demo/script.md` по «Полному сценарию защиты» из `research/acceptance-matrix.md`.

Первая живая версия API: `/api/health`, `/api/teams`, `/api/tasks` — к 13:50.

## Формат seed-файлов (загрузчик `internal/store/seed.go`)

Загрузка при старте, только если таблица `tasks` пуста. Каждый файл — **JSON-массив объектов** (top-level `[...]`). ID задаются явно целыми числами и используются как FK. Порядок загрузки: teams → tasks → proposals. `drafts.json` — не таблица, а сырьё для демо (список слабых описаний, которые вводят руками), загрузчик его не читает.

```ts
// seed/teams.json
{ id: number; name: string; skills: string[]; interests: string[]; tech: string[]; points?: number /* default 0 */ }[]

// seed/tasks.json  — карточки разной полноты; score/level/breakdown/missing НЕ указывать, считает бэкенд
{ id: number; industry: string; draft_text: string;
  owner_contact?: string;                      // email/телефон заявителя; без него задачу нельзя править и решать по откликам
  fields: Partial<Record<FieldKey, string>>;   // отсутствующие ключи = ""
  confirmed: boolean;                          // true → status "published", published_at = now - N минут по порядку id
  status?: "editing" | "published";            // необязательно; выводится из confirmed
  questions?: { text: string; field_key: FieldKey; answer: string }[]  // необязательно, id проставит загрузчик
}[]

// seed/proposals.json
{ id: number; task_id: number /* FK tasks.id */; team_id: number /* FK teams.id */;
  idea: string; plan: string; deadline: string /* "2026-10-15" или "3 недели" */; link: string;
  status?: "new" | "selected" | "rejected" /* default "new" */; stage_confirmed?: boolean /* default false */ }[]

// seed/drafts.json — 5 слабых описаний для ручного ввода в демо
{ id: number; industry: string; text: string; note?: string /* какие сведения отсутствуют, для сценария демо */ }[]
```

Минимум по ТЗ §6: 5 записей в каждом файле. `industry` — свободная строка, но одинаковая для одинаковых отраслей (по ней фильтр каталога). Для демо нужен хотя бы один task с `confirmed: true` и `score` в диапазоне 0–39 (то есть 1–2 поля заполнены) и хотя бы один с полной карточкой (90+). Загрузчик валидирует FK и падает с понятной ошибкой при рассинхроне.

## Вход команды по одноразовому коду (решение Абылая, ~13:40)

Регистрации нет (ТЗ §7). Студент вводит email или телефон, получает одноразовый код, вводит его — и он в системе как команда. **Демо-режим:** код никуда не отправляется и всегда равен `000000`; сервер прямо возвращает это в ответе, а фронт показывает подсказку «Демо-режим: введите код 000000». Бизнес входит так же (раздел «Контакт заявителя и вход бизнеса»).

| Метод и путь | Тело | Ответ | Заметки |
|---|---|---|---|
| POST `/api/auth/request-code` | `{ contact: string }` — email или телефон | `{ sent: true, demo: true, hint: "Демо-режим: код 000000" }` | 400 если contact не похож на email/телефон |
| POST `/api/auth/verify` | `{ contact: string, code: string, team_name?: string }` | `{ token: string, team: Team, created: boolean }` | 401 при неверном коде. Если команды с таким contact нет — создаётся: имя = `team_name`, либо, если `team_name` совпадает с именем существующей seed-команды без контакта, контакт привязывается к ней (так можно войти как «Демо-команда Qadam Data»), иначе имя = «Команда <contact>». Токен хранить в localStorage `team_token`, слать `Authorization: Bearer <token>` |
| POST `/api/auth/logout` | — | `{ ok: true }` | |
| GET `/api/me` | — | `{ team: Team, proposals: MyProposal[] }` | 401 без токена. `MyProposal = Proposal & { task: { id, title, industry, score, level, level_label, status } }` — «Мои отклики» |
| POST `/api/tasks/{id}/proposals` | `{ idea, plan, deadline, link }` | `Proposal` | **Изменение:** требует токен, `team_id` берётся из токена. 401 без токена |

`Team` получает поле `contact: string`, но в публичных ответах (`GET /api/teams`, отклики) оно всегда пустое; контакт виден только самой команде в `GET /api/me` (ТЗ §5: персональные признаки не раскрываются). Фронт: переключение в «Я команда» без токена → экран входа в два шага (контакт → код, с подсказкой про 000000, опционально имя команды на первом входе). Страница «Мои отклики». У бизнеса на странице задачи — отклики рядом для сравнения и ручной выбор. Сессии в памяти сервера: перезапуск контейнера разлогинивает (известное упрощение).

## Контакт заявителя и вход бизнеса (решение владельца: заявитель обязателен)

Правила: **без входа задачу не создать, не дополнить и не решить по откликам.** Заявитель входит по email или телефону одноразовым кодом (`000000` в демо); задача, созданная в этой сессии, получает его контакт как `owner_contact`. Вернувшись, заявитель видит свои задачи, кто откликнулся, статусы и свои решения (`GET /api/business/me`). Задачи **без** `owner_contact` (seed без поля `owner_contact`, старые) можно смотреть и на них можно откликаться, но править и решать по откликам их не может никто (403 «у задачи нет заявителя»).

| Метод и путь | Тело | Ответ | Заметки |
|---|---|---|---|
| POST `/api/tasks` | `{ draft_text, industry }` (Bearer business) | `Task` (+ `owner_contact` в маскированном виде `o***@mail.kz`) | контакт — из сессии; без токена 401 |
| POST `/api/tasks/{id}/owner` | `{ owner_contact }` | `Task` | передать задачу другому контакту: только текущий заявитель (401 без токена, 403 чужой, 403 у задачи без заявителя) |
| POST `/api/auth/business/request-code` | `{ contact }` | `{ sent, demo, hint }` | как у команд |
| POST `/api/auth/business/verify` | `{ contact, code }` | `{ token, contact }` | 401 при неверном коде; команда не создаётся |
| GET `/api/business/me` | — (Bearer) | `{ contact, tasks: Task[] }` | задачи с этим `owner_contact` (новые сверху, любой статус), у каждой `proposals` — «кто откликнулся, какие решения»: `team {id,name}`, `status`, `created_at`, `accepted_at`, `messages_count` |
| POST `/api/proposals/{id}/select` / `reject` / `hold` / `confirm-stage` | — | `Proposal` | только заявитель задачи: без токена 401, чужой 403, у задачи без `owner_contact` — 403 |

`Task` получает `owner_contact: string` (в публичных ответах маскирован, полный — только в `/api/business/me`). Один токен-хранилище на фронте для обеих ролей: `team_token` и `business_token`. Флаг `REQUIRE_TEAM_LOGIN=false` касается только отклика команды и не влияет на бизнес-правила.

Seed: в `seed/tasks.json` у задачи можно указать необязательное `owner_contact` (email/телефон; нормализуется как при входе — lower/trim, телефон → `+7…`; невалидный → ошибка загрузки). Без поля задача остаётся без заявителя.

## Чат по отклику и двустороннее принятие (решение Абылая, ~14:30)

Статусы отклика: `new` → `selected` (бизнес выбрал) → `accepted` (команда подтвердила; проект принят обеими сторонами) | `declined` (команда отказалась после выбора). Также `rejected` (бизнес отклонил) и `on_hold` (бизнес отложил). `confirm-stage` доступен для `selected` и `accepted` (в интерфейсе пока нет кнопки «Принять», поэтому этап подтверждается и по выбранному отклику; `accepted` = принятие с двух сторон). Решения бизнеса (`select`/`reject`/`hold`) нельзя менять у отклика в статусе `accepted` или `declined` → 400.

| Метод и путь | Тело | Ответ | Кто |
|---|---|---|---|
| POST `/api/proposals/{id}/accept` | — | `Proposal` (status `accepted`) | команда отклика (Bearer team), только из `selected` |
| POST `/api/proposals/{id}/decline` | — | `Proposal` (status `declined`) | команда отклика, из `selected` |
| POST `/api/proposals/{id}/hold` | — | `Proposal` (status `on_hold`) | заявитель (правила как у select) |
| GET `/api/proposals/{id}/messages` | — | `{ messages: [{ id, author: "team"\|"business", text, created_at }] }` | команда отклика или заявитель задачи; без токена 401 |
| POST `/api/proposals/{id}/messages` | `{ text }` | `Message` | автор определяется токеном: команда отклика → "team"; заявитель задачи → "business". Без токена 401; чужая команда / чужой бизнес / бизнес у задачи без заявителя → 403 |

Чат без realtime: фронт обновляет список при открытии и по кнопке/таймеру 5 с. В `Proposal` добавляется `messages_count: number` и `accepted_at: string|null`. В `/api/me` и `/api/business/me` отклики приходят с этими полями.

## WebSocket чата

`GET /api/proposals/{id}/ws?token=<team_token | business_token>` — апгрейд до WebSocket. Права те же, что у `GET /api/proposals/{id}/messages`: команда отклика → автор `team`; заявитель задачи (бизнес-токен с её `owner_contact`) → `business`. Отказ — обычный JSON-ответ **до** апгрейда: 401 (нет токена), 403 (чужая команда / чужой бизнес / бизнес у задачи без заявителя), 404 (нет отклика). Токен можно передать и заголовком `Authorization: Bearer`, но браузерный WebSocket заголовки не шлёт — поэтому `?token=`.

Origin: разрешён хост самого сервера и шаблоны из `WS_ORIGINS` (через запятую, `path.Match` по `host:port`), по умолчанию `localhost:*,127.0.0.1:*`; чужой Origin → 403.

Кадры — текстовые JSON:

| Направление | Кадр | Когда |
|---|---|---|
| сервер → клиент | `{"type":"history","messages":[Message…]}` | сразу после подключения, старые сверху |
| клиент → сервер | `{"type":"message","text":"…"}` | отправить сообщение: trim, непустое, ≤ 2000 символов; автор — по токену соединения |
| сервер → все подписчики отклика | `{"type":"message","message":{id, author, text, created_at}}` | новое сообщение из WS **или** из `POST /api/proposals/{id}/messages` (отправитель тоже получает) |
| сервер → клиент | `{"type":"ping"}` | каждые 30 с; клиент может ответить `{"type":"pong"}` |
| клиент → сервер | `{"type":"ping"}` | сервер отвечает `{"type":"pong"}` |
| сервер → клиент | `{"type":"error","error":"…"}` | невалидный JSON, пустой/длинный текст, неизвестный `type`; соединение не рвётся |

Сообщение, пришедшее в момент подключения, может оказаться и в `history`, и отдельным `message` — клиент дедуплицирует по `id`. Клиент, не успевающий читать (очередь 32 кадра), отключается; переподключение снова присылает `history`. REST `GET/POST …/messages` остаётся для первоначальной загрузки и как запасной путь.

## Разговор без входа (анонимный черновик)

`POST /api/tasks` **без токена** создаёт анонимный черновик: `owner_contact` пустой, статус `clarifying`. Для него без токена работают `next-question` и `result-options` (разговор сразу). `answers`, `apply-result`, `PUT /fields`, `confirm` без токена → 401 «чтобы собрать карточку, войдите — беседа сохранится»; с чужим бизнес-токеном → 403 «сначала присвойте задачу». `POST /api/tasks/{id}/owner` с бизнес-токеном и телом `{}` присваивает анонимный черновик вошедшему заявителю (200, `Task`); дальше правила прежние (только владелец). Опубликованные задачи без заявителя (seed без `owner_contact`) по-прежнему 403 всем. Фронт: чат открывается без входа, модалка входа появляется при «Собрать карточку»/выборе варианта, затем `owner` и повтор действия.
