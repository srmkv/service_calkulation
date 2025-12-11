package handlers

import (
    "context"
    "database/sql"
    "encoding/json"
    "log"
    "net/http"
    "time"

    "saas-calc-backend/internal/domain"
)


// Env хранит зависимости для хендлеров.
type Env struct {
    DB *sql.DB

    LayeredConfig  *domain.LayeredConfig
    DistanceConfig *domain.DistanceConfig

    UploadDir string
    Plans     []domain.Plan

    // старые in-memory поля можно оставить, но не использовать как источник истины
    Users       []*domain.User
    Calculators []*domain.Calculator
    NextCalcID  int

    // базовые URL для сервисов карт
    OSRMBaseURL      string
    NominatimBaseURL string
    TelegramBotToken string
}

// writeJSON — простой helper для JSON-ответов
func (e *Env) writeJSON(w http.ResponseWriter, v interface{}) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    enc := json.NewEncoder(w)
    if err := enc.Encode(v); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}
// IncrementCalcCount увеличивает calc_count для указанного калькулятора
// и в БД, и (по возможности) в in-memory кэше.
func (e *Env) IncrementCalcCount(calcID string) {
    if e == nil || calcID == "" {
        return
    }

    // 1) Обновляем БД — это источник истины для /me и /api/calculators
    if e.DB != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()

        _, err := e.DB.ExecContext(ctx,
            `UPDATE calculators
               SET calc_count = calc_count + 1
             WHERE id = $1`,
            calcID,
        )
        if err != nil {
            log.Printf("IncrementCalcCount: db error for %s: %v", calcID, err)
        }
    }

    // 2) Обновляем in-memory кэш (чисто чтобы e.Calculators тоже не отставал)
    for _, c := range e.Calculators {
        if c != nil && c.ID == calcID {
            c.CalcCount++
            break
        }
    }
}

// WithCORS — простой CORS-мидлвар для dev.
func WithCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// CurrentUser — теперь берём пользователя из БД по ?as=admin|user1|user2
// (то есть выбор в шапке <select id="user-switch"> остаётся тем же, просто источник истины — БД).
func (e *Env) CurrentUser(r *http.Request) *domain.User {
    as := r.URL.Query().Get("as")
    if as == "" {
        as = "admin"
    }

    // сначала пытаемся достать пользователя из БД
    if e.DB != nil {
        row := e.DB.QueryRow(`
SELECT id, email, name, role, plan_id, plan_active, created_at
FROM users
WHERE id = $1
`, as)

        var u domain.User
        var planID string
        var planActive bool

        if err := row.Scan(
            &u.ID,
            &u.Email,
            &u.Name,
            &u.Role,
            &planID,
            &planActive,
            &u.CreatedAt,
        ); err == nil {
            u.PlanID = planID
            u.PlanActive = planActive
            return &u
        }
    }

    // фолбэк на старый in-memory список, если что-то не так с БД
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

