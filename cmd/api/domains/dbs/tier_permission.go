package dbs

import "time"

// TierPermission es la tabla intermedia que asocia permisos a tiers.
// Cada registro indica que un permiso está asignado a un tier específico.
type TierPermission struct {
	ID             int64      `gorm:"column:id;primaryKey"`                    // ID único de la asignación
	TierID         int64      `gorm:"column:tier_id;not null"`                 // ID del tier al que se asigna el permiso
	PermissionID   int64      `gorm:"column:permission_id;not null"`           // ID del permiso asignado
	AsignationDate time.Time  `gorm:"column:asignation_date;not null"`         // Fecha en que se asignó el permiso al tier
	DeletedAt      *time.Time `gorm:"column:deleted_at"`                       // Fecha de eliminación lógica (nil = activo)
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`        // Fecha de creación
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`        // Fecha de última actualización
}

func (TierPermission) TableName() string {
	return "tier_permissions"
}
