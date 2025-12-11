package handlers

import (
	"database/sql"
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
// GET    -> список калькуляторов для текущего пользователя (админ видит все).
// POST   -> создать новый калькулятор для текущего пользователя (+ проверка лимита тарифа).
// DELETE -> удалить калькулятор по id (админ — любой, пользователь — только свой).
func (e *Env) HandleCalculators(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		e.handleGetCalculators(w, r)
	case http.MethodPost:
		e.handlePostCalculator(w, r)
	case http.MethodDelete:
		e.handleDeleteCalculator(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- GET /api/calculators ---

func (e *Env) handleGetCalculators(w http.ResponseWriter, r *http.Request) {
	u := e.CurrentUser(r)
	if u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	// Если есть БД — читаем из неё
	if e.DB != nil {
		var rows *sql.Rows
		var err error

		if u.Role == domain.RoleAdmin {
			rows, err = e.DB.Query(`
				SELECT id, name, type, owner_id, status, created_at, public_token, public_path, calc_count
				FROM calculators
				ORDER BY created_at DESC
			`)
		} else {
			rows, err = e.DB.Query(`
				SELECT id, name, type, owner_id, status, created_at, public_token, public_path, calc_count
				FROM calculators
				WHERE owner_id = $1
				ORDER BY created_at DESC
			`, u.ID)
		}

		if err != nil {
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		items := make([]*domain.Calculator, 0)
		for rows.Next() {
			var (
				id, name, ctypeStr, ownerID, status, publicToken, publicPath string
				createdAt                                                    time.Time
				calcCount                                                    int
			)
			if err := rows.Scan(
				&id,
				&name,
				&ctypeStr,
				&ownerID,
				&status,
				&createdAt,
				&publicToken,
				&publicPath,
				&calcCount,
			); err != nil {
				http.Error(w, "db scan error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			calc := &domain.Calculator{
				ID:          id,
				Name:        name,
				Type:        domain.CalculatorType(ctypeStr),
				OwnerID:     ownerID,
				Status:      status,
				CreatedAt:   createdAt,
				PublicToken: publicToken,
				PublicPath:  publicPath,
				CalcCount:   calcCount,
			}
			items = append(items, calc)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "db rows error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Обновим in-memory кэш для совместимости с существующим кодом
		if u.Role == domain.RoleAdmin {
			e.Calculators = items
		} else {
			// для обычного пользователя кэш не трогаем, чтобы админский список не терять
		}

		e.writeJSON(w, calculatorsResponse{Items: items})
		return
	}

	// --- Fallback: старая in-memory логика, если БД нет ---

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

// --- POST /api/calculators ---

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
		domain.CalculatorTypeOnSite,
		domain.CalculatorTypeMortgage:
		// ok
	default:
		http.Error(w, "unknown calculator type", http.StatusBadRequest)
		return
	}

	// --- проверка лимита по тарифу (кроме администратора) ---
	plan := domain.FindPlan(e.Plans, u.PlanID)
	if u.Role != domain.RoleAdmin && plan != nil && plan.MaxCalculators > 0 {
		// считаем существующие калькуляторы владельца
		count := 0

		if e.DB != nil {
			if err := e.DB.QueryRow(
				`SELECT COUNT(*) FROM calculators WHERE owner_id = $1`,
				u.ID,
			).Scan(&count); err != nil {
				http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			for _, existing := range e.Calculators {
				if existing.OwnerID == u.ID {
					count++
				}
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
		CalcCount:   0,
	}

	// Сначала сохраняем в БД, если она есть
	if e.DB != nil {
		_, err := e.DB.Exec(`
			INSERT INTO calculators (
				id, name, type, owner_id, status, created_at,
				public_token, public_path, calc_count
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		`,
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
			http.Error(w, "db insert error: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Обновляем in-memory список для совместимости
	e.Calculators = append(e.Calculators, c)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(c)
}

// --- DELETE /api/calculators?id=calc_123 ---

func (e *Env) handleDeleteCalculator(w http.ResponseWriter, r *http.Request) {
	u := e.CurrentUser(r)
	if u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	// Удаляем в БД, если она есть
	if e.DB != nil {
		if u.Role == domain.RoleAdmin {
			_, err := e.DB.Exec(`DELETE FROM calculators WHERE id = $1`, id)
			if err != nil {
				http.Error(w, "db delete error: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			res, err := e.DB.Exec(
				`DELETE FROM calculators WHERE id = $1 AND owner_id = $2`,
				id, u.ID,
			)
			if err != nil {
				http.Error(w, "db delete error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
		}
	}

	// Синхронизируем in-memory кэш
	filtered := e.Calculators[:0]
	for _, c := range e.Calculators {
		if c.ID != id {
			filtered = append(filtered, c)
		}
	}
	e.Calculators = filtered

	w.WriteHeader(http.StatusNoContent)
}
