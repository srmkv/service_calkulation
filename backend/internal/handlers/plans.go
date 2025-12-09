package handlers

import (
	"net/http"
)

// GET /api/plans
func (e *Env) HandlePlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	e.writeJSON(w, e.Plans)
}
