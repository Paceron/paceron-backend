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
	FindByID(ctx *gin.Context, userID int64) (*dbs.User, error)
	FindByEmail(ctx *gin.Context, email string) (*dbs.User, error)
	Update(ctx *gin.Context, user *dbs.User) error
	UpdateStatus(ctx *gin.Context, userID int64, status string) error
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

func (ud *userDao) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	var user dbs.User
	err := ud.DB.First(&user, userID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding user by id: %w", err)
	}
	return &user, nil
}

func (ud *userDao) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	var user dbs.User
	err := ud.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding user by email: %w", err)
	}
	return &user, nil
}

func (ud *userDao) Update(ctx *gin.Context, user *dbs.User) error {
	err := ud.DB.Save(user).Error
	if err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}
	return nil
}

func (ud *userDao) UpdateStatus(ctx *gin.Context, userID int64, status string) error {
	err := ud.DB.Model(&dbs.User{}).Where("id = ?", userID).Update("status", status).Error
	if err != nil {
		return fmt.Errorf("error updating user status: %w", err)
	}
	return nil
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
