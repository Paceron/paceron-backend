package dbs

import "time"

// WorkoutFeedbackPoint es un punto del recorrido GPS de una serie de
// workout_feedback. feedback_id es una FK opaca al feedback activo (sin
// constraint ni cascade — si el feedback se soft-borrase, los puntos se
// conservan como historial). "Un punto por posición de serie" lo garantiza el
// índice único parcial uq_feedback_point_order (feedback_id, "order") definido
// en postgres.go (SQL crudo, como unique_feedback_per_set); por eso el struct
// no lleva tag index. session_instance_id / exercise_instance_id van
// denormalizadas para consultar por sesión/ejercicio sin join. "order" se llama
// así en DB (ORDER es palabra reservada; GORM la escapa siendo el campo Order).
type WorkoutFeedbackPoint struct {
	ID                 int64     `gorm:"column:id;primaryKey"`
	FeedbackID         int64     `gorm:"column:feedback_id;not null"`
	SessionInstanceID  int64     `gorm:"column:session_instance_id;not null"`
	ExerciseInstanceID int64     `gorm:"column:exercise_instance_id;not null"`
	Order              int       `gorm:"column:order;not null"`
	Latitude           float64   `gorm:"column:latitude;not null"`
	Longitude          float64   `gorm:"column:longitude;not null"`
	RecordedAt         time.Time `gorm:"column:recorded_at;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (WorkoutFeedbackPoint) TableName() string {
	return "workout_feedback_points"
}