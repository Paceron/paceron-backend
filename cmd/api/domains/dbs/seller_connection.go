package dbs

import "time"

// SellerConnection guarda la conexión OAuth mp-connect de un entrenador con su
// cuenta de Mercado Pago para poder cobrar las mensualidades de su equipo con
// split payments. Los tokens se persisten CIFRADOS en repositorio (AES-GCM vía
// infrastructure/crypto) y nunca se loguean ni se exponen en ninguna respuesta.
type SellerConnection struct {
	ID             int64      `gorm:"column:id;primaryKey"`
	UserID         int64      `gorm:"column:user_id;not null;uniqueIndex"` // entrenador (un solo dueño de equipo por cuenta)
	MPUserID       string     `gorm:"column:mp_user_id"`                   // id de la cuenta de Mercado Pago conectada
	AccessToken    string     `gorm:"column:access_token;type:text"`       // CIFRADO
	RefreshToken   string     `gorm:"column:refresh_token;type:text"`      // CIFRADO
	TokenExpiresAt *time.Time `gorm:"column:token_expires_at"`
	Status         string     `gorm:"column:status;not null;default:authorized"` // authorized | deauthorized
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (SellerConnection) TableName() string {
	return "seller_connections"
}