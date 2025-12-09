package handlers

// IncrementCalcCount увеличивает счётчик расчётов для калькулятора.
func (e *Env) IncrementCalcCount(calcID string) {
	if calcID == "" {
		return
	}
	for _, c := range e.Calculators {
		if c.ID == calcID {
			c.CalcCount++
			break
		}
	}
}
