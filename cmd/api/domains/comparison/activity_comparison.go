// Package comparison contiene los DTOs de la comparación planificado vs
// realizado (change comparacion-planificado-realizado).
package comparison

// Estados de una serie en la comparación.
const (
	SetStatusCompleted    = "completed"
	SetStatusSkipped      = "skipped"
	SetStatusUnregistered = "unregistered"
)

// SessionStatusNone es el estado de una sesión sin runner_session.
const SessionStatusNone = "none"

// PlannedTarget es el objetivo planificado de un ejercicio.
type PlannedTarget struct {
	Sets         int      `json:"sets"`
	Minutes      *int     `json:"minutes"`
	DistanceM    *int     `json:"distance_m"`
	SpeedKph     *float64 `json:"speed_kph"`
	PaceSecPerKm *float64 `json:"pace_sec_per_km"`
}

// SetComparison compara una serie planificada (o una registrada de más) con lo
// que registró el atleta.
type SetComparison struct {
	SetNumber           int      `json:"set_number"`
	Status              string   `json:"status"`
	Extra               bool     `json:"extra"`
	FeedbackID          *int64   `json:"feedback_id"`
	DurationMs          *int64   `json:"duration_ms"`
	DistanceMeters      *float64 `json:"distance_meters"`
	PaceSecPerKm        *float64 `json:"pace_sec_per_km"`
	RPE                 *int16   `json:"rpe"`
	CompliancePct       *float64 `json:"compliance_pct"`
	DeviationDurationMs *int64   `json:"deviation_duration_ms"`
	DeviationDistanceM  *float64 `json:"deviation_distance_m"`
}

// ExerciseComparison compara un ejercicio planificado de la sesión.
type ExerciseComparison struct {
	ExerciseInstanceID int64           `json:"exercise_instance_id"`
	Name               string          `json:"name"`
	Role               string          `json:"role"`
	Kind               string          `json:"kind"`
	Intensity          *string         `json:"intensity"`
	HasTarget          bool            `json:"has_target"`
	Planned            PlannedTarget   `json:"planned"`
	Sets               []SetComparison `json:"sets"`
	CompletedSets      int             `json:"completed_sets"`
	SkippedSets        int             `json:"skipped_sets"`
	CompliancePct      float64         `json:"compliance_pct"`
}

// SessionTotals son los totales planificados y reales de una sesión.
// PlannedDurationMs y PlannedDistanceM son nil si ningún ejercicio tiene ese
// tipo de objetivo.
type SessionTotals struct {
	PlannedSets       int      `json:"planned_sets"`
	CompletedSets     int      `json:"completed_sets"`
	SkippedSets       int      `json:"skipped_sets"`
	UnregisteredSets  int      `json:"unregistered_sets"`
	PlannedDurationMs *int64   `json:"planned_duration_ms"`
	ActualDurationMs  int64    `json:"actual_duration_ms"`
	PlannedDistanceM  *float64 `json:"planned_distance_m"`
	ActualDistanceM   float64  `json:"actual_distance_m"`
}

// SessionHeader son los datos de cabecera de una sesión comparada.
type SessionHeader struct {
	SessionInstanceID int64    `json:"session_instance_id"`
	Date              string   `json:"date"`
	SessionName       string   `json:"session_name"`
	TeamID            *int64   `json:"team_id"`
	TeamName          *string  `json:"team_name"`
	GroupID           *int64   `json:"group_id"`
	GroupName         *string  `json:"group_name"`
	IsPresencial      bool     `json:"is_presencial"`
	Kinds             []string `json:"kinds"`
	Status            string   `json:"status"`
}

// SessionComparisonResponse es la respuesta de
// GET /api/v1/activities/comparison/:sessionInstanceId.
type SessionComparisonResponse struct {
	SessionHeader
	AthleteUserID int64                `json:"athlete_user_id"`
	StartedAt     *string              `json:"started_at"`
	EndedAt       *string              `json:"ended_at"`
	Exercises     []ExerciseComparison `json:"exercises"`
	Totals        SessionTotals        `json:"totals"`
	CompliancePct float64              `json:"compliance_pct"`
}

// SessionComparisonSummary es una sesión de la lista por rango.
type SessionComparisonSummary struct {
	SessionHeader
	SessionTotals
	CompliancePct float64 `json:"compliance_pct"`
}

// RangeSummary resume el cumplimiento de las sesiones de la lista.
type RangeSummary struct {
	SessionsPlanned      int     `json:"sessions_planned"`
	SessionsWithActivity int     `json:"sessions_with_activity"`
	AvgCompliancePct     float64 `json:"avg_compliance_pct"`
}

// ComparisonListResponse es la respuesta de GET /api/v1/activities/comparison.
type ComparisonListResponse struct {
	DateFrom string                     `json:"date_from"`
	DateTo   string                     `json:"date_to"`
	Sessions []SessionComparisonSummary `json:"sessions"`
	Summary  RangeSummary               `json:"summary"`
}

// ListQuery son los parámetros de la lista por rango, ya parseados por el
// controller. Las fechas llegan como texto y las valida el service.
type ListQuery struct {
	DateFrom      string
	DateTo        string
	TeamID        *int64
	GroupID       *int64
	Kind          string
	AthleteUserID *int64
}
