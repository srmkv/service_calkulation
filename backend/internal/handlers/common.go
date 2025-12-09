// internal/handlers/common.go

package handlers

import (
    "encoding/json"
    "net/http"

    "saas-calc-backend/internal/domain"
)

// Env хранит зависимости для хендлеров.
type Env struct {
    LayeredConfig  *domain.LayeredConfig
    DistanceConfig *domain.DistanceConfig

    UploadDir string
    Plans     []domain.Plan
    Users     []*domain.User

    Calculators []*domain.Calculator
    NextCalcID  int

    // базовые URL для сервисов карт
    OSRMBaseURL      string // например, "https://router.project-osrm.org"
    NominatimBaseURL string // например, "https://nominatim.openstreetmap.org"
}

// writeJSON — простой helper для JSON-ответов
func (e *Env) writeJSON(w http.ResponseWriter, v interface{}) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    enc := json.NewEncoder(w)
    if err := enc.Encode(v); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

// WithCORS — простой CORS-мидлвар для dev.
func WithCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// CurrentUser — простой резолвер "текущего пользователя".
// Для демо берём query-параметр ?as=admin|user1|user2, иначе admin.
func (e *Env) CurrentUser(r *http.Request) *domain.User {
    as := r.URL.Query().Get("as")
    if as == "" {
        as = "admin"
    }

    for _, u := range e.Users {
        if u.ID == as {
            return u
        }
    }

    if len(e.Users) > 0 {
        return e.Users[0]
    }

    return nil
}
