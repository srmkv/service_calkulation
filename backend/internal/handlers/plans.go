package handlers

import (
	"net/http"

	"saas-calc-backend/internal/domain"
)

// GET /api/plans
func (e *Env) HandlePlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if e.DB == nil {
		http.Error(w, "database is not configured", http.StatusInternalServerError)
		return
	}

	rows, err := e.DB.QueryContext(
		r.Context(),
		`SELECT id, name, description, price, max_calculators, max_leads, max_calcs
         FROM plans
         ORDER BY price ASC`,
	)
	if err != nil {
		http.Error(w, "failed to query plans: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var plans []domain.Plan

	for rows.Next() {
		var p domain.Plan
		if err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.MaxCalculators,
			&p.MaxLeads,
			&p.MaxCalcs,
		); err != nil {
			http.Error(w, "failed to scan plan: "+err.Error(), http.StatusInternalServerError)
			return
		}
		plans = append(plans, p)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "rows error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Если тарифов нет в БД — считаем это ошибкой конфигурации
	if len(plans) == 0 {
		http.Error(w, "no plans configured in database", http.StatusInternalServerError)
		return
	}

	e.writeJSON(w, plans)
}
