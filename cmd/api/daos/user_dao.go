package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type UserDaoInterface interface {
	GetByID(ctx *gin.Context, userID int64) (*dbs.User, error)
	Create(ctx *gin.Context, name, password string) (*dbs.User, error)
}

type userDao struct {
	DB *gorm.DB
}

func NewUserDao(database *gorm.DB) UserDaoInterface {
	return &userDao{
		DB: database,
	}
}

func (ud *userDao) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	var user dbs.User
	err := ud.DB.First(&user, userID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error retrieving user: %w", err)
	}
	return &user, nil
}

func (ud *userDao) Create(ctx *gin.Context, name, password string) (*dbs.User, error) {
	user := dbs.User{
		Name:     name,
		Password: password,
	}

	err := ud.DB.Create(&user).Error
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return &user, nil
}
