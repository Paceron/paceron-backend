package dbs

import "time"

// SessionInstance es la copia inmutable de una Session del catálogo, creada al
// asignar contenido a un día de calendario. Sin owner_id/deleted_at/updated_at
// (design.md D1); su detalle vive en session_exercise_instances.
// SourceSessionID es una referencia informativa (opaca, app-managed, sin FK de
// DB) a la Session del catálogo de la que se instanció; NULL en instancias
// anteriores al change instancia-referencia-catalogo.
type SessionInstance struct {
	ID              int64     `gorm:"column:id;primaryKey"`
	Name            string    `gorm:"column:name;not null"`
	Description     *string   `gorm:"column:description"`
	SourceSessionID *int64    `gorm:"column:source_session_id"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (SessionInstance) TableName() string { return "session_instances" }
