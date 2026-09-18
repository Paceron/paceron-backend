package dbs

import (
	"time"

	"github.com/jackc/pgtype"
)

// WorkoutFeedback representa el feedback de un entrenamiento realizado por un
// atleta, reportado por el propio atleta o por un entrenador (feedback_owner).
// assigned_session_id / assigned_exercise_id son FK opacas: las tablas de
// asignación (assigned_*) no existen aún y las constraints reales se agregarán en
// el change que las introduzca. La unicidad de "un feedback activo por set" la
// garantiza el índice único parcial unique_feedback_per_set (raw SQL en
// postgres.go) con WHERE deleted_at IS NULL — el soft-delete no bloquea recrear el
// mismo set. route_summary (GEOGRAPHY postgis) queda postergado.
type WorkoutFeedback struct {
	ID                  int64            `gorm:"column:id;primaryKey"`                                                                                                   // ID del feedback (autoincremental)
	TeamID              *int64           `gorm:"column:team_id;index:idx_feedback_team_date,priority:1"`                                                                  // Equipo que agrupa el contexto organizacional (nullable)
	AssignedSessionID   int64            `gorm:"column:assigned_session_id;not null;index:idx_feedback_session_exercise,priority:1"`                                      // Sesión asignada (FK opaca, > 0)
	AssignedExerciseID  int64            `gorm:"column:assigned_exercise_id;not null;index:idx_feedback_session_exercise,priority:2"`                                     // Ejercicio asignado (FK opaca, > 0)
	AthleteUserID       int64            `gorm:"column:athlete_user_id;not null;index:idx_feedback_athlete_date,priority:1"`                                              // Atleta que ejecutó el entrenamiento
	FeedbackOwnerUserID int64            `gorm:"column:feedback_owner_user_id;not null"`                                                                                 // Usuario que reportó el feedback (atleta o entrenador)
	ReportSource        string           `gorm:"column:report_source;not null"`                                                                                           // Origen del reporte
	SessionDate         time.Time        `gorm:"column:session_date;type:date;not null;index:idx_feedback_team_date,priority:2;index:idx_feedback_athlete_date,priority:2"` // Fecha de la sesión
	SetNumber           int              `gorm:"column:set_number;not null;default:0;index:idx_feedback_session_exercise,priority:3"`                                     // Número de serie dentro del set
	StartedAt           *time.Time       `gorm:"column:started_at"`                                                                                                       // Inicio (tiempo de pared)
	EndedAt             *time.Time       `gorm:"column:ended_at"`                                                                                                         // Fin (tiempo de pared)
	DurationMs          *int64           `gorm:"column:duration_ms"`                                                                                                      // Duración en ms
	ActiveDurationMs    *int64           `gorm:"column:active_duration_ms"`                                                                                               // Tiempo activo en ms
	WeightKg            *float64         `gorm:"column:weight_kg"`                                                                                                        // Carga en kg
	Reps                *int             `gorm:"column:reps"`                                                                                                             // Repeticiones
	DistanceMeters      *float64         `gorm:"column:distance_meters"`                                                                                                  // Distancia en m
	RPE                 *int16           `gorm:"column:rpe"`                                                                                                              // RPE 1..10 (CHECK en DB)
	AvgHeartRate        *int16           `gorm:"column:avg_heart_rate"`                                                                                                   // Pulso promedio
	MaxHeartRate        *int16           `gorm:"column:max_heart_rate"`                                                                                                   // Pulso máximo
	CompletionStatus    *string          `gorm:"column:completion_status"`                                                                                                // Estado de completado
	ElevationGainMeters *float64         `gorm:"column:elevation_gain_meters"`                                                                                            // Desnivel acumulado en m
	Cadence             *int16           `gorm:"column:cadence"`                                                                                                          // Cadencia
	Annotations         *string          `gorm:"column:annotations;type:text"`                                                                                            // Notas del entrenador/atleta
	MediaURLs           pgtype.TextArray `gorm:"column:media_urls;type:text[]"`                                                                                           // URLs de multimedia (TEXT[])
	CreatedAt           time.Time        `gorm:"column:created_at;autoCreateTime"`                                                                                        // Fecha de creación
	UpdatedAt           time.Time        `gorm:"column:updated_at;autoUpdateTime"`                                                                                        // Fecha de última modificación
	DeletedAt           *time.Time       `gorm:"column:deleted_at"`                                                                                                       // Baja lógica (nil = activo)
}

func (WorkoutFeedback) TableName() string {
	return "workout_feedback"
}