# Контракт main.py → шаблоны (v1, 13:20)

Владелец контракта: Абылай (backend). Изменения контракта — только через этот файл и уведомление в inbox/outbox.
Шаблоны Jinja2 в `app/templates/`, статика в `app/static/` (URL `/static/...`). Владелец шаблонов и CSS: Ильяс.

## Общее для всех шаблонов

- `base.html` — layout: шапка с переключателем режима, блок `{% block content %}`.
- В каждом контексте: `mode` — `"business"` | `"team"` (берётся из cookie `mode`, по умолчанию `business`); `teams` — список профилей `[ {id, name, skills:[str], interests:[str], tech:[str], points:int} ]`; `flash` — строка или `None`.
- Переключение режима: `POST /mode` с полем `mode`. Ставит cookie, редирект назад.
- Все формы — обычные HTML `<form method="post">`, без JS-обязательности. Ответ на POST — редирект (PRG).

## Объекты

**Task** (карточка задачи):
```
id:int, title:str, industry:str, status: "draft"|"clarifying"|"editing"|"published",
draft_text:str,                      # исходное слабое описание
fields: {                            # 10 полей ТЗ §3, каждое str или ""
  title, context, need, users, data, constraints, expected_result, success_criteria, contact, interaction_format
},
confirmed: bool,                     # ручное подтверждение карточки
score:int (0–100), level: "draft"|"working"|"ready"|"priority",
level_label:str ("черновик"|"рабочая"|"готовая"|"приоритетная"),
breakdown: [ {key, label, weight:int, earned:int, reason:str} ],   # 7 показателей ТЗ §4
missing: [ {key, label, gain:int, hint:str} ],                     # что добавить, чтобы +gain
questions: [ {id:int, text:str, field_key:str, answer:str} ],     # ≥3 от AI
ai_mode: "openai"|"mock",
created_at:str, published_at:str|None,
proposals: [Proposal]                # только на странице задачи
```
**Proposal** (отклик команды):
```
id:int, task_id:int, team:{id,name}, idea:str, plan:str, deadline:str, link:str,
status: "new"|"selected"|"rejected", stage_confirmed:bool, created_at:str
```

## Маршруты и контекст

| Метод и путь | Шаблон | Контекст (сверх общего) | Форма (имена полей) |
|---|---|---|---|
| GET `/` | `catalog.html` | `tasks:[Task]` (только published, уже отсортированы score DESC, published_at ASC), `industries:[str]`, `levels:[{key,label}]`, `filters:{industry:str|"", level:str|""}` | GET-параметры `industry`, `level` |
| GET `/task/new` | `task_new.html` | `industries:[str]` | POST `/task/new`: `draft_text` (textarea, обязательное), `industry` (select) → редирект на `/task/{id}/clarify` |
| GET `/task/{id}/clarify` | `task_clarify.html` | `task:Task` (с `questions`, `ai_mode`) | POST `/task/{id}/clarify`: `answer_{question.id}` для каждого вопроса → редирект на `/task/{id}/edit` |
| GET `/task/{id}/edit` | `task_edit.html` | `task:Task` (fields, score, breakdown, missing, confirmed) | POST `/task/{id}/edit`: 10 полей по именам `fields` + кнопка `action` = `save` (пересчёт, остаёмся) или `confirm` (подтвердить и опубликовать → редирект на `/task/{id}`) |
| GET `/task/{id}` | `task_show.html` | `task:Task` (с `proposals`), `can_respond:bool` (mode==team), `can_decide:bool` (mode==business) | POST `/task/{id}/propose`: `team_id`, `idea`, `plan`, `deadline`, `link` (все обязательные) → редирект на `/task/{id}` |
| POST `/proposal/{id}/select` | — | — | без полей → status=selected, редирект назад |
| POST `/proposal/{id}/reject` | — | — | без полей → status=rejected, редирект назад |
| POST `/proposal/{id}/confirm-stage` | — | — | без полей → stage_confirmed=true, команде +баллы, редирект назад. **Вне критического пути демо**, кнопка может быть скрыта до 16:00 |
| GET `/teams` | `teams.html` | `teams` (общий), `recommended:{team_id:[Task]}` | **Вне критического пути**, делаем последним |
| GET `/ai` | `ai_info.html` | `ai_mode`, `prompt_questions:str`, `prompt_card:str`, `schema_example:str` | Страница «как работает AI» для показа промпта и формата (ТЗ §5) |

## Ошибки

- Пустой `draft_text`, пустые обязательные поля отклика → тот же шаблон с `flash` и заполненными значениями, HTTP 400.
- Ошибка AI → `task.ai_mode="mock"`, вопросы из заглушки, `flash="AI недоступен, использован локальный режим"`.
- 404 → `error.html` с `message`.

## Что Ильяс может делать прямо сейчас

1. `base.html`, `catalog.html`, `task_new.html`, `task_clarify.html`, `task_edit.html`, `task_show.html`, `error.html`, `app/static/app.css` по контексту выше. Для проверки вёрстки без бэкенда — любые фиктивные данные.
2. `seed/drafts.json`, `seed/tasks.json`, `seed/teams.json`, `seed/proposals.json` — по объектам выше (для Task в seed достаточно `title, industry, draft_text, fields, confirmed, status`; score считает бэкенд).
3. `demo/script.md` по «Полному сценарию защиты» из `research/acceptance-matrix.md`.
