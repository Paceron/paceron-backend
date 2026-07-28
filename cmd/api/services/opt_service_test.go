package services

import (
	"context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
	"github.com/stretchr/testify/assert"
)

// mockMailer es el doble de prueba compartido por todos los tests de services
// que dependen de mailer.MailerInterface. Registra la última invocación para
// poder assertar sin tener que definir un closure en cada test.
type mockMailer struct {
	sendEmailCalled bool
	lastTo          string
	lastEmailType   mailer.EmailType
	lastData        mailer.EmailData

	mockSend      func(ctx context.Context, to, subject, htmlBody string) error
	mockSendEmail func(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error
}

func (m *mockMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	if m.mockSend != nil {
		return m.mockSend(ctx, to, subject, htmlBody)
	}
	return nil
}

func (m *mockMailer) SendEmail(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error {
	m.sendEmailCalled = true
	m.lastTo = to
	m.lastEmailType = emailType
	m.lastData = data

	if m.mockSendEmail != nil {
		return m.mockSendEmail(ctx, to, emailType, data)
	}
	return nil
}

type mockUserDao struct {
	mockGetByID      func(ctx *gin.Context, userID int64) (*dbs.User, error)
	mockCreate       func(ctx *gin.Context, name, password string) (*dbs.User, error)
	mockFindByID     func(ctx *gin.Context, userID int64) (*dbs.User, error)
	mockFindByEmail  func(ctx *gin.Context, email string) (*dbs.User, error)
	mockUpdate       func(ctx *gin.Context, user *dbs.User) error
	mockUpdateStatus func(ctx *gin.Context, userID int64, status string) error
}

func (m mockUserDao) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return m.mockGetByID(ctx, userID)
}

func (m mockUserDao) Create(ctx *gin.Context, name, password string) (*dbs.User, error) {
	return m.mockCreate(ctx, name, password)
}

func (m mockUserDao) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return m.mockFindByID(ctx, userID)
}

func (m mockUserDao) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	return m.mockFindByEmail(ctx, email)
}

func (m mockUserDao) Update(ctx *gin.Context, user *dbs.User) error {
	return m.mockUpdate(ctx, user)
}

func (m mockUserDao) UpdateStatus(ctx *gin.Context, userID int64, status string) error {
	return m.mockUpdateStatus(ctx, userID, status)
}

func TestGetUser_Success(t *testing.T) {
	expectedUser := &dbs.User{ID: 1, Name: "test"}

	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return expectedUser, nil
		},
	}

	service := NewUserService(mockDao, nil)
	result, err := service.GetUser(nil, 1)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestGetUser_NotFound(t *testing.T) {
	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.GetUser(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetUser_DaoError(t *testing.T) {
	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("dao error")
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.GetUser(nil, 1)

	assert.Error(t, err)
}

func TestCreateUser_Success(t *testing.T) {
	createdUser := &dbs.User{ID: 1, Name: "test"}

	mockDao := mockUserDao{
		mockCreate: func(ctx *gin.Context, name, password string) (*dbs.User, error) {
			return createdUser, nil
		},
	}

	service := NewUserService(mockDao, nil)
	result, err := service.CreateUser(nil, "test", "secret")

	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestCreateUser_DaoError(t *testing.T) {
	mockDao := mockUserDao{
		mockCreate: func(ctx *gin.Context, name, password string) (*dbs.User, error) {
			return nil, errors.New("dao error")
		},
	}

	service := NewUserService(mockDao, nil)
	_, err := service.CreateUser(nil, "test", "secret")

	assert.Error(t, err)
}
