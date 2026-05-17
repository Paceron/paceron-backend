package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/user"
	"github.com/stretchr/testify/assert"
)

type mockUserService struct {
	mockGetUser    func(ctx *gin.Context, userID int64) (user.User, error)
	mockCreateUser func(ctx *gin.Context, name, password string) (user.User, error)
}

func (m mockUserService) GetUser(ctx *gin.Context, userID int64) (user.User, error) {
	return m.mockGetUser(ctx, userID)
}

func (m mockUserService) CreateUser(ctx *gin.Context, name, password string) (user.User, error) {
	return m.mockCreateUser(ctx, name, password)
}

func TestGetUser_Success(t *testing.T) {
	expectedUser := user.User{ID: 1, Name: "test"}

	mockService := mockUserService{
		mockGetUser: func(ctx *gin.Context, userID int64) (user.User, error) {
			return expectedUser, nil
		},
	}

	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}

	controller.GetUser(c)

	assert.Equal(t, http.StatusOK, response.Code)
}

func TestGetUser_InvalidID(t *testing.T) {
	mockService := mockUserService{}
	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Params = []gin.Param{{Key: "user_id", Value: "abc"}}

	controller.GetUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestGetUser_NotFound(t *testing.T) {
	mockService := mockUserService{
		mockGetUser: func(ctx *gin.Context, userID int64) (user.User, error) {
			return user.User{}, errors.New("user not found")
		},
	}

	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Params = []gin.Param{{Key: "user_id", Value: "999"}}

	controller.GetUser(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestCreateUser_Success(t *testing.T) {
	createdUser := user.User{ID: 1, Name: "test"}

	mockService := mockUserService{
		mockCreateUser: func(ctx *gin.Context, name, password string) (user.User, error) {
			return createdUser, nil
		},
	}

	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	body := `{"name":"test","password":"secret"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/user", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.CreateUser(c)

	assert.Equal(t, http.StatusCreated, response.Code)

	var result user.User
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "test", result.Name)
}

func TestCreateUser_InvalidBody(t *testing.T) {
	mockService := mockUserService{}
	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	body := `{"invalid":"data"}`
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodPost, "/user", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	controller.CreateUser(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}
