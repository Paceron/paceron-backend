package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type PasswordResetDaoInterface interface {
	Create(ctx *gin.Context, token *dbs.PasswordResetToken) error
	FindActiveByUserID(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error)
	IncrementAttempts(ctx *gin.Context, id int64) error
	MarkUsed(ctx *gin.Context, id int64) error
	SoftDeleteByUserID(ctx *gin.Context, userID int64) error
}

type passwordResetDao struct {
	DB *gorm.DB
}

func NewPasswordResetDao(database *gorm.DB) PasswordResetDaoInterface {
	return &passwordResetDao{
		DB: database,
	}
}

func (d *passwordResetDao) Create(ctx *gin.Context, token *dbs.PasswordResetToken) error {
	return d.DB.Create(token).Error
}

func (d *passwordResetDao) FindActiveByUserID(ctx *gin.Context, userID int64) (*dbs.PasswordResetToken, error) {
	var token dbs.PasswordResetToken
	err := d.DB.
		Where("user_id = ? AND deleted_at IS NULL AND used_at IS NULL", userID).
		Order("created_at DESC").
		First(&token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding active password reset token: %w", err)
	}
	return &token, nil
}

func (d *passwordResetDao) IncrementAttempts(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.PasswordResetToken{}).
		Where("id = ?", id).
		Update("attempts", gorm.Expr("attempts + 1")).Error
}

func (d *passwordResetDao) MarkUsed(ctx *gin.Context, id int64) error {
	return d.DB.Model(&dbs.PasswordResetToken{}).
		Where("id = ?", id).
		Update("used_at", gorm.Expr("NOW()")).Error
}

func (d *passwordResetDao) SoftDeleteByUserID(ctx *gin.Context, userID int64) error {
	return d.DB.Model(&dbs.PasswordResetToken{}).
		Where("user_id = ? AND deleted_at IS NULL AND used_at IS NULL", userID).
		Update("deleted_at", gorm.Expr("NOW()")).Error
}
