package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// RefreshTokenDaoInterface define las operaciones de acceso a datos para sesiones de refresh.
type RefreshTokenDaoInterface interface {
	Create(ctx *gin.Context, token *dbs.RefreshToken) error
	FindActiveByHash(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error)
	Revoke(ctx *gin.Context, id int64, replacedBy *int64) error
	RevokeBySessionID(ctx *gin.Context, sessionID string) error
}

type refreshTokenDao struct {
	DB *gorm.DB
}

// NewRefreshTokenDao crea una nueva instancia de RefreshTokenDao.
func NewRefreshTokenDao(database *gorm.DB) RefreshTokenDaoInterface {
	return &refreshTokenDao{
		DB: database,
	}
}

// Create inserta una nueva sesión de refresh en la base de datos.
func (d *refreshTokenDao) Create(ctx *gin.Context, token *dbs.RefreshToken) error {
	return d.DB.Create(token).Error
}

// FindActiveByHash busca una sesión de refresh activa (no revocada, no vencida) por el
// hash del token.
func (d *refreshTokenDao) FindActiveByHash(ctx *gin.Context, tokenHash string) (*dbs.RefreshToken, error) {
	var token dbs.RefreshToken
	err := d.DB.Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", tokenHash, time.Now()).
		First(&token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding refresh token: %w", err)
	}
	return &token, nil
}

// Revoke marca una sesión de refresh como revocada. replacedBy es opcional (nil si es
// logout, seteado si es rotación por refresh).
func (d *refreshTokenDao) Revoke(ctx *gin.Context, id int64, replacedBy *int64) error {
	updates := map[string]interface{}{"revoked_at": time.Now()}
	if replacedBy != nil {
		updates["replaced_by"] = *replacedBy
	}
	return d.DB.Model(&dbs.RefreshToken{}).Where("id = ?", id).Updates(updates).Error
}

// RevokeBySessionID revoca todas las sesiones de refresh activas de un session_id.
func (d *refreshTokenDao) RevokeBySessionID(ctx *gin.Context, sessionID string) error {
	return d.DB.Model(&dbs.RefreshToken{}).
		Where("session_id = ? AND revoked_at IS NULL", sessionID).
		Update("revoked_at", time.Now()).Error
}
