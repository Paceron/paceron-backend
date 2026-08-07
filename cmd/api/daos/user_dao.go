package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type UserDaoInterface interface {
	GetByID(ctx *gin.Context, userID int64) (*dbs.User, error)
	FindByID(ctx *gin.Context, userID int64) (*dbs.User, error)
	FindByEmail(ctx *gin.Context, email string) (*dbs.User, error)
	Update(ctx *gin.Context, user *dbs.User) error
	UpdateStatus(ctx *gin.Context, userID int64, status string) error
	SearchActive(ctx *gin.Context, query string, limit int) ([]*dbs.User, error)
	FindByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error)
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

// SearchActive busca usuarios activos por coincidencia parcial (case-insensitive) en
// nombre, apellido o email — pensado para autocompletar al invitar a un equipo.
func (ud *userDao) SearchActive(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
	var users []*dbs.User
	pattern := "%" + query + "%"
	err := ud.DB.
		Where("status = ?", "active").
		Where("name ILIKE ? OR surname ILIKE ? OR email ILIKE ?", pattern, pattern, pattern).
		Order("name ASC").
		Limit(limit).
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("error searching users: %w", err)
	}
	return users, nil
}

// FindByIDs trae varios usuarios de una sola consulta — pensado para resolver nombre/email
// del roster de un equipo/grupo (que solo trae user_id) sin un fan-out N+1 por cliente.
func (ud *userDao) FindByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	var users []*dbs.User
	err := ud.DB.Where("id IN ?", userIDs).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("error finding users by ids: %w", err)
	}
	return users, nil
}
