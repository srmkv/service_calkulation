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

// читаем токен бота из БД (settings.id = 1), при ошибках — из Env
func (e *Env) loadTelegramBotToken(ctx context.Context) string {
    if e.DB == nil {
        return strings.TrimSpace(e.TelegramBotToken)
    }

    var token sql.NullString
    err := e.DB.QueryRowContext(
        ctx,
        `SELECT telegram_bot_token FROM settings WHERE id = 1`,
    ).Scan(&token)
    if err != nil {
        if err != sql.ErrNoRows {
            log.Printf("telegram: load token from db error: %v", err)
        }
        return strings.TrimSpace(e.TelegramBotToken)
    }

    return strings.TrimSpace(token.String)
}

// sendTelegramMessage — низкоуровневый отправитель сообщений
func (e *Env) sendTelegramMessage(ctx context.Context, chatID, text string) {
    chatID = strings.TrimSpace(chatID)
    token := e.loadTelegramBotToken(ctx)

    if token == "" || chatID == "" {
        log.Printf("telegram: skip send — empty bot token or chat id (token=%q, chatID=%q)", token, chatID)
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
        log.Printf("telegram: build request (token=%q): %v", token, err)
        return
    }
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        log.Printf("telegram: send error (token=%q): %v", token, err)
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        log.Printf("telegram: non-OK status %s (token=%q)", resp.Status, token)
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

    // ВАЖНО: не используем request-context, а отдельный фоновой контекст
    go func() {
        bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        e.sendTelegramMessage(bgCtx, chatID, text)
    }()
}

// NotifyTelegramMortgageCalc — уведомление о новом расчёте ипотеки
func (e *Env) NotifyTelegramMortgageCalc(
    ctx context.Context,
    calcID string,
    amount float64,
    rate float64,
    years int,
    monthly float64,
    total float64,
    overpayment float64,
) {
    chatID, calcName, calcType, err := e.lookupTelegramForCalc(ctx, calcID)
    if err != nil {
        log.Printf("telegram: lookup failed for calc %s: %v", calcID, err)
        return
    }
    if chatID == "" {
        return
    }

    if calcName == "" {
        calcName = calcID
    }

    text := fmt.Sprintf(
        "🏠 Новый расчёт ипотеки по калькулятору «%s» (%s)\n\n"+
            "Сумма кредита: %.0f ₽\n"+
            "Ставка: %.2f %% годовых\n"+
            "Срок: %d лет\n\n"+
            "Ежемесячный платёж: %.0f ₽\n"+
            "Всего выплат: %.0f ₽\n"+
            "Переплата: %.0f ₽",
        calcName,
        calcType,
        amount,
        rate,
        years,
        monthly,
        total,
        overpayment,
    )

    // тоже отправляем на фоне с независимым контекстом
    go func() {
        bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        e.sendTelegramMessage(bgCtx, chatID, text)
    }()
}
