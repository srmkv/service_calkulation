package main

import (
    "log"
    "net/http"
    "os"

    "saas-calc-backend/internal/app"
)

func main() {
    a := app.New()

    addr := ":3040"
    if p := os.Getenv("PORT"); p != "" {
        addr = ":" + p
    }

    log.Printf("Server listening on %s\n", addr)

    if err := http.ListenAndServe(addr, a.Router()); err != nil {
        log.Fatal(err)
    }
}
