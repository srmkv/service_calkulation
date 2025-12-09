package domain

// DistanceConfig — конфигурация калькулятора расстояний/доставки
type DistanceConfig struct {
	BasePrice      float64            `json:"basePrice"`      // базовая стоимость, ₽
	PricePerKm     float64            `json:"pricePerKm"`     // цена за км, ₽
	LoadingPrice   float64            `json:"loadingPrice"`   // погрузка, ₽
	UnloadingPrice float64            `json:"unloadingPrice"` // разгрузка, ₽
	VehicleCoefs   map[string]float64 `json:"vehicleCoefs"`   // коэффициенты по типу ТС (small/medium/large)
}

// NewDefaultDistanceConfig — дефолтные значения для демо
func NewDefaultDistanceConfig() *DistanceConfig {
	return &DistanceConfig{
		BasePrice:      1500,
		PricePerKm:     45,
		LoadingPrice:   0,
		UnloadingPrice: 0,
		VehicleCoefs: map[string]float64{
			"small":  1.0,
			"medium": 1.2,
			"large":  1.5,
		},
	}
}
