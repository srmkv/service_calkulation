package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"

    "saas-calc-backend/internal/domain"
)

// AdminSettings — настройки, доступные администратору.
type AdminSettings struct {
    OSRMBaseURL      string `json:"osrmBaseUrl"`
    NominatimBaseURL string `json:"nominatimBaseUrl"`
    TelegramBotToken string `json:"telegramBotToken"`
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
        e.handleAdminSettingsGet(w, r)
    case http.MethodPost:
        e.handleAdminSettingsPost(w, r)
    default:
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
    }
}

func (e *Env) handleAdminSettingsGet(w http.ResponseWriter, r *http.Request) {
    // если БД нет — просто отдаём то, что лежит в Env
    if e.DB == nil {
        resp := AdminSettings{
            OSRMBaseURL:      e.OSRMBaseURL,
            NominatimBaseURL: e.NominatimBaseURL,
            TelegramBotToken: e.TelegramBotToken,
        }
        e.writeJSON(w, resp)
        return
    }

    row := e.DB.QueryRowContext(
        r.Context(),
        `SELECT osrm_base_url, nominatim_base_url, telegram_bot_token
         FROM settings
         WHERE id = 1`,
    )

    var osrm, nom, token sql.NullString
    err := row.Scan(&osrm, &nom, &token)
    if err != nil {
        if err != sql.ErrNoRows {
            http.Error(w, "failed to load settings: "+err.Error(), http.StatusInternalServerError)
            return
        }
        // если строки нет — просто пустые значения
    }

    resp := AdminSettings{
        OSRMBaseURL:      osrm.String,
        NominatimBaseURL: nom.String,
        TelegramBotToken: token.String,
    }

    // заодно синхронизируем Env (чтобы distance/telegram использовали актуальное)
    if resp.OSRMBaseURL != "" {
        e.OSRMBaseURL = resp.OSRMBaseURL
    }
    if resp.NominatimBaseURL != "" {
        e.NominatimBaseURL = resp.NominatimBaseURL
    }
    if resp.TelegramBotToken != "" {
        e.TelegramBotToken = resp.TelegramBotToken
    }

    e.writeJSON(w, resp)
}

func (e *Env) handleAdminSettingsPost(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req AdminSettings
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
        return
    }

    // сохраняем в БД, если она есть
    if e.DB != nil {
        _, err := e.DB.ExecContext(
            r.Context(),
            `INSERT INTO settings (id, osrm_base_url, nominatim_base_url, telegram_bot_token)
             VALUES (1, $1, $2, $3)
             ON CONFLICT (id) DO UPDATE
               SET osrm_base_url      = EXCLUDED.osrm_base_url,
                   nominatim_base_url = EXCLUDED.nominatim_base_url,
                   telegram_bot_token = EXCLUDED.telegram_bot_token`,
            req.OSRMBaseURL,
            req.NominatimBaseURL,
            req.TelegramBotToken,
        )
        if err != nil {
            http.Error(w, "failed to save settings: "+err.Error(), http.StatusInternalServerError)
            return
        }
    }

    // обновляем Env, чтобы всё в рантайме брало свежие значения
    e.OSRMBaseURL = req.OSRMBaseURL
    e.NominatimBaseURL = req.NominatimBaseURL
    e.TelegramBotToken = req.TelegramBotToken

    e.writeJSON(w, map[string]interface{}{
        "status":           "ok",
        "osrmBaseUrl":      req.OSRMBaseURL,
        "nominatimBaseUrl": req.NominatimBaseURL,
        "telegramBotToken": req.TelegramBotToken,
    })
}
