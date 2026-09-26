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
	listReceivedSinceFn  func(ctx *gin.Context, sellerID int64, since time.Time) ([]daos.ReceivedPaymentRow, error)
	listMyTierPaymentsFn func(ctx *gin.Context, userID int64, roleName string, page, pageSize int) ([]daos.TierPaymentRow, bool, error)
}

func (m *mockPaymentHistoryDao) ListReceived(ctx *gin.Context, sellerID int64, filters daos.ReceivedPaymentFilters, page, pageSize int) ([]daos.ReceivedPaymentRow, bool, error) {
	if m.listReceivedFn != nil {
		return m.listReceivedFn(ctx, sellerID, filters, page, pageSize)
	}
	return nil, false, nil
}

func (m *mockPaymentHistoryDao) ListReceivedSince(ctx *gin.Context, sellerID int64, since time.Time) ([]daos.ReceivedPaymentRow, error) {
	if m.listReceivedSinceFn != nil {
		return m.listReceivedSinceFn(ctx, sellerID, since)
	}
	return nil, nil
}

func (m *mockPaymentHistoryDao) ListMyTierPayments(ctx *gin.Context, userID int64, roleName string, page, pageSize int) ([]daos.TierPaymentRow, bool, error) {
	if m.listMyTierPaymentsFn != nil {
		return m.listMyTierPaymentsFn(ctx, userID, roleName, page, pageSize)
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

// --- ListMyTierPayments ---

func TestPaymentHistoryService_ListMyTierPayments_InvalidPage(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{})
	_, err := svc.ListMyTierPayments(nil, 7, "entrenador", -1)
	assert.ErrorIs(t, err, ErrInvalidPaymentHistoryQuery)
}

func TestPaymentHistoryService_ListMyTierPayments_MapsRows(t *testing.T) {
	tierID := int64(4)
	due := time.Date(2026, 9, 5, 3, 0, 0, 0, time.UTC)
	dao := &mockPaymentHistoryDao{listMyTierPaymentsFn: func(ctx *gin.Context, userID int64, roleName string, page, pageSize int) ([]daos.TierPaymentRow, bool, error) {
		assert.Equal(t, "entrenador", roleName)
		return []daos.TierPaymentRow{
			{ID: 790, Status: "approved", Amount: 9999, InstallmentNumber: 2, DueDate: &due, SubscriptionID: 55,
				TierID: &tierID, TierName: strPtr("Premium_entrenador"), TierRoleName: strPtr("entrenador")},
			{ID: 791, Status: "in_process", Amount: 9999, InstallmentNumber: 1, SubscriptionID: 55},
		}, false, nil
	}}
	svc := NewPaymentHistoryService(dao)

	resp, err := svc.ListMyTierPayments(nil, 7, "entrenador", 1)

	require.NoError(t, err)
	assert.False(t, resp.HasMore)
	require.Len(t, resp.Payments, 2)
	first := resp.Payments[0]
	assert.Equal(t, "approved", first.StatusGroup)
	require.NotNil(t, first.DueDate)
	assert.Equal(t, "2026-09-05T03:00:00Z", *first.DueDate)
	require.NotNil(t, first.Tier)
	assert.Equal(t, "Premium_entrenador", first.Tier.Name)
	assert.Equal(t, "pending", resp.Payments[1].StatusGroup)
	assert.Nil(t, resp.Payments[1].DueDate)
	assert.Nil(t, resp.Payments[1].Tier)
}

func TestPaymentHistoryService_ListMyTierPayments_DaoError(t *testing.T) {
	svc := NewPaymentHistoryService(&mockPaymentHistoryDao{listMyTierPaymentsFn: func(*gin.Context, int64, string, int, int) ([]daos.TierPaymentRow, bool, error) {
		return nil, false, errors.New("db caída")
	}})
	_, err := svc.ListMyTierPayments(nil, 7, "", 1)
	require.Error(t, err)
}
