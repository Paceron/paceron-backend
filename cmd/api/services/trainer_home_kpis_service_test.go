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

type mockTrainerHomeKpisDao struct {
	sessions  daos.TrainerHomeSessionCounts
	structure daos.TrainerHomeStructureCounts
	err       error
	from      time.Time
	to        time.Time
}

func (m *mockTrainerHomeKpisDao) SessionCounts(_ *gin.Context, _ int64, from, to time.Time) (daos.TrainerHomeSessionCounts, error) {
	m.from, m.to = from, to
	return m.sessions, m.err
}

func (m *mockTrainerHomeKpisDao) StructureCounts(*gin.Context, int64) (daos.TrainerHomeStructureCounts, error) {
	return m.structure, m.err
}

func TestTrainerHomeKpisService_GetAggregatesPeriodAndUsesExistingPaymentRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dao := &mockTrainerHomeKpisDao{
		sessions:  daos.TrainerHomeSessionCounts{Scheduled: 18, Realized: 12, PresencialScheduled: 8, PresencialRealized: 6},
		structure: daos.TrainerHomeStructureCounts{ActiveTeams: 2, ActiveRunners: 17},
	}
	payments := &mockPaymentHistoryDao{
		listReceivedBetweenFn: func(_ *gin.Context, sellerID int64, from, to time.Time) ([]daos.ReceivedPaymentRow, error) {
			require.EqualValues(t, 44, sellerID)
			assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, argentinaTZ), from)
			assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, argentinaTZ), to)
			return []daos.ReceivedPaymentRow{
				{Status: "approved", CurrencyID: "ARS", GrossAmount: 100, FeeAmount: 5, NetAmount: floatPtr(95)},
				{Status: "approved", CurrencyID: "ARS", GrossAmount: 200, FeeAmount: 10, NetAmount: floatPtr(190)},
				{Status: "pending", CurrencyID: "ARS", GrossAmount: 1000, FeeAmount: 50},
			}, nil
		},
		trainerMemberships: []daos.FeeMembershipRow{
			// October's cuota is outside the selected period; the next due is Nov 10.
			{AssignmentDate: time.Date(2026, 10, 10, 12, 0, 0, 0, argentinaTZ), InitAmount: 100, PaidInstallments: 0},
			// Paid first installment means the next obligation falls outside November.
			{AssignmentDate: time.Date(2026, 11, 5, 12, 0, 0, 0, argentinaTZ), InitAmount: 50, PaidInstallments: 1},
			// This one is due in November.
			{AssignmentDate: time.Date(2026, 11, 20, 12, 0, 0, 0, argentinaTZ), InitAmount: 75, PaidInstallments: 0},
		},
	}
	service := &trainerHomeKpisService{
		dao: dao, paymentHistoryDao: payments,
		now: func() time.Time { return time.Date(2026, 11, 5, 12, 0, 0, 0, argentinaTZ) },
	}

	response, err := service.Get(nil, 44, TrainerHomeKpisQuery{Period: "month", ReferenceDate: "2026-11-05"})

	require.NoError(t, err)
	assert.Equal(t, "month", response.Period)
	assert.Equal(t, "2026-11-01", response.From)
	assert.Equal(t, "2026-11-30", response.To)
	assert.Equal(t, 30, response.DaysConsidered)
	assert.Equal(t, daos.TrainerHomeSessionCounts{Scheduled: 18, Realized: 12, PresencialScheduled: 8, PresencialRealized: 6}, dao.sessions)
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), dao.from)
	assert.Equal(t, time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC), dao.to)
	assert.Equal(t, int64(2), response.Structure.ActiveTeams)
	assert.Equal(t, int64(17), response.Structure.ActiveRunners)
	assert.Equal(t, "ARS", response.Collections.CurrencyID)
	assert.Equal(t, 285.0, response.Collections.NetCollected)
	assert.Equal(t, 175.0, response.Collections.PendingAmount)
	assert.Equal(t, 2, response.Collections.PendingCount)
}

func TestTrainerHomeKpisService_GetDefaultsMonthAndValidatesQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newService := func() *trainerHomeKpisService {
		return &trainerHomeKpisService{
			dao: &mockTrainerHomeKpisDao{}, paymentHistoryDao: &mockPaymentHistoryDao{},
			now: func() time.Time { return time.Date(2026, 11, 5, 12, 0, 0, 0, argentinaTZ) },
		}
	}

	response, err := newService().Get(nil, 1, TrainerHomeKpisQuery{})
	require.NoError(t, err)
	assert.Equal(t, "month", response.Period)
	assert.Equal(t, "2026-11-01", response.From)
	assert.Equal(t, "2026-11-30", response.To)

	_, err = newService().Get(nil, 1, TrainerHomeKpisQuery{Period: "year"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardInvalidQuery)

	_, err = newService().Get(nil, 1, TrainerHomeKpisQuery{ReferenceDate: "2026-02-30"})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDashboardInvalidQuery)
}

func TestTrainerHomeKpisService_GetPropagatesDaoErrorsWithoutPartialResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dao := &mockTrainerHomeKpisDao{err: errors.New("database offline")}
	service := &trainerHomeKpisService{
		dao: dao, paymentHistoryDao: &mockPaymentHistoryDao{}, now: time.Now,
	}
	response, err := service.Get(nil, 1, TrainerHomeKpisQuery{Period: "day", ReferenceDate: "2026-11-05"})
	assert.Nil(t, response)
	assert.Error(t, err)
}
