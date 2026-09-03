package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// SellerConnectionDaoInterface define el acceso a datos de la conexión OAuth
// mp-connect de un entrenador.
type SellerConnectionDaoInterface interface {
	Upsert(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error)
	FindByUser(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error)
	SetStatus(ctx *gin.Context, userID int64, status string) error
	SetStatusByMPUser(ctx *gin.Context, mpUserID int64, status string) error
	FindAuthorizedByUser(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error)
}

type sellerConnectionDao struct {
	DB *gorm.DB
}

func NewSellerConnectionDao(database *gorm.DB) SellerConnectionDaoInterface {
	return &sellerConnectionDao{DB: database}
}

// Upsert inserta o actualiza la conexión de un usuario (user_id es único). Setea
// los campos de tokens/estado y actualiza updated_at.
func (d *sellerConnectionDao) Upsert(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error) {
	var existing dbs.SellerConnection
	err := d.DB.Where("user_id = ?", conn.UserID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		if err := d.DB.Create(conn).Error; err != nil {
			return nil, fmt.Errorf("error creating seller connection: %w", err)
		}
		return conn, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error finding seller connection: %w", err)
	}

	existing.MPUserID = conn.MPUserID
	existing.AccessToken = conn.AccessToken
	existing.RefreshToken = conn.RefreshToken
	existing.TokenExpiresAt = conn.TokenExpiresAt
	existing.Status = conn.Status
	if err := d.DB.Save(&existing).Error; err != nil {
		return nil, fmt.Errorf("error updating seller connection: %w", err)
	}
	return &existing, nil
}

func (d *sellerConnectionDao) FindByUser(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
	var conn dbs.SellerConnection
	err := d.DB.Where("user_id = ?", userID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding seller connection: %w", err)
	}
	return &conn, nil
}

// SetStatus actualiza el estado de la conexión de un usuario (authorized/deauthorized).
func (d *sellerConnectionDao) SetStatus(ctx *gin.Context, userID int64, status string) error {
	return d.DB.Model(&dbs.SellerConnection{}).
		Where("user_id = ?", userID).
		Update("status", status).Error
}

// SetStatusByMPUser actualiza el estado de la conexión localizando por el MP user id
// del vendedor (aquí habla el webhook de desautorización que solo conoce ese id).
func (d *sellerConnectionDao) SetStatusByMPUser(ctx *gin.Context, mpUserID int64, status string) error {
	return d.DB.Model(&dbs.SellerConnection{}).
		Where("mp_user_id = ?", mpUserID).
		Update("status", status).Error
}

// FindAuthorizedByUser devuelve la conexión de un usuario solo si está authorized
// (el entrenador debe haber conectado su cuenta para cobrar con split).
func (d *sellerConnectionDao) FindAuthorizedByUser(ctx *gin.Context, userID int64) (*dbs.SellerConnection, error) {
	var conn dbs.SellerConnection
	err := d.DB.
		Where("user_id = ? AND status = ?", userID, string(constants.SellerConnectionStatusAuthorized)).
		First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding authorized seller connection: %w", err)
	}
	return &conn, nil
}
