package dbs

import "time"

// Group representa un grupo dentro de un equipo.
// Cada grupo pertenece a un solo equipo y puede tener múltiples usuarios asociados.
type Group struct {
	ID          int64      `gorm:"column:id;primaryKey"`               // ID único del grupo (autoincremental)
	Name        string     `gorm:"column:name;not null"`               // Nombre del grupo
	Description string     `gorm:"column:description"`                 // Descripción del grupo
	TeamID      int64      `gorm:"column:team_id;not null"`            // ID del equipo al que pertenece
	IsMain      bool       `gorm:"column:is_main;not null;default:0"`  // Si es el grupo principal del equipo
	DeletedAt   *time.Time `gorm:"column:deleted_at"`                  // Fecha de eliminación lógica (nil = activo)
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`   // Fecha de creación
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`   // Fecha de última actualización
}

func (Group) TableName() string {
	return "groups"
}
