package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"saas-calc-backend/internal/domain"
)

type adminUserDTO struct {
	ID         string        `json:"id"`
	Email      string        `json:"email"`
	Name       string        `json:"name"`
	Role       string        `json:"role"`
	Plan       *domain.Plan  `json:"plan,omitempty"`
	PlanActive bool          `json:"planActive"`
	CreatedAt  time.Time     `json:"createdAt"`
}

// requireAdmin — проверка, что текущий пользователь админ
func (e *Env) requireAdmin(w http.ResponseWriter, r *http.Request) (*domain.User, bool) {
	u := e.CurrentUser(r)
	if u == nil {
		http.Error(w, "user not found", http.StatusUnauthorized)
		return nil, false
	}
	if u.Role != domain.RoleAdmin {
		http.Error(w, "forbidden", http.StatusForbidden)
		return nil, false
	}
	return u, true
}

// GET /api/admin/users  — список пользователей (только админ)
func (e *Env) HandleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	_, ok := e.requireAdmin(w, r)
	if !ok {
		return
	}

	list := make([]adminUserDTO, 0, len(e.Users))
	for _, u := range e.Users {
		var plan *domain.Plan
		if u.PlanID != "" {
			plan = domain.FindPlan(e.Plans, u.PlanID)
		}
		list = append(list, adminUserDTO{
			ID:         u.ID,
			Email:      u.Email,
			Name:       u.Name,
			Role:       string(u.Role),
			Plan:       plan,
			PlanActive: u.PlanActive,
			CreatedAt:  u.CreatedAt,
		})
	}

	e.writeJSON(w, list)
}

// HandleAdminUserDetail обслуживает:
//  PUT /api/admin/users/{id}          — обновление
//  DELETE /api/admin/users/{id}       — удаление
//  POST /api/admin/users/{id}/password — смена пароля
func (e *Env) HandleAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	admin, ok := e.requireAdmin(w, r)
	if !ok {
		return
	}

	const prefix = "/api/admin/users/"
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	if rest == "" || rest == r.URL.Path {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(rest, "/")
	userID := parts[0]

	// ищем пользователя
	var user *domain.User
	var idx int
	for i, u := range e.Users {
		if u.ID == userID {
			user = u
			idx = i
			break
		}
	}
	if user == nil {
		http.NotFound(w, r)
		return
	}

	// смена пароля
	if len(parts) == 2 && parts[1] == "password" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		e.handleAdminUserPassword(w, r, user)
		return
	}

	// операции над самим пользователем
	switch r.Method {
	case http.MethodPut:
		e.handleAdminUserUpdate(w, r, user)
	case http.MethodDelete:
		// нельзя удалить самого себя
		if user.ID == admin.ID {
			http.Error(w, "нельзя удалить текущего администратора", http.StatusBadRequest)
			return
		}
		// нельзя удалить последнего админа
		adminCount := 0
		for _, u := range e.Users {
			if u.Role == domain.RoleAdmin {
				adminCount++
			}
		}
		if user.Role == domain.RoleAdmin && adminCount <= 1 {
			http.Error(w, "нельзя удалить последнего администратора", http.StatusBadRequest)
			return
		}

		e.Users = append(e.Users[:idx], e.Users[idx+1:]...)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type updateUserRequest struct {
	Email      *string `json:"email,omitempty"`
	Name       *string `json:"name,omitempty"`
	Role       *string `json:"role,omitempty"`
	PlanID     *string `json:"planId,omitempty"`
	PlanActive *bool   `json:"planActive,omitempty"`
}

func (e *Env) handleAdminUserUpdate(w http.ResponseWriter, r *http.Request, user *domain.User) {
	defer r.Body.Close()

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Role != nil {
		switch *req.Role {
		case "admin":
			user.Role = domain.RoleAdmin
		case "user":
			user.Role = domain.RoleUser
		default:
			http.Error(w, "unknown role", http.StatusBadRequest)
			return
		}
	}
	if req.PlanID != nil {
		if *req.PlanID != "" && domain.FindPlan(e.Plans, *req.PlanID) == nil {
			http.Error(w, "unknown planId", http.StatusBadRequest)
			return
		}
		user.PlanID = *req.PlanID
	}
	if req.PlanActive != nil {
		user.PlanActive = *req.PlanActive
	}

	var plan *domain.Plan
	if user.PlanID != "" {
		plan = domain.FindPlan(e.Plans, user.PlanID)
	}

	dto := adminUserDTO{
		ID:         user.ID,
		Email:      user.Email,
		Name:       user.Name,
		Role:       string(user.Role),
		Plan:       plan,
		PlanActive: user.PlanActive,
		CreatedAt:  user.CreatedAt,
	}

	e.writeJSON(w, dto)
}

type changePasswordRequest struct {
	Password string `json:"password"`
}

func (e *Env) handleAdminUserPassword(w http.ResponseWriter, r *http.Request, user *domain.User) {
	defer r.Body.Close()

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Password) == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}

	// В демо просто сохраняем как есть
	user.Password = req.Password

	e.writeJSON(w, map[string]string{
		"status": "ok",
	})
}
