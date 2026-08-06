package dbs

import "time"

// RefreshToken representa una sesión de refresh persistida. El token en sí nunca se
// guarda en texto plano, solo su hash (SHA256) para poder buscarlo por igualdad exacta.
// SessionID se mantiene constante a través de toda la cadena de rotación de una misma
// sesión (login → refresh → refresh → ...), útil a futuro para listar/revocar sesiones
// activas por dispositivo sin depender del ID de fila.
type RefreshToken struct {
	ID         int64      `gorm:"column:id;primaryKey"`
	UserID     int64      `gorm:"column:user_id;not null;index"`
	SessionID  string     `gorm:"column:session_id;not null;index"`
	TokenHash  string     `gorm:"column:token_hash;not null;uniqueIndex"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	ReplacedBy *int64     `gorm:"column:replaced_by"`
	IP         string     `gorm:"column:ip"`
	UserAgent  string     `gorm:"column:user_agent"`
	CreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
