package services

import (
	"context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/invitation"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

// El doble de prueba del mailer es compartido: ver mockMailer en opt_service_test.go.

type mockUserDaoForInvitation struct {
	findByEmailFn func(ctx *gin.Context, email string) (*dbs.User, error)
}

func (m *mockUserDaoForInvitation) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForInvitation) Create(ctx *gin.Context, name, password string) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForInvitation) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
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

func TestInvitationService_InviteRunner_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Equipo Alpha"}, nil
		},
	}
	userDaoForInvitation := &mockUserDaoForInvitation{
		findByEmailFn: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Juan", Email: "juan@test.com"}, nil
		},
	}

	mailerMock := &mockMailer{}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, mailerMock)
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockMailer{})
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
		Email: "noexiste@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se encontró un usuario")
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, mailerMock)
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockMailer{})
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockMailer{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al enviar invitación")
}
