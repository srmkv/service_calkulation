package handlers

import (
	"encoding/json"
	"net/http"

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

// HandleMe — отдаёт информацию о текущем пользователе и тарифах.
// Разрешаем вызывать и GET, и POST, т.к. из HandleMePlan мы дергаем его после изменения тарифа.
func (e *Env) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "POST" {
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

	leadsUsed, calcsUsed := e.usageForUser(u)

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

// POST /api/me/plan  { "planId": "pro" }
func (e *Env) HandleMePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
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

	if domain.FindPlan(e.Plans, req.PlanID) == nil {
		http.Error(w, "unknown planId", http.StatusBadRequest)
		return
	}

	// меняем тариф
	u.PlanID = req.PlanID
	// считаем, что при смене тарифа он активируется
	u.PlanActive = true

	// и сразу отдаём обновлённое состояние /me
	e.HandleMe(w, r)
}

// usageForUser — считает использованные заявки/расчёты для конкретного пользователя.
func (e *Env) usageForUser(u *domain.User) (leadsUsed int, calcsUsed int) {
	if e == nil || u == nil {
		return 0, 0
	}

	// расчёты — суммируем CalcCount по калькуляторам владельца
	for _, c := range e.Calculators {
		if c.OwnerID == u.ID {
			calcsUsed += c.CalcCount
		}
	}

	// заявки — если у тебя уже есть сущность заявок/лидов, сюда можно добавить логику.
	// Пока оставляем 0, чтобы компилялось и не падало.
	// Пример (если заведёшь e.Leads):
	//
	// for _, lead := range e.Leads {
	//     if lead.OwnerID == u.ID {
	//         leadsUsed++
	//     }
	// }

	return leadsUsed, calcsUsed
}
