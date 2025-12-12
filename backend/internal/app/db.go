package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
	"saas-calc-backend/internal/domain"
)

// Settings — конфигурация сервисов маршрутизации и Telegram-бота,
// которая хранится в таблице settings (одна строка с id = 1).
type Settings struct {
	OSRMBaseURL      string
	NominatimBaseURL string
	TelegramBotToken string
}

// OpenDBFromEnv открывает PostgreSQL по DSN из переменной окружения SAAS_PG_DSN.
// Пример DSN: postgres://user:pass@localhost:5432/saas?sslmode=disable
func OpenDBFromEnv() (*sql.DB, error) {
	dsn := os.Getenv("SAAS_PG_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("SAAS_PG_DSN is empty")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return db, nil
}

// ensureSchema создаёт нужные таблицы, если их ещё нет.
func ensureSchema(db *sql.DB) error {
	if db == nil {
		return nil
	}

	// --- settings ---
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS settings (
    id                  INTEGER PRIMARY KEY,
    osrm_base_url       TEXT,
    nominatim_base_url  TEXT,
    telegram_bot_token  TEXT
);
`); err != nil {
		return err
	}

	// гарантируем, что запись с id = 1 существует
	if _, err := db.Exec(`
INSERT INTO settings (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;
`); err != nil {
		return err
	}

	// --- users ---
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS users (
    id               TEXT PRIMARY KEY,
    email            TEXT NOT NULL,
    name             TEXT NOT NULL,
    role             TEXT NOT NULL,          -- admin / user
    password_hash    TEXT NOT NULL DEFAULT '',
    plan_id          TEXT NOT NULL,
    plan_active      BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    telegram_chat_id TEXT
);
`); err != nil {
		return err
	}

	// на случай уже существующей таблицы без telegram_chat_id
	if _, err := db.Exec(`
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS telegram_chat_id TEXT;
`); err != nil {
		return err
	}

	// --- plans ---
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS plans (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL,
    price           NUMERIC(12,2) NOT NULL,
    max_calculators INTEGER NOT NULL,
    max_leads       INTEGER NOT NULL,
    max_calcs       INTEGER NOT NULL
);
`); err != nil {
		return err
	}

	// --- calculators ---
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS calculators (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL,
    owner_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    public_token TEXT NOT NULL,
    public_path  TEXT NOT NULL,
    calc_count   INTEGER NOT NULL DEFAULT 0
);
`); err != nil {
		return err
	}

	return nil
}

// seedPlans заполняет таблицу plans тарифами, если она пустая.
func seedPlans(ctx context.Context, db *sql.DB, plans []domain.Plan) error {
	if db == nil {
		return nil
	}

	var cnt int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM plans`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		// уже есть тарифы – ничего не делаем
		return nil
	}

	// если вдруг plans не передали – используем дефолтные
	if len(plans) == 0 {
		plans = domain.DefaultPlans()
	}

	for _, p := range plans {
		_, err := db.ExecContext(ctx, `
INSERT INTO plans (id, name, description, price, max_calculators, max_leads, max_calcs)
VALUES ($1,$2,$3,$4,$5,$6,$7)
ON CONFLICT (id) DO NOTHING;
`,
			p.ID,
			p.Name,
			p.Description,
			p.Price,
			p.MaxCalculators,
			p.MaxLeads,
			p.MaxCalcs,
		)
		if err != nil {
			return err
		}
	}

	log.Println("seedPlans: inserted default plans")
	return nil
}

// loadPlans загружает все тарифы из БД в []domain.Plan.
func loadPlans(ctx context.Context, db *sql.DB) ([]domain.Plan, error) {
	if db == nil {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, `
SELECT id, name, description, price, max_calculators, max_leads, max_calcs
FROM plans
ORDER BY price ASC, id ASC;
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plans []domain.Plan
	for rows.Next() {
		var p domain.Plan
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.MaxCalculators,
			&p.MaxLeads,
			&p.MaxCalcs,
		); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return plans, nil
}

// LoadSettings загружает настройки из таблицы settings (id = 1).
func LoadSettings(ctx context.Context, db *sql.DB) (*Settings, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}

	row := db.QueryRowContext(ctx, `
SELECT osrm_base_url, nominatim_base_url, telegram_bot_token
FROM settings
WHERE id = 1;
`)

	var s Settings
	if err := row.Scan(&s.OSRMBaseURL, &s.NominatimBaseURL, &s.TelegramBotToken); err != nil {
		if err == sql.ErrNoRows {
			// настроек ещё нет — вернём пустую структуру
			return &Settings{}, nil
		}
		return nil, err
	}

	return &s, nil
}

// SaveSettings сохраняет настройки в таблицу settings (id всегда = 1).
func SaveSettings(ctx context.Context, db *sql.DB, s *Settings) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if s == nil {
		return fmt.Errorf("settings is nil")
	}

	_, err := db.ExecContext(ctx, `
INSERT INTO settings (id, osrm_base_url, nominatim_base_url, telegram_bot_token)
VALUES (1, $1, $2, $3)
ON CONFLICT (id) DO UPDATE
  SET osrm_base_url      = EXCLUDED.osrm_base_url,
      nominatim_base_url = EXCLUDED.nominatim_base_url,
      telegram_bot_token = EXCLUDED.telegram_bot_token;
`,
		s.OSRMBaseURL,
		s.NominatimBaseURL,
		s.TelegramBotToken,
	)
	return err
}
