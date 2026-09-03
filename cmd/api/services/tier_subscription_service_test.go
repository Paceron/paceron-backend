package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/tiersubscription"
)

type mockTierSubscriptionDao struct {
	createFn            func(ctx *gin.Context, sub *dbs.UserRoleTierSubscription) error
	findByIDFn          func(ctx *gin.Context, id int64) (*dbs.UserRoleTierSubscription, error)
	findActiveFn        func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error)
	findLatestFn        func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error)
	setEndedFn          func(ctx *gin.Context, id int64) error
	activateFn          func(ctx *gin.Context, id int64) error
	incrementPaidFn     func(ctx *gin.Context, id int64) error
}

func (m *mockTierSubscriptionDao) Create(ctx *gin.Context, sub *dbs.UserRoleTierSubscription) error {
	if m.createFn != nil {
		return m.createFn(ctx, sub)
	}
	sub.ID = 99
	return nil
}

func (m *mockTierSubscriptionDao) FindByID(ctx *gin.Context, id int64) (*dbs.UserRoleTierSubscription, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockTierSubscriptionDao) FindActiveByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
	if m.findActiveFn != nil {
		return m.findActiveFn(ctx, userID, roleID)
	}
	return nil, nil
}

func (m *mockTierSubscriptionDao) FindLatestByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
	if m.findLatestFn != nil {
		return m.findLatestFn(ctx, userID, roleID)
	}
	return nil, nil
}

func (m *mockTierSubscriptionDao) SetEnded(ctx *gin.Context, id int64) error {
	if m.setEndedFn != nil {
		return m.setEndedFn(ctx, id)
	}
	return nil
}

func (m *mockTierSubscriptionDao) Activate(ctx *gin.Context, id int64) error {
	if m.activateFn != nil {
		return m.activateFn(ctx, id)
	}
	return nil
}

func (m *mockTierSubscriptionDao) IncrementPaidInstallments(ctx *gin.Context, id int64) error {
	if m.incrementPaidFn != nil {
		return m.incrementPaidFn(ctx, id)
	}
	return nil
}

type mockInstallmentDao struct {
	createFn            func(ctx *gin.Context, ins *dbs.Installment) error
	findByIDFn          func(ctx *gin.Context, id int64) (*dbs.Installment, error)
	markPaidFn          func(ctx *gin.Context, id int64, internalID *int64, externalID *string) (bool, error)
	findPendingBySubFn  func(ctx *gin.Context, subscriptionID int64) ([]dbs.Installment, error)
	findNextFn          func(ctx *gin.Context, subscriptionID int64) (*dbs.Installment, error)
	findPendingByTeamFn func(ctx *gin.Context, teamID, userID int64) ([]dbs.Installment, error)
}

func (m *mockInstallmentDao) Create(ctx *gin.Context, ins *dbs.Installment) error {
	if m.createFn != nil {
		return m.createFn(ctx, ins)
	}
	ins.ID = int64(ins.InstallmentNumber) + 1000
	return nil
}

func (m *mockInstallmentDao) FindByID(ctx *gin.Context, id int64) (*dbs.Installment, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockInstallmentDao) MarkPaidConditional(ctx *gin.Context, id int64, internalID *int64, externalID *string) (bool, error) {
	if m.markPaidFn != nil {
		return m.markPaidFn(ctx, id, internalID, externalID)
	}
	return true, nil
}

func (m *mockInstallmentDao) FindPendingBySubscription(ctx *gin.Context, subscriptionID int64) ([]dbs.Installment, error) {
	if m.findPendingBySubFn != nil {
		return m.findPendingBySubFn(ctx, subscriptionID)
	}
	return nil, nil
}

func (m *mockInstallmentDao) FindNext(ctx *gin.Context, subscriptionID int64) (*dbs.Installment, error) {
	if m.findNextFn != nil {
		return m.findNextFn(ctx, subscriptionID)
	}
	return nil, nil
}

func (m *mockInstallmentDao) FindPendingByUserTeam(ctx *gin.Context, teamID, userID int64) ([]dbs.Installment, error) {
	if m.findPendingByTeamFn != nil {
		return m.findPendingByTeamFn(ctx, teamID, userID)
	}
	return nil, nil
}

func (m *mockInstallmentDao) FindNextByUserTeam(ctx *gin.Context, teamID, userID int64) (*dbs.Installment, error) {
	return nil, nil
}

func newTierSubscriptionService(ur *mockUserRoleDao, role *mockRoleDao, tier *mockTierDao, sub *mockTierSubscriptionDao, ins *mockInstallmentDao) TierSubscriptionServiceInterface {
	return NewTierSubscriptionService(nil, ur, role, tier, sub, ins)
}

func TestGetCurrentSubscription_PaidWithNextInstallment(t *testing.T) {
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID, TierID: 1}, nil
		},
	}
	roleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: id, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 2, Name: "premium", Hierarchy: 3, PaymentRequired: true}, nil
		},
	}
	due := time.Now().AddDate(0, 1, 0)
	blocked := due.AddDate(0, 0, 7)
	subDao := &mockTierSubscriptionDao{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{ID: 7, UserID: userID, RoleID: roleID, TierID: 2,
				Status: string(constants.SubscriptionStatusActive), PaidInstallments: 2}, nil
		},
	}
	insDao := &mockInstallmentDao{
		findNextFn: func(ctx *gin.Context, subscriptionID int64) (*dbs.Installment, error) {
			return &dbs.Installment{ID: 11, SubscriptionID: &subscriptionID, InstallmentNumber: 3,
				Status: string(constants.InstallmentStatusPending), Amount: 1500, DueDate: &due, BlockedDate: &blocked}, nil
		},
	}

	svc := newTierSubscriptionService(urDao, roleDao, tierDao, subDao, insDao)
	resp, err := svc.GetCurrentSubscription(nil, 1, 1)

	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(7), resp.SubscriptionID)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.SubscriptionStatus)
	assert.NotNil(t, resp.InstallmentID)
	assert.Equal(t, 3, *resp.InstallmentNumber)
	assert.Equal(t, float64(1500), *resp.InstallmentAmount)
	assert.Equal(t, 2, *resp.PaidInstallments)
	assert.Equal(t, due, *resp.NextDueDate)
	assert.Equal(t, blocked, *resp.BlockedDate)
	assert.True(t, resp.Tier.PaymentRequired)
	assert.Equal(t, "premium", resp.Tier.Name)
	assert.Equal(t, "corredor", resp.Role.Name)
	assert.NotNil(t, resp.MercadoPago)
	assert.Equal(t, "test-public-key", resp.MercadoPago.PublicKey)
}

func TestGetCurrentSubscription_FreeRoleWithoutSubscription(t *testing.T) {
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID, TierID: 1}, nil
		},
	}
	roleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: id, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", Hierarchy: 1, PaymentRequired: false}, nil
		},
	}

	svc := newTierSubscriptionService(urDao, roleDao, tierDao, &mockTierSubscriptionDao{}, &mockInstallmentDao{})
	resp, err := svc.GetCurrentSubscription(nil, 1, 1)

	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Zero(t, resp.SubscriptionID)
	assert.Zero(t, resp.SubscriptionStatus)
	assert.Equal(t, "base", resp.Tier.Name)
	assert.False(t, resp.Tier.PaymentRequired)
	assert.Nil(t, resp.MercadoPago)
}

func TestGetCurrentSubscription_RoleNotAssigned(t *testing.T) {
	svc := newTierSubscriptionService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, &mockTierSubscriptionDao{}, &mockInstallmentDao{})
	_, err := svc.GetCurrentSubscription(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no tiene asignado este rol")
}

func TestChangeTier_SuccessToPaid(t *testing.T) {
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID, TierID: 1}, nil
		},
	}
	roleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: id, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 2, Name: "premium", RoleID: 1, Hierarchy: 3, PaymentRequired: true, TierAmount: 1500}, nil
		},
	}
	setEndedCalled := false
	subDao := &mockTierSubscriptionDao{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{ID: 7, UserID: userID, RoleID: roleID, TierID: 1,
				Status: string(constants.SubscriptionStatusActive)}, nil
		},
		setEndedFn: func(ctx *gin.Context, id int64) error {
			setEndedCalled = true
			return nil
		},
	}
	createdIns := &dbs.Installment{}
	insDao := &mockInstallmentDao{
		findPendingBySubFn: func(ctx *gin.Context, subscriptionID int64) ([]dbs.Installment, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, ins *dbs.Installment) error {
			*createdIns = *ins
			ins.ID = 2001
			return nil
		},
	}

	svc := NewTierSubscriptionService(nil, urDao, roleDao, tierDao, subDao, insDao)
	resp, err := svc.ChangeTier(nil, 1, 1, &tiersubscription.ChangeTierRequest{TierID: 2})

	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, setEndedCalled)
	assert.Equal(t, string(constants.SubscriptionStatusFirstPaymentPending), resp.SubscriptionStatus)
	assert.NotNil(t, resp.InstallmentID)
	assert.Equal(t, 1, *resp.InstallmentNumber)
	assert.Equal(t, float64(1500), *resp.InstallmentAmount)
	assert.Equal(t, float64(1500), createdIns.Amount)
	assert.NotNil(t, resp.MercadoPago)
	assert.Equal(t, int64(2), resp.Tier.ID)
}

func TestChangeTier_SuccessToFree(t *testing.T) {
	updateTierCalled := false
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID, TierID: 2}, nil
		},
		updateTierFn: func(ctx *gin.Context, userID, roleID, tierID int64) error {
			updateTierCalled = true
			return nil
		},
	}
	roleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: id, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1, Hierarchy: 1, PaymentRequired: false}, nil
		},
	}
	subDao := &mockTierSubscriptionDao{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{ID: 7, UserID: userID, RoleID: roleID, TierID: 2,
				Status: string(constants.SubscriptionStatusActive)}, nil
		},
	}

	svc := newTierSubscriptionService(urDao, roleDao, tierDao, subDao, &mockInstallmentDao{})
	resp, err := svc.ChangeTier(nil, 1, 1, &tiersubscription.ChangeTierRequest{TierID: 1})

	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, updateTierCalled)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.SubscriptionStatus)
	assert.Nil(t, resp.InstallmentID)
	assert.Equal(t, int64(1), resp.Tier.ID)
	assert.Nil(t, resp.MercadoPago)
}

func TestChangeTier_RoleNotAssigned(t *testing.T) {
	svc := newTierSubscriptionService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, &mockTierSubscriptionDao{}, &mockInstallmentDao{})

	_, err := svc.ChangeTier(nil, 1, 1, &tiersubscription.ChangeTierRequest{TierID: 2})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no tiene asignado este rol")
}

func TestChangeTier_TierNotFound(t *testing.T) {
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID}, nil
		},
	}
	svc := newTierSubscriptionService(urDao, &mockRoleDao{}, &mockTierDao{}, &mockTierSubscriptionDao{}, &mockInstallmentDao{})

	_, err := svc.ChangeTier(nil, 1, 1, &tiersubscription.ChangeTierRequest{TierID: 999})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestChangeTier_TierWrongRole(t *testing.T) {
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID}, nil
		},
	}
	tierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 2, Name: "premium", RoleID: 5}, nil
		},
	}
	svc := newTierSubscriptionService(urDao, &mockRoleDao{}, tierDao, &mockTierSubscriptionDao{}, &mockInstallmentDao{})

	_, err := svc.ChangeTier(nil, 1, 1, &tiersubscription.ChangeTierRequest{TierID: 2})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el tier no pertenece al rol especificado")
}

func TestChangeTier_DebtBlocks(t *testing.T) {
	past := time.Now().AddDate(0, 0, -3)
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID}, nil
		},
	}
	tierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 2, Name: "premium", RoleID: 1, PaymentRequired: true}, nil
		},
	}
	roleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: id, Name: "corredor"}, nil
		},
	}
	subDao := &mockTierSubscriptionDao{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{ID: 7, UserID: userID, RoleID: roleID,
				Status: string(constants.SubscriptionStatusActive)}, nil
		},
	}
	insDao := &mockInstallmentDao{
		findPendingBySubFn: func(ctx *gin.Context, subscriptionID int64) ([]dbs.Installment, error) {
			return []dbs.Installment{{ID: 3, Status: string(constants.InstallmentStatusPending), BlockedDate: &past}}, nil
		},
	}

	svc := newTierSubscriptionService(urDao, roleDao, tierDao, subDao, insDao)
	_, err := svc.ChangeTier(nil, 1, 1, &tiersubscription.ChangeTierRequest{TierID: 2})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no podés cambiar de tier con deuda pendiente")
}

func TestChangeTier_PendingFirstPaymentBlocks(t *testing.T) {
	urDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID}, nil
		},
	}
	tierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 2, Name: "premium", RoleID: 1, PaymentRequired: true}, nil
		},
	}
	roleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: id, Name: "corredor"}, nil
		},
	}
	subDao := &mockTierSubscriptionDao{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{ID: 7, UserID: userID, RoleID: roleID,
				Status: string(constants.SubscriptionStatusFirstPaymentPending)}, nil
		},
	}

	svc := newTierSubscriptionService(urDao, roleDao, tierDao, subDao, &mockInstallmentDao{})
	_, err := svc.ChangeTier(nil, 1, 1, &tiersubscription.ChangeTierRequest{TierID: 2})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no podés cambiar de tier con el primer pago pendiente")
}