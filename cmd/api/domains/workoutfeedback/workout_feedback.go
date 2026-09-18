package workoutfeedback

import "time"

// Mensajes de respuesta fijos del módulo.
const (
	MsgFeedbackCreated = "feedback registrado"
	MsgFeedbackUpdated = "feedback actualizado"
	MsgFeedbackDeleted = "feedback eliminado"
)

// CreateFeedbackRequest es el body de POST /api/v1/workout-feedback. El
// feedback_owner_user_id nunca viaja en el body: siempre es el auth_user_id del
// token. athlete_user_id es opcional: si no viene, el atleta es el auth_user_id.
// session_date viaja como string YYYY-MM-DD y se parsea en el controller.
type CreateFeedbackRequest struct {
	TeamID             *int64    `json:"team_id"`
	AssignedSessionID  int64     `json:"assigned_session_id" binding:"required"`
	AssignedExerciseID int64     `json:"assigned_exercise_id" binding:"required"`
	AthleteUserID      *int64    `json:"athlete_user_id"`
	ReportSource       string    `json:"report_source" binding:"required"`
	SessionDate        string    `json:"session_date" binding:"required"`
	SetNumber          int       `json:"set_number"`
	StartedAt          *time.Time `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at"`
	DurationMs         *int64    `json:"duration_ms"`
	ActiveDurationMs   *int64    `json:"active_duration_ms"`
	WeightKg           *float64  `json:"weight_kg"`
	Reps               *int      `json:"reps"`
	DistanceMeters     *float64  `json:"distance_meters"`
	RPE                *int16    `json:"rpe"`
	AvgHeartRate       *int16    `json:"avg_heart_rate"`
	MaxHeartRate       *int16    `json:"max_heart_rate"`
	CompletionStatus   *string   `json:"completion_status"`
	ElevationGainMeters *float64 `json:"elevation_gain_meters"`
	Cadence            *int16    `json:"cadence"`
	Annotations        *string   `json:"annotations"`
	MediaURLs          []string  `json:"media_urls"`
}

// UpdateFeedbackRequest es el body de PUT /api/v1/workout-feedback/:id. Todos los
// campos son opcionales (edit parcial). athlete_user_id y feedback_owner_user_id
// NO son editables.
type UpdateFeedbackRequest struct {
	TeamID             *int64    `json:"team_id"`
	AssignedSessionID  *int64    `json:"assigned_session_id"`
	AssignedExerciseID *int64    `json:"assigned_exercise_id"`
	ReportSource       *string   `json:"report_source"`
	SessionDate        *string   `json:"session_date"`
	SetNumber          *int      `json:"set_number"`
	StartedAt          *time.Time `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at"`
	DurationMs         *int64    `json:"duration_ms"`
	ActiveDurationMs   *int64    `json:"active_duration_ms"`
	WeightKg           *float64  `json:"weight_kg"`
	Reps               *int      `json:"reps"`
	DistanceMeters     *float64  `json:"distance_meters"`
	RPE                *int16    `json:"rpe"`
	AvgHeartRate       *int16    `json:"avg_heart_rate"`
	MaxHeartRate       *int16    `json:"max_heart_rate"`
	CompletionStatus   *string   `json:"completion_status"`
	ElevationGainMeters *float64 `json:"elevation_gain_meters"`
	Cadence            *int16    `json:"cadence"`
	Annotations        *string   `json:"annotations"`
	MediaURLs          []string  `json:"media_urls"`
}

// WorkoutFeedbackResponse es la respuesta de detalle/lista: shape plano espejo del
// modelo con media_urls como []string (el modelo usa pgtype.TextArray, JSON feo).
type WorkoutFeedbackResponse struct {
	ID                  int64    `json:"id"`
	TeamID              *int64   `json:"team_id"`
	AssignedSessionID   int64    `json:"assigned_session_id"`
	AssignedExerciseID  int64    `json:"assigned_exercise_id"`
	AthleteUserID       int64    `json:"athlete_user_id"`
	FeedbackOwnerUserID int64    `json:"feedback_owner_user_id"`
	ReportSource        string   `json:"report_source"`
	SessionDate         string   `json:"session_date"`
	SetNumber           int      `json:"set_number"`
	StartedAt           *time.Time `json:"started_at"`
	EndedAt             *time.Time `json:"ended_at"`
	DurationMs          *int64   `json:"duration_ms"`
	ActiveDurationMs    *int64   `json:"active_duration_ms"`
	WeightKg            *float64 `json:"weight_kg"`
	Reps                *int     `json:"reps"`
	DistanceMeters      *float64 `json:"distance_meters"`
	RPE                 *int16   `json:"rpe"`
	AvgHeartRate        *int16   `json:"avg_heart_rate"`
	MaxHeartRate        *int16   `json:"max_heart_rate"`
	CompletionStatus    *string  `json:"completion_status"`
	ElevationGainMeters *float64 `json:"elevation_gain_meters"`
	Cadence             *int16   `json:"cadence"`
	Annotations         *string  `json:"annotations"`
	MediaURLs           []string `json:"media_urls"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

// MutationResponse es la respuesta de create/update/delete (mensaje + recurso).
type MutationResponse struct {
	Message string                 `json:"message"`
	Data    *WorkoutFeedbackResponse `json:"data,omitempty"`
}

// SearchFilters son los query params de GET /api/v1/workout-feedback/search,
// parseados y validados en el controller.
type SearchFilters struct {
	TeamID             *int64
	AthleteUserID      *int64
	FeedbackOwnerUserID *int64
	AssignedSessionID  *int64
	AssignedExerciseID *int64
	SessionDateFrom    *string // YYYY-MM-DD
	SessionDateTo      *string // YYYY-MM-DD
}

// SearchResponse es la respuesta de GET /search: lista de feedbacks autorizados.
type SearchResponse struct {
	Data []WorkoutFeedbackResponse `json:"data"`
}