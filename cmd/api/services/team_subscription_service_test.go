package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

func TestGetTeamSubscription_FreeTeam(t *testing.T) {
	ctx := &gin.Context{}
	start := time.Now().AddDate(0, 0, -10)

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Libres", MembershipFee: 0}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, InitAmount: 0, PaidInstallments: 3, AssignmentDate: start}, nil
		},
	}
	svc := NewTeamSubscriptionService(teamDao, teamUserDao, &mockInstallmentDao{})

	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(99), resp.Team.ID)
	assert.Equal(t, "Libres", resp.Team.Name)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.Membership.SubscriptionStatus)
	assert.Equal(t, 3, resp.Membership.PaidInstallments)
	assert.NotNil(t, resp.Membership.StartDate)
	assert.Nil(t, resp.NextInstallment)
	assert.Nil(t, resp.MercadoPago)
}

func TestGetTeamSubscription_FreeTeam_NoAssignmentDate(t *testing.T) {
	ctx := &gin.Context{}

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Libres", MembershipFee: 0}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
		},
	}
	svc := NewTeamSubscriptionService(teamDao, teamUserDao, &mockInstallmentDao{})

	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	require.NoError(t, err)
	assert.Nil(t, resp.Membership.StartDate)
}

func TestGetTeamSubscription_PaidTeam_WithNextInstallment(t *testing.T) {
	ctx := &gin.Context{}
	overdue := time.Now().AddDate(0, 0, -5)
	due := time.Now().AddDate(0, 1, 0)
	blocked := due.AddDate(0, 0, 7)

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Elite", MembershipFee: 15000.0}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID,
				SubscriptionStatus: string(constants.SubscriptionStatusActive),
				InitAmount:         15000.0, PaidInstallments: 2}, nil
		},
	}
	installDao := &mockInstallmentDao{
		findPendingByTeamFn: func(ctx *gin.Context, teamID, userID int64) ([]dbs.Installment, error) {
			return []dbs.Installment{{ID: 10, UserID: userID, Status: string(constants.InstallmentStatusPending), Amount: 15000, DueDate: &overdue}}, nil
		},
		findNextByTeamFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.Installment, error) {
			return &dbs.Installment{ID: 11, UserID: 1, InstallmentNumber: 3, Amount: 15000, DueDate: &due, BlockedDate: &blocked}, nil
		},
	}

	svc := NewTeamSubscriptionService(teamDao, teamUserDao, installDao)
	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	require.NoError(t, err)
	assert.True(t, resp.HasDebt)
	require.NotNil(t, resp.NextInstallment)
	assert.Equal(t, int64(11), resp.NextInstallment.InstallmentID)
	assert.Equal(t, 3, resp.NextInstallment.InstallmentNumber)
	assert.Equal(t, float64(15000), resp.NextInstallment.InstallmentAmount)
	require.NotNil(t, resp.MercadoPago)
	assert.True(t, resp.MercadoPago.Marketplace)
	assert.Equal(t, string(constants.PaymentConceptTeamSubscription), resp.MercadoPago.Concept)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.Membership.SubscriptionStatus)
}

func TestGetTeamSubscription_PaidTeam_EmptyStatusBecomesActive(t *testing.T) {
	ctx := &gin.Context{}

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Elite", MembershipFee: 15000.0}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, SubscriptionStatus: "", InitAmount: 15000, PaidInstallments: 1}, nil
		},
	}
	installDao := &mockInstallmentDao{
		findPendingByTeamFn: func(ctx *gin.Context, teamID, userID int64) ([]dbs.Installment, error) {
			return nil, nil
		},
	}
	svc := NewTeamSubscriptionService(teamDao, teamUserDao, installDao)

	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	require.NoError(t, err)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.Membership.SubscriptionStatus)
	assert.False(t, resp.HasDebt)
	assert.Nil(t, resp.NextInstallment)
}

func TestGetTeamSubscription_TeamNil(t *testing.T) {
	ctx := &gin.Context{}

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}
	svc := NewTeamSubscriptionService(teamDao, &mockTeamUserDao{}, &mockInstallmentDao{})

	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "equipo no encontrado")
}

func TestGetTeamSubscription_TeamDaoError(t *testing.T) {
	ctx := &gin.Context{}

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, assert.AnError
		},
	}
	svc := NewTeamSubscriptionService(teamDao, &mockTeamUserDao{}, &mockInstallmentDao{})

	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error al obtener el estado de cuenta")
}

func TestGetTeamSubscription_NotMember(t *testing.T) {
	ctx := &gin.Context{}

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Elite", MembershipFee: 15000}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}
	svc := NewTeamSubscriptionService(teamDao, teamUserDao, &mockInstallmentDao{})

	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "el usuario no pertenece a este equipo")
}

func TestGetTeamSubscription_PendingInstallmentDaoError(t *testing.T) {
	ctx := &gin.Context{}

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, MembershipFee: 15000}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, SubscriptionStatus: string(constants.SubscriptionStatusActive)}, nil
		},
	}
	installDao := &mockInstallmentDao{
		findPendingByTeamFn: func(ctx *gin.Context, teamID, userID int64) ([]dbs.Installment, error) {
			return nil, assert.AnError
		},
	}
	svc := NewTeamSubscriptionService(teamDao, teamUserDao, installDao)

	resp, err := svc.GetTeamSubscription(ctx, 1, 99)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error al obtener el estado de cuenta")
}