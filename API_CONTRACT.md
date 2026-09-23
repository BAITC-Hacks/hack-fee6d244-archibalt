# Контракт API: Go backend ⇄ Vite frontend (v3, 13:30)

Владелец контракта: Абылай (backend). Изменения — только через этот файл + уведомление в inbox/outbox.
Фронт: **Vite + React + TypeScript** в `web/`, владелец Ильяс. Бэкенд отдаёт JSON под `/api/*` и раздаёт собранный `web/dist` на `/` с SPA-fallback (любой не-`/api` путь → `index.html`).

Dev: бэкенд `go run ./cmd/server` на `:8080`; фронт `npm run dev` на `:5173` с proxy `/api → http://localhost:8080` (в `vite.config.ts`). Prod: `docker compose up` собирает фронт и Go в одном образе.

## Общие правила

- JSON, UTF-8. Даты ISO-8601 строкой. Ошибка: `{ "error": "текст" }` с 400 (валидация) / 404 / 502 (AI недоступен и mock не сработал — не должно случаться).
- Режим «Я бизнес / Я команда» — только на фронте (localStorage `mode`), бэкенд роли не проверяет (ТЗ §7: сложная ролевая модель не нужна).
- Все запросы синхронные, без polling. AI-вызовы могут занимать до ~10 с — фронт показывает спиннер.

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
  status: "new" | "selected" | "rejected"; stage_confirmed: boolean; created_at: string;
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

## Вход команды (добавлено 15:35 по решению Абылая)

Регистрации нет (ТЗ §7): команды заранее заведены (seed), вход по паролю. Все seed-команды имеют тестовый пароль `demo` (переменная `TEAM_PASSWORD`, по умолчанию `demo`, описана в README как тестовый доступ по Положению п. 5.6.6). Бизнес по-прежнему без входа.

| Метод и путь | Тело | Ответ | Заметки |
|---|---|---|---|
| POST `/api/auth/login` | `{ team_id: number, password: string }` | `{ token: string, team: Team }` | 401 при неверном пароле. Токен хранить в localStorage `team_token`, слать `Authorization: Bearer <token>` |
| POST `/api/auth/logout` | — (заголовок) | `{ ok: true }` | |
| GET `/api/me` | — (заголовок) | `{ team: Team, proposals: MyProposal[] }` | 401 без токена. `MyProposal = Proposal & { task: { id, title, industry, score, level, level_label } }` — «Мои отклики» со статусом new/selected/rejected и баллом задачи |
| POST `/api/tasks/{id}/proposals` | `{ idea, plan, deadline, link }` | `Proposal` | **Изменение:** требует токен; `team_id` берётся из токена, поле в теле игнорируется. 401 без токена |

Фронт: переключение в режим «Я команда» → если нет токена, экран входа (select команды из GET /api/teams + пароль). Страница «Мои отклики» (GET /api/me). Для бизнеса на странице задачи — сравнение откликов рядом (идея / план / срок / ссылка / команда с навыками) и ручной выбор, как раньше. Сессии в памяти сервера: перезапуск контейнера разлогинивает (известное упрощение).
