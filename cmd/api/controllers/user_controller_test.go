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
	"simple-arq-golang/cmd/api/domains/user"
)

func TestUserUpdate_Success(t *testing.T) {
	mockSvc := mockUserService{
		mockUpdate: func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
			return &user.UserUpdateResponse{
				UserID: 1,
				Name:   "John Updated",
				Status: "active",
			}, nil
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John Updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result user.UserUpdateResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.UserID)
	assert.Equal(t, "John Updated", result.Name)
}

func TestUserUpdate_InvalidID(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/abc", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "ID de usuario inválido", result.Message)
}

func TestUserUpdate_Forbidden_NotSelf(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"name":"John Updated"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.Update(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestUserUpdate_InvalidBody(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(`{invalid json}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserUpdate_UserNotFound(t *testing.T) {
	mockSvc := mockUserService{
		mockUpdate: func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"John"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/999", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 999)

	controller.Update(c)

	assert.Equal(t, http.StatusNotFound, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Not Found", result.Code)
}

func TestUserUpdate_EmailWithoutPassword(t *testing.T) {
	mockSvc := mockUserService{
		mockUpdate: func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
			return nil, errors.New("para cambiar el email debe proporcionar la contraseña actual (header X-Current-Password)")
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"new@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserUpdate_WrongPassword(t *testing.T) {
	mockSvc := mockUserService{
		mockUpdate: func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
			return nil, errors.New("contraseña actual incorrecta")
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"new@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Current-Password", "wrong")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestUserUpdate_EmailConflict(t *testing.T) {
	mockSvc := mockUserService{
		mockUpdate: func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
			return nil, errors.New("el email ya está registrado")
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"email":"existing@test.com"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-Current-Password", "pass")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusConflict, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Conflict", result.Code)
	assert.Contains(t, result.Message, "email ya está registrado")
}

func TestChangeStatus_Success(t *testing.T) {
	mockSvc := mockUserService{
		mockChangeStatus: func(ctx *gin.Context, id int64, status string) (*user.UserUpdateResponse, error) {
			return &user.UserUpdateResponse{
				UserID: 1,
				Name:   "John",
				Status: "pause",
			}, nil
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"status":"pause"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/status", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeStatus(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result user.UserUpdateResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "pause", result.Status)
}

func TestChangeStatus_InvalidID(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/abc/status", strings.NewReader(`{"status":"pause"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.ChangeStatus(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestChangeStatus_Forbidden_NotSelf(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"status":"pause"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/status", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.ChangeStatus(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestChangeStatus_InvalidStatus(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"status":"invalid-status"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/status", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeStatus(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Contains(t, result.Message, "Estado inválido")
	assert.Contains(t, result.Message, "Estados permitidos")
}

func TestChangeStatus_InvalidBody(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/status", strings.NewReader(`{invalid}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangeStatus(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestChangeStatus_UserNotFound(t *testing.T) {
	mockSvc := mockUserService{
		mockChangeStatus: func(ctx *gin.Context, id int64, status string) (*user.UserUpdateResponse, error) {
			return nil, errors.New("usuario no encontrado")
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"status":"blocked"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/999/status", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 999)

	controller.ChangeStatus(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestUserUpdate_ValidationError_EmptyName(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"name":""}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestUserUpdate_ValidationError_InvalidEmail(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"email":"invalid-email"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserUpdate_BirthDateFormatError(t *testing.T) {
	mockSvc := mockUserService{
		mockUpdate: func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
			return nil, errors.New("birth_date debe tener formato dd/mm/aaaa")
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"birth_date":"15/04/1990"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Bad request", result.Code)
}

func TestUserUpdate_InternalError(t *testing.T) {
	mockSvc := mockUserService{
		mockUpdate: func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
			return nil, errors.New("error interno")
		},
	}

	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"name":"test"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.Update(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestChangePassword_Success(t *testing.T) {
	mockSvc := mockUserService{
		mockChangePassword: func(ctx *gin.Context, id int64, currentPassword, newPassword string) error {
			return nil
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"NewPass456","confirm_password":"NewPass456"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestChangePassword_InvalidUserID(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"NewPass456","confirm_password":"NewPass456"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/abc/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "abc"}}

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestChangePassword_Forbidden_NotSelf(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"NewPass456","confirm_password":"NewPass456"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestChangePassword_InvalidBody(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{invalid json`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestChangePassword_PasswordMismatch(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"NewPass456","confirm_password":"Different789"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Contains(t, result.Message, "no coinciden")
}

func TestChangePassword_WeakPassword(t *testing.T) {
	controller := NewUserController(mockUserService{})
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"weak","confirm_password":"weak"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestChangePassword_WrongCurrentPassword(t *testing.T) {
	mockSvc := mockUserService{
		mockChangePassword: func(ctx *gin.Context, id int64, currentPassword, newPassword string) error {
			return errors.New("contraseña actual incorrecta")
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"current_password":"WrongPass","new_password":"NewPass456","confirm_password":"NewPass456"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestChangePassword_UserNotFound(t *testing.T) {
	mockSvc := mockUserService{
		mockChangePassword: func(ctx *gin.Context, id int64, currentPassword, newPassword string) error {
			return errors.New("usuario no encontrado")
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"NewPass456","confirm_password":"NewPass456"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/999/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "999"}}
	setAuthUserID(c, 999)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func TestChangePassword_SameAsCurrent(t *testing.T) {
	mockSvc := mockUserService{
		mockChangePassword: func(ctx *gin.Context, id int64, currentPassword, newPassword string) error {
			return errors.New("la nueva contraseña debe ser distinta a la actual")
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"OldPass123","confirm_password":"OldPass123"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestChangePassword_InternalError(t *testing.T) {
	mockSvc := mockUserService{
		mockChangePassword: func(ctx *gin.Context, id int64, currentPassword, newPassword string) error {
			return errors.New("error al cambiar la contraseña")
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	body := `{"current_password":"OldPass123","new_password":"NewPass456","confirm_password":"NewPass456"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPatch, "/api/v1/users/1/password", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.ChangePassword(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
