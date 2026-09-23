-- Схема Postgres (применяется при каждом старте, идемпотентно).
-- Rating хранится целиком в tasks.rating (jsonb), score дублируется колонкой для сортировки.

CREATE TABLE IF NOT EXISTS teams (
    id        serial PRIMARY KEY,
    name      text   NOT NULL,
    skills    text[] NOT NULL DEFAULT '{}',
    interests text[] NOT NULL DEFAULT '{}',
    tech      text[] NOT NULL DEFAULT '{}',
    points    int    NOT NULL DEFAULT 0
);

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
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS proposals_task_idx ON proposals (task_id);

-- После загрузки seed с явными id последовательности выставляются на max(id)
-- (см. internal/store/seed.go, функция resetSequences) — иначе следующий INSERT упадёт на дубликате PK.
