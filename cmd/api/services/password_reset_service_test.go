package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type mockPasswordResetDao struct {
	mockCreate             func(ctx *gin.Context, token *dbs.PasswordResetToken) error
	mockFindActiveByUserID func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error)
	mockIncrementAttempts  func(ctx *gin.Context, id int64) error
	mockMarkUsed           func(ctx *gin.Context, id int64) error
	mockSoftDeleteByUserID func(ctx *gin.Context, userID int64) error
}

func (m mockPasswordResetDao) Create(ctx *gin.Context, token *dbs.PasswordResetToken) error {
	return m.mockCreate(ctx, token)
}

func (m mockPasswordResetDao) FindActiveByUserID(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
	return m.mockFindActiveByUserID(ctx, userID)
}

func (m mockPasswordResetDao) IncrementAttempts(ctx *gin.Context, id int64) error {
	return m.mockIncrementAttempts(ctx, id)
}

func (m mockPasswordResetDao) MarkUsed(ctx *gin.Context, id int64) error {
	return m.mockMarkUsed(ctx, id)
}

func (m mockPasswordResetDao) SoftDeleteByUserID(ctx *gin.Context, userID int64) error {
	return m.mockSoftDeleteByUserID(ctx, userID)
}

type mockMailer struct {
	sendPasswordResetEmailCalled bool
	mockSendPasswordResetEmail   func(ctx context.Context, to, name, code string) error
}

func (m *mockMailer) Send(ctx context.Context, to, subject, htmlBody string) error {
	return nil
}

func (m *mockMailer) SendWelcomeEmail(ctx context.Context, to, name string) error {
	return nil
}

func (m *mockMailer) SendPasswordResetEmail(ctx context.Context, to, name, code string) error {
	m.sendPasswordResetEmailCalled = true
	if m.mockSendPasswordResetEmail != nil {
		return m.mockSendPasswordResetEmail(ctx, to, name, code)
	}
	return nil
}

func (m *mockMailer) SendInvitationEmail(ctx context.Context, to, name, teamName string) error {
	return nil
}

func TestRequestPasswordReset_UserNotFound_NoErrorNoMail(t *testing.T) {
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
	}
	mailerMock := &mockMailer{}
	svc := NewPasswordResetService(authDao, mockUserDao{}, mockPasswordResetDao{}, mailerMock)

	err := svc.RequestPasswordReset(nil, "nobody@test.com")

	assert.NoError(t, err)
	assert.False(t, mailerMock.sendPasswordResetEmailCalled)
}

func TestRequestPasswordReset_UserNotActive_NoErrorNoMail(t *testing.T) {
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Email: "blocked@test.com", Status: "blocked"}, nil
		},
	}
	mailerMock := &mockMailer{}
	svc := NewPasswordResetService(authDao, mockUserDao{}, mockPasswordResetDao{}, mailerMock)

	err := svc.RequestPasswordReset(nil, "blocked@test.com")

	assert.NoError(t, err)
	assert.False(t, mailerMock.sendPasswordResetEmailCalled)
}

func TestRequestPasswordReset_FindByEmailError_ReturnsError(t *testing.T) {
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewPasswordResetService(authDao, mockUserDao{}, mockPasswordResetDao{}, nil)

	err := svc.RequestPasswordReset(nil, "user@test.com")

	assert.Error(t, err)
}

func TestRequestPasswordReset_ActiveUser_InvalidatesPreviousAndSendsMail(t *testing.T) {
	softDeleteCalled := false
	createCalled := false

	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 42, Email: "user@test.com", Name: "Juan", Status: "active"}, nil
		},
	}
	resetDao := mockPasswordResetDao{
		mockSoftDeleteByUserID: func(ctx *gin.Context, userID int64) error {
			softDeleteCalled = true
			assert.Equal(t, int64(42), userID)
			return nil
		},
		mockCreate: func(ctx *gin.Context, token *dbs.PasswordResetToken) error {
			createCalled = true
			assert.Equal(t, int64(42), token.UserID)
			assert.NotEmpty(t, token.CodeHash)
			assert.True(t, token.ExpiresAt.After(time.Now()))
			return nil
		},
	}
	mailerMock := &mockMailer{
		mockSendPasswordResetEmail: func(ctx context.Context, to, name, code string) error {
			assert.Equal(t, "user@test.com", to)
			assert.Equal(t, "Juan", name)
			assert.Len(t, code, 6)
			return nil
		},
	}

	svc := NewPasswordResetService(authDao, mockUserDao{}, resetDao, mailerMock)
	err := svc.RequestPasswordReset(nil, "user@test.com")

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
	assert.True(t, createCalled)
	assert.True(t, mailerMock.sendPasswordResetEmailCalled)
}

func TestResetPassword_UserNotFound_GenericError(t *testing.T) {
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
	}
	svc := NewPasswordResetService(authDao, mockUserDao{}, mockPasswordResetDao{}, nil)

	err := svc.ResetPassword(nil, "nobody@test.com", "123456", "NewPass123")

	require.Error(t, err)
	assert.Equal(t, "código inválido o expirado", err.Error())
}

func TestResetPassword_UserNotActive_GenericError(t *testing.T) {
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Status: "blocked"}, nil
		},
	}
	svc := NewPasswordResetService(authDao, mockUserDao{}, mockPasswordResetDao{}, nil)

	err := svc.ResetPassword(nil, "blocked@test.com", "123456", "NewPass123")

	require.Error(t, err)
	assert.Equal(t, "código inválido o expirado", err.Error())
}

func TestResetPassword_NoActiveToken_GenericError(t *testing.T) {
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Status: "active"}, nil
		},
	}
	resetDao := mockPasswordResetDao{
		mockFindActiveByUserID: func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
			return nil, nil
		},
	}
	svc := NewPasswordResetService(authDao, mockUserDao{}, resetDao, nil)

	err := svc.ResetPassword(nil, "user@test.com", "123456", "NewPass123")

	require.Error(t, err)
	assert.Equal(t, "código inválido o expirado", err.Error())
}

func TestResetPassword_ExpiredToken_GenericError(t *testing.T) {
	codeHash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Status: "active"}, nil
		},
	}
	resetDao := mockPasswordResetDao{
		mockFindActiveByUserID: func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
			return &dbs.PasswordResetToken{
				ID:        7,
				UserID:    1,
				CodeHash:  string(codeHash),
				ExpiresAt: time.Now().Add(-1 * time.Minute),
			}, nil
		},
	}
	svc := NewPasswordResetService(authDao, mockUserDao{}, resetDao, nil)

	err := svc.ResetPassword(nil, "user@test.com", "123456", "NewPass123")

	require.Error(t, err)
	assert.Equal(t, "código inválido o expirado", err.Error())
}

func TestResetPassword_WrongCode_IncrementsAttempts(t *testing.T) {
	codeHash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	incrementCalled := false

	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Status: "active"}, nil
		},
	}
	resetDao := mockPasswordResetDao{
		mockFindActiveByUserID: func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
			return &dbs.PasswordResetToken{
				ID:        7,
				UserID:    1,
				CodeHash:  string(codeHash),
				Attempts:  0,
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}, nil
		},
		mockIncrementAttempts: func(ctx *gin.Context, id int64) error {
			incrementCalled = true
			assert.Equal(t, int64(7), id)
			return nil
		},
	}
	svc := NewPasswordResetService(authDao, mockUserDao{}, resetDao, nil)

	err := svc.ResetPassword(nil, "user@test.com", "000000", "NewPass123")

	require.Error(t, err)
	assert.Equal(t, "código inválido o expirado", err.Error())
	assert.True(t, incrementCalled)
}

func TestResetPassword_WrongCode_ExceedsMaxAttempts_LocksOut(t *testing.T) {
	codeHash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	lockoutCalled := false

	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Status: "active"}, nil
		},
	}
	resetDao := mockPasswordResetDao{
		mockFindActiveByUserID: func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
			return &dbs.PasswordResetToken{
				ID:        7,
				UserID:    1,
				CodeHash:  string(codeHash),
				Attempts:  otpMaxAttempts - 1,
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}, nil
		},
		mockIncrementAttempts: func(ctx *gin.Context, id int64) error {
			return nil
		},
		mockSoftDeleteByUserID: func(ctx *gin.Context, userID int64) error {
			lockoutCalled = true
			return nil
		},
	}
	svc := NewPasswordResetService(authDao, mockUserDao{}, resetDao, nil)

	err := svc.ResetPassword(nil, "user@test.com", "000000", "NewPass123")

	require.Error(t, err)
	assert.True(t, lockoutCalled)
}

func TestResetPassword_HappyPath_UpdatesPasswordAndMarksUsed(t *testing.T) {
	codeHash, _ := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	updateCalled := false
	markUsedCalled := false

	authDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Email: "user@test.com", Password: "old-hash", Status: "active"}, nil
		},
	}
	userDao := mockUserDao{
		mockUpdate: func(ctx *gin.Context, user *dbs.User) error {
			updateCalled = true
			assert.NotEqual(t, "old-hash", user.Password)
			return nil
		},
	}
	resetDao := mockPasswordResetDao{
		mockFindActiveByUserID: func(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
			return &dbs.PasswordResetToken{
				ID:        7,
				UserID:    1,
				CodeHash:  string(codeHash),
				ExpiresAt: time.Now().Add(5 * time.Minute),
			}, nil
		},
		mockMarkUsed: func(ctx *gin.Context, id int64) error {
			markUsedCalled = true
			assert.Equal(t, int64(7), id)
			return nil
		},
	}
	svc := NewPasswordResetService(authDao, userDao, resetDao, nil)

	err := svc.ResetPassword(nil, "user@test.com", "123456", "NewPass123")

	assert.NoError(t, err)
	assert.True(t, updateCalled)
	assert.True(t, markUsedCalled)
}
