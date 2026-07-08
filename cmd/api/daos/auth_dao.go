package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type AuthDaoInterface interface {
	FindByEmail(ctx *gin.Context, email string) (*dbs.User, error)
	FindByDNI(ctx *gin.Context, dni string) (*dbs.User, error)
	FindByID(ctx *gin.Context, id int64) (*dbs.User, error)
	Create(ctx *gin.Context, user *dbs.User) (*dbs.User, error)
}

type authDao struct {
	DB *gorm.DB
}

func NewAuthDao(database *gorm.DB) AuthDaoInterface {
	return &authDao{
		DB: database,
	}
}

func (ad *authDao) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	var user dbs.User
	err := ad.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding user by email: %w", err)
	}
	return &user, nil
}

func (ad *authDao) FindByDNI(ctx *gin.Context, dni string) (*dbs.User, error) {
	var user dbs.User
	err := ad.DB.Where("dni = ?", dni).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding user by dni: %w", err)
	}
	return &user, nil
}

func (ad *authDao) FindByID(ctx *gin.Context, id int64) (*dbs.User, error) {
	var user dbs.User
	err := ad.DB.Where("id = ?", id).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding user by id: %w", err)
	}
	return &user, nil
}

func (ad *authDao) Create(ctx *gin.Context, user *dbs.User) (*dbs.User, error) {
	err := ad.DB.Create(user).Error
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}
	return user, nil
}
