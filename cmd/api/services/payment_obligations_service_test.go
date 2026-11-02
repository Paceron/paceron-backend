package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// "Hoy" de estos tests: 2026-11-02 al mediodía, hora argentina.
var obligationsNow = time.Date(2026, 11, 2, 12, 0, 0, 0, argentinaTZ)

func obligationsService(dao *mockPaymentHistoryDao, settingDao daos.PlatformSettingDaoInterface) *paymentHistoryService {
	svc := NewPaymentHistoryService(dao, settingDao).(*paymentHistoryService)
	svc.now = func() time.Time { return obligationsNow }
	return svc
}

func membership(teamID, userID int64, start time.Time, amount float64, paid int) daos.FeeMembershipRow {
	return daos.FeeMembershipRow{TeamID: teamID, TeamName: strPtr("Equipo"), UserID: userID, UserName: strPtr("Ana"), UserSurname: strPtr("Paz"),
		AssignmentDate: start, InitAmount: amount, PaidInstallments: paid}
}


func TestGetReceivedSummary_PendingObligations(t *testing.T) {
	dao := &mockPaymentHistoryDao{trainerMemberships: []daos.FeeMembershipRow{
		// Dejó de pagar: la 2 venció el 6/10 (octubre) y la 3 vence el 6/11.
		membership(20, 4, artDate(2026, 9, 6), 1500, 1),
		// La 2 venció ayer, 1/11: vencida del mes.
		membership(21, 5, artDate(2026, 10, 1), 2000, 1),
		// La 2 vence hoy: a vencer, no vencida.
		membership(21, 6, artDate(2026, 10, 2), 3000, 1),
		// Al día: la próxima es de diciembre, fuera del mes y de la ventana.
		membership(22, 7, artDate(2026, 11, 3), 4000, 1),
	}}
	resp, err := obligationsService(dao, nil).GetReceivedSummary(nil, 3, 6, "")

	require.NoError(t, err)
	byMonth := map[string]int{}
	for i, m := range resp.Monthly {
		byMonth[m.Month] = i
	}
	oct, nov := resp.Monthly[byMonth["2026-10"]], resp.Monthly[byMonth["2026-11"]]
	assert.Equal(t, 1500.0, oct.PendingAmount)
	assert.Equal(t, 1425.0, oct.PendingNetAmount, "neto con el 5% por defecto")
	assert.Equal(t, 1, oct.PendingCount)
	assert.Equal(t, 1500.0+2000+3000, nov.PendingAmount)
	assert.Equal(t, 3, nov.PendingCount)

	assert.Equal(t, 1, resp.OverdueThisMonth.Count)
	assert.Equal(t, 2000.0, resp.OverdueThisMonth.Amount)
	assert.Equal(t, int64(5), resp.OverdueThisMonth.Items[0].Payer.ID)
	assert.Equal(t, 2, resp.OverdueThisMonth.Items[0].InstallmentNumber)

	assert.Equal(t, 2, resp.DueThisMonth.Count, "vence hoy y vence el 6/11")
	assert.Equal(t, 4500.0, resp.DueThisMonth.Amount)
	assert.Equal(t, 4275.0, resp.DueThisMonth.NetAmount)
	require.Len(t, resp.DueThisMonth.Items, 2)
	assert.Equal(t, "2026-11-02T15:00:00Z", resp.DueThisMonth.Items[0].DueDate, "ordenadas por vencimiento")
	assert.Equal(t, int64(6), resp.DueThisMonth.Items[0].Payer.ID)
}

func TestGetReceivedSummary_PendingUsesCurrentFeePercent(t *testing.T) {
	dao := &mockPaymentHistoryDao{trainerMemberships: []daos.FeeMembershipRow{membership(20, 4, artDate(2026, 10, 1), 2000, 1)}}
	settings := &mockPlatformSettingDao{getFn: func(*gin.Context, string, float64) (float64, *dbs.PlatformSetting, error) {
		return 10, &dbs.PlatformSetting{Key: "marketplace_fee_percent"}, nil
	}}
	resp, err := obligationsService(dao, settings).GetReceivedSummary(nil, 3, 6, "")

	require.NoError(t, err)
	assert.Equal(t, 1800.0, resp.OverdueThisMonth.NetAmount)
}

func TestGetReceivedSummary_NoMembershipsKeepsEmptyBlocks(t *testing.T) {
	resp, err := obligationsService(&mockPaymentHistoryDao{}, nil).GetReceivedSummary(nil, 3, 6, "")

	require.NoError(t, err)
	assert.NotNil(t, resp.DueThisMonth.Items)
	assert.Empty(t, resp.DueThisMonth.Items)
	assert.NotNil(t, resp.OverdueThisMonth.Items)
}

func TestGetReceivedSummary_MembershipsError(t *testing.T) {
	_, err := obligationsService(&mockPaymentHistoryDao{obligationsErr: errors.New("db down")}, nil).GetReceivedSummary(nil, 3, 6, "")
	assert.Error(t, err)
}

func TestGetUpcoming(t *testing.T) {
	trainerID := int64(33)
	team := membership(32, 12, artDate(2026, 8, 10), 1000, 1)
	team.TrainerID, team.TrainerName, team.TrainerSurname = &trainerID, strPtr("Pep"), strPtr("Guardiola")
	dao := &mockPaymentHistoryDao{
		userMemberships: []daos.FeeMembershipRow{team, membership(40, 12, artDate(2026, 9, 1), 0, 0)},
		tierSubscriptions: []daos.TierSubscriptionRow{
			{ID: 55, TierID: 4, TierName: strPtr("premium_corredor"), TierRoleName: strPtr("corredor"), StartDate: artDate(2026, 10, 20), InitAmount: 50000, PaidInstallments: 1},
		},
	}
	resp, err := obligationsService(dao, nil).GetUpcoming(nil, 12)

	require.NoError(t, err)
	require.Len(t, resp.Items, 2, "la membresía sin cuota no aparece")
	team0 := resp.Items[0]
	assert.Equal(t, "trainer_payment", team0.Type)
	assert.Equal(t, 2, team0.InstallmentNumber)
	assert.Equal(t, "2026-09-10T15:00:00Z", team0.DueDate)
	assert.True(t, team0.Overdue)
	assert.Equal(t, 2, team0.OverdueCount, "vencieron la 2 (10/9) y la 3 (10/10)")
	require.NotNil(t, team0.Trainer)
	assert.Equal(t, "Guardiola", team0.Trainer.Surname)

	tier := resp.Items[1]
	assert.Equal(t, "subscription", tier.Type)
	assert.Equal(t, "premium_corredor", tier.Tier.Name)
	assert.Equal(t, 2, tier.InstallmentNumber)
	assert.False(t, tier.Overdue)
	assert.Equal(t, 0, tier.OverdueCount)
}

func TestGetUpcoming_Errors(t *testing.T) {
	_, err := obligationsService(&mockPaymentHistoryDao{obligationsErr: errors.New("db down")}, nil).GetUpcoming(nil, 12)
	assert.Error(t, err)
}

func TestCurrentFeePercent(t *testing.T) {
	svc := obligationsService(&mockPaymentHistoryDao{}, nil)
	assert.Equal(t, 5.0, svc.currentFeePercent(nil), "sin DAO de ajustes")

	svc = obligationsService(&mockPaymentHistoryDao{}, &mockPlatformSettingDao{getFn: func(*gin.Context, string, float64) (float64, *dbs.PlatformSetting, error) {
		return 0, nil, errors.New("db down")
	}})
	assert.Equal(t, 5.0, svc.currentFeePercent(nil), "si no se puede leer")

	svc = obligationsService(&mockPaymentHistoryDao{}, &mockPlatformSettingDao{getFn: func(*gin.Context, string, float64) (float64, *dbs.PlatformSetting, error) {
		return 5, nil, nil
	}})
	assert.Equal(t, 5.0, svc.currentFeePercent(nil), "sin fila guardada")
}
