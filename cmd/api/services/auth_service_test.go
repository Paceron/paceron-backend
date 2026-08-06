package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/auth"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/mailer"
)

type mockRefreshTokenDao struct {
	createFn            func(ctx *gin.Context, token *dbs.RefreshToken) error
	findActiveByHashFn  func(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error)
	revokeFn            func(ctx *gin.Context, id int64, replacedBy *int64) error
	revokeBySessionIDFn func(ctx *gin.Context, sessionID string) error
}

func (m *mockRefreshTokenDao) Create(ctx *gin.Context, token *dbs.RefreshToken) error {
	if m.createFn != nil {
		return m.createFn(ctx, token)
	}
	return nil
}

func (m *mockRefreshTokenDao) FindActiveByHash(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error) {
	if m.findActiveByHashFn != nil {
		return m.findActiveByHashFn(ctx, tokenHash)
	}
	return nil, nil
}

func (m *mockRefreshTokenDao) Revoke(ctx *gin.Context, id int64, replacedBy *int64) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, id, replacedBy)
	}
	return nil
}

func (m *mockRefreshTokenDao) RevokeBySessionID(ctx *gin.Context, sessionID string) error {
	if m.revokeBySessionIDFn != nil {
		return m.revokeBySessionIDFn(ctx, sessionID)
	}
	return nil
}

type mockAuthDao struct {
	mockFindByEmail func(ctx *gin.Context, email string) (*dbs.User, error)
	mockFindByDNI   func(ctx *gin.Context, dni string) (*dbs.User, error)
	mockFindByID    func(ctx *gin.Context, id int64) (*dbs.User, error)
	mockCreate      func(ctx *gin.Context, user *dbs.User) (*dbs.User, error)
}

func (m mockAuthDao) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	return m.mockFindByEmail(ctx, email)
}

func (m mockAuthDao) FindByDNI(ctx *gin.Context, dni string) (*dbs.User, error) {
	return m.mockFindByDNI(ctx, dni)
}

func (m mockAuthDao) FindByID(ctx *gin.Context, id int64) (*dbs.User, error) {
	return m.mockFindByID(ctx, id)
}

func (m mockAuthDao) Create(ctx *gin.Context, user *dbs.User) (*dbs.User, error) {
	return m.mockCreate(ctx, user)
}

func TestValidateRegisterRequest_NameRequired(t *testing.T) {
	req := &auth.RegisterRequest{
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "name es requerido", msg)
}

func TestValidateRegisterRequest_SurnameRequired(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "surname es requerido", msg)
}

func TestValidateRegisterRequest_InvalidEmail(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "invalid",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "email no tiene un formato válido", msg)
}

func TestValidateRegisterRequest_InvalidPhone(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Phone:     "abc",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "phone solo acepta números", msg)
}

func TestValidateRegisterRequest_InvalidPhoneContact(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:         "John",
		Surname:      "Doe",
		Email:        "john@test.com",
		PhoneContact: "abc",
		Dni:          "12345678",
		BirthDate:    "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "phone_contact solo acepta números", msg)
}

func TestValidateRegisterRequest_InvalidCountry(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Country:   "Argentina123",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "country solo acepta letras y espacios", msg)
}

func TestValidateRegisterRequest_InvalidCity(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		City:      "Buenos@Aires",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "city solo acepta letras, números y espacios", msg)
}

func TestValidateRegisterRequest_DniRequired(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "dni es requerido", msg)
}

func TestValidateRegisterRequest_InvalidDni(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "abc123",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "dni solo acepta números", msg)
}

func TestValidateRegisterRequest_InvalidStreet(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Street:    "Calle@123",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "street solo acepta letras, números y espacios", msg)
}

func TestValidateRegisterRequest_InvalidNumber(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Number:    "Casa#5",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "number solo acepta letras, números y espacios", msg)
}

func TestValidateRegisterRequest_EmptyBirthDate(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "birth_date es requerido", msg)
}

func TestValidateRegisterRequest_InvalidBirthDate(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "2024-01-01",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "birth_date debe tener formato dd/mm/aaaa", msg)
}

func TestValidateRegisterRequest_Success(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "", msg)
}

func TestValidateRegisterRequest_ValidOptionalFields(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:         "John",
		Surname:      "Doe",
		Email:        "john@test.com",
		Phone:        "123456789",
		PhoneContact: "987654321",
		Country:      "Argentina",
		City:         "Buenos Aires",
		Street:       "Av Siempre Viva 123",
		Number:       "742",
		Dni:          "12345678",
		BirthDate:    "01/01/2000",
	}
	msg := ValidateRegisterRequest(req)
	assert.Equal(t, "", msg)
}

func TestValidatePassword_Empty(t *testing.T) {
	msg := ValidatePassword("")
	assert.Equal(t, "la contraseña es requerida", msg)
}

func TestValidatePassword_TooShort(t *testing.T) {
	msg := ValidatePassword("1234567")
	assert.Equal(t, "la contraseña debe tener al menos 8 caracteres", msg)
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	msg := ValidatePassword("securepassword123")
	assert.Equal(t, "la contraseña debe contener al menos una letra mayúscula", msg)
}

func TestValidatePassword_NoLowercase(t *testing.T) {
	msg := ValidatePassword("SECUREPASSWORD123")
	assert.Equal(t, "la contraseña debe contener al menos una letra minúscula", msg)
}

func TestValidatePassword_NoDigit(t *testing.T) {
	msg := ValidatePassword("SecurePassword")
	assert.Equal(t, "la contraseña debe contener al menos un número", msg)
}

func TestValidatePassword_SensitiveChars(t *testing.T) {
	tests := []string{"Pass<word1", "Pass>word1", "Pass\"word1", "Pass'word1", "Pass&word1", "Pass;word1", "Pass%word1"}
	for _, pwd := range tests {
		msg := ValidatePassword(pwd)
		assert.Equal(t, "la contraseña contiene caracteres no permitidos", msg, "failed for: %s", pwd)
	}
}

func TestValidatePassword_Success(t *testing.T) {
	msg := ValidatePassword("SecurePassword123")
	assert.Equal(t, "", msg)
}

func TestRegister_Success(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
		mockFindByDNI: func(ctx *gin.Context, dni string) (*dbs.User, error) {
			return nil, nil
		},
		mockCreate: func(ctx *gin.Context, user *dbs.User) (*dbs.User, error) {
			user.ID = 1
			bankAlias := ""
			user.BankAlias = &bankAlias
			return user, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "15/04/1990",
	}

	resp, err := svc.Register(nil, req, "securePass123")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John", resp.Name)
	assert.Equal(t, "15/04/1990", resp.BirthDate)
}

func registerMockDao() mockAuthDao {
	return mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
		mockFindByDNI: func(ctx *gin.Context, dni string) (*dbs.User, error) {
			return nil, nil
		},
		mockCreate: func(ctx *gin.Context, user *dbs.User) (*dbs.User, error) {
			user.ID = 1
			bankAlias := ""
			user.BankAlias = &bankAlias
			return user, nil
		},
	}
}

func registerRequest() *auth.RegisterRequest {
	return &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "15/04/1990",
	}
}

func TestRegister_SendsWelcomeEmail(t *testing.T) {
	mailerMock := &mockMailer{}

	svc := NewAuthService(registerMockDao(), &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, mailerMock)
	resp, err := svc.Register(nil, registerRequest(), "securePass123")

	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.UserID)
	assert.True(t, mailerMock.sendEmailCalled)
	assert.Equal(t, "john@test.com", mailerMock.lastTo)
	assert.Equal(t, mailer.EmailTypeWelcome, mailerMock.lastEmailType)
	assert.Equal(t, "John", mailerMock.lastData.Name)
}

func TestRegister_MailerErrorDoesNotBlock(t *testing.T) {
	mailerMock := &mockMailer{
		mockSendEmail: func(ctx context.Context, to string, emailType mailer.EmailType, data mailer.EmailData) error {
			return errors.New("smtp down")
		},
	}

	svc := NewAuthService(registerMockDao(), &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, mailerMock)
	resp, err := svc.Register(nil, registerRequest(), "securePass123")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.UserID)
}

func TestRegister_NilMailerDoesNotPanic(t *testing.T) {
	svc := NewAuthService(registerMockDao(), &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)

	assert.NotPanics(t, func() {
		resp, err := svc.Register(nil, registerRequest(), "securePass123")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), resp.UserID)
	})
}

func TestRegister_DuplicateEmailDoesNotSendWelcomeEmail(t *testing.T) {
	mockDao := registerMockDao()
	mockDao.mockFindByEmail = func(ctx *gin.Context, email string) (*dbs.User, error) {
		return &dbs.User{ID: 99, Email: email}, nil
	}

	mailerMock := &mockMailer{}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, mailerMock)
	_, err := svc.Register(nil, registerRequest(), "securePass123")

	assert.Error(t, err)
	assert.False(t, mailerMock.sendEmailCalled, "no debe enviarse el correo si el alta falló")
}

func TestRegister_EmailDuplicate(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Email: email}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "existing@test.com",
		Dni:       "12345678",
		BirthDate: "15/04/1990",
	}

	_, err := svc.Register(nil, req, "securePass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email ya está registrado")
}

func TestRegister_DNIDuplicate(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
		mockFindByDNI: func(ctx *gin.Context, dni string) (*dbs.User, error) {
			return &dbs.User{ID: 2, DNI: dni}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "existing-dni",
		BirthDate: "15/04/1990",
	}

	_, err := svc.Register(nil, req, "securePass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DNI ya está registrado")
}

func TestRegister_FindByDNIError(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
		mockFindByDNI: func(ctx *gin.Context, dni string) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "15/04/1990",
	}

	_, err := svc.Register(nil, req, "securePass123")
	assert.Error(t, err)
}

func TestRegister_FindByEmailError(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "15/04/1990",
	}

	_, err := svc.Register(nil, req, "securePass123")
	assert.Error(t, err)
}

func TestRegister_CreateError(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
		mockFindByDNI: func(ctx *gin.Context, dni string) (*dbs.User, error) {
			return nil, nil
		},
		mockCreate: func(ctx *gin.Context, user *dbs.User) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	req := &auth.RegisterRequest{
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		Dni:       "12345678",
		BirthDate: "15/04/1990",
	}

	_, err := svc.Register(nil, req, "securePass123")
	assert.Error(t, err)
}

func TestGetUser_ByID(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) {
			return &dbs.User{
				ID:        id,
				Name:      "John",
				Surname:   "Doe",
				Email:     "john@test.com",
				DNI:       "12345678",
				BirthDate: time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	resp, err := svc.GetUser(nil, 1, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John", resp.Name)
}

func TestGetUser_ByEmail(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{
				ID:        2,
				Name:      "Jane",
				Surname:   "Smith",
				Email:     email,
				DNI:       "87654321",
				BirthDate: time.Date(1992, 6, 20, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	resp, err := svc.GetUser(nil, 0, "jane@test.com")
	assert.NoError(t, err)
	assert.Equal(t, int64(2), resp.UserID)
	assert.Equal(t, "jane@test.com", resp.Email)
}

func TestAuthGetUser_NotFound(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.GetUser(nil, 999, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestGetUser_FindByIDError(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.GetUser(nil, 1, "")
	assert.Error(t, err)
}

func TestGetUser_NoParams(t *testing.T) {
	mockDao := mockAuthDao{}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.GetUser(nil, 0, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "debe proporcionar id o email")
}

func TestGetUser_BothParams(t *testing.T) {
	mockDao := mockAuthDao{}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.GetUser(nil, 1, "john@test.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no ambos")
}

func TestToDBModel(t *testing.T) {
	req := &auth.RegisterRequest{
		Name:      "  John  ",
		Surname:   "Doe",
		Email:     "john@test.com",
		Phone:     "123456789",
		Dni:       "12345678",
		BirthDate: "15/04/1990",
	}

	userDB := toDBModel(req, "hashedPassword")
	assert.Equal(t, "John", userDB.Name)
	assert.Equal(t, "Doe", userDB.Surname)
	assert.Equal(t, "john@test.com", userDB.Email)
	assert.Equal(t, "123456789", userDB.Phone)
	assert.Equal(t, "12345678", userDB.DNI)
	assert.Equal(t, time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC), userDB.BirthDate)
	assert.Equal(t, "hashedPassword", userDB.Password)
}

func TestToResponse(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	bankAlias := "mi-banco-123"
	userDB := &dbs.User{
		ID:        1,
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		DNI:       "12345678",
		BirthDate: birthDate,
		BankAlias: &bankAlias,
	}

	resp := toResponse(userDB)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John", resp.Name)
	assert.Equal(t, "15/04/1990", resp.BirthDate)
	assert.Equal(t, "mi-banco-123", resp.BankAlias)
}

func TestToResponse_BankAliasNil(t *testing.T) {
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	userDB := &dbs.User{
		ID:        1,
		Name:      "John",
		Surname:   "Doe",
		Email:     "john@test.com",
		DNI:       "12345678",
		BirthDate: birthDate,
		BankAlias: nil,
	}

	resp := toResponse(userDB)
	assert.Equal(t, "", resp.BankAlias)
}

func TestLogin_Success(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("securePass123"), bcrypt.DefaultCost)
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)
	bankAlias := "mi-banco"

	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{
				ID:        1,
				Name:      "John",
				Surname:   "Doe",
				Email:     "john@test.com",
				Password:  string(hashedPassword),
				Status:    "active",
				BirthDate: birthDate,
				BankAlias: &bankAlias,
			}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	resp, err := svc.Login(nil, "john@test.com", "securePass123")
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.Equal(t, int(config.AccessTokenDuration.Seconds()), resp.ExpiresIn)
	assert.Equal(t, int64(1), resp.User.UserID)
	assert.Equal(t, "John", resp.User.Name)
	assert.Equal(t, "john@test.com", resp.User.Email)
}

func TestLogin_PersistsRefreshTokenAndRoles(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("securePass123"), bcrypt.DefaultCost)

	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{
				ID:        1,
				Name:      "John",
				Email:     "john@test.com",
				Password:  string(hashedPassword),
				Status:    "active",
				BirthDate: time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}
	userRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	roleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	createCalled := false
	refreshTokenDao := &mockRefreshTokenDao{
		createFn: func(ctx *gin.Context, token *dbs.RefreshToken) error {
			createCalled = true
			assert.Equal(t, int64(1), token.UserID)
			assert.NotEmpty(t, token.SessionID)
			assert.NotEmpty(t, token.TokenHash)
			return nil
		},
	}

	svc := NewAuthService(mockDao, userRoleDao, roleDao, refreshTokenDao, nil)
	_, err := svc.Login(nil, "john@test.com", "securePass123")

	assert.NoError(t, err)
	assert.True(t, createCalled)
}

func TestRefresh_Success(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"

	existing := &dbs.RefreshToken{ID: 1, UserID: 1, SessionID: "session-abc", ExpiresAt: time.Now().Add(time.Hour)}
	createCalled := false
	revokeCalled := false

	refreshTokenDao := &mockRefreshTokenDao{
		findActiveByHashFn: func(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error) {
			return existing, nil
		},
		createFn: func(ctx *gin.Context, token *dbs.RefreshToken) error {
			createCalled = true
			token.ID = 2
			assert.Equal(t, existing.SessionID, token.SessionID)
			return nil
		},
		revokeFn: func(ctx *gin.Context, id int64, replacedBy *int64) error {
			revokeCalled = true
			assert.Equal(t, int64(1), id)
			assert.NotNil(t, replacedBy)
			assert.Equal(t, int64(2), *replacedBy)
			return nil
		},
	}
	mockDao := mockAuthDao{
		mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Status: "active"}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, refreshTokenDao, nil)
	resp, err := svc.Refresh(nil, "some-refresh-token")

	assert.NoError(t, err)
	assert.NotEmpty(t, resp.AccessToken)
	assert.NotEmpty(t, resp.RefreshToken)
	assert.True(t, createCalled)
	assert.True(t, revokeCalled)
}

func TestRefresh_InvalidOrExpiredToken(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"

	refreshTokenDao := &mockRefreshTokenDao{
		findActiveByHashFn: func(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error) {
			return nil, nil
		},
	}

	svc := NewAuthService(mockAuthDao{}, &mockUserRoleDao{}, &mockRoleDao{}, refreshTokenDao, nil)
	_, err := svc.Refresh(nil, "bogus-token")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inválido o expirado")
}

func TestRefresh_InactiveUser(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"

	refreshTokenDao := &mockRefreshTokenDao{
		findActiveByHashFn: func(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error) {
			return &dbs.RefreshToken{ID: 1, UserID: 1, SessionID: "s1", ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
	}
	mockDao := mockAuthDao{
		mockFindByID: func(ctx *gin.Context, id int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Status: "blocked"}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, refreshTokenDao, nil)
	_, err := svc.Refresh(nil, "some-token")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inválido o expirado")
}

func TestLogout_Success(t *testing.T) {
	revokeCalled := false
	refreshTokenDao := &mockRefreshTokenDao{
		findActiveByHashFn: func(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error) {
			return &dbs.RefreshToken{ID: 1, UserID: 1, SessionID: "s1"}, nil
		},
		revokeFn: func(ctx *gin.Context, id int64, replacedBy *int64) error {
			revokeCalled = true
			assert.Nil(t, replacedBy)
			return nil
		},
	}

	svc := NewAuthService(mockAuthDao{}, &mockUserRoleDao{}, &mockRoleDao{}, refreshTokenDao, nil)
	err := svc.Logout(nil, "some-refresh-token")

	assert.NoError(t, err)
	assert.True(t, revokeCalled)
}

func TestLogout_UnknownTokenIsIdempotent(t *testing.T) {
	refreshTokenDao := &mockRefreshTokenDao{
		findActiveByHashFn: func(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error) {
			return nil, nil
		},
	}

	svc := NewAuthService(mockAuthDao{}, &mockUserRoleDao{}, &mockRoleDao{}, refreshTokenDao, nil)
	err := svc.Logout(nil, "unknown-token")

	assert.NoError(t, err)
}

func TestLogin_EmailNotFound(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"

	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.Login(nil, "nonexistent@test.com", "securePass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No se pudo autenticar")
}

func TestLogin_InactiveUser(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("securePass123"), bcrypt.DefaultCost)

	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{
				ID:       1,
				Name:     "John",
				Email:    "john@test.com",
				Password: string(hashedPassword),
				Status:   "blocked",
			}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.Login(nil, "john@test.com", "securePass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No se pudo autenticar")
}

func TestLogin_WrongPassword(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("correctPassword"), bcrypt.DefaultCost)

	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{
				ID:       1,
				Name:     "John",
				Email:    "john@test.com",
				Password: string(hashedPassword),
				Status:   "active",
			}, nil
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.Login(nil, "john@test.com", "wrongPassword")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No se pudo autenticar")
}

func TestLogin_FindByEmailError(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"

	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewAuthService(mockDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockRefreshTokenDao{}, nil)
	_, err := svc.Login(nil, "john@test.com", "securePass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No se pudo autenticar")
}
