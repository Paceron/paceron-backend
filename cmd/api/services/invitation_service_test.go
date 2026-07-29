package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

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

func (m *mockUserDaoForInvitation) Create(ctx *gin.Context, name, password string) (*dbs.User, error) {
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

type mockInvitationDao struct {
	createFn                      func(ctx *gin.Context, inv *dbs.Invitation) error
	findByIDFn                    func(ctx *gin.Context, id int64) (*dbs.Invitation, error)
	findPendingByTeamAndInviteeFn func(ctx *gin.Context, teamID, inviteeID int64) (*dbs.Invitation, error)
	findPendingByTeamIDFn         func(ctx *gin.Context, teamID int64) ([]dbs.Invitation, error)
	updateStatusFn                func(ctx *gin.Context, id int64, status string, respondedAt time.Time) error
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

func (m *mockInvitationDao) UpdateStatus(ctx *gin.Context, id int64, status string, respondedAt time.Time) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status, respondedAt)
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
			assert.Equal(t, int64(5), inv.InviterID)
			assert.Equal(t, int64(1), inv.InviteeID)
			assert.Equal(t, string(constants.InvitationStatusPending), inv.Status)
			return nil
		},
	}
	mailerMock := &mockMailer{}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, &mockTeamUserDao{}, mailerMock)
	resp, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
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

func TestInvitationService_InviteRunner_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.InviteRunner(nil, 999, &invitation.InviteRunnerRequest{
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
		Email: "noexiste@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se encontró un usuario")
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
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, TeamID: teamID, UserID: userID}, nil
		},
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, teamUserDao, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya existe una invitación pendiente")
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, mailerMock)
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, invDao, &mockTeamUserDao{}, &mockMailer{})
	resp, err := svc.ListPendingInvitations(nil, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Pedro", resp[0].InviteeName)
	assert.Equal(t, "pedro@test.com", resp[0].InviteeEmail)
}

func TestInvitationService_ListPendingInvitations_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockInvitationDao{}, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.ListPendingInvitations(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, invDao, &mockTeamUserDao{}, &mockMailer{})
	resp, err := svc.ListPendingInvitations(nil, 1)

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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, invDao, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.ListPendingInvitations(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar invitaciones")
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, teamUserDao, &mockMailer{})
	resp, err := svc.AcceptInvitation(nil, 1, 2)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, createCalled)
	assert.True(t, updateStatusCalled)
}

func TestInvitationService_AcceptInvitation_NotFound(t *testing.T) {
	invDao := &mockInvitationDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Invitation, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, teamUserDao, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, teamUserDao, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, teamUserDao, &mockMailer{})
	_, err := svc.AcceptInvitation(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al procesar la invitación")
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, &mockTeamDao{}, invDao, &mockTeamUserDao{}, &mockMailer{})
	_, err := svc.RejectInvitation(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya fue respondida")
}
