package dbs

import "time"

// GroupUser representa la asociación de un usuario a un grupo.
// Cada registro indica que un usuario pertenece a un grupo, con fechas de
// inicio y fin de la membresía (fin nil = membresía activa).
type GroupUser struct {
	ID            int64      `gorm:"column:id;primaryKey"`               // ID único de la asociación
	GroupID       int64      `gorm:"column:group_id;not null"`           // ID del grupo
	UserID        int64      `gorm:"column:user_id;not null"`            // ID del usuario
	DateStart     time.Time  `gorm:"column:date_start;not null"`         // Fecha de inicio de la membresía
	DateEnd       *time.Time `gorm:"column:date_end"`                    // Fecha de fin de la membresía (nil = activa)
	DeletedAt     *time.Time `gorm:"column:deleted_at"`                  // Fecha de eliminación lógica (nil = activo)
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`   // Fecha de creación
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime"`   // Fecha de última actualización
}

func (GroupUser) TableName() string {
	return "group_users"
}
