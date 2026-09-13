package dbs

import "time"

// Exercise es un ítem del catálogo reusable de un entrenador. kind/intensity/
// muscle_group son independientes entre sí — sin combinación obligatoria.
type Exercise struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	OwnerID     int64      `gorm:"column:owner_id;not null"`
	Name        string     `gorm:"column:name;not null"`
	Description *string    `gorm:"column:description"`
	Kind        string     `gorm:"column:kind;not null"`
	Intensity   *string    `gorm:"column:intensity"`
	Minutes     *int       `gorm:"column:minutes"`
	DistanceM   *int       `gorm:"column:distance_m"`
	SpeedKph    *float64   `gorm:"column:speed_kph;type:numeric(4,1)"`
	MuscleGroup *string    `gorm:"column:muscle_group"`
	VideoURL    *string    `gorm:"column:video_url"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Exercise) TableName() string { return "exercises" }
