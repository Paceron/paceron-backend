package dbs

import "time"

// Session es una plantilla de entrenamiento reusable, compuesta por N
// SessionExercise (tabla propia, no array embebido).
type Session struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	OwnerID     int64      `gorm:"column:owner_id;not null"`
	Name        string     `gorm:"column:name;not null"`
	Description *string    `gorm:"column:description"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Session) TableName() string { return "sessions" }
