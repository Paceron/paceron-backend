package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/user"
	"simple-arq-golang/cmd/api/services"
)

// newMultipartPhotoRequest arma un request multipart/form-data con un único
// campo de archivo "photo" — compartido por los tests de upload de foto de
// usuario e ícono de equipo (mismo shape de endpoint).
func newMultipartPhotoRequest(t *testing.T, method, url, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("photo", filename)
	if err != nil {
		t.Fatalf("error creating form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("error writing form file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("error closing multipart writer: %v", err)
	}

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// validPNGContentForTest es el prefijo mínimo que http.DetectContentType
// reconoce como image/png — alcanza para los tests de validación, no
// necesita ser un PNG completo.
var validPNGContentForTest = []byte("\x89PNG\r\n\x1a\n")

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

func TestUserController_UploadPhoto_Success(t *testing.T) {
	expectedURL := "https://bucket.example.com/avatars/user-1.png?v=123"
	mockSvc := mockUserService{
		mockUploadPhoto: func(ctx *gin.Context, userID int64, content []byte) (*string, error) {
			assert.Equal(t, int64(1), userID)
			return &expectedURL, nil
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = newMultipartPhotoRequest(t, http.MethodPut, "/api/v1/users/1/photo", "photo.png", validPNGContentForTest)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.UploadPhoto(c)

	assert.Equal(t, http.StatusOK, response.Code)
	var result map[string]string
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, expectedURL, result["photo_url"])
}

func TestUserController_UploadPhoto_Forbidden_NotSelf(t *testing.T) {
	mockSvc := mockUserService{}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = newMultipartPhotoRequest(t, http.MethodPut, "/api/v1/users/1/photo", "photo.png", validPNGContentForTest)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.UploadPhoto(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestUserController_UploadPhoto_MissingFile(t *testing.T) {
	mockSvc := mockUserService{}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPut, "/api/v1/users/1/photo", strings.NewReader(""))
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.UploadPhoto(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserController_UploadPhoto_ServiceInvalidType(t *testing.T) {
	mockSvc := mockUserService{
		mockUploadPhoto: func(ctx *gin.Context, userID int64, content []byte) (*string, error) {
			return nil, services.ErrPhotoInvalidType
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = newMultipartPhotoRequest(t, http.MethodPut, "/api/v1/users/1/photo", "photo.txt", []byte("not an image"))
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.UploadPhoto(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	var apiErr apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &apiErr)
	assert.Equal(t, "PHOTO_INVALID_TYPE", apiErr.Code)
}

func TestUserController_DeletePhoto_Success(t *testing.T) {
	deleteCalled := false
	mockSvc := mockUserService{
		mockDeletePhoto: func(ctx *gin.Context, userID int64) error {
			deleteCalled = true
			assert.Equal(t, int64(1), userID)
			return nil
		},
	}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/photo", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 1)

	controller.DeletePhoto(c)
	c.Writer.WriteHeaderNow() // gin.CreateTestContext no pasa por engine.ServeHTTP, que normalmente hace este flush

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.True(t, deleteCalled)
}

func TestUserController_DeletePhoto_Forbidden_NotSelf(t *testing.T) {
	mockSvc := mockUserService{}
	controller := NewUserController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodDelete, "/api/v1/users/1/photo", nil)
	c.Params = []gin.Param{{Key: "id", Value: "1"}}
	setAuthUserID(c, 2)

	controller.DeletePhoto(c)

	assert.Equal(t, http.StatusForbidden, response.Code)
}
