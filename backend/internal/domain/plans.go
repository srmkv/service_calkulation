package domain

// Plan описывает тариф
type Plan struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Price          float64 `json:"price"`
	MaxCalculators int     `json:"maxCalculators"`
	MaxLeads       int     `json:"maxLeads"`
	MaxCalcs       int     `json:"maxCalcs"`
}

// DefaultPlans возвращает три базовых тарифа
func DefaultPlans() []Plan {
	return []Plan{
		{
			ID:             "basic",
			Name:           "Базовый",
			Description:    "Для небольшого бизнеса: 1–2 калькулятора и базовая аналитика.",
			Price:          990,
			MaxCalculators: 2,
			MaxLeads:       200,
			MaxCalcs:       400,   // в 2 раза больше заявок
		},
		{
			ID:             "pro",
			Name:           "Pro",
			Description:    "Для компаний, у которых несколько направлений и реклама.",
			Price:          2990,
			MaxCalculators: 10,
			MaxLeads:       2000,
			MaxCalcs:       4000,  // в 2 раза больше заявок
		},
		{
			ID:             "max",
			Name:           "Max",
			Description:    "Агентства и сети: безлим по калькуляторам и расширенные лимиты.",
			Price:          7990,
			MaxCalculators: 999,
			MaxLeads:       100000,
			MaxCalcs:       200000, // в 2 раза больше заявок
		},
	}
}

// FindPlan ищет тариф по ID в слайсе тарифов
func FindPlan(plans []Plan, id string) *Plan {
	for i := range plans {
		if plans[i].ID == id {
			return &plans[i]
		}
	}
	return nil
}
