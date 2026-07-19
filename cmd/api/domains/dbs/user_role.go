package dbs

import "time"

// UserRole representa la asignación de un rol a un usuario.
// Cada registro indica que un usuario tiene un rol con un tier específico.
// Si no se especifica tier_id, se usa el tier por defecto "base" del rol.
type UserRole struct {
	ID             int64      `gorm:"column:id;primaryKey"`                    // ID único de la asignación
	UserID         int64      `gorm:"column:user_id;not null"`                 // ID del usuario al que se asigna el rol
	RoleID         int64      `gorm:"column:role_id;not null"`                 // ID del rol asignado
	TierID         int64      `gorm:"column:tier_id;not null"`                 // ID del tier asignado (default: "base" del rol)
	AssignmentDate time.Time  `gorm:"column:assignment_date;not null"`         // Fecha en que se asignó el rol al usuario
	DateSince      *time.Time `gorm:"column:date_since"`                       // Fecha desde la que es válido (nil = desde la asignación)
	DateTo         *time.Time `gorm:"column:date_to"`                          // Fecha hasta la que es válido (nil = sin expiración)
	Status         string     `gorm:"column:status;not null;default:active"`   // Estado de la asignación (active, inactive)
	DeletedAt      *time.Time `gorm:"column:deleted_at"`                       // Fecha de eliminación lógica (nil = activo)
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`        // Fecha de creación
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`        // Fecha de última actualización
}

func (UserRole) TableName() string {
	return "user_roles"
}
