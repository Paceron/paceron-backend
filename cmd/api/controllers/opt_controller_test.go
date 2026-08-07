package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"simple-arq-golang/cmd/api/domains/user"
)

type mockUserService struct {
	mockGetUser        func(ctx *gin.Context, userID int64) (user.User, error)
	mockUpdate         func(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error)
	mockChangeStatus   func(ctx *gin.Context, id int64, status string) (*user.UserUpdateResponse, error)
	mockChangePassword func(ctx *gin.Context, id int64, currentPassword, newPassword string) error
	mockSearch         func(ctx *gin.Context, query string) (*user.SearchResponse, error)
}

func (m mockUserService) GetUser(ctx *gin.Context, userID int64) (user.User, error) {
	return m.mockGetUser(ctx, userID)
}

func (m mockUserService) Update(ctx *gin.Context, id int64, req *user.UserUpdateRequest, currentPassword string) (*user.UserUpdateResponse, error) {
	return m.mockUpdate(ctx, id, req, currentPassword)
}

func (m mockUserService) ChangeStatus(ctx *gin.Context, id int64, status string) (*user.UserUpdateResponse, error) {
	return m.mockChangeStatus(ctx, id, status)
}

func (m mockUserService) ChangePassword(ctx *gin.Context, id int64, currentPassword, newPassword string) error {
	return m.mockChangePassword(ctx, id, currentPassword, newPassword)
}

func (m mockUserService) Search(ctx *gin.Context, query string) (*user.SearchResponse, error) {
	return m.mockSearch(ctx, query)
}

func TestUserController_Search_Success(t *testing.T) {
	mockService := mockUserService{
		mockSearch: func(ctx *gin.Context, query string) (*user.SearchResponse, error) {
			assert.Equal(t, "ana", query)
			return &user.SearchResponse{
				Results: []user.SearchResultItem{
					{UserID: 1, Name: "Ana", Surname: "Gomez", Email: "ana@test.com"},
				},
			}, nil
		},
	}

	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/search", nil)
	c.Request.URL.RawQuery = "q=ana"

	controller.Search(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result user.SearchResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "Ana", result.Results[0].Name)
}

func TestUserController_Search_QueryTooShort(t *testing.T) {
	mockService := mockUserService{
		mockSearch: func(ctx *gin.Context, query string) (*user.SearchResponse, error) {
			return nil, errors.New("la búsqueda requiere al menos 3 caracteres")
		},
	}

	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/search", nil)
	c.Request.URL.RawQuery = "q=an"

	controller.Search(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserController_Search_InternalError(t *testing.T) {
	mockService := mockUserService{
		mockSearch: func(ctx *gin.Context, query string) (*user.SearchResponse, error) {
			return nil, errors.New("error al buscar usuarios")
		},
	}

	controller := NewUserController(mockService)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/search", nil)
	c.Request.URL.RawQuery = "q=ana"

	controller.Search(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
