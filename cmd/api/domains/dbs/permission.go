package dbs

import "time"

// Permission representa un permiso del sistema.
// Cada permiso define una capacidad específica que puede ser asignada a un tier.
type Permission struct {
	ID          int64      `gorm:"column:id;primaryKey"`                    // ID único del permiso
	Name        string     `gorm:"column:name;uniqueIndex;not null"`        // Nombre único del permiso (ej: "crear_venta")
	Description string     `gorm:"column:description"`                      // Descripción opcional del permiso
	DeletedAt   *time.Time `gorm:"column:deleted_at"`                       // Fecha de eliminación lógica (nil = activo)
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`        // Fecha de creación
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`        // Fecha de última actualización
}

func (Permission) TableName() string {
	return "permissions"
}
