package services

import (
	"context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/invitation"
)

type mockMailerForInvitation struct {
	sendFn              func(ctx context.Context, to, subject, htmlBody string) error
	sendInvitationFn    func(ctx context.Context, to, name, teamName string) error
}

func (m *mockMailerForInvitation) Send(ctx context.Context, to, subject, htmlBody string) error {
	if m.sendFn != nil {
		return m.sendFn(ctx, to, subject, htmlBody)
	}
	return nil
}

func (m *mockMailerForInvitation) SendWelcomeEmail(ctx context.Context, to, name string) error {
	return nil
}

func (m *mockMailerForInvitation) SendPasswordResetEmail(ctx context.Context, to, name, code string) error {
	return nil
}

func (m *mockMailerForInvitation) SendInvitationEmail(ctx context.Context, to, name, teamName string) error {
	if m.sendInvitationFn != nil {
		return m.sendInvitationFn(ctx, to, name, teamName)
	}
	return nil
}

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

	sent := false
	mailer := &mockMailerForInvitation{
		sendInvitationFn: func(ctx context.Context, to, name, teamName string) error {
			sent = true
			assert.Equal(t, "juan@test.com", to)
			assert.Equal(t, "Equipo Alpha", teamName)
			return nil
		},
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, mailer)
	resp, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, sent)
	assert.Contains(t, resp.Message, "juan@test.com")
}

func TestInvitationService_InviteRunner_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockMailerForInvitation{})
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockMailerForInvitation{})
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
	mailer := &mockMailerForInvitation{
		sendInvitationFn: func(ctx context.Context, to, name, teamName string) error {
			return errors.New("smtp error")
		},
	}

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, mailer)
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

	svc := NewInvitationService(&mockUserDaoForInvitation{}, mockTeamDao, &mockMailerForInvitation{})
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

	svc := NewInvitationService(userDaoForInvitation, mockTeamDao, &mockMailerForInvitation{})
	_, err := svc.InviteRunner(nil, 1, &invitation.InviteRunnerRequest{
		Email: "juan@test.com",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al enviar invitación")
}
