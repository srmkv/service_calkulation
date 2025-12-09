package domain

// LayerOption описывает одну опцию/слой
type LayerOption struct {
    ID      string            `json:"id"`
    Label   string            `json:"label"`
    Price   float64           `json:"price"`
    Default bool              `json:"default"`
    Order   int               `json:"order"`
    Layers  map[string]string `json:"layers"` // view -> image url (front, rear, side...)
}

// LayeredConfig — конфигурация послойного калькулятора
type LayeredConfig struct {
    BaseViews       map[string]string `json:"baseViews"`       // view -> base image url
    Options         []LayerOption     `json:"options"`
    BasePrice       float64           `json:"basePrice"`       // базовая цена "нулевого" слоя
    BaseDescription string            `json:"baseDescription"` // описание базовой комплектации
    ShowRear        bool              `json:"showRear"`        // показывать ли вид "rear" пользователю
}

// NewDefaultLayeredConfig возвращает стартовый конфиг
func NewDefaultLayeredConfig() *LayeredConfig {
    return &LayeredConfig{
        BaseViews: map[string]string{
            "front": "/img/trailer_front_base.png",
            "rear":  "/img/trailer_rear_base.png",
        },
        Options: []LayerOption{
            {
                ID:      "frame_tent",
                Label:   "Подъёмный каркас с тентом",
                Price:   40000,
                Default: true,
                Order:   1,
                Layers: map[string]string{
                    "front": "/img/trailer_front_tent.png",
                    "rear":  "/img/trailer_rear_tent.png",
                },
            },
            {
                ID:      "spare_wheel",
                Label:   "Крепление запасного колеса",
                Price:   4700,
                Default: false,
                Order:   2,
                Layers: map[string]string{
                    "front": "/img/trailer_front_spare.png",
                    "rear":  "/img/trailer_rear_spare.png",
                },
            },
        },
        BasePrice:       0,
        BaseDescription: "Базовая комплектация без дополнительных опций.",
        ShowRear:        true,
    }
}
