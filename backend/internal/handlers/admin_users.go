package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"saas-calc-backend/internal/domain"
)

// структура запроса на обновление пользователя из фронта
type adminUpdateUserRequest struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Role       string `json:"role"`
	PlanID     string `json:"planId"`
	PlanActive bool   `json:"planActive"`
}

// DTO для ответа — пользователь + его план
type adminUserDTO struct {
	*domain.User        `json:",inline"`
	Plan         *domain.Plan `json:"plan,omitempty"`
}

// GET /api/admin/users
func (e *Env) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	cur := e.CurrentUser(r)
	if cur == nil || cur.Role != domain.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	users, err := e.ListUsers(r.Context())
	if err != nil {
		http.Error(w, "failed to list users: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		dto := adminUserDTO{User: u}
		if u.PlanID != "" {
			dto.Plan = domain.FindPlan(e.Plans, u.PlanID)
		}
		resp = append(resp, dto)
	}

	e.writeJSON(w, resp)
}

// /api/admin/users/{id}
// /api/admin/users/{id}/password
func (e *Env) HandleAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	cur := e.CurrentUser(r)
	if cur == nil || cur.Role != domain.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	const prefix = "/api/admin/users/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix) // "{id}" или "{id}/password"

	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		// /api/admin/users/{id}  -> PUT (update), DELETE
		id := parts[0]
		switch r.Method {
		case http.MethodPut:
			e.handleAdminUserUpdate(w, r, id)
		case http.MethodDelete:
			e.handleAdminUserDelete(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	if len(parts) == 2 && parts[1] == "password" {
		// /api/admin/users/{id}/password  -> POST (смена пароля)
		id := parts[0]
		e.handleAdminUserPassword(w, r, id)
		return
	}

	http.NotFound(w, r)
}

func (e *Env) handleAdminUserUpdate(w http.ResponseWriter, r *http.Request, id string) {
	defer r.Body.Close()

	u, err := e.GetUserByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to load user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if u == nil {
		http.NotFound(w, r)
		return
	}

	var req adminUpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	if req.Role != "admin" && req.Role != "user" {
		http.Error(w, "invalid role", http.StatusBadRequest)
		return
	}
	if req.PlanID != "" && domain.FindPlan(e.Plans, req.PlanID) == nil {
		http.Error(w, "unknown planId", http.StatusBadRequest)
		return
	}

	u.Name = req.Name
	u.Email = req.Email
	u.Role = domain.Role(req.Role)
	u.PlanID = req.PlanID
	u.PlanActive = req.PlanActive

	if err := e.UpdateUser(r.Context(), u); err != nil {
		http.Error(w, "failed to update user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	dto := adminUserDTO{
		User: u,
	}
	if u.PlanID != "" {
		dto.Plan = domain.FindPlan(e.Plans, u.PlanID)
	}

	e.writeJSON(w, dto)
}

func (e *Env) handleAdminUserDelete(w http.ResponseWriter, r *http.Request, id string) {
	if err := e.DeleteUser(r.Context(), id); err != nil {
		http.Error(w, "failed to delete user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (e *Env) handleAdminUserPassword(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return;
	}

	if err := e.SetUserPassword(r.Context(), id, body.Password); err != nil {
		http.Error(w, "failed to set password: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
