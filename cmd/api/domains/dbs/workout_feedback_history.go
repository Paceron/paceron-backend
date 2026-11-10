package dbs

import "time"

// IDName es el par genérico id/nombre que devuelven los pools de filtros del
// historial (available_athletes / available_exercises, design.md D5/D6).
type IDName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// WorkoutFeedbackHistoryRow es la fila enriquecida del historial que devuelve el
// DAO con los joins de design.md D1/D2: group_id se deriva del día de calendario
// que referencia la session_instance (LEFT JOIN → nil en feedbacks huérfanos o
// sin día), exercise_name/session_name vienen de las tablas de instancias y
// athlete_name del LEFT JOIN users (nil si el user ya no existe). TotalCount es
// el COUNT(*) OVER() de la consulta: el total sin paginar viaja en cada fila y
// evita el COUNT separado salvo página vacía. Es fila cruda, no DTO: el service
// completa los nombres de equipo/grupo en batch y arma el response.
type WorkoutFeedbackHistoryRow struct {
	ID                int64      `gorm:"column:id"`
	AthleteUserID     int64      `gorm:"column:athlete_user_id"`
	AthleteName       *string    `gorm:"column:athlete_name"`
	TotalCount        int64      `gorm:"column:total_count"`
	TeamID            *int64     `gorm:"column:team_id"`
	GroupID           *int64     `gorm:"column:group_id"`
	IsPresencial      bool       `gorm:"column:is_presencial"`
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
