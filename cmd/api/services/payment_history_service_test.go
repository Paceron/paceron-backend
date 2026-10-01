package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
)

type mockPaymentHistoryDao struct {
	listReceivedFn       func(ctx *gin.Context, sellerID int64, filters daos.ReceivedPaymentFilters, page, pageSize int) ([]daos.ReceivedPaymentRow, bool, error)
	listReceivedBetweenFn func(ctx *gin.Context, sellerID int64, from, to time.Time) ([]daos.ReceivedPaymentRow, error)
	earliestReceivedAtFn  func(ctx *gin.Context, sellerID int64) (*time.Time, error)
	listHistoryFn        func(ctx *gin.Context, userID int64, filters daos.HistoryPaymentFilters, page, pageSize int) ([]daos.HistoryPaymentRow, bool, error)
}

func (m *mockPaymentHistoryDao) ListReceived(ctx *gin.Context, sellerID int64, filters daos.ReceivedPaymentFilters, page, pageSize int) ([]daos.ReceivedPaymentRow, bool, error) {
	if m.listReceivedFn != nil {
		return m.listReceivedFn(ctx, sellerID, filters, page, pageSize)
	}
	return nil, false, nil
}

func (m *mockPaymentHistoryDao) ListReceivedBetween(ctx *gin.Context, sellerID int64, from, to time.Time) ([]daos.ReceivedPaymentRow, error) {
	if m.listReceivedBetweenFn != nil {
		return m.listReceivedBetweenFn(ctx, sellerID, from, to)
	}
	return nil, nil
}

func (m *mockPaymentHistoryDao) EarliestReceivedAt(ctx *gin.Context, sellerID int64) (*time.Time, error) {
	if m.earliestReceivedAtFn != nil {
		return m.earliestReceivedAtFn(ctx, sellerID)
	}
	return nil, nil
}

func (m *mockPaymentHistoryDao) ListHistory(ctx *gin.Context, userID int64, filters daos.HistoryPaymentFilters, page, pageSize int) ([]daos.HistoryPaymentRow, bool, error) {
	if m.listHistoryFn != nil {
		return m.listHistoryFn(ctx, userID, filters, page, pageSize)
	}
	return nil, false, nil
}

func floatPtr(f float64) *float64 { return &f }

func TestNewPaymentHistoryService(t *testing.T) {
	assert.NotNil(t, NewPaymentHistoryService(&mockPaymentHistoryDao{}))
}

// --- ListReceived ---

func TestPaymentHistoryService_ListReceived_InvalidPage(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{})
	_, err := svc.ListReceived(nil, 7, nil, "", 0)
	assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}

func TestPaymentHistoryService_ListReceived_InvalidStatus(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{})
	_, err := svc.ListReceived(nil, 7, nil, "other", 1)
	assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}

func TestPaymentHistoryService_ListReceived_MapsRowsAndFilters(t *testing.T) {
	teamID := int64(12)
	created := time.Date(2026, 9, 14, 13, 22, 5, 0, time.FixedZone("ART", -3*3600))
	dao := &mockPaymentHistoryDao{listReceivedFn: func(ctx *gin.Context, sellerID int64, f daos.ReceivedPaymentFilters, page, pageSize int) ([]daos.ReceivedPaymentRow, bool, error) {
		assert.Equal(t, int64(7), sellerID)
		assert.Equal(t, &teamID, f.TeamID)
		assert.ElementsMatch(t, []string{"rejected", "cancelled"}, f.Statuses)
		assert.Equal(t, 2, page)
		assert.Equal(t, 20, pageSize)
		return []daos.ReceivedPaymentRow{
			{ID: 812, MPPaymentID: "131", Status: "cancelled", GrossAmount: 15000.005, NetAmount: floatPtr(14101.456),
				CurrencyID: "ARS", CreatedAt: created, InstallmentID: 301, InstallmentNumber: 3,
				TeamID: 12, TeamName: strPtr("Runners"), PayerID: 45, PayerName: strPtr("Lucía"), PayerSurname: strPtr("Gómez"), PayerEmail: strPtr("l@g.com")},
			{ID: 813, Status: "approved", TeamID: 13, PayerID: 46},
		}, true, nil
	}}
	svc := NewPaymentHistoryService(dao)

	resp, err := svc.ListReceived(nil, 7, &teamID, "rejected", 2)

	require.NoError(t, err)
	assert.True(t, resp.HasMore)
	require.Len(t, resp.Payments, 2)
	p := resp.Payments[0]
	assert.Equal(t, "rejected", p.StatusGroup)
	assert.Equal(t, 15000.01, p.GrossAmount)
	require.NotNil(t, p.NetAmount)
	assert.Equal(t, 14101.46, *p.NetAmount)
	assert.Equal(t, "2026-09-14T16:22:05Z", p.CreatedAt)
	assert.Equal(t, "Runners", p.Team.Name)
	assert.Equal(t, "Gómez", p.Payer.Surname)
	// Refs nulas (equipo o usuario borrado) quedan como strings vacíos.
	assert.Nil(t, resp.Payments[1].NetAmount)
	assert.Equal(t, "", resp.Payments[1].Team.Name)
	assert.Equal(t, "", resp.Payments[1].Payer.Email)
}

func TestPaymentHistoryService_ListReceived_EmptyIsNotNil(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{})
	resp, err := svc.ListReceived(nil, 7, nil, "", 1)
	require.NoError(t, err)
	assert.NotNil(t, resp.Payments)
	assert.Empty(t, resp.Payments)
}

func TestPaymentHistoryService_ListReceived_DaoError(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{listReceivedFn: func(*gin.Context, int64, daos.ReceivedPaymentFilters, int, int) ([]daos.ReceivedPaymentRow, bool, error) {
		return nil, false, errors.New("db caída")
	}})
	_, err := svc.ListReceived(nil, 7, nil, "", 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}

// --- ListHistory ---


func TestPaymentHistoryService_ListHistory_InvalidParams(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{})
	_, err := svc.ListHistory(nil, 7, "", "", 0)
	assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
	_, err = svc.ListHistory(nil, 7, "order", "", 1)
	assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
	_, err = svc.ListHistory(nil, 7, "", "other", 1)
	assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}

func TestPaymentHistoryService_ListHistory_MapsBothTypes(t *testing.T) {
	due := time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC)
	dao := &mockPaymentHistoryDao{listHistoryFn: func(ctx *gin.Context, userID int64, f daos.HistoryPaymentFilters, page, pageSize int) ([]daos.HistoryPaymentRow, bool, error) {
		assert.Equal(t, int64(7), userID)
		assert.Equal(t, "trainer_payment", f.Type)
		assert.ElementsMatch(t, []string{"approved"}, f.Statuses)
		assert.Equal(t, 20, pageSize)
		return []daos.HistoryPaymentRow{
			{ID: 790, Status: "approved", Amount: 9999, PaymentMethodID: "master", InstallmentNumber: 2, DueDate: &due,
				SubscriptionID: int64Ptr(55), TierID: int64Ptr(4), TierName: strPtr("Premium_entrenador"), TierRoleName: strPtr("entrenador")},
			{ID: 791, Status: "in_process", Amount: 15000, InstallmentNumber: 1,
				TeamID: int64Ptr(12), TeamName: strPtr("Runners"), TrainerID: int64Ptr(3), TrainerName: strPtr("Pepa"), TrainerSurname: strPtr("Lota")},
			{ID: 792, Status: "approved", Amount: 100, SubscriptionID: int64Ptr(56)},
			{ID: 793, Status: "approved", Amount: 100},
		}, true, nil
	}}

	resp, err := NewPaymentHistoryService(dao).ListHistory(nil, 7, "trainer_payment", "approved", 1)

	require.NoError(t, err)
	assert.True(t, resp.HasMore)
	require.Len(t, resp.Payments, 4)

	tier := resp.Payments[0]
	assert.Equal(t, "subscription", tier.Type)
	assert.Equal(t, "master", tier.PaymentMethodID)
	require.NotNil(t, tier.Tier)
	assert.Equal(t, "Premium_entrenador", tier.Tier.Name)
	require.NotNil(t, tier.DueDate)
	assert.Equal(t, "2026-09-05T03:00:00Z", *tier.DueDate)
	assert.Nil(t, tier.Team)
	assert.Nil(t, tier.Trainer)

	team := resp.Payments[1]
	assert.Equal(t, "trainer_payment", team.Type)
	assert.Equal(t, "pending", team.StatusGroup)
	require.NotNil(t, team.Team)
	assert.Equal(t, "Runners", team.Team.Name)
	require.NotNil(t, team.Trainer)
	assert.Equal(t, "Lota", team.Trainer.Surname)
	assert.Nil(t, team.Tier)
	assert.Nil(t, team.DueDate)

	// Referencias borradas: tipo resuelto igual, refs en null.
	assert.Equal(t, "subscription", resp.Payments[2].Type)
	assert.Nil(t, resp.Payments[2].Tier)
	assert.Equal(t, "trainer_payment", resp.Payments[3].Type)
	assert.Nil(t, resp.Payments[3].Team)
	assert.Nil(t, resp.Payments[3].Trainer)
}

func TestPaymentHistoryService_ListHistory_EmptyIsNotNil(t *testing.T) {
	resp, err := NewPaymentHistoryService(&mockPaymentHistoryDao{}).ListHistory(nil, 7, "", "", 1)
	require.NoError(t, err)
	assert.NotNil(t, resp.Payments)
	assert.Empty(t, resp.Payments)
}

func TestPaymentHistoryService_ListHistory_DaoError(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{listHistoryFn: func(*gin.Context, int64, daos.HistoryPaymentFilters, int, int) ([]daos.HistoryPaymentRow, bool, error) {
		return nil, false, errors.New("db caída")
	}})
	_, err := svc.ListHistory(nil, 7, "", "", 1)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}

// --- GetReceivedSummary ---

// summaryNow es un sábado 26/09/2026 a las 15:00 en Argentina.
var summaryNow = time.Date(2026, 9, 26, 18, 0, 0, 0, time.UTC)

func newSummaryService(dao *mockPaymentHistoryDao) *paymentHistoryService {
	return &paymentHistoryService{dao: dao, now: func() time.Time { return summaryNow }}
}

func receivedRow(id, inst, team int64, teamName, status string, gross float64, net *float64, at time.Time) daos.ReceivedPaymentRow {
	return daos.ReceivedPaymentRow{ID: id, InstallmentID: inst, TeamID: team, TeamName: strPtr(teamName),
		Status: status, GrossAmount: gross, NetAmount: net, CurrencyID: "ARS", CreatedAt: at}
}

func TestPaymentHistoryService_GetReceivedSummary_InvalidMonths(t *testing.T) {
	svc := newSummaryService(&mockPaymentHistoryDao{})
	for _, m := range []int{0, 1, 13} {
		_, err := svc.GetReceivedSummary(nil, 7, m, "")
		assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery, m)
	}
}

func TestPaymentHistoryService_GetReceivedSummary_EmptyWindow(t *testing.T) {
	var since, until time.Time
	svc := newSummaryService(&mockPaymentHistoryDao{listReceivedBetweenFn: func(ctx *gin.Context, sellerID int64, f, to time.Time) ([]daos.ReceivedPaymentRow, error) {
		since, until = f, to
		return nil, nil
	}})

	resp, err := svc.GetReceivedSummary(nil, 7, 6, "")

	require.NoError(t, err)
	// 1/4/2026 00:00 en Argentina.
	assert.True(t, since.Equal(time.Date(2026, 4, 1, 3, 0, 0, 0, time.UTC)), since)
	// Fin exclusivo: 1/10/2026 00:00 en Argentina.
	assert.True(t, until.Equal(time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)), until)
	assert.Nil(t, resp.EarliestMonth)
	assert.Equal(t, "ARS", resp.CurrencyID)
	assert.Equal(t, 6, resp.Months)
	require.Len(t, resp.Monthly, 6)
	months := []string{}
	for _, m := range resp.Monthly {
		months = append(months, m.Month)
		assert.Equal(t, float64(0), m.GrossAmount)
		assert.Nil(t, m.NetAmount)
	}
	assert.Equal(t, []string{"2026-04", "2026-05", "2026-06", "2026-07", "2026-08", "2026-09"}, months)
	assert.NotNil(t, resp.ByTeam)
	assert.Empty(t, resp.ByTeam)
	assert.Equal(t, "2026-09-26T18:00:00Z", resp.GeneratedAt)
}

func TestPaymentHistoryService_GetReceivedSummary_Aggregates(t *testing.T) {
	sep10 := time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC)
	// 01/09 a las 02:00 UTC es el 31/08 a las 23:00 en Argentina.
	borde := time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC)
	rows := []daos.ReceivedPaymentRow{
		receivedRow(1, 101, 12, "Runners", "approved", 15000, floatPtr(14000), sep10),
		// Cuota 102: rechazo seguido de un aprobado -> no cuenta como rechazada.
		receivedRow(2, 102, 12, "Runners", "rejected", 15000, nil, sep10.Add(-time.Hour)),
		receivedRow(3, 102, 12, "Runners", "approved", 15000, nil, sep10.Add(time.Hour)),
		receivedRow(4, 103, 12, "Runners", "approved", 10000, nil, borde),
		// Cuota 104: el último intento está en proceso -> pendiente.
		receivedRow(5, 104, 12, "Runners", "rejected", 15000, nil, sep10),
		receivedRow(6, 104, 12, "Runners", "in_process", 15000, nil, sep10.Add(2*time.Hour)),
		// Cuota 105: dos rechazos -> una sola rechazada.
		receivedRow(7, 105, 18, "Trail", "rejected", 30000, nil, sep10),
		receivedRow(8, 105, 18, "Trail", "cancelled", 30000, nil, sep10.Add(time.Hour)),
		receivedRow(9, 106, 18, "Trail", "approved", 30000, floatPtr(28000.004), sep10),
		// Reembolsado: no suma ni cuenta.
		receivedRow(10, 107, 18, "Trail", "refunded", 30000, nil, sep10),
	}
	svc := newSummaryService(&mockPaymentHistoryDao{listReceivedBetweenFn: func(*gin.Context, int64, time.Time, time.Time) ([]daos.ReceivedPaymentRow, error) {
		return rows, nil
	}})

	resp, err := svc.GetReceivedSummary(nil, 7, 6, "")

	require.NoError(t, err)
	sep := resp.Monthly[5]
	assert.Equal(t, "2026-09", sep.Month)
	assert.Equal(t, float64(60000), sep.GrossAmount)
	assert.Equal(t, 3, sep.ApprovedCount)
	assert.Equal(t, 2, sep.NetKnownCount)
	require.NotNil(t, sep.NetAmount)
	assert.Equal(t, 42000.0, *sep.NetAmount)

	aug := resp.Monthly[4]
	assert.Equal(t, "2026-08", aug.Month)
	assert.Equal(t, float64(10000), aug.GrossAmount)
	assert.Nil(t, aug.NetAmount)

	assert.Equal(t, 1, resp.PendingCount)
	assert.Equal(t, 1, resp.RejectedCount)

	require.Len(t, resp.ByTeam, 2)
	runners, trail := resp.ByTeam[0], resp.ByTeam[1]
	assert.Equal(t, "Runners", runners.TeamName)
	assert.Equal(t, float64(40000), runners.GrossAmount)
	assert.Equal(t, 3, runners.ApprovedCount)
	assert.Equal(t, 1, runners.PendingCount)
	assert.Equal(t, 0, runners.RejectedCount)
	require.NotNil(t, runners.NetAmount)
	assert.Equal(t, 14000.0, *runners.NetAmount)
	assert.Equal(t, "Trail", trail.TeamName)
	assert.Equal(t, 1, trail.RejectedCount)
	require.NotNil(t, trail.NetAmount)
	assert.Equal(t, 28000.0, *trail.NetAmount)
}

func TestPaymentHistoryService_GetReceivedSummary_TeamOrderTiesAndOutOfWindow(t *testing.T) {
	at := time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC)
	rows := []daos.ReceivedPaymentRow{
		receivedRow(1, 201, 30, "Beta", "approved", 5000, nil, at),
		receivedRow(2, 202, 31, "Alfa", "approved", 5000, nil, at),
		receivedRow(3, 203, 29, "Alfa", "approved", 5000, nil, at),
		// Defensivo: una fila anterior a la ventana suma al equipo pero no a ningún mes.
		receivedRow(4, 204, 40, "Viejo", "approved", 1000, nil, time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)),
	}
	svc := newSummaryService(&mockPaymentHistoryDao{listReceivedBetweenFn: func(*gin.Context, int64, time.Time, time.Time) ([]daos.ReceivedPaymentRow, error) {
		return rows, nil
	}})

	resp, err := svc.GetReceivedSummary(nil, 7, 3, "")

	require.NoError(t, err)
	require.Len(t, resp.Monthly, 3)
	assert.Equal(t, "2026-07", resp.Monthly[0].Month)
	assert.Equal(t, float64(15000), resp.Monthly[2].GrossAmount)
	ids := []int64{}
	for _, tm := range resp.ByTeam {
		ids = append(ids, tm.TeamID)
	}
	assert.Equal(t, []int64{29, 31, 30, 40}, ids)
}

func TestPaymentHistoryService_GetReceivedSummary_LatestAttemptTieBreaksByID(t *testing.T) {
	at := time.Date(2026, 9, 10, 15, 0, 0, 0, time.UTC)
	rows := []daos.ReceivedPaymentRow{
		receivedRow(51, 301, 12, "Runners", "pending", 100, nil, at),
		receivedRow(50, 301, 12, "Runners", "rejected", 100, nil, at),
	}
	svc := newSummaryService(&mockPaymentHistoryDao{listReceivedBetweenFn: func(*gin.Context, int64, time.Time, time.Time) ([]daos.ReceivedPaymentRow, error) {
		return rows, nil
	}})

	resp, err := svc.GetReceivedSummary(nil, 7, 6, "")

	require.NoError(t, err)
	assert.Equal(t, 1, resp.PendingCount)
	assert.Equal(t, 0, resp.RejectedCount)
}

func TestPaymentHistoryService_GetReceivedSummary_DaoError(t *testing.T) {
	svc := newSummaryService(&mockPaymentHistoryDao{listReceivedBetweenFn: func(*gin.Context, int64, time.Time, time.Time) ([]daos.ReceivedPaymentRow, error) {
		return nil, errors.New("db caída")
	}})
	_, err := svc.GetReceivedSummary(nil, 7, 6, "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}

func TestPaymentHistoryService_GetReceivedSummary_Until(t *testing.T) {
	var from, to time.Time
	may := time.Date(2026, 5, 20, 15, 0, 0, 0, time.UTC)
	svc := newSummaryService(&mockPaymentHistoryDao{listReceivedBetweenFn: func(ctx *gin.Context, sellerID int64, f, tt time.Time) ([]daos.ReceivedPaymentRow, error) {
		from, to = f, tt
		return []daos.ReceivedPaymentRow{receivedRow(1, 401, 12, "Runners", "approved", 9000, nil, may)}, nil
	}})

	resp, err := svc.GetReceivedSummary(nil, 7, 6, "2026-05")

	require.NoError(t, err)
	assert.True(t, from.Equal(time.Date(2025, 12, 1, 3, 0, 0, 0, time.UTC)), from)
	assert.True(t, to.Equal(time.Date(2026, 6, 1, 3, 0, 0, 0, time.UTC)), to)
	require.Len(t, resp.Monthly, 6)
	assert.Equal(t, "2025-12", resp.Monthly[0].Month)
	assert.Equal(t, "2026-05", resp.Monthly[5].Month)
	assert.Equal(t, float64(9000), resp.Monthly[5].GrossAmount)
}

func TestPaymentHistoryService_GetReceivedSummary_UntilCurrentMonthIsValid(t *testing.T) {
	svc := newSummaryService(&mockPaymentHistoryDao{})
	resp, err := svc.GetReceivedSummary(nil, 7, 6, "2026-09")
	require.NoError(t, err)
	assert.Equal(t, "2026-09", resp.Monthly[5].Month)
}

func TestPaymentHistoryService_GetReceivedSummary_InvalidUntil(t *testing.T) {
	svc := newSummaryService(&mockPaymentHistoryDao{})
	for _, u := range []string{"mayo", "2026-13", "2026-5", "2026-10", "2099-01"} {
		_, err := svc.GetReceivedSummary(nil, 7, 6, u)
		assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery, u)
	}
}

func TestPaymentHistoryService_GetReceivedSummary_EarliestMonth(t *testing.T) {
	// 01/02 a la 01:00 UTC es el 31/01 en Argentina.
	first := time.Date(2026, 2, 1, 1, 0, 0, 0, time.UTC)
	svc := newSummaryService(&mockPaymentHistoryDao{earliestReceivedAtFn: func(*gin.Context, int64) (*time.Time, error) {
		return &first, nil
	}})

	resp, err := svc.GetReceivedSummary(nil, 7, 6, "")

	require.NoError(t, err)
	require.NotNil(t, resp.EarliestMonth)
	assert.Equal(t, "2026-01", *resp.EarliestMonth)
}

func TestPaymentHistoryService_GetReceivedSummary_EarliestError(t *testing.T) {
	svc := newSummaryService(&mockPaymentHistoryDao{earliestReceivedAtFn: func(*gin.Context, int64) (*time.Time, error) {
		return nil, errors.New("db caída")
	}})
	_, err := svc.GetReceivedSummary(nil, 7, 6, "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}
