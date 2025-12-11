package handlers

import (
	"encoding/json"
	"math"
	"net/http"
)

type MortgageCalcRequest struct {
	Amount       float64 `json:"amount"`       // сумма кредита
	Rate         float64 `json:"rate"`         // годовая ставка, %
	Years        int     `json:"years"`        // срок в годах
	CalculatorID string  `json:"calculatorId"` // ID калькулятора для счётчика
}

type MortgageCalcResponse struct {
	Monthly     float64 `json:"monthly"`     // ежемесячный платёж
	Total       float64 `json:"total"`       // общая сумма выплат
	Overpayment float64 `json:"overpayment"` // переплата
}

// POST /api/mortgage/calc
func (e *Env) HandleMortgageCalc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()

	var req MortgageCalcRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 || req.Years <= 0 || req.Rate < 0 {
		http.Error(w, "amount, years, rate must be > 0", http.StatusBadRequest)
		return
	}

	n := float64(req.Years * 12) // месяцев
	monthlyRate := req.Rate / 100.0 / 12.0

	var payment float64
	if monthlyRate == 0 {
		payment = req.Amount / n
	} else {
		// аннуитетная формула
		pow := math.Pow(1+monthlyRate, n)
		k := monthlyRate * pow / (pow - 1)
		payment = req.Amount * k
	}

	total := payment * n
	over := total - req.Amount

	// инкремент счётчика
	if req.CalculatorID != "" {
		e.IncrementCalcCount(req.CalculatorID)
	}

	resp := MortgageCalcResponse{
		Monthly:     math.Round(payment*100) / 100,
		Total:       math.Round(total*100) / 100,
		Overpayment: math.Round(over*100) / 100,
	}

	e.writeJSON(w, resp)
}
