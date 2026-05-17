package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/dbs"
	"github.com/stretchr/testify/assert"
)

type mockUserDao struct {
	mockGetByID func(ctx *gin.Context, userID int64) (*dbs.User, error)
	mockCreate  func(ctx *gin.Context, name, password string) (*dbs.User, error)
}

func (m mockUserDao) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return m.mockGetByID(ctx, userID)
}

func (m mockUserDao) Create(ctx *gin.Context, name, password string) (*dbs.User, error) {
	return m.mockCreate(ctx, name, password)
}

func TestGetUser_Success(t *testing.T) {
	expectedUser := &dbs.User{ID: 1, Name: "test"}

	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return expectedUser, nil
		},
	}

	service := NewUserService(mockDao)
	result, err := service.GetUser(nil, 1)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestGetUser_NotFound(t *testing.T) {
	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	service := NewUserService(mockDao)
	_, err := service.GetUser(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetUser_DaoError(t *testing.T) {
	mockDao := mockUserDao{
		mockGetByID: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("dao error")
		},
	}

	service := NewUserService(mockDao)
	_, err := service.GetUser(nil, 1)

	assert.Error(t, err)
}

func TestCreateUser_Success(t *testing.T) {
	createdUser := &dbs.User{ID: 1, Name: "test"}

	mockDao := mockUserDao{
		mockCreate: func(ctx *gin.Context, name, password string) (*dbs.User, error) {
			return createdUser, nil
		},
	}

	service := NewUserService(mockDao)
	result, err := service.CreateUser(nil, "test", "secret")

	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestCreateUser_DaoError(t *testing.T) {
	mockDao := mockUserDao{
		mockCreate: func(ctx *gin.Context, name, password string) (*dbs.User, error) {
			return nil, errors.New("dao error")
		},
	}

	service := NewUserService(mockDao)
	_, err := service.CreateUser(nil, "test", "secret")

	assert.Error(t, err)
}
