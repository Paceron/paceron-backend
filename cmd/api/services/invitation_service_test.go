package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/invitation"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

// El doble de prueba del mailer es compartido: ver mockMailer en opt_service_test.go.

type mockUserDaoForInvitation struct {
	findByEmailFn func(ctx *gin.Context, email string) (*dbs.User, error)
	findByIDFn    func(ctx *gin.Context, userID int64) (*dbs.User, error)
}

func (m *mockUserDaoForInvitation) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForInvitation) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockUserDaoForInvitation) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *mockUserDaoForInvitation) Update(ctx *gin.Context, user *dbs.User) error {
	return nil
}

func (m *mockUserDaoForInvitation) UpdateStatus(ctx *gin.Context, userID int64, status string) error {
	return nil
}

func (m *mockUserDaoForInvitation) SearchActive(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForInvitation) FindByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	return nil, nil
}

type mockInvitationDao struct {
	createFn                      func(ctx *gin.Context, inv *dbs.Invitation) error
	findByIDFn                    func(ctx *gin.Context, id int64) (*dbs.Invitation, error)
	findPendingByTeamAndInviteeFn func(ctx *gin.Context, teamID, inviteeID int64) (*dbs.Invitation, error)
	findPendingByTeamIDFn         func(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error)
	findPendingByInviteeIDFn      func(ctx *gin.Context, inviteeID int64) ([]dbs.Invitation, error)
	updateStatusFn                func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error
	softDeleteByTeamIDFn          func(ctx *gin.Context, teamID int64) error
}

func (m *mockInvitationDao) Create(ctx *gin.Context, inv *dbs.Invitation) error {
	if m.createFn != nil {
		return m.createFn(ctx, inv)
	}
	return nil
}

func (m *mockInvitationDao) FindByID(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockInvitationDao) FindPendingByTeamAndInvitee(ctx *gin.Context, teamID, inviteeID int64) (*dbs.Invitation, error) {
	if m.findPendingByTeamAndInviteeFn != nil {
		return m.findPendingByTeamAndInviteeFn(ctx, teamID, inviteeID)
	}
	return nil, nil
}

func (m *mockInvitationDao) FindPendingByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error) {
	if m.findPendingByTeamIDFn != nil {
		return m.findPendingByTeamIDFn(ctx, teamID)
	}
	return nil, nil
}

func (m *mockInvitationDao) FindPendingByInviteeID(ctx *gin.Context, inviteeID int64) ([]dbs.Invitation, error) {
	if m.findPendingByInviteeIDFn != nil {
		return m.findPendingByInviteeIDFn(ctx, inviteeID)
	}
	return nil, nil
}

func (m *mockInvitationDao) UpdateStatus(ctx *gin.Context, id int64, status string, respondedAt time.Time) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status, respondedAt)
	}
	return nil
}

func (m *mockInvitationDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	if m.softDeleteByTeamIDFn != nil {
		return m.softDeleteByTeamIDFn(ctx, teamID)
	}
	return nil
}

func TestInvitationService_InviteRunner_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Equipo Alpha", OwnerID: 5}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	invDao := &mockInvitationDao{
		createFn: func(ctx *gin.Context, inv *dbs.Invitation) error {
			assert.Equal(t, int64(1), inv.TeamID)
			assert.Equal(t, testEntrenadorCallerID, inv.InviterID)
			assert.Equal(t, int64(1), inv.InviteeID)
			assert.Equal(t, string(constants.InvitationStatusPending), inv.Status)
			return nil
		},
	}
	mailerMock := &mockMailer{}
	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "juan@test.com", mailerMock.lastTo)
	assert.Equal(t, mailer.EmailTypeInvitation, mailerMock.lastEmailType)
	assert.Equal(t, "Equipo Alpha", mailerMock.lastData.TeamName)
	assert.Contains(t, resp.Message, "juan@test.com")
}

// TestInvitationService_InviteRunner_SendsPushToInvitee cubre el trigger nuevo:
// el invitado recibe un push cuando lo invitan (el mail ya existía antes de esta rama).
func TestInvitationService_InviteRunner_SendsPushToInvitee(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Equipo Alpha", OwnerID: testEntrenadorCallerID}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: testEntrenadorCallerID, Name: "Coach"}, nil
		},
	}
	invDao := &mockInvitationDao{
		createFn: func(ctx *gin.Context, inv *dbs.Invitation) error { return nil },
	}
	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	pushClient := &mockExpoPushClient{}
	pushTokenDao := mockPushTokenDao{
		mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
			return []dbs.PushToken{{UserID: 1, Token: "ExponentPushToken[juan]"}}, nil
		},
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, pushTokenDao, pushClient, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	require.NoError(t, err)
	assert.Equal(t, 1, pushClient.sendCallCount)
	assert.Equal(t, "ExponentPushToken[juan]", pushClient.lastToken)
	assert.Equal(t, "invitation_received", pushClient.lastData["type"])
	assert.Equal(t, "/invitations", pushClient.lastData["route"])
	assert.Contains(t, pushClient.lastBody, "Coach")
}

func TestInvitationService_InviteRunner_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 999, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestInvitationService_InviteRunner_UserNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
	}

	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "noexiste@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se encontró un usuario")
}

func TestInvitationService_InviteRunner_NotEntrenador(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: "corredor"}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, 2, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede invitar usuarios al equipo")
}

func TestInvitationService_InviteRunner_UserAlreadyMember(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, TeamID: teamID, UserID: userID}, nil
		}),
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario ya pertenece a este equipo")
}

func TestInvitationService_InviteRunner_DuplicatePendingInvitation(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	invDao := &mockInvitationDao{
		findPendingByTeamAndInviteeFn: func(ctx *gin.Context, teamID, inviteeID int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 9, TeamID: teamID, InviteeID: inviteeID}, nil
		},
	}

	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya existe una invitación pendiente")
}

func TestInvitationService_InviteRunner_WithValidGroupID(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha", OwnerID: 5}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	mockGroup := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			assert.Equal(t, int64(3), groupID)
			assert.Equal(t, int64(1), teamID)
			return &dbs.Group{ID: 3, TeamID: 1}, nil
		},
	}
	invDao := &mockInvitationDao{
		createFn: func(ctx *gin.Context, inv *dbs.Invitation) error {
			assert.NotNil(t, inv.GroupID)
			assert.Equal(t, int64(3), *inv.GroupID)
			return nil
		},
	}

	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, teamUserDao, mockGroup, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	groupID := int64(3)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email:   "juan@test.com",
		GroupID: &groupID,
	})

	assert.NoError(t, err)
}

func TestInvitationService_InviteRunner_GroupNotInTeam(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	mockGroup := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return nil, nil
		},
	}

	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, teamUserDao, mockGroup, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	groupID := int64(999)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email:   "juan@test.com",
		GroupID: &groupID,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el grupo no existe en este equipo")
}

func TestInvitationService_InviteRunner_InvitationDaoCreateError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	invDao := &mockInvitationDao{
		createFn: func(ctx *gin.Context, inv *dbs.Invitation) error {
			return errors.New("db error")
		},
	}

	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al enviar invitación")
}

func TestInvitationService_InviteRunner_MailerError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	mailerMock := &mockMailer{
		mockSendEmail: func(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error {
			return errors.New("smtp error")
		},
	}

	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al enviar el email")
}

func TestInvitationService_InviteRunner_TeamDaoError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al enviar invitación")
}

func TestInvitationService_InviteRunner_CallerRoleCheckError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al enviar invitación")
}

func TestInvitationService_InviteRunner_UserFindByEmailError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	teamUserDao := &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}
	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.InviteRunner(nil, 1, testEntrenadorCallerID, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al enviar invitación")
}

func TestInvitationService_ListPendingInvitations_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	invDao := &mockInvitationDao{
		findPendingByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error) {
			return []dbs.Invitation{
				{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)},
			}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 2, Name: "Pedro", Email: "pedro@test.com"}, nil
		},
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.ListPendingInvitations(nil, 1, testEntrenadorCallerID)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Pedro", resp[0].InviteeName)
	assert.Equal(t, "pedro@test.com", resp[0].InviteeEmail)
}

func TestInvitationService_ListPendingInvitations_IncludesInviterInfo(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	invDao := &mockInvitationDao{
		findPendingByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error) {
			return []dbs.Invitation{
				{ID: 1, TeamID: 1, InviterID: 5, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)},
			}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			if userID == 5 {
				return &dbs.User{ID: 5, Name: "Entrenador Juan"}, nil
			}
			return &dbs.User{ID: 2, Name: "Pedro", Email: "pedro@test.com"}, nil
		},
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.ListPendingInvitations(nil, 1, testEntrenadorCallerID)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, int64(5), resp[0].InviterID)
	assert.Equal(t, "Entrenador Juan", resp[0].InviterName)
}

func TestInvitationService_ListPendingInvitations_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.ListPendingInvitations(nil, 999, testEntrenadorCallerID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestInvitationService_ListPendingInvitations_CallerRoleCheckError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.ListPendingInvitations(nil, 1, testEntrenadorCallerID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar invitaciones")
}

func TestInvitationService_ListPendingInvitations_NotEntrenador(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: "corredor"}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.ListPendingInvitations(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede ver las invitaciones del equipo")
}

func TestInvitationService_ListPendingInvitations_FiltersExpired(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	invDao := &mockInvitationDao{
		findPendingByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error) {
			return []dbs.Invitation{
				{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(-time.Hour)},
			}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, invDao, &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.ListPendingInvitations(nil, 1, testEntrenadorCallerID)

	assert.NoError(t, err)
	assert.Len(t, resp, 0)
}

func TestInvitationService_ListPendingInvitations_DaoError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	invDao := &mockInvitationDao{
		findPendingByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, invDao, &mockTeamUserDao{findByTeamAndUserFn: entrenadorCallerFindByTeamAndUser(nil)}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.ListPendingInvitations(nil, 1, testEntrenadorCallerID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar invitaciones")
}

func TestInvitationService_ListPendingInvitationsForUser_Success(t *testing.T) {
	invDao := &mockInvitationDao{
		findPendingByInviteeIDFn: func(ctx *gin.Context, inviteeID int64) ([]dbs.Invitation, error) {
			assert.Equal(t, int64(2), inviteeID)
			return []dbs.Invitation{
				{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)},
			}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 2, Name: "Pedro", Email: "pedro@test.com"}, nil
		},
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.ListPendingInvitationsForUser(nil, 2)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Alpha", resp[0].TeamName)
}

func TestInvitationService_ListPendingInvitationsForUser_FiltersExpired(t *testing.T) {
	invDao := &mockInvitationDao{
		findPendingByInviteeIDFn: func(ctx *gin.Context, inviteeID int64) ([]dbs.Invitation, error) {
			return []dbs.Invitation{
				{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(-time.Hour)},
			}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.ListPendingInvitationsForUser(nil, 2)

	assert.NoError(t, err)
	assert.Len(t, resp, 0)
}

func TestInvitationService_ListPendingInvitationsForUser_DaoError(t *testing.T) {
	invDao := &mockInvitationDao{
		findPendingByInviteeIDFn: func(ctx *gin.Context, inviteeID int64) ([]dbs.Invitation, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.ListPendingInvitationsForUser(nil, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar invitaciones")
}

func TestInvitationService_GetInvitationDetail_Success(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.GetInvitationDetail(nil, 1, 2)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Alpha", resp.TeamName)
}

func TestInvitationService_GetInvitationDetail_NotFound(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.GetInvitationDetail(nil, 999, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitación no encontrada")
}

func TestInvitationService_GetInvitationDetail_WrongUser(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.GetInvitationDetail(nil, 1, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pertenece a este usuario")
}

// freeMockTeamDao devuelve un mockTeamDao cuyo FindByID responde un equipo
// gratis (membership_fee = 0), como requiere el gate de membresía (D2) al
// aceptar una invitación.
func freeMockTeamDao() *mockTeamDao {
	return &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: id, Name: "Los Pumas"}, nil
		},
	}
}

func TestInvitationService_AcceptInvitation_Success(t *testing.T) {
	createCalled := false
	updateStatusCalled := false
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error {
			updateStatusCalled = true
			assert.Equal(t, string(constants.InvitationStatusAccepted), status)
			return nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			createCalled = true
			assert.Equal(t, int64(1), tu.TeamID)
			assert.Equal(t, int64(2), tu.UserID)
			assert.Equal(t, "corredor", tu.RoleInTeam)
			return nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.AcceptInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, createCalled)
	assert.True(t, updateStatusCalled)
}

// TestInvitationService_AcceptInvitation_NotifiesInviter cubre el trigger nuevo:
// el entrenador que invitó recibe mail + push cuando el invitado acepta.
func TestInvitationService_AcceptInvitation_NotifiesInviter(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviterID: 10, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error { return nil },
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		createFn:            func(ctx *gin.Context, tu *dbs.TeamUser) error { return nil },
	}
	userDao := &mockUserDaoForInvitation{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			if userID == 10 {
				return &dbs.User{ID: 10, Name: "Coach", Email: "coach@test.com"}, nil
			}
			return &dbs.User{ID: 2, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Los Pumas"}, nil
		},
	}
	mailerMock := &mockMailer{}
	pushClient := &mockExpoPushClient{}
	pushTokenDao := mockPushTokenDao{
		mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
			return []dbs.PushToken{{UserID: 10, Token: "ExponentPushToken[coach]"}}, nil
		},
	}

	svc := NewInvitationService(userDao, teamDao, invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, mailerMock, pushTokenDao, pushClient, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	require.NoError(t, err)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "coach@test.com", mailerMock.lastTo)
	assert.Equal(t, mailer.EmailTypeInvitationResponse, mailerMock.lastEmailType)
	assert.Equal(t, "Coach", mailerMock.lastData.Name)
	assert.Equal(t, "Juan", mailerMock.lastData.RelatedUserName)
	assert.Equal(t, "aceptó", mailerMock.lastData.ResponseStatus)
	assert.Equal(t, 1, pushClient.sendCallCount)
	assert.Equal(t, "ExponentPushToken[coach]", pushClient.lastToken)
	assert.Equal(t, "invitation_response", pushClient.lastData["type"])
	assert.Equal(t, "/teams/1", pushClient.lastData["route"])
}

// TestInvitationService_RejectInvitation_NotifiesInviter cubre el mismo trigger
// para el camino de rechazo.
func TestInvitationService_RejectInvitation_NotifiesInviter(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviterID: 10, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error { return nil },
	}
	userDao := &mockUserDaoForInvitation{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			if userID == 10 {
				return &dbs.User{ID: 10, Name: "Coach", Email: "coach@test.com"}, nil
			}
			return &dbs.User{ID: 2, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Los Pumas"}, nil
		},
	}
	mailerMock := &mockMailer{}

	svc := NewInvitationService(userDao, teamDao, invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.RejectInvitation(nil, 1, 2)

	require.NoError(t, err)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "rechazó", mailerMock.lastData.ResponseStatus)
}

// TestInvitationService_AcceptInvitation_NotificationFailureDoesNotBlock verifica
// el criterio best-effort: si mail y push fallan, accept igual se completa.
func TestInvitationService_AcceptInvitation_NotificationFailureDoesNotBlock(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviterID: 10, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error { return nil },
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) { return nil, nil },
		createFn:            func(ctx *gin.Context, tu *dbs.TeamUser) error { return nil },
	}
	userDao := &mockUserDaoForInvitation{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Name: "User", Email: "user@test.com"}, nil
		},
	}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Los Pumas"}, nil
		},
	}
	mailerMock := &mockMailer{mockSendEmail: func(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error {
		return errors.New("resend down")
	}}
	pushClient := &mockExpoPushClient{mockSend: func(ctx context.Context, token, title, body string, data map[string]string) error {
		return errors.New("expo down")
	}}
	pushTokenDao := mockPushTokenDao{
		mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
			return []dbs.PushToken{{UserID: userID, Token: "tok"}}, nil
		},
	}

	svc := NewInvitationService(userDao, teamDao, invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, mailerMock, pushTokenDao, pushClient, nil, nil)
	resp, err := svc.AcceptInvitation(nil, 1, 2)

	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestInvitationService_AcceptInvitation_NotFound(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 999, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitación no encontrada")
}

func TestInvitationService_AcceptInvitation_WrongUser(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pertenece a este usuario")
}

func TestInvitationService_AcceptInvitation_AlreadyResponded(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "accepted", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya fue respondida")
}

func TestInvitationService_AcceptInvitation_Expired(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(-time.Hour)}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ha expirado")
}

func TestInvitationService_AcceptInvitation_AlreadyMember_MarksAcceptedWithoutDuplicating(t *testing.T) {
	createCalled := false
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 5, TeamID: teamID, UserID: userID}, nil
		},
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			createCalled = true
			return nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.AcceptInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, createCalled)
}

func TestInvitationService_AcceptInvitation_TeamUserCreateError(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			return errors.New("db error")
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al procesar la invitación")
}

func TestInvitationService_AcceptInvitation_UpdateStatusErrorAfterTeamUserCreated(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error {
			return errors.New("db error")
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al procesar la invitación")
}

func TestInvitationService_AcceptInvitation_AssignsToInvitationGroup(t *testing.T) {
	groupID := int64(7)
	groupUserCreateCalled := false
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, GroupID: &groupID, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}
	mockGroupUser := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, gID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			groupUserCreateCalled = true
			assert.Equal(t, int64(7), gu.GroupID)
			assert.Equal(t, int64(2), gu.UserID)
			return nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, &mockGroupDao{}, mockGroupUser, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.True(t, groupUserCreateCalled)
}

func TestInvitationService_AcceptInvitation_AssignsToTeamMainGroup_WhenNoGroupID(t *testing.T) {
	groupUserCreateCalled := false
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}
	mockGroup := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{
				{ID: 4, TeamID: 1, IsMain: false},
				{ID: 5, TeamID: 1, IsMain: true},
			}, nil
		},
	}
	mockGroupUser := &mockGroupUserDao{
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			groupUserCreateCalled = true
			assert.Equal(t, int64(5), gu.GroupID)
			return nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, mockGroup, mockGroupUser, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.True(t, groupUserCreateCalled)
}

func TestInvitationService_AcceptInvitation_NoMainGroup_StillSucceeds(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}
	mockGroup := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, mockGroup, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.AcceptInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestInvitationService_AcceptInvitation_AlreadyGroupMember_DoesNotDuplicate(t *testing.T) {
	groupID := int64(7)
	groupUserCreateCalled := false
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, GroupID: &groupID, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	teamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}
	mockGroupUser := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, gID, userID int64) (*dbs.GroupUser, error) {
			return &dbs.GroupUser{ID: 1, GroupID: gID, UserID: userID}, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			groupUserCreateCalled = true
			return nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, teamUserDao, &mockGroupDao{}, mockGroupUser, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.False(t, groupUserCreateCalled)
}

func TestInvitationService_RejectInvitation_Success(t *testing.T) {
	updateStatusCalled := false
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		updateStatusFn: func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error {
			updateStatusCalled = true
			assert.Equal(t, string(constants.InvitationStatusRejected), status)
			return nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	resp, err := svc.RejectInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, updateStatusCalled)
}

func TestInvitationService_RejectInvitation_NotFound(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.RejectInvitation(nil, 999, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitación no encontrada")
}

func TestInvitationService_RejectInvitation_WrongUser(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "pending", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.RejectInvitation(nil, 1, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pertenece a este usuario")
}

func TestInvitationService_RejectInvitation_AlreadyResponded(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return &dbs.Invitation{ID: 1, TeamID: 1, InviteeID: 2, Status: "rejected", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, freeMockTeamDao(), invDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockMailer{}, mockPushTokenDao{}, &mockExpoPushClient{}, nil, nil)
	_, err := svc.RejectInvitation(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya fue respondida")
}
