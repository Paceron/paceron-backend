package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type PushTokenDaoInterface interface {
	Upsert(ctx *gin.Context, userID int64, token, platform string) error
	FindByUserID(ctx *gin.Context, userID int64) ([]dbs.PushToken, error)
}

type pushTokenDao struct {
	DB *gorm.DB
}

func NewPushTokenDao(database *gorm.DB) PushTokenDaoInterface {
	return &pushTokenDao{
		DB: database,
	}
}

// Upsert crea o actualiza el token de un dispositivo. La clave de conflicto es
// Token, no UserID: si el mismo dispositivo se loguea con otra cuenta, este mismo
// registro reescribe el dueño, sin depender de que el cliente llame algo en logout.
func (d *pushTokenDao) Upsert(ctx *gin.Context, userID int64, token, platform string) error {
	pt := dbs.PushToken{
		UserID:   userID,
		Token:    token,
		Platform: platform,
	}
	err := d.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "platform", "updated_at"}),
	}).Create(&pt).Error
	if err != nil {
		return fmt.Errorf("error upserting push token: %w", err)
	}
	return nil
}

// FindByUserID devuelve todos los tokens de un usuario (puede tener 1+ dispositivos).
func (d *pushTokenDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
	var tokens []dbs.PushToken
	err := d.DB.Where("user_id = ?", userID).Find(&tokens).Error
	if err != nil {
		return nil, fmt.Errorf("error finding push tokens: %w", err)
	}
	return tokens, nil
}
