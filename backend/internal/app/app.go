package app

import (
    "context"
    "database/sql"
    "log"
    "net/http"

    "saas-calc-backend/internal/domain"
    "saas-calc-backend/internal/handlers"
)

type App struct {
    mux *http.ServeMux
    Env *handlers.Env
}

func New(db *sql.DB) *App {
    mux := http.NewServeMux()
    ctx := context.Background()

    // 1. Схема БД (ensureSchema объявлен в db.go)
    if err := ensureSchema(db); err != nil {
        log.Fatalf("ensureSchema: %v", err)
    }

    // 2. Базовые тарифы из кода
    basePlans := domain.DefaultPlans()

    // 3. Засеять тарифы в БД (если таблица plans пуста)
    //    seedPlans и loadPlans объявлены в db.go
    if err := seedPlans(ctx, db, basePlans); err != nil {
        log.Fatalf("seedPlans: %v", err)
    }

    // 4. Загрузить тарифы из БД
    plans, err := loadPlans(ctx, db)
    if err != nil {
        log.Fatalf("loadPlans: %v", err)
    }
    if len(plans) == 0 {
        // на всякий случай, если по какой-то причине таблица пустая
        plans = basePlans
    }

    // 5. Засеять демо-пользователей (admin / user1 / user2), если users пустая
    if err := seedDemoUsersIfEmpty(ctx, db, plans); err != nil {
        log.Fatalf("seedDemoUsersIfEmpty: %v", err)
    }

    // 6. Загрузить всех пользователей из БД
    users, err := loadUsers(ctx, db)
    if err != nil {
        log.Fatalf("loadUsers: %v", err)
    }

    // 7. Демо-калькуляторы для этих пользователей (admin / user1 / user2)
    demoCalcs := domain.MockCalculators(users)

    // 8. Инициализация/загрузка калькуляторов в БД
    calculators, err := initCalculators(ctx, db, demoCalcs)
    if err != nil {
        log.Fatalf("initCalculators: %v", err)
    }

    env := &handlers.Env{
        DB:             db,
        LayeredConfig:  domain.NewDefaultLayeredConfig(),
        DistanceConfig: domain.NewDefaultDistanceConfig(),
        UploadDir:      "../frontend/uploads",

        Plans:       plans,
        Users:       users,       // пользователи из БД
        Calculators: calculators, // калькуляторы из БД
        NextCalcID:  len(calculators) + 1,

        OSRMBaseURL:      "https://router.project-osrm.org",
        NominatimBaseURL: "https://nominatim.openstreetmap.org",
        TelegramBotToken: "",
    }

    registerRoutes(mux, env)

    return &App{
        mux: mux,
        Env: env,
    }
}

func (a *App) Router() *http.ServeMux {
    return a.mux
}

//
// ---- Демо-пользователи (admin / user1 / user2) --------------------------
//

func seedDemoUsersIfEmpty(ctx context.Context, db *sql.DB, plans []domain.Plan) error {
    if db == nil {
        return nil
    }

    var cnt int
    if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&cnt); err != nil {
        return err
    }
    if cnt > 0 {
        // уже есть пользователи – ничего не делаем
        return nil
    }

    // найдём ID тарифов по их кодам
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
    // подстрахуемся, если что-то не нашли
    if basicID == "" && len(plans) > 0 {
        basicID = plans[0].ID
    }
    if proID == "" {
        proID = basicID
    }
    if maxID == "" {
        maxID = proID
    }

    _, err := db.ExecContext(ctx, `
INSERT INTO users (id, email, name, role, password_hash, plan_id, plan_active)
VALUES
  ('admin', 'admin@example.com', 'Администратор', 'admin', 'x', $1, TRUE),
  ('user1', 'user1@example.com', 'Клиент 1', 'user', 'x', $2, TRUE),
  ('user2', 'user2@example.com', 'Клиент 2', 'user', 'x', $3, TRUE)
ON CONFLICT (id) DO NOTHING;
`, maxID, proID, basicID)
    return err
}

//
// ---- Загрузка пользователей из БД ---------------------------------------
//

func loadUsers(ctx context.Context, db *sql.DB) ([]*domain.User, error) {
    if db == nil {
        return nil, nil
    }

    rows, err := db.QueryContext(ctx, `
SELECT id, email, name, role, plan_id, plan_active, created_at,telegram_chat_id
FROM users
ORDER BY created_at;
`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []*domain.User
    for rows.Next() {
        var u domain.User
        var planID string
        var planActive bool
        var tgChatID string

        if err := rows.Scan(
            &u.ID,
            &u.Email,
            &u.Name,
            &u.Role,
            &planID,
            &planActive,
            &u.CreatedAt,
             &tgChatID,
        ); err != nil {
            return nil, err
        }

        u.PlanID = planID
        u.PlanActive = planActive
        u.TelegramChatID = tgChatID

        out = append(out, &u)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }

    return out, nil
}

//
// ---- Инициализация / загрузка калькуляторов -----------------------------
//

func initCalculators(ctx context.Context, db *sql.DB, demoCalcs []*domain.Calculator) ([]*domain.Calculator, error) {
    if db == nil {
        // fallback только на in-memory
        return demoCalcs, nil
    }

    var count int
    if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM calculators`).Scan(&count); err != nil {
        return nil, err
    }

    // если калькуляторов нет — засеваем демо
    if count == 0 && len(demoCalcs) > 0 {
        for _, c := range demoCalcs {
            if c == nil {
                continue
            }
            _, err := db.ExecContext(
                ctx,
                `INSERT INTO calculators
                  (id, name, type, owner_id, status, created_at, public_token, public_path, calc_count)
                 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
                c.ID,
                c.Name,
                string(c.Type),
                c.OwnerID,
                c.Status,
                c.CreatedAt,
                c.PublicToken,
                c.PublicPath,
                c.CalcCount,
            )
            if err != nil {
                return nil, err
            }
        }
    }

    // грузим всё из БД
    rows, err := db.QueryContext(ctx, `
SELECT id, name, type, owner_id, status, created_at, public_token, public_path, calc_count
FROM calculators
ORDER BY created_at DESC;
`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []*domain.Calculator
    for rows.Next() {
        var c domain.Calculator
        var t string
        if err := rows.Scan(
            &c.ID,
            &c.Name,
            &t,
            &c.OwnerID,
            &c.Status,
            &c.CreatedAt,
            &c.PublicToken,
            &c.PublicPath,
            &c.CalcCount,
        ); err != nil {
            return nil, err
        }
        c.Type = domain.CalculatorType(t)
        out = append(out, &c)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }

    return out, nil
}
