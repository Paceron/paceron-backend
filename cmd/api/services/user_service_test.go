package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/user"
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
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

	svc := NewUserService(mockDao, nil)
	_, err := svc.ChangeStatus(nil, 1, "invalid-status")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "estado inválido")
	assert.Contains(t, err.Error(), "Estados permitidos")
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
