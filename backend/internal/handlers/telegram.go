package handlers

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "net/http"
    "net/url"
    "strings"
    "time"
)

// sendTelegramMessage — низкоуровневый отправитель сообщений
func (e *Env) sendTelegramMessage(ctx context.Context, chatID, text string) {
    token := strings.TrimSpace(e.TelegramBotToken)
    chatID = strings.TrimSpace(chatID)

    if token == "" || chatID == "" {
        return
    }

    apiURL := "https://api.telegram.org/bot" + token + "/sendMessage"

    form := url.Values{}
    form.Set("chat_id", chatID)
    form.Set("text", text)
    form.Set("parse_mode", "HTML")

    req, err := http.NewRequestWithContext(
        ctx,
        http.MethodPost,
        apiURL,
        strings.NewReader(form.Encode()),
    )
    if err != nil {
        log.Printf("telegram: build request: %v", err)
        return
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("telegram: send error: %v", err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        log.Printf("telegram: non-OK status: %s", resp.Status)
    }
}

// ищем чат и метаданные калькулятора по ID
func (e *Env) lookupTelegramForCalc(ctx context.Context, calcID string) (chatID, calcName, calcType string, err error) {
    if e.DB == nil || calcID == "" {
        return "", "", "", nil
    }

    query := `
SELECT c.name, c.type, COALESCE(u.telegram_chat_id, '')
FROM calculators c
JOIN users u ON u.id = c.owner_id
WHERE c.id = $1
`
    var name, ctype, tgID string
    err = e.DB.QueryRowContext(ctx, query, calcID).Scan(&name, &ctype, &tgID)
    if err != nil {
        if err == sql.ErrNoRows {
            return "", "", "", nil
        }
        return "", "", "", err
    }

    tgID = strings.TrimSpace(tgID)
    return tgID, name, ctype, nil
}

// NotifyTelegramDistanceCalc — уведомление о новом расчёте доставки
func (e *Env) NotifyTelegramDistanceCalc(
    ctx context.Context,
    calcID string,
    from string,
    to string,
    vehicle string,
    roundTrip bool,
    distanceKm float64,
    totalPrice float64,
) {
    chatID, calcName, calcType, err := e.lookupTelegramForCalc(ctx, calcID)
    if err != nil {
        log.Printf("telegram: lookup failed for calc %s: %v", calcID, err)
        return
    }
    if chatID == "" {
        // у владельца нет Telegram-ID — просто выходим
        return
    }

    rt := "в одну сторону"
    if roundTrip {
        rt = "туда-обратно"
    }

    if calcName == "" {
        calcName = calcID
    }

    text := fmt.Sprintf(
        "📦 Новый расчёт по калькулятору «%s» (%s)\n\n"+
            "Откуда: %s\n"+
            "Куда: %s\n"+
            "Транспорт: %s\n"+
            "Маршрут: %s\n"+
            "Расстояние: %.1f км\n"+
            "Итого: %.0f ₽",
        calcName,
        calcType,
        from,
        to,
        vehicle,
        rt,
        distanceKm,
        totalPrice,
    )

    // отправляем асинхронно, чтобы не блокировать ответ API
    go e.sendTelegramMessage(ctx, chatID, text)
}
