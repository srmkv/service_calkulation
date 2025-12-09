package app

import (
    "net/http"

    "saas-calc-backend/internal/handlers"
)

func withCORS(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func registerRoutes(mux *http.ServeMux, env *handlers.Env) {
    // --- API ---
    mux.Handle("/api/layers/config", withCORS(http.HandlerFunc(env.HandleLayeredConfig)))
    mux.Handle("/api/calculators", withCORS(http.HandlerFunc(env.HandleCalculators)))
    mux.Handle("/api/me", withCORS(http.HandlerFunc(env.HandleMe)))
    mux.Handle("/api/me/plan", withCORS(http.HandlerFunc(env.HandleMePlan)))

    // админские пользователи
    mux.Handle("/api/admin/users", withCORS(http.HandlerFunc(env.HandleAdminUsers)))
    mux.Handle("/api/admin/users/", withCORS(http.HandlerFunc(env.HandleAdminUserDetail)))
    // настройки для администратора (ключи и т.п.)
    mux.Handle("/api/admin/settings", withCORS(http.HandlerFunc(env.HandleAdminSettings)))
    // конфиг калькулятора расстояний
    mux.Handle("/api/distance/config", withCORS(http.HandlerFunc(env.HandleDistanceConfig)))
    // НОВОЕ: расчёт расстояния
    mux.Handle("/api/distance/calc", withCORS(http.HandlerFunc(env.HandleDistanceCalc)))
    // загрузка файлов (картинки для слоёв)
    mux.Handle("/api/upload", withCORS(http.HandlerFunc(env.HandleUpload)))

    // публичные калькуляторы
    mux.Handle("/p/", http.HandlerFunc(env.HandlePublicCalculatorPage))

    // --- Статика и страницы ---

    // всё, что лежит в ../frontend
    fileServer := http.FileServer(http.Dir("../frontend"))

    // корневые файлы статики (к которым обращается index.html / landing.html)
    // /styles.css  -> ../frontend/styles.css
    // /app.js      -> ../frontend/app.js
    // /img/...     -> ../frontend/img/...
    // /uploads/... -> ../frontend/uploads/...
    mux.Handle("/styles.css", fileServer)
    mux.Handle("/app.js", fileServer)
    mux.Handle("/img/", fileServer)
    mux.Handle("/uploads/", fileServer)
    // если есть favicon
    mux.Handle("/favicon.ico", fileServer)

    // Личный кабинет на /app и /app/
    mux.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
        // /app и /app/ -> просто index.html
        if r.URL.Path == "/app" || r.URL.Path == "/app/" {
            http.ServeFile(w, r, "../frontend/index.html")
            return
        }
        // /app/что-то -> файлы из ../frontend/...
        http.StripPrefix("/app/", fileServer).ServeHTTP(w, r)
    })

    // Лендинг на /
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" {
            // всё, что не "/", и не попало в хендлеры выше — 404
            http.NotFound(w, r)
            return
        }
        http.ServeFile(w, r, "../frontend/landing.html")
    })
}
