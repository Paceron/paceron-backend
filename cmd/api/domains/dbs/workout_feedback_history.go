package dbs

import "time"

// IDName es el par genérico id/nombre que devuelven los pools de filtros del
// historial (available_athletes / available_exercises, design.md D5/D6).
type IDName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// WorkoutFeedbackHistoryRow es la fila enriquecida del historial que devuelve el
// DAO con los joins de design.md D1: group_id se deriva del día de calendario que
// referencia la session_instance (LEFT JOIN → nil en feedbacks huérfanos o sin
// día), exercise_name/session_name vienen de las tablas de instancias. Es fila
// cruda, no DTO: el service completa los nombres restantes (atleta/equipo/grupo)
// en batch y arma el response.
type WorkoutFeedbackHistoryRow struct {
	ID                int64      `gorm:"column:id"`
	AthleteUserID     int64      `gorm:"column:athlete_user_id"`
	TeamID            *int64     `gorm:"column:team_id"`
	GroupID           *int64     `gorm:"column:group_id"`
	SessionDate       time.Time  `gorm:"column:session_date"`
	SetNumber         int        `gorm:"column:set_number"`
	CompletionStatus  *string    `gorm:"column:completion_status"`
	DurationMs        *int64     `gorm:"column:duration_ms"`
	ActiveDurationMs  *int64     `gorm:"column:active_duration_ms"`
	DistanceMeters    *float64   `gorm:"column:distance_meters"`
	StartedAt         *time.Time `gorm:"column:started_at"`
	EndedAt           *time.Time `gorm:"column:ended_at"`
	ExerciseID        int64      `gorm:"column:exercise_id"`
	ExerciseName      *string    `gorm:"column:exercise_name"`
	CatalogExerciseID *int64     `gorm:"column:catalog_exercise_id"`
	SessionInstanceID int64      `gorm:"column:session_instance_id"`
	SessionName       *string    `gorm:"column:session_name"`
}
