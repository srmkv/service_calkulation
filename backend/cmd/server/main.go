package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"

    _ "github.com/lib/pq"

    "saas-calc-backend/internal/app"
)

func main() {
    // строку подключения берём из env
    dsn := os.Getenv("DATABASE_URL")
    if dsn == "" {
        // пример:
        // postgres://user:pass@localhost:5432/saas_calc?sslmode=disable
        dsn = "postgres://saas:saas@localhost:5432/saas_calc?sslmode=disable"
    }

    db, err := sql.Open("postgres", dsn)
    if err != nil {
        log.Fatalf("open db: %v", err)
    }
    if err := db.Ping(); err != nil {
        log.Fatalf("ping db: %v", err)
    }
    log.Println("DB connected")

    a := app.New(db) // <-- передаём db в приложение

    addr := ":3040"
    if p := os.Getenv("PORT"); p != "" {
        addr = ":" + p
    }

    log.Printf("Server listening on %s\n", addr)
    if err := http.ListenAndServe(addr, a.Router()); err != nil {
        log.Fatal(err)
    }
}
