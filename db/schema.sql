-- Схема Postgres (применяется при каждом старте, идемпотентно).
-- Rating хранится целиком в tasks.rating (jsonb), score дублируется колонкой для сортировки.

CREATE TABLE IF NOT EXISTS teams (
    id        serial PRIMARY KEY,
    name      text   NOT NULL,
    skills    text[] NOT NULL DEFAULT '{}',
    interests text[] NOT NULL DEFAULT '{}',
    tech      text[] NOT NULL DEFAULT '{}',
    experience text   NOT NULL DEFAULT '',
    achievements text NOT NULL DEFAULT '',
    points    int    NOT NULL DEFAULT 0
);

ALTER TABLE teams ADD COLUMN IF NOT EXISTS experience text NOT NULL DEFAULT '';
ALTER TABLE teams ADD COLUMN IF NOT EXISTS achievements text NOT NULL DEFAULT '';

-- Вход команды по одноразовому коду: contact хранится нормализованным (lower, телефон с «+»).
-- ADD COLUMN IF NOT EXISTS — идемпотентно на существующей БД с данными; у seed-команд contact = NULL.
ALTER TABLE teams ADD COLUMN IF NOT EXISTS contact text;
CREATE UNIQUE INDEX IF NOT EXISTS teams_contact_idx ON teams (lower(contact)) WHERE contact <> '';

CREATE TABLE IF NOT EXISTS tasks (
    id           serial PRIMARY KEY,
    industry     text        NOT NULL DEFAULT '',
    status       text        NOT NULL DEFAULT 'draft',
    draft_text   text        NOT NULL DEFAULT '',
    fields       jsonb       NOT NULL DEFAULT '{}',
    confirmed    bool        NOT NULL DEFAULT false,
    score        int         NOT NULL DEFAULT 0,
    rating       jsonb,
    questions    jsonb       NOT NULL DEFAULT '[]',
    ai_mode      text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz NULL
);

CREATE INDEX IF NOT EXISTS tasks_catalog_idx ON tasks (status, score DESC, published_at);

-- Контакт заявителя (вход бизнеса тем же одноразовым кодом): нормализован как у команд; '' у seed-задач.
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS owner_contact text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS tasks_owner_idx ON tasks (lower(owner_contact)) WHERE owner_contact <> '';

CREATE TABLE IF NOT EXISTS proposals (
    id              serial PRIMARY KEY,
    task_id         int         NOT NULL REFERENCES tasks (id),
    team_id         int         NOT NULL REFERENCES teams (id),
    idea            text        NOT NULL,
    plan            text        NOT NULL,
    deadline        text        NOT NULL,
    link            text        NOT NULL,
    status          text        NOT NULL DEFAULT 'new',
    stage_confirmed bool        NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    quick           bool        NOT NULL DEFAULT false,
    profile_snapshot jsonb      NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS proposals_task_idx ON proposals (task_id);

-- Двустороннее принятие: момент, когда команда подтвердила выбранный отклик (status = 'accepted').
ALTER TABLE proposals ADD COLUMN IF NOT EXISTS accepted_at timestamptz NULL;
ALTER TABLE proposals ADD COLUMN IF NOT EXISTS quick bool NOT NULL DEFAULT false;
ALTER TABLE proposals ADD COLUMN IF NOT EXISTS profile_snapshot jsonb NOT NULL DEFAULT '{}';
CREATE UNIQUE INDEX IF NOT EXISTS proposals_quick_task_team_idx ON proposals (task_id, team_id) WHERE quick;

-- Чат по отклику: команда отклика ↔ заявитель задачи.
CREATE TABLE IF NOT EXISTS messages (
    id          serial PRIMARY KEY,
    proposal_id int         NOT NULL REFERENCES proposals (id),
    author      text        NOT NULL CHECK (author IN ('team', 'business')),
    text        text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS messages_proposal_idx ON messages (proposal_id, created_at, id);

-- После загрузки seed с явными id последовательности выставляются на max(id)
-- (см. internal/store/seed.go, функция resetSequences) — иначе следующий INSERT упадёт на дубликате PK.

-- Визуальный концепт первого результата (OpenAI GPT Image или mock): одна картинка на задачу, повтор перезаписывает.
CREATE TABLE IF NOT EXISTS task_visuals (
    task_id    int         PRIMARY KEY REFERENCES tasks (id),
    mime       text        NOT NULL,
    data       bytea       NOT NULL,
    prompt     text        NOT NULL DEFAULT '',
    ai_mode    text        NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
