package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/auth"
	"simple-arq-golang/cmd/api/domains/dbs"
)

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
			return user, nil
		},
	}

	svc := NewAuthService(mockDao, nil)
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

func TestRegister_EmailDuplicate(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{ID: 1, Email: email}, nil
		},
	}

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
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
				ID:       id,
				Name:     "John",
				Surname:  "Doe",
				Email:    "john@test.com",
				DNI:      "12345678",
				BirthDate: time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	svc := NewAuthService(mockDao, nil)
	resp, err := svc.GetUser(nil, 1, "")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John", resp.Name)
}

func TestGetUser_ByEmail(t *testing.T) {
	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return &dbs.User{
				ID:       2,
				Name:     "Jane",
				Surname:  "Smith",
				Email:    email,
				DNI:      "87654321",
				BirthDate: time.Date(1992, 6, 20, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
	_, err := svc.GetUser(nil, 1, "")
	assert.Error(t, err)
}

func TestGetUser_NoParams(t *testing.T) {
	mockDao := mockAuthDao{}

	svc := NewAuthService(mockDao, nil)
	_, err := svc.GetUser(nil, 0, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "debe proporcionar id o email")
}

func TestGetUser_BothParams(t *testing.T) {
	mockDao := mockAuthDao{}

	svc := NewAuthService(mockDao, nil)
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
	userDB := &dbs.User{
		ID:       1,
		Name:     "John",
		Surname:  "Doe",
		Email:    "john@test.com",
		DNI:      "12345678",
		BirthDate: birthDate,
	}

	resp := toResponse(userDB)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "John", resp.Name)
	assert.Equal(t, "15/04/1990", resp.BirthDate)
}

func TestLogin_Success(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("securePass123"), bcrypt.DefaultCost)
	birthDate := time.Date(1990, 4, 15, 0, 0, 0, 0, time.UTC)

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
			}, nil
		},
	}

	svc := NewAuthService(mockDao, nil)
	resp, err := svc.Login(nil, "john@test.com", "securePass123")
	assert.NoError(t, err)
	assert.NotEmpty(t, resp.Authorization.AccessToken)
	assert.NotEmpty(t, resp.Authorization.RefreshToken)
	assert.Equal(t, 3600, resp.Authorization.ExpiresIn)
	assert.Equal(t, int64(1), resp.User.UserID)
	assert.Equal(t, "John", resp.User.Name)
	assert.Equal(t, "john@test.com", resp.User.Email)
}

func TestLogin_EmailNotFound(t *testing.T) {
	config.JWTSecret = "test-secret-key-for-testing"

	mockDao := mockAuthDao{
		mockFindByEmail: func(ctx *gin.Context, email string) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
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

	svc := NewAuthService(mockDao, nil)
	_, err := svc.Login(nil, "john@test.com", "securePass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No se pudo autenticar")
}
