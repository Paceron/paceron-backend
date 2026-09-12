package dbs

import "time"

// PlanDay es un día secuencial (1..N) de un TrainingPlan. Los campos
// default_* son informativos para el momento del stamp (ver el change de
// calendario), no afectan nada del catálogo en sí.
type PlanDay struct {
	ID                int64      `gorm:"column:id;primaryKey"`
	PlanID            int64      `gorm:"column:plan_id;not null"`
	SequenceNo        int        `gorm:"column:sequence_no;not null"`
	Kind              string     `gorm:"column:kind;not null"`
	OtherName         *string    `gorm:"column:other_name"`
	SessionID         *int64     `gorm:"column:session_id"`
	DefaultPresencial bool       `gorm:"column:default_presencial;not null;default:false"`
	DefaultTime       *time.Time `gorm:"column:default_time;type:time"`
	DefaultLocation   *string    `gorm:"column:default_location;type:jsonb"`
}

func (PlanDay) TableName() string { return "plan_days" }
