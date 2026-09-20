package dbs

// SessionExerciseInstance vincula una SessionInstance con un ExerciseInstance y
// su rol — copia inmutable de SessionExercise (design.md D1).
type SessionExerciseInstance struct {
	ID                 int64  `gorm:"column:id;primaryKey"`
	SessionInstanceID  int64  `gorm:"column:session_instance_id;not null"`
	ExerciseInstanceID int64  `gorm:"column:exercise_instance_id;not null"`
	Role               string `gorm:"column:role;not null"`
	RepeatCount        int    `gorm:"column:repeat_count;not null;default:1"`
	RestMinutes        int    `gorm:"column:rest_minutes;not null;default:0"`
}

func (SessionExerciseInstance) TableName() string { return "session_exercise_instances" }
