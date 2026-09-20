package dbs

import "time"

// SessionInstance es la copia inmutable de una Session del catálogo, creada al
// asignar contenido a un día de calendario. Sin owner_id/deleted_at/updated_at
// (design.md D1); su detalle vive en session_exercise_instances.
type SessionInstance struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	Name        string    `gorm:"column:name;not null"`
	Description *string   `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (SessionInstance) TableName() string { return "session_instances" }
