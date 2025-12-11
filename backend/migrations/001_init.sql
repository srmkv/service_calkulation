-- пользователи
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL,
    name          TEXT,
    role          TEXT NOT NULL,         -- 'admin' / 'user'
    plan_id       TEXT,
    plan_active   BOOLEAN NOT NULL DEFAULT TRUE,
    password_hash TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- тарифы
CREATE TABLE IF NOT EXISTS plans (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    price           BIGINT NOT NULL DEFAULT 0,  -- в копейках или целые рубли – как удобнее
    max_calculators INT    NOT NULL DEFAULT 0,
    max_leads       INT    NOT NULL DEFAULT 0,
    max_calcs       INT    NOT NULL DEFAULT 0
);

-- калькуляторы
CREATE TABLE IF NOT EXISTS calculators (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL, -- 'layered' / 'distance' / 'on_site'
    owner_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL, -- 'draft' / 'published'
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    public_token TEXT,
    public_path  TEXT,
    calc_count   INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_calculators_owner ON calculators(owner_id);

-- заявки (на будущее)
CREATE TABLE IF NOT EXISTS leads (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    calc_id    TEXT REFERENCES calculators(id) ON DELETE SET NULL,
    payload    JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_leads_owner ON leads(owner_id);

-- события/действия (лог всего, что происходит)
CREATE TABLE IF NOT EXISTS events (
    id         BIGSERIAL PRIMARY KEY,
    user_id    TEXT,
    calc_id    TEXT,
    event_type TEXT NOT NULL,  -- 'calc_run', 'lead_created', 'plan_changed', ...
    data       JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_user ON events(user_id);
CREATE INDEX IF NOT EXISTS idx_events_calc ON events(calc_id);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);

-- простые key/value настройки – можно сюда положить distance/layered конфиги
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value JSONB NOT NULL
);
