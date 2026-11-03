package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
)

func artDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, argentinaTZ)
}

func dueDays(obs []obligation) []string {
	out := make([]string, 0, len(obs))
	for _, o := range obs {
		out = append(out, o.DueDate.In(argentinaTZ).Format("2006-01-02"))
	}
	return out
}

func TestObligationDueDate_MatchesInstallmentEngine(t *testing.T) {
	// El motor encadena: #2 = alta + 1 mes, #k = vencimiento de #k-1 + 1 mes.
	// Con un alta a fin de mes, AddDate normaliza (31/1 + 1 mes = 3/3) y la
	// cadena no coincide con alta + (k-1) meses: hay que reproducirla igual.
	start := time.Date(2026, 1, 31, 15, 0, 0, 0, time.UTC)
	tu := &dbs.TeamUser{AssignmentDate: start, InitAmount: 1000}
	paid := &dbs.Installment{InstallmentNumber: 1}
	for k := 2; k <= 5; k++ {
		next := NextTeamInstallment(tu, paid)
		require.NotNil(t, next.DueDate)
		assert.Equal(t, *next.DueDate, obligationDueDate(start, k), "cuota %d", k)
		paid = next
	}
	assert.Equal(t, start, obligationDueDate(start, 1), "la #1 vence en el alta")
}

func TestObligationsUntil(t *testing.T) {
	tests := []struct {
		name  string
		cycle obligationCycle
		until time.Time
		want  []string
	}{
		{"sin cuotas pagas: la #1 vence en el alta", obligationCycle{Start: artDate(2026, 9, 27), Amount: 15000, Paid: 0},
			artDate(2026, 11, 30), []string{"2026-09-27", "2026-10-27", "2026-11-27"}},
		{"al día: la próxima es futura", obligationCycle{Start: artDate(2026, 4, 6), Amount: 20000, Paid: 7},
			artDate(2026, 11, 30), []string{"2026-11-06"}},
		{"dejó de pagar: varias pendientes", obligationCycle{Start: artDate(2026, 9, 6), Amount: 1500, Paid: 1},
			artDate(2026, 11, 30), []string{"2026-10-06", "2026-11-06"}},
		{"nada pendiente hasta el corte", obligationCycle{Start: artDate(2026, 9, 29), Amount: 40000, Paid: 2},
			artDate(2026, 11, 28), nil},
		{"monto 0 no genera obligaciones", obligationCycle{Start: artDate(2026, 9, 6), Amount: 0, Paid: 0},
			artDate(2026, 11, 30), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := obligationsUntil(tt.cycle, tt.until)
			if tt.want == nil {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tt.want, dueDays(got))
			assert.Equal(t, tt.cycle.Paid+1, got[0].Number)
			assert.Equal(t, tt.cycle.Amount, got[0].Amount)
		})
	}
}

func TestNextObligation(t *testing.T) {
	ob, ok := nextObligation(obligationCycle{Start: artDate(2026, 4, 6), Amount: 20000, Paid: 7})
	require.True(t, ok)
	assert.Equal(t, 8, ob.Number)
	assert.Equal(t, "2026-11-06", ob.DueDate.In(argentinaTZ).Format("2006-01-02"))

	_, ok = nextObligation(obligationCycle{Start: artDate(2026, 4, 6), Amount: 0, Paid: 0})
	assert.False(t, ok, "sin monto no hay próxima cuota")
}

func TestIsOverdue_ComparesArgentinaDays(t *testing.T) {
	today := artDate(2026, 11, 2)
	tests := []struct {
		name string
		due  time.Time
		want bool
	}{
		{"venció ayer", artDate(2026, 11, 1), true},
		{"vence hoy no está vencida", artDate(2026, 11, 2), false},
		{"vence mañana", artDate(2026, 11, 3), false},
		// 2/11 01:00 UTC es todavía 1/11 en Argentina: ya venció.
		{"borde de huso", time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isOverdue(tt.due, today))
		})
	}
}

func TestCountOverdue(t *testing.T) {
	// Alta 10/8, 1 cuota pagada, hoy 2/11: vencieron la 2 (10/9) y la 3 (10/10).
	c := obligationCycle{Start: artDate(2026, 8, 10), Amount: 1000, Paid: 1}
	assert.Equal(t, 2, countOverdue(c, artDate(2026, 11, 2)))
	assert.Equal(t, 0, countOverdue(obligationCycle{Start: artDate(2026, 10, 30), Amount: 1000, Paid: 1}, artDate(2026, 11, 2)))
}

func TestMembershipDebt(t *testing.T) {
	now := artDate(2026, 11, 2)
	// El caso de "Los cracks": alta el 22/08 a $12.000 sin ninguna cuota paga.
	unpaid := &dbs.TeamUser{AssignmentDate: artDate(2026, 8, 22), InitAmount: 12000}
	assert.Equal(t, []string{"2026-08-22", "2026-09-22", "2026-10-22"}, dueDays(membershipDebt(unpaid, now)))
	assert.Equal(t, 3, membershipOverdue(unpaid, now))

	// La #1 vence en el alta: alguien que se unió hoy ya la debe, pero no está vencida.
	joinedToday := &dbs.TeamUser{AssignmentDate: artDate(2026, 11, 2), InitAmount: 12000}
	assert.Len(t, membershipDebt(joinedToday, now), 1)
	assert.Equal(t, 0, membershipOverdue(joinedToday, now))

	// Al día: la próxima vence el 22/11.
	upToDate := &dbs.TeamUser{AssignmentDate: artDate(2026, 8, 22), InitAmount: 12000, PaidInstallments: 3}
	assert.Empty(t, membershipDebt(upToDate, now))

	// Sin monto, sin alta o sin membresía no hay deuda.
	assert.Empty(t, membershipDebt(&dbs.TeamUser{AssignmentDate: artDate(2026, 8, 22)}, now))
	assert.Empty(t, membershipDebt(&dbs.TeamUser{InitAmount: 12000}, now))
	assert.Empty(t, membershipDebt(nil, now))
	assert.Equal(t, 0, membershipOverdue(&dbs.TeamUser{InitAmount: 12000}, now))
	assert.Equal(t, 0, membershipOverdue(nil, now))
}
