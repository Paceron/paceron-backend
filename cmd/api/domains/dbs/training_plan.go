package dbs

import "time"

// TrainingPlan es un template reusable, sin caducidad propia. Se borra
// físico (no soft-delete) — ver design.md D1.
type TrainingPlan struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	OwnerID     int64     `gorm:"column:owner_id;not null"`
	Name        string    `gorm:"column:name;not null"`
	Description *string   `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TrainingPlan) TableName() string { return "training_plans" }
