package handlers

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"

	"saas-calc-backend/internal/domain"
)

// GetUserByID достаёт пользователя по id из БД.
func (e *Env) GetUserByID(ctx context.Context, id string) (*domain.User, error) {
	if e.DB == nil {
		return nil, errors.New("db is nil")
	}

	row := e.DB.QueryRowContext(ctx, `
SELECT id, email, name, role, plan_id, plan_active, created_at
FROM users
WHERE id = $1
`, id)

	var u domain.User
	var role string
	var planID sql.NullString
	var planActive sql.NullBool
	var createdAt time.Time

	if err := row.Scan(
		&u.ID,
		&u.Email,
		&u.Name,
		&role,
		&planID,
		&planActive,
		&createdAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	u.Role = domain.Role(role)
	if planID.Valid {
		u.PlanID = planID.String
	}
	u.PlanActive = true
	if planActive.Valid {
		u.PlanActive = planActive.Bool
	}
	u.CreatedAt = createdAt

	return &u, nil
}

// ListUsers возвращает всех пользователей (для /admin/users).
func (e *Env) ListUsers(ctx context.Context) ([]*domain.User, error) {
	if e.DB == nil {
		return nil, errors.New("db is nil")
	}

	rows, err := e.DB.QueryContext(ctx, `
SELECT id, email, name, role, plan_id, plan_active, created_at
FROM users
ORDER BY created_at ASC
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*domain.User

	for rows.Next() {
		var u domain.User
		var role string
		var planID sql.NullString
		var planActive sql.NullBool
		var createdAt time.Time

		if err := rows.Scan(
			&u.ID,
			&u.Email,
			&u.Name,
			&role,
			&planID,
			&planActive,
			&createdAt,
		); err != nil {
			return nil, err
		}

		u.Role = domain.Role(role)
		if planID.Valid {
			u.PlanID = planID.String
		}
		u.PlanActive = true
		if planActive.Valid {
			u.PlanActive = planActive.Bool
		}
		u.CreatedAt = createdAt

		res = append(res, &u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}
// nullableString превращает пустую строку в NULL для БД.
// Удобно использовать в Exec/Query: nullableString(u.Name), nullableString(u.Email) и т.п.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// UpdateUser сохраняет изменения пользователя.
func (e *Env) UpdateUser(ctx context.Context, u *domain.User) error {
	if e.DB == nil {
		return errors.New("db is nil")
	}

	_, err := e.DB.ExecContext(ctx, `
UPDATE users
SET email = $1,
    name = $2,
    role = $3,
    plan_id = $4,
    plan_active = $5
WHERE id = $6
`,
		u.Email,
		u.Name,
		string(u.Role),
		nullableString(u.PlanID),
		u.PlanActive,
		u.ID,
	)
	return err
}

// DeleteUser удаляет пользователя.
func (e *Env) DeleteUser(ctx context.Context, id string) error {
	if e.DB == nil {
		return errors.New("db is nil")
	}
	_, err := e.DB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// SetUserPassword устанавливает новый пароль (bcrypt-хеш).
func (e *Env) SetUserPassword(ctx context.Context, id, password string) error {
	if e.DB == nil {
		return errors.New("db is nil")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = e.DB.ExecContext(ctx, `
UPDATE users SET password_hash = $1 WHERE id = $2
`,
		string(hash),
		id,
	)
	return err
}
