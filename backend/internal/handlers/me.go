package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"saas-calc-backend/internal/domain"
)

type MeResponse struct {
	User       *domain.User  `json:"user"`
	Plan       *domain.Plan  `json:"plan"`
	Plans      []domain.Plan `json:"plans"`      // все тарифы для фронта
	PlanActive bool          `json:"planActive"` // активен ли тариф у пользователя

	LeadsUsed int `json:"leadsUsed"` // сколько заявок уже создано пользователем
	CalcsUsed int `json:"calcsUsed"` // сколько расчётов уже сделано
}
type changeTelegramRequest struct {
    ChatID string `json:"chatId"`
}

// HandleMe — отдаёт информацию о текущем пользователе и тарифах.
// Разрешаем вызывать и GET, и POST, т.к. из HandleMePlan мы дергаем его после изменения тарифа.
func (e *Env) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	u := e.CurrentUser(r)
	if u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	var currentPlan *domain.Plan
	if u.PlanID != "" {
		currentPlan = domain.FindPlan(e.Plans, u.PlanID)
	}

	leadsUsed, calcsUsed := e.usageForUser(r.Context(), u)

	resp := MeResponse{
		User:       u,
		Plan:       currentPlan,
		Plans:      e.Plans,
		PlanActive: u.PlanActive,

		LeadsUsed: leadsUsed,
		CalcsUsed: calcsUsed,
	}

	e.writeJSON(w, resp)
}

type changePlanRequest struct {
	PlanID string `json:"planId"`
}
// POST /api/me/telegram { "chatId": "123456789" }
func (e *Env) HandleMeTelegram(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    u := e.CurrentUser(r)
    if u == nil {
        http.Error(w, "user not found", http.StatusUnauthorized)
        return
    }

    defer r.Body.Close()
    var req changeTelegramRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
        return
    }

    chatID := strings.TrimSpace(req.ChatID)
    u.TelegramChatID = chatID

    if e.DB != nil {
        if _, err := e.DB.ExecContext(
            r.Context(),
            `UPDATE users SET telegram_chat_id = $1 WHERE id = $2`,
            chatID, u.ID,
        ); err != nil {
            http.Error(w, "failed to update telegram id: "+err.Error(), http.StatusInternalServerError)
            return
        }
    }

    // сразу отдаём обновлённый /me
    e.HandleMe(w, r)
}

// POST /api/me/plan  { "planId": "pro" }
func (e *Env) HandleMePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	u := e.CurrentUser(r)
	if u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	defer r.Body.Close()

	var req changePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.PlanID == "" {
		http.Error(w, "planId is required", http.StatusBadRequest)
		return
	}

	// --- проверяем, что план существует ---

	// если есть БД — проверяем в таблице plans
	if e.DB != nil {
		var exists bool
		if err := e.DB.QueryRowContext(
			r.Context(),
			`SELECT EXISTS(SELECT 1 FROM plans WHERE id = $1)`,
			req.PlanID,
		).Scan(&exists); err != nil {
			http.Error(w, "failed to check plan: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "unknown planId", http.StatusBadRequest)
			return
		}
	} else {
		// фоллбек — проверяем по e.Plans в памяти
		if domain.FindPlan(e.Plans, req.PlanID) == nil {
			http.Error(w, "unknown planId", http.StatusBadRequest)
			return
		}
	}

	// --- меняем тариф в памяти ---
	u.PlanID = req.PlanID
	// считаем, что при смене тарифа он активируется
	u.PlanActive = true

	// --- и в БД, если она есть ---
	if e.DB != nil {
		if _, err := e.DB.ExecContext(
			r.Context(),
			`UPDATE users
			   SET plan_id = $1,
			       plan_active = TRUE
			 WHERE id = $2`,
			req.PlanID,
			u.ID,
		); err != nil {
			http.Error(w, "failed to update user plan in db: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// и сразу отдаём обновлённое состояние /me
	e.HandleMe(w, r)
}


// usageForUser — считает использованные заявки/расчёты для конкретного пользователя.
// Сначала пытается взять данные из БД (если Env.DB != nil), иначе падает в in-memory логику по e.Calculators.
func (e *Env) usageForUser(ctx context.Context, u *domain.User) (leadsUsed int, calcsUsed int) {
	if e == nil || u == nil {
		return 0, 0
	}

	// --- Вариант с БД ---
	if e.DB != nil {
		// заявки: COUNT(*) из таблицы leads по owner_id
		// если таблицы ещё нет — ошибки просто игнорируем, оставляем 0
		if row := e.DB.QueryRowContext(ctx,
			`SELECT COALESCE(COUNT(*), 0) FROM leads WHERE owner_id = $1`,
			u.ID,
		); row != nil {
			_ = row.Scan(&leadsUsed)
		}

		// расчёты: SUM(calc_count) из calculators по owner_id
		if row := e.DB.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(calc_count), 0) FROM calculators WHERE owner_id = $1`,
			u.ID,
		); row != nil {
			_ = row.Scan(&calcsUsed)
		}

		return leadsUsed, calcsUsed
	}

	// --- Фоллбэк: старая in-memory логика, если БД ещё не подключена ---

	// расчёты — суммируем CalcCount по калькуляторам владельца
	for _, c := range e.Calculators {
		if c.OwnerID == u.ID {
			calcsUsed += c.CalcCount
		}
	}

	// заявки — пока 0, до реализации сущности лидов в памяти
	return leadsUsed, calcsUsed
}
