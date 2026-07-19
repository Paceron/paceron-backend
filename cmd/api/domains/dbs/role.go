package dbs

import "time"

// Role representa un rol del sistema.
// Cada rol agrupa un conjunto de tiers que definen los niveles de acceso.
// Ejemplo: "corredor", "entrenador", "admin".
type Role struct {
	ID          int64      `gorm:"column:id;primaryKey"`                    // ID único del rol
	Name        string     `gorm:"column:name;uniqueIndex;not null"`        // Nombre único del rol (ej: "corredor")
	Description string     `gorm:"column:description"`                      // Descripción opcional del rol
	DeletedAt   *time.Time `gorm:"column:deleted_at"`                       // Fecha de eliminación lógica (nil = activo)
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`        // Fecha de creación
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`        // Fecha de última actualización
}

func (Role) TableName() string {
	return "roles"
}
