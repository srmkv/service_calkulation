package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"saas-calc-backend/internal/domain"
)

type calculatorsResponse struct {
	Items []*domain.Calculator `json:"items"`
}

type createCalculatorRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// HandleCalculators обслуживает /api/calculators.
//
// GET  -> список калькуляторов для текущего пользователя (админ видит все).
// POST -> создать новый калькулятор для текущего пользователя (+ проверка лимита тарифа).
func (e *Env) HandleCalculators(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		e.handleGetCalculators(w, r)
	case http.MethodPost:
		e.handlePostCalculator(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (e *Env) handleGetCalculators(w http.ResponseWriter, r *http.Request) {
	u := e.CurrentUser(r)
	if u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// администратор видит все калькуляторы
	if u.Role == domain.RoleAdmin {
		e.writeJSON(w, calculatorsResponse{Items: e.Calculators})
		return
	}

	// обычный пользователь видит только свои
	items := make([]*domain.Calculator, 0)
	for _, c := range e.Calculators {
		if c.OwnerID == u.ID {
			items = append(items, c)
		}
	}

	e.writeJSON(w, calculatorsResponse{Items: items})
}

func (e *Env) handlePostCalculator(w http.ResponseWriter, r *http.Request) {
	u := e.CurrentUser(r)
	if u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()

	var req createCalculatorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, "type is required", http.StatusBadRequest)
		return
	}

	calcType := domain.CalculatorType(req.Type)
	switch calcType {
	case domain.CalculatorTypeLayered,
		domain.CalculatorTypeDistance,
		domain.CalculatorTypeOnSite:
		// ok
	default:
		http.Error(w, "unknown calculator type", http.StatusBadRequest)
		return
	}

	// --- проверка лимита по тарифу (кроме администратора) ---
	plan := domain.FindPlan(e.Plans, u.PlanID)
	if u.Role != domain.RoleAdmin && plan != nil && plan.MaxCalculators > 0 {
		count := 0
		for _, existing := range e.Calculators {
			if existing.OwnerID == u.ID {
				count++
			}
		}
		if count >= plan.MaxCalculators {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Вы достигли лимита калькуляторов для вашего тарифа",
				"limit": plan.MaxCalculators,
			})
			return
		}
	}
	// --- конец проверки лимита ---

		id := "calc_" + strconv.Itoa(e.NextCalcID)
		e.NextCalcID++

	token := domain.GeneratePublicToken()
	publicPath := "/p/" + u.ID + "/" + token

	c := &domain.Calculator{
		ID:          id,
		Name:        req.Name,
		Type:        calcType,
		OwnerID:     u.ID,
		Status:      "draft",
		CreatedAt:   time.Now(),
		PublicToken: token,
		PublicPath:  publicPath,
		CalcCount:   0, // <-- новый счётчик
	}

	e.Calculators = append(e.Calculators, c)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(c)
}
