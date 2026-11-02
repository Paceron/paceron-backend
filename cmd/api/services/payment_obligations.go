package services

import "time"

// Obligaciones de pago calculadas (change cobros-pendientes-y-vencimientos,
// design D3). Las cuotas se crean de a una: la N+1 recién existe cuando se paga
// la N, así que lo que un corredor todavía debe no está en ninguna tabla. Acá se
// deriva a partir del alta, el monto y las cuotas pagadas, con las mismas
// fechas que usa el motor de cuotas (installment_engine.go).

// obligationCycle es el ciclo de una membresía de equipo o de una suscripción
// de tier: alta, monto de cada cuota y cuántas cuotas se pagaron.
type obligationCycle struct {
	Start  time.Time
	Amount float64
	Paid   int
}

// obligation es una cuota esperada que todavía no se pagó.
type obligation struct {
	Number  int
	DueDate time.Time
	Amount  float64
}

// maxObligations acota el cálculo: 10 años de cuotas mensuales.
const maxObligations = 120

// obligationDueDate es el vencimiento de la cuota k, con la misma cadena que
// BuildNextInstallment: la #1 vence en el alta, la #2 un mes después y cada una
// de las siguientes un mes después de la anterior. Se encadena en vez de sumar
// (k-1) meses al alta porque AddDate normaliza las fechas de fin de mes
// (31/1 + 1 mes = 3/3), y así queda igual a la cuota que crea el webhook.
func obligationDueDate(start time.Time, k int) time.Time {
	due := start
	for i := 2; i <= k; i++ {
		due = due.AddDate(0, 1, 0)
	}
	return due
}

// nextObligation es la próxima cuota sin pagar de un ciclo, o false si el ciclo
// no tiene monto.
func nextObligation(c obligationCycle) (obligation, bool) {
	if c.Amount <= 0 {
		return obligation{}, false
	}
	k := c.Paid + 1
	return obligation{Number: k, DueDate: obligationDueDate(c.Start, k), Amount: c.Amount}, true
}

// obligationsUntil devuelve las cuotas sin pagar que vencen hasta el día de
// until inclusive, en hora argentina.
func obligationsUntil(c obligationCycle, until time.Time) []obligation {
	if c.Amount <= 0 {
		return nil
	}
	limit := argentinaDay(until)
	var out []obligation
	due := obligationDueDate(c.Start, c.Paid+1)
	for k := c.Paid + 1; k <= c.Paid+maxObligations; k++ {
		if k > c.Paid+1 {
			due = due.AddDate(0, 1, 0)
		}
		if argentinaDay(due).After(limit) {
			break
		}
		out = append(out, obligation{Number: k, DueDate: due, Amount: c.Amount})
	}
	return out
}

// isOverdue indica si una cuota con ese vencimiento ya venció hoy: el día del
// vencimiento es anterior a hoy, en hora argentina. Vencer hoy no es estar
// vencida (design D4).
func isOverdue(due, today time.Time) bool {
	return argentinaDay(due).Before(argentinaDay(today))
}

// countOverdue cuenta las cuotas sin pagar de un ciclo que ya vencieron.
func countOverdue(c obligationCycle, today time.Time) int {
	n := 0
	for _, o := range obligationsUntil(c, argentinaDay(today).AddDate(0, 0, -1)) {
		if isOverdue(o.DueDate, today) {
			n++
		}
	}
	return n
}

// argentinaDay es la medianoche, en hora argentina, del día de t.
func argentinaDay(t time.Time) time.Time {
	a := t.In(argentinaTZ)
	return time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, argentinaTZ)
}
