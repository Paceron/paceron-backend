package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

type mockJoinRequestDao struct {
	createFn                   func(ctx *gin.Context, jr *dbs.JoinRequest) error
	findByIDFn                 func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error)
	findPendingByTeamAndUserFn func(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error)
	findPendingByTeamFn        func(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error)
	findByUserFn               func(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error)
	updateStatusFn             func(ctx *gin.Context, id int64, status string) error
	deleteFn                   func(ctx *gin.Context, id int64) error
	countPendingByOwnerFn      func(ctx *gin.Context, ownerID int64) (int64, error)
}

func (m *mockJoinRequestDao) Create(ctx *gin.Context, jr *dbs.JoinRequest) error {
	if m.createFn != nil {
		return m.createFn(ctx, jr)
	}
	jr.ID = 1
	return nil
}
func (m *mockJoinRequestDao) FindByID(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) FindPendingByTeamAndUser(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error) {
	if m.findPendingByTeamAndUserFn != nil {
		return m.findPendingByTeamAndUserFn(ctx, teamID, runnerID)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) FindPendingByTeam(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error) {
	if m.findPendingByTeamFn != nil {
		return m.findPendingByTeamFn(ctx, teamID)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) FindByUser(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error) {
	if m.findByUserFn != nil {
		return m.findByUserFn(ctx, runnerID)
	}
	return nil, nil
}
func (m *mockJoinRequestDao) UpdateStatus(ctx *gin.Context, id int64, status string) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}
func (m *mockJoinRequestDao) Delete(ctx *gin.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}
func (m *mockJoinRequestDao) CountPendingByOwner(ctx *gin.Context, ownerID int64) (int64, error) {
	if m.countPendingByOwnerFn != nil {
		return m.countPendingByOwnerFn(ctx, ownerID)
	}
	return 0, nil
}

func TestJoinRequestService_Create_Success(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 2, nil },
	}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	resp, err := svc.Create(nil, 5, 7)

	require.NoError(t, err)
	assert.Equal(t, int64(5), resp.TeamID)
	assert.Equal(t, string(constants.InvitationStatusPending), resp.Status)
}

func TestJoinRequestService_Create_TeamNotFound(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return nil, nil }}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrTeamNotFound)
}

func TestJoinRequestService_Create_TeamNotPublic(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: false}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrTeamNotPublic)
}

func TestJoinRequestService_Create_AlreadyMember(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
		return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrAlreadyMember)
}

func TestJoinRequestService_Create_OwnerCannotRequestOwnTeam(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 7, IsPublic: true, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
		t.Fatal("no debería consultar team_users: el chequeo de owner corta antes")
		return nil, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrAlreadyMember)
}

func TestJoinRequestService_Create_TeamFull(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 2}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 2, nil },
	}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrTeamFull)
}

func TestJoinRequestService_Create_AlreadyPending(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, IsPublic: true, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 1, nil },
	}
	jrDao := &mockJoinRequestDao{findPendingByTeamAndUserFn: func(ctx *gin.Context, teamID, runnerID int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: 1, TeamID: teamID, RunnerID: runnerID}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.Create(nil, 5, 7)

	assert.ErrorIs(t, err, ErrJoinRequestAlreadyPending)
}

func TestJoinRequestService_Cancel_Success(t *testing.T) {
	deleted := false
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		deleteFn: func(ctx *gin.Context, id int64) error {
			deleted = true
			return nil
		},
	}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestJoinRequestService_Cancel_NotFound(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) { return nil, nil }}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	assert.ErrorIs(t, err, ErrJoinRequestNotFound)
}

func TestJoinRequestService_Cancel_NotOwner(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, RunnerID: 99, Status: string(constants.InvitationStatusPending)}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}

func TestJoinRequestService_Cancel_NotPending(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, RunnerID: 7, Status: string(constants.InvitationStatusAccepted)}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Cancel(nil, 1, 7)

	assert.ErrorIs(t, err, ErrJoinRequestNotPending)
}

func TestJoinRequestService_Accept_FreeTeam_CreatesActiveMembership(t *testing.T) {
	var createdTeamUser *dbs.TeamUser
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string) error {
			assert.Equal(t, string(constants.InvitationStatusAccepted), status)
			return nil
		},
	}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1, MaxMembers: 10, MembershipFee: 0}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 1, nil },
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			createdTeamUser = tu
			return nil
		},
	}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	require.NoError(t, err)
	require.NotNil(t, createdTeamUser)
	assert.Equal(t, string(constants.SubscriptionStatusActive), createdTeamUser.SubscriptionStatus)
}

func TestJoinRequestService_Accept_AlreadyMember_SkipsGateStillMarksAccepted(t *testing.T) {
	gateCreateCalled := false
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string) error { return nil },
	}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1, MaxMembers: 10}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID}, nil
		},
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			gateCreateCalled = true
			return nil
		},
	}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	require.NoError(t, err)
	assert.False(t, gateCreateCalled)
}

func TestJoinRequestService_Accept_TeamFull(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1, MaxMembers: 1}, nil
	}}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) { return 1, nil },
	}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, teamDao, teamUserDao, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	assert.ErrorIs(t, err, ErrTeamFull)
}

func TestJoinRequestService_Accept_NotOwner(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 99}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}

func TestJoinRequestService_Accept_NotPending(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusAccepted)}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Accept(nil, 1, 1)

	assert.ErrorIs(t, err, ErrJoinRequestNotPending)
}

func TestJoinRequestService_Reject_Success(t *testing.T) {
	statusSet := ""
	jrDao := &mockJoinRequestDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
			return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string) error {
			statusSet = status
			return nil
		},
	}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 1}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Reject(nil, 1, 1)

	require.NoError(t, err)
	assert.Equal(t, string(constants.InvitationStatusRejected), statusSet)
}

func TestJoinRequestService_Reject_NotOwner(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.JoinRequest, error) {
		return &dbs.JoinRequest{ID: id, TeamID: 5, RunnerID: 7, Status: string(constants.InvitationStatusPending)}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: 5, OwnerID: 99}, nil
	}}
	userDao := mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, userDao, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	err := svc.Reject(nil, 1, 1)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}

func TestJoinRequestService_ListMine(t *testing.T) {
	jrDao := &mockJoinRequestDao{findByUserFn: func(ctx *gin.Context, runnerID int64) ([]dbs.JoinRequest, error) {
		return []dbs.JoinRequest{{ID: 1, TeamID: 5, RunnerID: runnerID, Status: string(constants.InvitationStatusPending)}}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, Name: "equipo test"}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	results, err := svc.ListMine(nil, 7)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "equipo test", results[0].TeamName)
}

func TestJoinRequestService_ListByTeam_Success(t *testing.T) {
	jrDao := &mockJoinRequestDao{findPendingByTeamFn: func(ctx *gin.Context, teamID int64) ([]dbs.JoinRequest, error) {
		return []dbs.JoinRequest{{ID: 1, TeamID: teamID, RunnerID: 7, Status: string(constants.InvitationStatusPending)}}, nil
	}}
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 1, Name: "equipo test"}, nil
	}}
	svc := NewJoinRequestService(jrDao, teamDao, &mockTeamUserDao{}, mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	results, err := svc.ListByTeam(nil, 5, 1)

	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestJoinRequestService_ListByTeam_NotOwner(t *testing.T) {
	teamDao := &mockTeamDao{findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
		return &dbs.Team{ID: id, OwnerID: 99}, nil
	}}
	svc := NewJoinRequestService(&mockJoinRequestDao{}, teamDao, &mockTeamUserDao{}, mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	_, err := svc.ListByTeam(nil, 5, 1)

	assert.ErrorIs(t, err, ErrJoinRequestForbidden)
}

func TestJoinRequestService_PendingCount(t *testing.T) {
	jrDao := &mockJoinRequestDao{countPendingByOwnerFn: func(ctx *gin.Context, ownerID int64) (int64, error) {
		return 3, nil
	}}
	svc := NewJoinRequestService(jrDao, &mockTeamDao{}, &mockTeamUserDao{}, mockUserDao{mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) { return nil, nil }}, &mockGroupDao{}, &mockGroupUserDao{}, nil, nil)

	count, err := svc.PendingCount(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}
