package handlers

import (
    "encoding/json"
    "net/http"

    "saas-calc-backend/internal/domain"
)

// HandleLayeredConfig — конфиг послойного калькулятора.
//
// Доступен только если у текущего пользователя есть хотя бы один калькулятор
// типа layered (или он админ и в системе вообще есть layered-калькуляторы).
func (e *Env) HandleLayeredConfig(w http.ResponseWriter, r *http.Request) {
    u := e.CurrentUser(r)
    if u == nil {
        http.Error(w, "user not found", http.StatusUnauthorized)
        return
    }

    // проверяем наличие доступного layered-калькулятора
    hasLayered := false
    for _, c := range e.Calculators {
        if c.Type == domain.CalculatorTypeLayered &&
            (u.Role == domain.RoleAdmin || c.OwnerID == u.ID) {
            hasLayered = true
            break
        }
    }

    if !hasLayered {
        http.Error(w, "no layered calculator for this user", http.StatusForbidden)
        return
    }

    switch r.Method {
    case http.MethodGet:
        e.writeJSON(w, e.LayeredConfig)
    case http.MethodPost:
        defer r.Body.Close()

        var cfg domain.LayeredConfig
        if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
            http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
            return
        }

        e.LayeredConfig = &cfg
        e.writeJSON(w, e.LayeredConfig)

    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}
