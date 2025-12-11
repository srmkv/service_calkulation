package domain

import "time"

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User описывает клиента/админа SaaS
type User struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	Role       Role      `json:"role"`
	PlanID     string    `json:"planId"`
	PlanActive bool      `json:"planActive"` // активен ли тариф (подписка)
	CreatedAt  time.Time `json:"createdAt"`
    TelegramChatID string `json:"telegramChatId"`
	Password string `json:"-"` // для смены пароля (в демо, без хэшей)
}

// MockUsers — моковые пользователи под демо
func MockUsers(plans []Plan) []*User {
	var basicID, proID, maxID string
	for _, p := range plans {
		switch p.ID {
		case "basic":
			basicID = p.ID
		case "pro":
			proID = p.ID
		case "max":
			maxID = p.ID
		}
	}
	now := time.Now()

	if basicID == "" {
		basicID = "basic"
	}
	if proID == "" {
		proID = "pro"
	}
	if maxID == "" {
		maxID = "max"
	}

	return []*User{
		{
			ID:         "admin",
			Email:      "admin@example.com",
			Name:       "Администратор",
			Role:       RoleAdmin,
			PlanID:     maxID,
			PlanActive: true,
			CreatedAt:  now.Add(-72 * time.Hour),
			Password:   "admin123",
		},
		{
			ID:         "user1",
			Email:      "client1@example.com",
			Name:       "Клиент 1 (активный тариф)",
			Role:       RoleUser,
			PlanID:     proID,
			PlanActive: true,
			CreatedAt:  now.Add(-48 * time.Hour),
			Password:   "user1pass",
		},
		{
			ID:         "user2",
			Email:      "client2@example.com",
			Name:       "Клиент 2 (тариф закончился)",
			Role:       RoleUser,
			PlanID:     basicID,
			PlanActive: false, // неактивный тариф
			CreatedAt:  now.Add(-24 * time.Hour),
			Password:   "user2pass",
		},
	}
}
