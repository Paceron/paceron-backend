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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

func TestUserUpdate_DefaultThemeAndAllowInvitationsSuccess(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	var savedUser *dbs.User
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{
				ID:                   userID,
				Name:                 "John",
				Surname:              "Doe",
				Email:                "john@test.com",
				Password:             "$2a$10$hashedpassword",
				BirthDate:            birthDate,
				DefaultTheme:         "dark",
				AllowTeamInvitations: true,
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
	theme := "light"
	allowInvitations := false
	req := &user.UserUpdateRequest{
		DefaultTheme:         &theme,
		AllowTeamInvitations: &allowInvitations,
	}

	resp, err := svc.Update(nil, 1, req, "")
	assert.NoError(t, err)
	assert.NotNil(t, savedUser)
	assert.Equal(t, "light", savedUser.DefaultTheme)
	assert.False(t, savedUser.AllowTeamInvitations)
	assert.Equal(t, "light", resp.DefaultTheme)
	assert.False(t, resp.AllowTeamInvitations)
}

func TestUserUpdate_UserNotFound(t *testing.T) {
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, mailerMock, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)

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

func TestValidateUserUpdateRequest_InvalidDefaultTheme(t *testing.T) {
	theme := "purple"
	req := &user.UserUpdateRequest{
		DefaultTheme: &theme,
	}
	msg := ValidateUserUpdateRequest(req)
	assert.Equal(t, "default_theme debe ser 'light' o 'dark'", msg)
}

func TestValidateUserUpdateRequest_DefaultThemeValid(t *testing.T) {
	for _, theme := range []string{"light", "dark"} {
		req := &user.UserUpdateRequest{DefaultTheme: &theme}
		msg := ValidateUserUpdateRequest(req)
		assert.Equal(t, "", msg)
	}
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
		ID:                   1,
		Name:                 "John",
		Surname:              "Doe",
		Email:                "john@test.com",
		Phone:                "123456789",
		DNI:                  "12345678",
		Status:               "active",
		BirthDate:            birthDate,
		DefaultTheme:         "dark",
		AllowTeamInvitations: true,
	}

	resp := toUserUpdateResponse(userDB)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John", resp.Name)
	assert.Equal(t, "active", resp.Status)
	assert.Equal(t, "15/04/1990", resp.BirthDate)
	assert.Equal(t, "dark", resp.DefaultTheme)
	assert.True(t, resp.AllowTeamInvitations)
	assert.Nil(t, resp.PhotoURL)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, mailerMock, pushTokenDao, pushClient, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
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

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)
	err := svc.ChangePassword(nil, 1, "OldPass123", "NewPass456")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al cambiar la contraseña")
}

// validPNGContent es el prefijo mínimo que http.DetectContentType reconoce
// como image/png (8 bytes de firma) — alcanza para las pruebas de validación,
// no necesita ser un PNG completo.
var validPNGContent = []byte("\x89PNG\r\n\x1a\n")

func TestUserService_UploadPhoto_Success(t *testing.T) {
	storage := &mockStorageClient{}
	updatePhotoCalled := false
	mockDao := mockUserDao{
		mockUpdatePhoto: func(ctx *gin.Context, userID int64, key string, updatedAt time.Time) error {
			updatePhotoCalled = true
			assert.Equal(t, "avatars/user-1.png", key)
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, storage)
	url, err := svc.UploadPhoto(nil, 1, validPNGContent)

	require.NoError(t, err)
	require.NotNil(t, url)
	assert.Contains(t, *url, "avatars/user-1.png")
	assert.True(t, updatePhotoCalled)
	assert.Equal(t, "avatars/user-1.png", storage.lastUploadKey)
	assert.Equal(t, "image/png", storage.lastUploadContentType)
}

func TestUserService_UploadPhoto_NilStorageClient_NoPanic(t *testing.T) {
	svc := NewUserService(mockUserDao{}, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)

	var url *string
	var err error
	assert.NotPanics(t, func() {
		url, err = svc.UploadPhoto(nil, 1, validPNGContent)
	})

	assert.Nil(t, url)
	assert.ErrorIs(t, err, ErrStorageUnavailable)
}

func TestUserService_DeletePhoto_NilStorageClient_NoPanic(t *testing.T) {
	svc := NewUserService(mockUserDao{}, nil, mockPushTokenDao{}, &mockExpoPushClient{}, nil)

	var err error
	assert.NotPanics(t, func() {
		err = svc.DeletePhoto(nil, 1)
	})

	assert.ErrorIs(t, err, ErrStorageUnavailable)
}

func TestUserService_UploadPhoto_TooLarge(t *testing.T) {
	storage := &mockStorageClient{}
	svc := NewUserService(mockUserDao{}, nil, mockPushTokenDao{}, &mockExpoPushClient{}, storage)

	oversized := make([]byte, MaxPhotoSizeBytes+1)
	url, err := svc.UploadPhoto(nil, 1, oversized)

	assert.Nil(t, url)
	assert.ErrorIs(t, err, ErrPhotoTooLarge)
	assert.Empty(t, storage.lastUploadKey)
}

func TestUserService_UploadPhoto_InvalidType(t *testing.T) {
	storage := &mockStorageClient{}
	svc := NewUserService(mockUserDao{}, nil, mockPushTokenDao{}, &mockExpoPushClient{}, storage)

	url, err := svc.UploadPhoto(nil, 1, []byte("this is not an image"))

	assert.Nil(t, url)
	assert.ErrorIs(t, err, ErrPhotoInvalidType)
	assert.Empty(t, storage.lastUploadKey)
}

func TestUserService_UploadPhoto_StorageUploadFails(t *testing.T) {
	storage := &mockStorageClient{
		mockUpload: func(ctx context.Context, key string, content []byte, contentType string) error {
			return errors.New("s3 down")
		},
	}
	photoUpdated := false
	mockDao := mockUserDao{
		mockUpdatePhoto: func(ctx *gin.Context, userID int64, key string, updatedAt time.Time) error {
			photoUpdated = true
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, storage)
	url, err := svc.UploadPhoto(nil, 1, validPNGContent)

	assert.Nil(t, url)
	assert.Error(t, err)
	assert.False(t, photoUpdated, "no debe actualizar la DB si falla el upload a S3")
}

func TestUserService_DeletePhoto_Success(t *testing.T) {
	storage := &mockStorageClient{}
	clearPhotoCalled := false
	key := "avatars/user-1.png"
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, PhotoKey: &key}, nil
		},
		mockClearPhoto: func(ctx *gin.Context, userID int64) error {
			clearPhotoCalled = true
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, storage)
	err := svc.DeletePhoto(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, key, storage.lastDeleteKey)
	assert.True(t, clearPhotoCalled)
}

func TestUserService_DeletePhoto_NoPhoto_Idempotent(t *testing.T) {
	storage := &mockStorageClient{}
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, PhotoKey: nil}, nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, storage)
	err := svc.DeletePhoto(nil, 1)

	require.NoError(t, err)
	assert.Empty(t, storage.lastDeleteKey)
}

func TestUserService_DeletePhoto_StorageDeleteFails(t *testing.T) {
	storage := &mockStorageClient{
		mockDelete: func(ctx context.Context, key string) error {
			return errors.New("s3 down")
		},
	}
	key := "avatars/user-1.png"
	clearPhotoCalled := false
	mockDao := mockUserDao{
		mockFindByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID, PhotoKey: &key}, nil
		},
		mockClearPhoto: func(ctx *gin.Context, userID int64) error {
			clearPhotoCalled = true
			return nil
		},
	}

	svc := NewUserService(mockDao, nil, mockPushTokenDao{}, &mockExpoPushClient{}, storage)
	err := svc.DeletePhoto(nil, 1)

	assert.Error(t, err)
	assert.False(t, clearPhotoCalled, "no debe limpiar la key en DB si falla el borrado en S3")
}
