package dbs

// SessionExercise vincula una Session con un Exercise y su rol dentro de la
// sesión. Sin campo de orden explícito — el ID autoincremental desempata.
type SessionExercise struct {
	ID          int64 `gorm:"column:id;primaryKey"`
	SessionID   int64 `gorm:"column:session_id;not null"`
	ExerciseID  int64 `gorm:"column:exercise_id;not null"`
	Role        string `gorm:"column:role;not null"`
	RepeatCount int   `gorm:"column:repeat_count;not null;default:1"`
	RestMinutes int   `gorm:"column:rest_minutes;not null;default:0"`
}

func (SessionExercise) TableName() string { return "session_exercises" }
