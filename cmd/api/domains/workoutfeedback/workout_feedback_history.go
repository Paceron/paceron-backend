package workoutfeedback

import (
	"time"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// HistoryQuery son los query params de los endpoints de historial
// (workout-feedback-history y administered-workout-feedback-history). Llega
// crudo del controller (strings de fecha, punteros para distinguir "ausente"
// de "fuera de rango") y se valida/normaliza en el service.
type HistoryQuery struct {
	TeamID        *int64
	GroupID       *int64
	ExerciseID    *int64  // id de instancia de ejercicio (filtro de 2do nivel)
	SetNumber     *int    // filtro de 2do nivel
	AthleteUserID *int64  // filtro de 2do nivel, solo endpoint del entrenador
	DateFrom      *string // YYYY-MM-DD
	DateTo        *string // YYYY-MM-DD
	Page          *int
	PageSize      *int
	Sort          string
	Order         string
}

// WorkoutFeedbackHistoryItem es un entrenamiento del historial (design.md D6):
// fila del DAO + nombres resueltos en batch. team_id/team_name/group_id/
// group_name son null en feedbacks huérfanos (sin día de calendario) o sin
// equipo registrado.
type WorkoutFeedbackHistoryItem struct {
	ID                int64      `json:"id"`
	AthleteUserID     int64      `json:"athlete_user_id"`
	AthleteName       string     `json:"athlete_name"`
	TeamID            *int64     `json:"team_id"`
	TeamName          *string    `json:"team_name"`
	GroupID           *int64     `json:"group_id"`
	GroupName         *string    `json:"group_name"`
	Date              string     `json:"date"`
	SessionName       *string    `json:"session_name"`
	ExerciseID        int64      `json:"exercise_id"`
	ExerciseName      *string    `json:"exercise_name"`
	CatalogExerciseID *int64     `json:"catalog_exercise_id"`
	SetNumber         int        `json:"set_number"`
	CompletionStatus  *string    `json:"completion_status"`
	DurationMs        *int64     `json:"duration_ms"`
	ActiveDurationMs  *int64     `json:"active_duration_ms"`
	DistanceMeters    *float64   `json:"distance_meters"`
	StartedAt         *time.Time `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at"`
}

// WorkoutFeedbackHistoryResponse es la respuesta de ambos endpoints de
// historial: página de ítems + total sin paginar + pools de filtros de la UI
// (primer nivel solamente, el DAO los ignora del 2do nivel por diseño D5).
type WorkoutFeedbackHistoryResponse struct {
	Items              []WorkoutFeedbackHistoryItem `json:"items"`
	Total              int64                        `json:"total"`
	Page               int                          `json:"page"`
	PageSize           int                          `json:"page_size"`
	AvailableAthletes  []dbs.IDName                 `json:"available_athletes"`
	AvailableExercises []dbs.IDName                 `json:"available_exercises"`
}
