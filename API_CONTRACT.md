# Контракт API: Go backend ⇄ Vite frontend (v3, 13:30)

Владелец контракта: Абылай (backend). Изменения — только через этот файл + уведомление в inbox/outbox.
Фронт: **Vite + React + TypeScript** в `web/`, владелец Ильяс. Бэкенд отдаёт JSON под `/api/*` и раздаёт собранный `web/dist` на `/` с SPA-fallback (любой не-`/api` путь → `index.html`).

Dev: бэкенд `go run ./cmd/server` на `:8080`; фронт `npm run dev` на `:5173` с proxy `/api → http://localhost:8080` (в `vite.config.ts`). Prod: `docker compose up` собирает фронт и Go в одном образе.

## Общие правила

- JSON, UTF-8. Даты ISO-8601 строкой. Ошибка: `{ "error": "текст" }` с 400 (валидация) / 404 / 502 (AI недоступен и mock не сработал — не должно случаться).
- Режим «Я бизнес / Я команда» — на фронте (localStorage `mode`); вход обеих ролей по одноразовому коду (разделы ниже); задачи без контакта заявителя открыты для решений (ТЗ §7: сложная ролевая модель не нужна).
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
  questions: { id: number; text: string; field_key: FieldKey; answer: string }[];            // ≥3 от AI
  ai_mode: "openai" | "mock";
  created_at: string; published_at: string | null;
  proposals?: Proposal[];                  // только в GET /api/tasks/{id}
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

## Эндпоинты

| Метод и путь | Тело запроса | Ответ | Заметки |
|---|---|---|---|
| GET `/api/tasks?industry=&level=` | — | `{ tasks: Task[], industries: string[], levels: {key,label}[] }` | только published; сортировка score DESC, published_at ASC; фильтры опциональны |
| POST `/api/tasks` | `{ draft_text: string, industry: string }` | `Task` (status `clarifying`, `questions` заполнены ≥3) | draft_text пустой → 400. AI вызывается здесь |
| GET `/api/tasks/{id}` | — | `Task` с `proposals` | любой статус |
| POST `/api/tasks/{id}/answers` | `{ answers: { [question_id: string]: string } }` | `Task` (status `editing`, `fields` собраны AI из черновика+ответов, score посчитан, confirmed=false) | AI вызывается здесь; поля без данных остаются "" |
| PUT `/api/tasks/{id}/fields` | `{ fields: Partial<Record<FieldKey,string>> }` | `Task` (score/breakdown/missing пересчитаны, confirmed=false) | ручное редактирование, можно вызывать много раз |
| POST `/api/tasks/{id}/confirm` | — | `Task` (confirmed=true, status `published`, published_at) | ручное подтверждение = публикация. Баллы начисляются только подтверждённым полям, поэтому score до confirm — «предварительный» (фронт так и подписывает) |
| POST `/api/tasks/{id}/proposals` | `{ team_id, idea, plan, deadline, link }` | `Proposal` | все поля обязательны → иначе 400; лимита нет |
| POST `/api/proposals/{id}/select` | — | `Proposal` | бизнес выбрал |
| POST `/api/proposals/{id}/reject` | — | `Proposal` | бизнес отклонил |
| POST `/api/proposals/{id}/confirm-stage` | — | `Proposal` (+ команде начислены баллы) | **вне критического пути**, после 16:00 |
| GET `/api/teams` | — | `Team[]` | для select в форме отклика |
| GET `/api/teams/{id}/recommended` | — | `Task[]` | **вне критического пути** |
| GET `/api/ai` | — | `{ mode, prompt_questions, prompt_card, schema_example, last_error: string\|null }` | страница «как работает AI» для ТЗ §5 |
| GET `/api/health` | — | `{ ok: true, db: true, ai_mode }` | для README и проверки экспертом |

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

## Вход команды по одноразовому коду (решение Абылая 15:40)

Регистрации нет (ТЗ §7). Студент вводит email или телефон, получает одноразовый код, вводит его — и он в системе как команда. **Демо-режим:** код никуда не отправляется и всегда равен `000000`; сервер прямо возвращает это в ответе, а фронт показывает подсказку «Демо-режим: введите код 000000». Бизнес по-прежнему без входа.

| Метод и путь | Тело | Ответ | Заметки |
|---|---|---|---|
| POST `/api/auth/request-code` | `{ contact: string }` — email или телефон | `{ sent: true, demo: true, hint: "Демо-режим: код 000000" }` | 400 если contact не похож на email/телефон |
| POST `/api/auth/verify` | `{ contact: string, code: string, team_name?: string }` | `{ token: string, team: Team, created: boolean }` | 401 при неверном коде. Если команды с таким contact нет — создаётся: имя = `team_name`, либо, если `team_name` совпадает с именем существующей seed-команды без контакта, контакт привязывается к ней (так можно войти как «Демо-команда Qadam Data»), иначе имя = «Команда <contact>». Токен хранить в localStorage `team_token`, слать `Authorization: Bearer <token>` |
| POST `/api/auth/logout` | — | `{ ok: true }` | |
| GET `/api/me` | — | `{ team: Team, proposals: MyProposal[] }` | 401 без токена. `MyProposal = Proposal & { task: { id, title, industry, score, level, level_label, status } }` — «Мои отклики» |
| POST `/api/tasks/{id}/proposals` | `{ idea, plan, deadline, link }` | `Proposal` | **Изменение:** требует токен, `team_id` берётся из токена. 401 без токена |

`Team` получает поле `contact: string`, но в публичных ответах (`GET /api/teams`, отклики) оно всегда пустое; контакт виден только самой команде в `GET /api/me` (ТЗ §5: персональные признаки не раскрываются). Фронт: переключение в «Я команда» без токена → экран входа в два шага (контакт → код, с подсказкой про 000000, опционально имя команды на первом входе). Страница «Мои отклики». У бизнеса на странице задачи — отклики рядом для сравнения и ручной выбор. Сессии в памяти сервера: перезапуск контейнера разлогинивает (известное упрощение).

## Контакт заявителя и вход бизнеса (решение Абылая 16:20)

Правила: **дополнять карточку может любой** (PUT fields без входа, как раньше). Контакт, оставленный при заявке, даёт бизнесу вход тем же одноразовым кодом (`000000` в демо), чтобы вернуться к своим задачам, обновить их, увидеть отклики и принять команду. Задачи **без** контакта (seed) остаются полностью открытыми, включая select/reject.

| Метод и путь | Тело | Ответ | Заметки |
|---|---|---|---|
| POST `/api/tasks` | `{ draft_text, industry, owner_contact?: string }` | `Task` (+ `owner_contact` в маскированном виде `o***@mail.kz`) | email/телефон заявителя; необязателен. Нормализация как у команд |
| POST `/api/tasks/{id}/owner` | `{ owner_contact }` | `Task` | задать контакт позже (например, на шаге подтверждения); только если у задачи контакта ещё нет или запрос с токеном бизнеса этой задачи |
| POST `/api/auth/business/request-code` | `{ contact }` | `{ sent, demo, hint }` | как у команд |
| POST `/api/auth/business/verify` | `{ contact, code }` | `{ token, contact }` | 401 при неверном коде; команда не создаётся |
| GET `/api/business/me` | — (Bearer) | `{ contact, tasks: Task[] }` | задачи с этим `owner_contact`, каждая с `proposals` |
| POST `/api/proposals/{id}/select` / `reject` / `confirm-stage` | — | `Proposal` | **Изменение:** если у задачи есть `owner_contact`, нужен токен бизнеса с тем же контактом, иначе 403; у задач без контакта — как раньше, открыто |

`Task` получает `owner_contact: string` (в публичных ответах маскирован, полный — только в `/api/business/me`). Один токен-хранилище на фронте для обеих ролей: `team_token` и `business_token`. Флаг `REQUIRE_TEAM_LOGIN=false` не влияет на бизнес-правила.

## Чат по отклику и двустороннее принятие (решение Абылая 16:30)

Статусы отклика: `new` → `selected` (бизнес выбрал) → `accepted` (команда подтвердила; проект принят обеими сторонами) | `declined` (команда отказалась после выбора). Также `rejected` (бизнес отклонил) и `on_hold` (бизнес отложил; «холд» — до уточнения третьей роли). `confirm-stage` доступен только для `accepted`.

| Метод и путь | Тело | Ответ | Кто |
|---|---|---|---|
| POST `/api/proposals/{id}/accept` | — | `Proposal` (status `accepted`) | команда отклика (Bearer team), только из `selected` |
| POST `/api/proposals/{id}/decline` | — | `Proposal` (status `declined`) | команда отклика, из `selected` |
| POST `/api/proposals/{id}/hold` | — | `Proposal` (status `on_hold`) | бизнес (правила как у select) |
| GET `/api/proposals/{id}/messages` | — | `{ messages: [{ id, author: "team"\|"business", text, created_at }] }` | команда отклика или бизнес задачи (у задач без контакта — любой в режиме бизнеса) |
| POST `/api/proposals/{id}/messages` | `{ text }` | `Message` | автор определяется токеном: team → "team"; business → "business"; без токена на задаче без контакта → "business" (демо) |

Чат без realtime: фронт обновляет список при открытии и по кнопке/таймеру 5 с. В `Proposal` добавляется `messages_count: number` и `accepted_at: string|null`. В `/api/me` и `/api/business/me` отклики приходят с этими полями.

## WebSocket чата

`GET /api/proposals/{id}/ws?token=<team_token | business_token>` — апгрейд до WebSocket. Права те же, что у `GET /api/proposals/{id}/messages`: команда отклика → автор `team`; заявитель задачи (бизнес-токен с её `owner_contact`) → `business`; у задачи без контакта — любой бизнес-токен или запрос без токена → `business` (демо). Отказ — обычный JSON-ответ **до** апгрейда: 401 (нет токена у задачи с контактом), 403 (чужая команда / чужой бизнес), 404 (нет отклика). Токен можно передать и заголовком `Authorization: Bearer`, но браузерный WebSocket заголовки не шлёт — поэтому `?token=`.

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
