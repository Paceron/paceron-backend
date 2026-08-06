package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/auth"
)

func setupGetUserTest() (*httptest.ResponseRecorder, *gin.Context) {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/auth/user", nil)
	return response, c
}

type mockAuthService struct {
	mockRegister func(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error)
	mockLogin    func(ctx *gin.Context, email, password string) (*auth.LoginResponse, error)
	mockGetUser  func(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error)
	mockRefresh  func(ctx *gin.Context, refreshToken string) (*auth.RefreshResponse, error)
	mockLogout   func(ctx *gin.Context, refreshToken string) error
}

func (m mockAuthService) Register(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error) {
	return m.mockRegister(ctx, req, password)
}

func (m mockAuthService) Login(ctx *gin.Context, email, password string) (*auth.LoginResponse, error) {
	return m.mockLogin(ctx, email, password)
}

func (m mockAuthService) GetUser(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
	return m.mockGetUser(ctx, id, email)
}

func (m mockAuthService) Refresh(ctx *gin.Context, refreshToken string) (*auth.RefreshResponse, error) {
	if m.mockRefresh != nil {
		return m.mockRefresh(ctx, refreshToken)
	}
	return nil, nil
}

func (m mockAuthService) Logout(ctx *gin.Context, refreshToken string) error {
	if m.mockLogout != nil {
		return m.mockLogout(ctx, refreshToken)
	}
	return nil
}

func TestRegister_Success(t *testing.T) {
	mockSvc := mockAuthService{
		mockRegister: func(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error) {
			return &auth.UserResponse{
				UserID:    1,
				Name:      "John",
				Surname:   "Doe",
				Email:     "john@test.com",
				Dni:       "12345678",
				BirthDate: "15/04/1990",
			}, nil
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"john@test.com","dni":"12345678","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result auth.UserResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
	assert.Equal(t, "John", result.Name)
	assert.Equal(t, "john@test.com", result.Email)
}

func TestRegister_MissingPasswordField(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"john@test.com","dni":"12345678","birth_date":"15/04/1990"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRegister_PasswordTooShort(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"john@test.com","dni":"12345678","birth_date":"15/04/1990","password":"1234567"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRegister_InvalidBody(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRegister_ValidationError(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"","surname":"Doe","email":"john@test.com","dni":"12345678","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRegister_ConflictEmail(t *testing.T) {
	mockSvc := mockAuthService{
		mockRegister: func(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error) {
			return nil, errors.New("el email ya está registrado")
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"existing@test.com","dni":"12345678","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestRegister_ConflictDNI(t *testing.T) {
	mockSvc := mockAuthService{
		mockRegister: func(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error) {
			return nil, errors.New("el DNI ya está registrado")
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"john@test.com","dni":"99999999","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusConflict, response.Code)
}

func TestGetUser_ByID_Success(t *testing.T) {
	mockSvc := mockAuthService{
		mockGetUser: func(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
			return &auth.UserResponse{
				UserID:    1,
				Name:      "John",
				Surname:   "Doe",
				Email:     "john@test.com",
				Dni:       "12345678",
				BirthDate: "15/04/1990",
			}, nil
		},
	}

	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "id=1"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result auth.UserResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
	assert.Equal(t, "John", result.Name)
}

func TestGetUser_ByEmail_Success(t *testing.T) {
	mockSvc := mockAuthService{
		mockGetUser: func(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
			return &auth.UserResponse{
				UserID:    2,
				Name:      "Jane",
				Surname:   "Smith",
				Email:     email,
				Dni:       "87654321",
				BirthDate: "20/06/1992",
			}, nil
		},
	}

	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "email=jane@test.com"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result auth.UserResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(2), result.UserID)
	assert.Equal(t, "jane@test.com", result.Email)
}

func TestAuthGetUser_NoParams(t *testing.T) {
	mockSvc := mockAuthService{}
	response, c := setupGetUserTest()
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Contains(t, result.Message, "debe proporcionar id o email")
}

func TestAuthGetUser_InvalidID(t *testing.T) {
	mockSvc := mockAuthService{}
	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "id=abc"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestAuthGetUser_NotFound(t *testing.T) {
	mockSvc := mockAuthService{
		mockGetUser: func(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "id=999"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Contains(t, result.Message, "usuario no encontrado")
}

func TestAuthGetUser_InternalError(t *testing.T) {
	mockSvc := mockAuthService{
		mockGetUser: func(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
			return nil, errors.New("internal error")
		},
	}

	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "id=1"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestRegister_InternalError(t *testing.T) {
	mockSvc := mockAuthService{
		mockRegister: func(ctx *gin.Context, req *auth.RegisterRequest, password string) (*auth.UserResponse, error) {
			return nil, errors.New("internal error")
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"john@test.com","dni":"12345678","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestLogin_Success(t *testing.T) {
	mockSvc := mockAuthService{
		mockLogin: func(ctx *gin.Context, email, password string) (*auth.LoginResponse, error) {
			return &auth.LoginResponse{
				AccessToken:  "access-token",
				RefreshToken: "refresh-token",
				ExpiresIn:    900,
				User: auth.UserResponse{
					UserID: 1, Name: "John", Email: "john@test.com",
				},
			}, nil
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"john@test.com","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Login(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result map[string]json.RawMessage
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Contains(t, result, "access_token")
	assert.Contains(t, result, "refresh_token")
	assert.Contains(t, result, "user")

	var resp auth.LoginResponse
	json.Unmarshal(response.Body.Bytes(), &resp)
	assert.Equal(t, "access-token", resp.AccessToken)
	assert.Equal(t, "refresh-token", resp.RefreshToken)
	assert.Equal(t, 900, resp.ExpiresIn)
}

func TestRefresh_Success(t *testing.T) {
	mockSvc := mockAuthService{
		mockRefresh: func(ctx *gin.Context, refreshToken string) (*auth.RefreshResponse, error) {
			assert.Equal(t, "old-refresh-token", refreshToken)
			return &auth.RefreshResponse{AccessToken: "new-access", RefreshToken: "new-refresh", ExpiresIn: 900}, nil
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"refresh_token":"old-refresh-token"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Refresh(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestRefresh_InvalidToken(t *testing.T) {
	mockSvc := mockAuthService{
		mockRefresh: func(ctx *gin.Context, refreshToken string) (*auth.RefreshResponse, error) {
			return nil, errors.New("refresh token inválido o expirado")
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"refresh_token":"bogus"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Refresh(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestRefresh_InvalidBody(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Refresh(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestLogout_Success(t *testing.T) {
	mockSvc := mockAuthService{
		mockLogout: func(ctx *gin.Context, refreshToken string) error {
			assert.Equal(t, "some-refresh-token", refreshToken)
			return nil
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"refresh_token":"some-refresh-token"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Logout(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestLogout_InvalidBody(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Logout(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestLogin_InvalidBody(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{invalid json}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Login(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestLogin_MissingFields(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":""}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Login(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestLogin_Unauthorized(t *testing.T) {
	mockSvc := mockAuthService{
		mockLogin: func(ctx *gin.Context, email, password string) (*auth.LoginResponse, error) {
			return nil, errors.New("No se pudo autenticar")
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"john@test.com","password":"wrong"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Login(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Unauthorized", result.Code)
	assert.Contains(t, result.Message, "No se pudo autenticar")
}

func TestLogin_InternalError(t *testing.T) {
	mockSvc := mockAuthService{
		mockLogin: func(ctx *gin.Context, email, password string) (*auth.LoginResponse, error) {
			return nil, errors.New("error interno")
		},
	}

	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"john@test.com","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Login(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestRegister_ValidationError_InvalidEmail(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"not-an-email","dni":"12345678","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestRegister_ValidationError_EmptyName(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"","surname":"Doe","email":"john@test.com","dni":"12345678","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestRegister_ValidationError_EmptyDNI(t *testing.T) {
	mockSvc := mockAuthService{}
	controller := NewAuthController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John","surname":"Doe","email":"john@test.com","dni":"","birth_date":"15/04/1990","password":"securePass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.Register(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGetUser_BothParams(t *testing.T) {
	mockSvc := mockAuthService{
		mockGetUser: func(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
			return nil, errors.New("debe proporcionar solo id o email, no ambos")
		},
	}

	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "id=1&email=john@test.com"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGetUser_InvalidIDFormat(t *testing.T) {
	mockSvc := mockAuthService{}
	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "id=abc"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGetUser_ByID_Success_WithServiceError(t *testing.T) {
	mockSvc := mockAuthService{
		mockGetUser: func(ctx *gin.Context, id int64, email string) (*auth.UserResponse, error) {
			return nil, errors.New("internal error")
		},
	}

	response, c := setupGetUserTest()
	c.Request.URL.RawQuery = "id=1"
	controller := NewAuthController(mockSvc)
	controller.GetUser(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
