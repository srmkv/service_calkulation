package handlers

import (
	"log"
	"net/http"

	"saas-calc-backend/internal/domain"
)

// GET /api/plans
func (e *Env) HandlePlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var plans []domain.Plan

	// 1) Пытаемся взять планы из БД, если она есть
	if e.DB != nil {
		rows, err := e.DB.QueryContext(
			r.Context(),
			`SELECT id, name, description, price, max_calculators, max_leads, max_calcs
             FROM plans
             ORDER BY price ASC, id ASC`,
		)
		if err != nil {
			// Логируем, но не роняем, ниже попробуем взять из памяти/дефолтных
			log.Printf("HandlePlans: failed to query plans from DB: %v", err)
		} else {
			defer rows.Close()

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
					log.Printf("HandlePlans: failed to scan plan: %v", err)
					plans = nil
					break
				}
				plans = append(plans, p)
			}
			if err := rows.Err(); err != nil {
				log.Printf("HandlePlans: rows error: %v", err)
				plans = nil
			}
		}
	}

	// 2) Если из БД ничего не получили — пробуем Env.Plans
	if len(plans) == 0 && len(e.Plans) > 0 {
		plans = e.Plans
	}

	// 3) Если и там пусто — дефолтные планы из домена
	if len(plans) == 0 {
		plans = domain.DefaultPlans()
	}

	// 4) Обновляем кэш в Env
	e.Plans = plans

	// 5) Отдаём фронту
	e.writeJSON(w, plans)
}
