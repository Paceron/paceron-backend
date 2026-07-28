package dbs

import "time"

// PasswordResetToken representa un código de recuperación de contraseña emitido para un usuario.
// Cada solicitud de "olvidé mi contraseña" crea un registro nuevo; los anteriores se invalidan
// (deleted_at) antes de crear uno nuevo, garantizando un único código activo por usuario.
type PasswordResetToken struct {
	ID        int64      `gorm:"column:id;primaryKey"`             // ID único del código
	UserID    int64      `gorm:"column:user_id;not null;index"`    // ID del usuario al que pertenece el código
	CodeHash  string     `gorm:"column:code_hash;not null"`        // Hash bcrypt del código OTP de 6 dígitos
	ExpiresAt time.Time  `gorm:"column:expires_at;not null"`       // Momento en que el código deja de ser válido
	UsedAt    *time.Time `gorm:"column:used_at"`                   // Momento en que el código fue canjeado (nil = no usado)
	Attempts  int        `gorm:"column:attempts;not null;default:0"` // Intentos fallidos de validación acumulados
	DeletedAt *time.Time `gorm:"column:deleted_at"`                // Fecha de invalidación lógica (nil = activo)
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime"` // Fecha de creación
}

func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}
