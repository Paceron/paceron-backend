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

// Una cuota vencida que no existe como fila (la N+1 recién se crea al pagar la
// N) también es deuda, y un equipo que pasó a ser gratis no la borra.
func TestGetTeamSubscription_ComputedOverdueIsDebt(t *testing.T) {
	ctx := &gin.Context{}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Los cracks", MembershipFee: 0}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID,
				SubscriptionStatus: string(constants.SubscriptionStatusFirstPaymentPending),
				InitAmount:         12000, AssignmentDate: time.Now().AddDate(0, -2, -3)}, nil
		},
	}

	svc := NewTeamSubscriptionService(teamDao, teamUserDao, &mockInstallmentDao{})
	resp, err := svc.GetTeamSubscription(ctx, 19, 16)
	require.NoError(t, err)
	assert.True(t, resp.HasDebt)
	assert.Equal(t, string(constants.SubscriptionStatusFirstPaymentPending), resp.Membership.SubscriptionStatus)
	require.NotNil(t, resp.MercadoPago, "sigue pudiendo pagar")
}

func TestGetTeamSubscriptions_OwnerBatchStatuses(t *testing.T) {
	ctx := &gin.Context{}
	now := time.Now()
	overdue := now.AddDate(0, 0, -5)
	future := now.AddDate(0, 1, 0)

	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Batch", MembershipFee: 1500, OwnerID: 10}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{
				{TeamID: teamID, UserID: 10, RoleInTeam: string(constants.TeamUserRoleEntrenador), Status: "active"},
				{TeamID: teamID, UserID: 1, RoleInTeam: string(constants.TeamUserRoleCorredor), Status: "active", SubscriptionStatus: string(constants.SubscriptionStatusActive), InitAmount: 1500, PaidInstallments: 1, AssignmentDate: now.AddDate(0, -1, 0)},
				{TeamID: teamID, UserID: 2, RoleInTeam: string(constants.TeamUserRoleCorredor), Status: "active", SubscriptionStatus: string(constants.SubscriptionStatusFirstPaymentPending), InitAmount: 1500, AssignmentDate: now},
				{TeamID: teamID, UserID: 3, RoleInTeam: string(constants.TeamUserRoleCorredor), Status: "inactive", SubscriptionStatus: string(constants.SubscriptionStatusActive)},
			}, nil
		},
	}
	installDao := &mockInstallmentDao{
		findPendingByTeamBatchFn: func(ctx *gin.Context, teamID int64) ([]dbs.Installment, error) {
			return []dbs.Installment{
				{UserID: 1, Status: string(constants.InstallmentStatusPending), DueDate: &overdue},
				{UserID: 2, Status: string(constants.InstallmentStatusPending), DueDate: &future},
			}, nil
		},
	}

	svc := NewTeamSubscriptionService(teamDao, teamUserDao, installDao)
	resp, err := svc.GetTeamSubscriptions(ctx, 10, 99)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int64(99), resp.TeamID)
	assert.Equal(t, float64(1500), resp.MembershipFee)
	require.Len(t, resp.Members, 2)
	assert.Equal(t, int64(1), resp.Members[0].UserID)
	assert.Equal(t, "overdue", resp.Members[0].Status)
	assert.True(t, resp.Members[0].HasDebt)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.Members[0].SubscriptionStatus)
	assert.Equal(t, int64(2), resp.Members[1].UserID)
	assert.Equal(t, string(constants.SubscriptionStatusFirstPaymentPending), resp.Members[1].Status)
	assert.False(t, resp.Members[1].HasDebt)
}

func TestGetTeamSubscriptions_TrainerAuthorized(t *testing.T) {
	ctx := &gin.Context{}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, MembershipFee: 1500, OwnerID: 10}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: string(constants.TeamUserRoleEntrenador), Status: "active"}, nil
		},
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{{TeamID: teamID, UserID: 2, RoleInTeam: string(constants.TeamUserRoleCorredor), Status: "active", SubscriptionStatus: string(constants.SubscriptionStatusActive)}}, nil
		},
	}
	svc := NewTeamSubscriptionService(teamDao, teamUserDao, &mockInstallmentDao{})

	resp, err := svc.GetTeamSubscriptions(ctx, 20, 99)
	require.NoError(t, err)
	require.Len(t, resp.Members, 1)
}

func TestGetTeamSubscriptions_ForbiddenWithoutTrainerRole(t *testing.T) {
	ctx := &gin.Context{}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, MembershipFee: 1500, OwnerID: 10}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: string(constants.TeamUserRoleCorredor), Status: "active"}, nil
		},
	}
	installDao := &mockInstallmentDao{
		findPendingByTeamBatchFn: func(ctx *gin.Context, teamID int64) ([]dbs.Installment, error) {
			t.Fatal("no debe consultar cuotas cuando el caller no administra el equipo")
			return nil, nil
		},
	}

	svc := NewTeamSubscriptionService(teamDao, teamUserDao, installDao)
	resp, err := svc.GetTeamSubscriptions(ctx, 20, 99)

	assert.Nil(t, resp)
	assert.ErrorIs(t, err, ErrTeamSubscriptionForbidden)
}

func TestGetTeamSubscriptions_TeamNotFound(t *testing.T) {
	ctx := &gin.Context{}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return nil, nil },
	}
	svc := NewTeamSubscriptionService(teamDao, &mockTeamUserDao{}, &mockInstallmentDao{})

	resp, err := svc.GetTeamSubscriptions(ctx, 1, 99)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "equipo no encontrado")
}

func TestGetTeamSubscriptions_FreeTeamIgnoresDebt(t *testing.T) {
	ctx := &gin.Context{}
	overdue := time.Now().AddDate(0, 0, -5)
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, MembershipFee: 0, OwnerID: 10}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{{TeamID: teamID, UserID: 2, RoleInTeam: string(constants.TeamUserRoleCorredor), Status: "active", SubscriptionStatus: string(constants.SubscriptionStatusFirstPaymentPending), InitAmount: 0}}, nil
		},
	}
	installDao := &mockInstallmentDao{
		findPendingByTeamBatchFn: func(ctx *gin.Context, teamID int64) ([]dbs.Installment, error) {
			return []dbs.Installment{{UserID: 2, Status: string(constants.InstallmentStatusPending), DueDate: &overdue}}, nil
		},
	}

	svc := NewTeamSubscriptionService(teamDao, teamUserDao, installDao)
	resp, err := svc.GetTeamSubscriptions(ctx, 10, 99)

	require.NoError(t, err)
	require.Len(t, resp.Members, 1)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.Members[0].Status)
	assert.Equal(t, string(constants.SubscriptionStatusActive), resp.Members[0].SubscriptionStatus)
	assert.False(t, resp.Members[0].HasDebt)
}

func TestGetTeamSubscriptions_BatchInstallmentsError(t *testing.T) {
	ctx := &gin.Context{}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, MembershipFee: 1500, OwnerID: 10}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{{TeamID: teamID, UserID: 2, RoleInTeam: string(constants.TeamUserRoleCorredor), Status: "active"}}, nil
		},
	}
	installDao := &mockInstallmentDao{
		findPendingByTeamBatchFn: func(ctx *gin.Context, teamID int64) ([]dbs.Installment, error) {
			return nil, assert.AnError
		},
	}

	svc := NewTeamSubscriptionService(teamDao, teamUserDao, installDao)
	resp, err := svc.GetTeamSubscriptions(ctx, 10, 99)

	assert.Nil(t, resp)
	assert.EqualError(t, err, "error al obtener el estado de cuenta")
}

func TestGetTeamSubscriptions_EmptyMembersIsArray(t *testing.T) {
	ctx := &gin.Context{}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, MembershipFee: 1500, OwnerID: 10}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{{TeamID: teamID, UserID: 10, RoleInTeam: string(constants.TeamUserRoleEntrenador), Status: "active"}}, nil
		},
	}

	svc := NewTeamSubscriptionService(teamDao, teamUserDao, &mockInstallmentDao{})
	resp, err := svc.GetTeamSubscriptions(ctx, 10, 99)

	require.NoError(t, err)
	require.NotNil(t, resp.Members)
	assert.Empty(t, resp.Members)
}
