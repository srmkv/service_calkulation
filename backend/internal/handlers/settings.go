package handlers

import (
	"encoding/json"
	"net/http"

	"saas-calc-backend/internal/domain"
)

// AdminSettings — настройки, доступные администратору.
// Яндекс-ключ убрали, оставили только базовые URL для сервисов маршрутизации.
type AdminSettings struct {
	OSRMBaseURL      string `json:"osrmBaseUrl"`
	NominatimBaseURL string `json:"nominatimBaseUrl"`
}

// GET/POST /api/admin/settings
func (e *Env) HandleAdminSettings(w http.ResponseWriter, r *http.Request) {
	u := e.CurrentUser(r)
	if u == nil || u.Role != domain.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		resp := AdminSettings{
			OSRMBaseURL:      e.OSRMBaseURL,
			NominatimBaseURL: e.NominatimBaseURL,
		}
		e.writeJSON(w, resp)

	case http.MethodPost:
		defer r.Body.Close()

		var req AdminSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Обновляем только если что-то прислали
		if req.OSRMBaseURL != "" {
			e.OSRMBaseURL = req.OSRMBaseURL
		}
		if req.NominatimBaseURL != "" {
			e.NominatimBaseURL = req.NominatimBaseURL
		}

		resp := AdminSettings{
			OSRMBaseURL:      e.OSRMBaseURL,
			NominatimBaseURL: e.NominatimBaseURL,
		}
		e.writeJSON(w, resp)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
