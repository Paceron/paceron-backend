package dbs

import "time"

// Tier representa un nivel de acceso dentro de un rol.
// Cada tier pertenece a un rol específico y contiene un conjunto de permisos.
// Ejemplo: el tier "premium" del rol "corredor" es diferente al tier "premium" del rol "entrenador".
type Tier struct {
	ID          int64      `gorm:"column:id;primaryKey"`                    // ID único del tier
	Name        string     `gorm:"column:name;not null"`                    // Nombre del tier (ej: "base", "premium")
	Description string     `gorm:"column:description"`                      // Descripción opcional del tier
	RoleID      int64      `gorm:"column:role_id;not null"`                 // ID del rol al que pertenece este tier
	RoleName    string     `gorm:"column:role_name;not null"`               // Nombre del rol (redundante, para consultas sin JOIN)
	DeletedAt   *time.Time `gorm:"column:deleted_at"`                       // Fecha de eliminación lógica (nil = activo)
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`        // Fecha de creación
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`        // Fecha de última actualización
}

func (Tier) TableName() string {
	return "tiers"
}
