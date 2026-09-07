package daos

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// SellerConnectionDaoInterface define el acceso a datos de la conexión OAuth
// mp-connect de un entrenador. La clave natural es (user_id, client_id): el
// access token OAuth solo es válido para la app que lo emitió, así que la misma
// cuenta puede tener una conexión por aplicación sin pisarse.
type SellerConnectionDaoInterface interface {
	Upsert(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error)
	FindByUserAndClient(ctx *gin.Context, userID int64, clientID string) (*dbs.SellerConnection, error)
	SetStatus(ctx *gin.Context, userID int64, clientID string, status string) error
	SetStatusByMPUser(ctx *gin.Context, mpUserID int64, status string) error
	FindAuthorizedByUserAndClient(ctx *gin.Context, userID int64, clientID string) (*dbs.SellerConnection, error)
}

type sellerConnectionDao struct {
	DB *gorm.DB
}

func NewSellerConnectionDao(database *gorm.DB) SellerConnectionDaoInterface {
	return &sellerConnectionDao{DB: database}
}

// Upsert inserta o actualiza la conexión de un usuario para una app (client_id).
// La clave natural (user_id, client_id) permite tener una conexión por
// aplicación sin que un reconnect contra otra app pise la vigente. Setea los
// campos de tokens/estado y actualiza updated_at.
func (d *sellerConnectionDao) Upsert(ctx *gin.Context, conn *dbs.SellerConnection) (*dbs.SellerConnection, error) {
	var existing dbs.SellerConnection
	err := d.DB.Where("user_id = ? AND client_id = ?", conn.UserID, conn.ClientID).First(&existing).Error
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
	existing.PublicKey = conn.PublicKey
	existing.TokenExpiresAt = conn.TokenExpiresAt
	existing.Status = conn.Status
	if err := d.DB.Save(&existing).Error; err != nil {
		return nil, fmt.Errorf("error updating seller connection: %w", err)
	}
	return &existing, nil
}

func (d *sellerConnectionDao) FindByUserAndClient(ctx *gin.Context, userID int64, clientID string) (*dbs.SellerConnection, error) {
	var conn dbs.SellerConnection
	err := d.DB.Where("user_id = ? AND client_id = ?", userID, clientID).First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding seller connection: %w", err)
	}
	return &conn, nil
}

// SetStatus actualiza el estado de la conexión de un usuario para una app
// (authorized/deauthorized).
func (d *sellerConnectionDao) SetStatus(ctx *gin.Context, userID int64, clientID string, status string) error {
	return d.DB.Model(&dbs.SellerConnection{}).
		Where("user_id = ? AND client_id = ?", userID, clientID).
		Update("status", status).Error
}

// SetStatusByMPUser actualiza el estado de la conexión localizando por el MP user id
// del vendedor (aquí habla el webhook de desautorización que solo conoce ese id).
// La columna mp_user_id es texto; el webhook la manda como entero.
func (d *sellerConnectionDao) SetStatusByMPUser(ctx *gin.Context, mpUserID int64, status string) error {
	return d.DB.Model(&dbs.SellerConnection{}).
		Where("mp_user_id = ?", strconv.FormatInt(mpUserID, 10)).
		Update("status", status).Error
}

// FindAuthorizedByUserAndClient devuelve la conexión de un usuario para una app
// solo si está authorized (el entrenador debe haber conectado su cuenta para
// cobrar con split).
func (d *sellerConnectionDao) FindAuthorizedByUserAndClient(ctx *gin.Context, userID int64, clientID string) (*dbs.SellerConnection, error) {
	var conn dbs.SellerConnection
	err := d.DB.
		Where("user_id = ? AND client_id = ? AND status = ?", userID, clientID, string(constants.SellerConnectionStatusAuthorized)).
		First(&conn).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error finding authorized seller connection: %w", err)
	}
	return &conn, nil
}
