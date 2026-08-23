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
	"simple-arq-golang/cmd/api/domains/user"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

func TestUserUpdate_Success(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Surname:   "Doe",
				Email:     "john@test.com",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
		mockUpdate: func(ctx *gin.Context, user *dbs.User) error {
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	newName := "John Updated"
	req := &user.UserUpdateRequest{
		Name: &newName,
	}

	resp, err := svc.Update(nil, 1, req, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John Updated", resp.Name)
}

func TestUserUpdate_BankAliasSuccess(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	var savedUser *dbs.User
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Surname:   "Doe",
				Email:     "john@test.com",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
		mockUpdate: func(ctx *gin.Context, user *dbs.User) error {
			savedUser = user
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	bankAlias := "mi-banco-123"
	req := &user.UserUpdateRequest{
		BankAlias: &bankAlias,
	}

	resp, err := svc.Update(nil, 1, req, "")
	assert.NoError(t, err)
	assert.NotNil(t, savedUser)
	assert.NotNil(t, resp.BankAlias)
	assert.Equal(t, "mi-banco-123", *resp.BankAlias)
}

func TestUserUpdate_UserNotFound(t *testing.T) {
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	name := "John"
	req := &user.UserUpdateRequest{
		Name: &name,
	}

	_, err := svc.Update(nil, 999, req, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestUserUpdate_EmailChangeRequiresPassword(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	newEmail := "newemail@test.com"
	req := &user.UserUpdateRequest{
		Email: &newEmail,
	}

	_, err := svc.Update(nil, 1, req, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "para cambiar el email debe proporcionar la contraseña actual")
}

func TestUserUpdate_EmailChangeWrongPassword(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctPassword"), bcrypt.DefaultCost)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Password:  string(hashedPassword),
				BirthDate: birthDate,
			}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	newEmail := "newemail@test.com"
	req := &user.UserUpdateRequest{
		Email: &newEmail,
	}

	_, err := svc.Update(nil, 1, req, "wrongPassword")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contraseña actual incorrecta")
}

func TestUserUpdate_EmailChangeDuplicate(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctPassword"), bcrypt.DefaultCost)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Password:  string(hashedPassword),
				BirthDate: birthDate,
			}, nil
		},
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 2, Email: email}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	newEmail := "existing@test.com"
	req := &user.UserUpdateRequest{
		Email: &newEmail,
	}

	_, err := svc.Update(nil, 1, req, "correctPassword")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email ya está registrado")
}

func TestUserUpdate_InvalidBirthDate(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	invalidDate := "2024/01/01"
	req := &user.UserUpdateRequest{
		BirthDate: &invalidDate,
	}

	_, err := svc.Update(nil, 1, req, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "birth_date debe tener formato dd/mm/aaaa")
}

func TestChangeStatus_Success(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Status:    "active",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockUpdateStatus: func(ctx *gin.Context, userID int64, status string) error {
			assert.Equal(t, "pause", status)
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.ChangeStatus(nil, 1, "pause")
	assert.NoError(t, err)
	assert.Equal(t, "pause", resp.Status)
}

func TestChangeStatus_UserNotFound(t *testing.T) {
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.ChangeStatus(nil, 999, "pause")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestChangeStatus_InvalidStatus(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Status:    "active",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	_, err := svc.ChangeStatus(nil, 1, "invalid-status")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "estado inválido")
	assert.Contains(t, err.Error(), "Estados permitidos")
}

func TestChangeStatus_InactiveSendsFarewellEmail(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Status:    "active",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockUpdateStatus: func(ctx *gin.Context, userID int64, status string) error {
			return nil
		},
	}

	mailerMock := &mockMailer{}

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.ChangeStatus(nil, 1, "inactive")

	assert.NoError(t, err)
	assert.Equal(t, "inactive", resp.Status)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "john@test.com", mailerMock.lastTo)
	assert.Equal(t, mailer.EmailTypeFarewell, mailerMock.lastEmailType)
	assert.Equal(t, "John", mailerMock.lastData.Name)
}

func TestChangeStatus_InactiveMailerErrorDoesNotBlock(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Status:    "active",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockUpdateStatus: func(ctx *gin.Context, userID int64, status string) error {
			return nil
		},
	}

	mailerMock := &mockMailer{
		mockSendEmail: func(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error {
			return errors.New("smtp down")
		},
	}

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.ChangeStatus(nil, 1, "inactive")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "inactive", resp.Status)
}

func TestChangeStatus_NonInactiveStatusDoesNotSendEmail(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Status:    "active",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockUpdateStatus: func(ctx *gin.Context, userID int64, status string) error {
			return nil
		},
	}

	mailerMock := &mockMailer{}

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.ChangeStatus(nil, 1, "pause")

	assert.NoError(t, err)
	assert.Equal(t, "pause", resp.Status)
	assert.False(t, mailerMock.sendEmailCalled)
}

func TestChangeStatus_RedundantInactiveDoesNotResend(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Status:    "inactive",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockUpdateStatus: func(ctx *gin.Context, userID int64, status string) error {
			return nil
		},
	}

	mailerMock := &mockMailer{}

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{})
	resp, err := svc.ChangeStatus(nil, 1, "inactive")

	assert.NoError(t, err)
	assert.Equal(t, "inactive", resp.Status)
	assert.False(t, mailerMock.sendEmailCalled)
}

func TestChangeStatus_NilMailerDoesNotPanic(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        userID,
				Name:      "John",
				Email:     "john@test.com",
				Status:    "active",
				Password:  "$2a$10$hashedpassword",
				BirthDate: birthDate,
			}, nil
		},
		mockUpdateStatus: func(ctx *gin.Context, userID int64, status string) error {
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})

	assert.NotPanics(t, func() {
		resp, err := svc.ChangeStatus(nil, 1, "inactive")
		assert.NoError(t, err)
		assert.Equal(t, "inactive", resp.Status)
	})
}

func TestValidateUserUpdateRequest_InvalidEmail(t *testing.T) {
	email := "invalid"
	req := &user.UserUpdateRequest{
		Email: &email,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Equal(t, "email no tiene un formato válido", msg)
}

func TestValidateUserUpdateRequest_InvalidBirthDate(t *testing.T) {
	birthDate := "2024/01/01"
	req := &user.UserUpdateRequest{
		BirthDate: &birthDate,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Equal(t, "birth_date debe tener formato dd/mm/aaaa", msg)
}

func TestValidateUserUpdateRequest_Success(t *testing.T) {
	name := "John"
	email := "john@test.com"
	birthDate := "15/04/1990"
	req := &user.UserUpdateRequest{
		Name:      &name,
		Email:     &email,
		BirthDate: &birthDate,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Equal(t, "", msg)
}

func TestToUserUpdateResponse(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	userDB := &dbs.User{
		ID:        1,
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Phone:     "123456789",
		DNI:       "12345678",
		Status:    "active",
		BirthDate: birthDate,
	}

	resp := toUserUpdateResponse(userDB)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John", resp.Name)
	assert.Equal(t, "active", resp.Status)
	assert.Equal(t, "15/04/1990", resp.BirthDate)
}

func TestValidateUserUpdateRequest_BankAliasValid(t *testing.T) {
	bankAlias := "mi-alias.banco"
	req := &user.UserUpdateRequest{
		BankAlias: &bankAlias,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Equal(t, "", msg)
}

func TestValidateUserUpdateRequest_BankAliasTooShort(t *testing.T) {
	bankAlias := "abc"
	req := &user.UserUpdateRequest{
		BankAlias: &bankAlias,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Contains(t, msg, "bank_alias")
}

func TestValidateUserUpdateRequest_BankAliasTooLong(t *testing.T) {
	bankAlias := "este-alias-es-demasiado-largo-para-validar"
	req := &user.UserUpdateRequest{
		BankAlias: &bankAlias,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Contains(t, msg, "bank_alias")
}

func TestValidateUserUpdateRequest_BankAliasInvalidChars(t *testing.T) {
	bankAlias := "alias@invalido!"
	req := &user.UserUpdateRequest{
		BankAlias: &bankAlias,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Contains(t, msg, "bank_alias")
}

func TestValidateUserUpdateRequest_BankAliasNull(t *testing.T) {
	req := &user.UserUpdateRequest{
		BankAlias: nil,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Equal(t, "", msg)
}

func TestChangePassword_Success(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	updateCalled := false
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Email: "john@test.com", Password: string(hashedPassword)}, nil
		},
		mockUpdate: func(ctx *gin.Context, u *dbs.User) error {
			updateCalled = true
			assert.NotEqual(t, string(hashedPassword), u.Password)
			assert.NotNil(t, u.PasswordChangedAt)
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.ChangePassword(nil, 1, "OldPass123", "NewPass456")

	assert.NoError(t, err)
	assert.True(t, updateCalled)
}

// TestChangePassword_SendsNotifications cubre el trigger nuevo: mail + push al
// propio usuario cuando cambia su contraseña.
func TestChangePassword_SendsNotifications(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Name: "Juan", Email: "juan@test.com", Password: string(hashedPassword)}, nil
		},
		mockUpdate: func(ctx *gin.Context, u *dbs.User) error { return nil },
	}
	mailerMock := &mockMailer{}
	pushClient := &mockExpoPushClient{}
	pushTokenDao := mockPushTokenDao{
		mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
			return []dbs.PushToken{{UserID: userID, Token: "ExponentPushToken[juan]"}}, nil
		},
	}

	svc := NewUserService(mockDao, mailerMock, pushTokenDao, pushClient)
	err := svc.ChangePassword(nil, 1, "OldPass123", "NewPass456")

	require.NoError(t, err)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "juan@test.com", mailerMock.lastTo)
	assert.Equal(t, mailer.EmailTypePasswordChanged, mailerMock.lastEmailType)
	assert.Equal(t, 1, pushClient.sendCallCount)
	assert.Equal(t, "ExponentPushToken[juan]", pushClient.lastToken)
	assert.Equal(t, "password_changed", pushClient.lastData["type"])
	_, hasRoute := pushClient.lastData["route"]
	assert.False(t, hasRoute)
}

func TestChangePassword_UserNotFound(t *testing.T) {
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.ChangePassword(nil, 1, "OldPass123", "NewPass456")

	assert.Error(t, err)
	assert.Equal(t, "usuario no encontrado", err.Error())
}

func TestChangePassword_FindByIDError(t *testing.T) {
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.ChangePassword(nil, 1, "OldPass123", "NewPass456")

	assert.Error(t, err)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Password: string(hashedPassword)}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.ChangePassword(nil, 1, "WrongPassword", "NewPass456")

	assert.Error(t, err)
	assert.Equal(t, "contraseña actual incorrecta", err.Error())
}

func TestChangePassword_NewPasswordSameAsCurrent(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Password: string(hashedPassword)}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.ChangePassword(nil, 1, "OldPass123", "OldPass123")

	assert.Error(t, err)
	assert.Equal(t, "la nueva contraseña debe ser distinta a la actual", err.Error())
}

func TestChangePassword_UpdateError(t *testing.T) {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("OldPass123"), bcrypt.DefaultCost)
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, Password: string(hashedPassword)}, nil
		},
		mockUpdate: func(ctx *gin.Context, u *dbs.User) error {
			return errors.New("db error")
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{})
	err := svc.ChangePassword(nil, 1, "OldPass123", "NewPass456")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al cambiar la contraseña")
}
