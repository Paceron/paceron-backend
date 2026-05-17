package services

import (
	"fmt"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/user"

	"github.com/gin-gonic/gin"
)

type UserServiceInterface interface {
	GetUser(ctx *gin.Context, userID int64) (user.User, error)
	CreateUser(ctx *gin.Context, name, password string) (user.User, error)
}

type userService struct {
	userDao daos.UserDaoInterface
}

func NewUserService(userDao daos.UserDaoInterface) UserServiceInterface {
	return &userService{
		userDao: userDao,
	}
}

func (s *userService) GetUser(ctx *gin.Context, userID int64) (user.User, error) {
	userDB, err := s.userDao.GetByID(ctx, userID)
	if err != nil {
		return user.User{}, fmt.Errorf("error getting user: %w", err)
	}
	if userDB == nil {
		return user.User{}, fmt.Errorf("user not found")
	}

	return user.User{
		ID:   userDB.ID,
		Name: userDB.Name,
	}, nil
}

func (s *userService) CreateUser(ctx *gin.Context, name, password string) (user.User, error) {
	userDB, err := s.userDao.Create(ctx, name, password)

	if err != nil {
		return user.User{}, fmt.Errorf("error creating user: %w", err)
	}

	return user.User{
		ID:   userDB.ID,
		Name: userDB.Name,
	}, nil
}
