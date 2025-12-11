package domain

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type CalculatorType string

const (
	CalculatorTypeLayered  CalculatorType = "layered"
	CalculatorTypeDistance CalculatorType = "distance"
	CalculatorTypeOnSite   CalculatorType = "on_site"
	CalculatorTypeMortgage CalculatorType = "mortgage"
)

// Calculator — описание калькулятора в системе
type Calculator struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        CalculatorType `json:"type"`
	OwnerID     string         `json:"ownerId"`
	Status      string         `json:"status"` // draft / published / archived и т.п.
	CreatedAt   time.Time      `json:"createdAt"`

	// Публичная часть: токен и путь, по которому доступен калькулятор
	PublicToken string `json:"publicToken"`
	PublicPath  string `json:"publicPath"`

	// Сколько раз этот калькулятор реально считали (учитываем в тарифе)
	CalcCount int `json:"calcCount"`
}

// GeneratePublicToken — простой генератор токена для публичного доступа к калькулятору
func GeneratePublicToken() string {
	const size = 16
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		// в крайнем случае — fallback, чтобы не паниковать
		return time.Now().Format("20060102150405")
	}
	return hex.EncodeToString(b)
}

// MockCalculators — демо-калькуляторы для старта
// MockCalculators — демо-калькуляторы для старта
func MockCalculators(users []*User) []*Calculator {
	now := time.Now()

	// найдём ID user1 и user2, если что — fallback
	user1ID := "user1"
	user2ID := "user2"

	for _, u := range users {
		if u.ID == "user1" {
			user1ID = u.ID
		}
		if u.ID == "user2" {
			user2ID = u.ID
		}
	}

	// генерим токены заранее, чтобы использовать и в токене, и в пути
	token1 := GeneratePublicToken()
	token2 := GeneratePublicToken()
	token3 := GeneratePublicToken()

	return []*Calculator{
		{
			ID:          "calc_1",
			Name:        "Прицеп – послойный калькулятор (демо)",
			Type:        CalculatorTypeLayered,
			OwnerID:     user1ID,
			Status:      "published",
			CreatedAt:   now.Add(-48 * time.Hour),
			PublicToken: token1,
			PublicPath:  "/p/" + user1ID + "/" + token1,
			CalcCount:   27,
		},
		{
			ID:          "calc_2",
			Name:        "Расчёт доставки по городу (демо)",
			Type:        CalculatorTypeDistance,
			OwnerID:     user1ID,
			Status:      "published",
			CreatedAt:   now.Add(-24 * time.Hour),
			PublicToken: token2,
			PublicPath:  "/p/" + user1ID + "/" + token2,
			CalcCount:   134,
		},
		{
			ID:          "calc_3",
			Name:        "Выезд замерщика (демо)",
			Type:        CalculatorTypeOnSite,
			OwnerID:     user2ID,
			Status:      "draft",
			CreatedAt:   now.Add(-12 * time.Hour),
			PublicToken: token3,
			PublicPath:  "/p/" + user2ID + "/" + token3,
			CalcCount:   5,
		},
	}
}

