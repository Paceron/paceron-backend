package dbs

import "time"

// Competition es una competencia del gamificacion-core (design.md D8). group_id (FK
// opaca al patrón del repo: sin constraint, existencia delegada al service) delimita
// la competencia a un grupo; nil = todo el team. Sin campo title: los pushes usan
// texto genérico. La FK opaca runner_session.competition_id apunta acá (BIGINT > 0
// sin constraint, igual que attendances.training_session_id).
type Competition struct {
	ID                     int64      `gorm:"column:id;primaryKey"`
	TeamID                 int64      `gorm:"column:team_id;not null;index:idx_competition_team,priority:1"`
	GroupID                *int64     `gorm:"column:group_id;index:idx_competition_team,priority:2"`
	Type                   string     `gorm:"column:type;type:varchar(16);not null"`
	Metric                 *string    `gorm:"column:metric;type:varchar(32)"`
	StartAt                time.Time  `gorm:"column:start_at;not null"`
	EndAt                  *time.Time `gorm:"column:end_at"`
	RouteGeoJSON           *string    `gorm:"column:route_geojson;type:text"`
	AcceptanceRadiusMeters *float64   `gorm:"column:acceptance_radius_meters"`
	CreatedByUserID        int64      `gorm:"column:created_by_user_id;not null"`
	CreatedByRole          string     `gorm:"column:created_by_role;type:varchar(16);not null"`
	Status                 string     `gorm:"column:status;type:varchar(32);not null"`
	ApprovedByUserID       *int64     `gorm:"column:approved_by_user_id"`
	RejectionReason        *string    `gorm:"column:rejection_reason;type:text"`
	FinishedAt             *time.Time `gorm:"column:finished_at"`
	CreatedAt              time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt              time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (Competition) TableName() string {
	return "competition"
}

// CompetitionMetricAgg es el acumulado por atleta de las métricas fuente de
// feedback dentro de la ventana de una competencia (detalle on-demand D9 y
// cierre E4.2). Los nulls significan "nunca reportó esa métrica".
type CompetitionMetricAgg struct {
	UserID        int64    `gorm:"column:user_id"`
	SumDistance   *float64 `gorm:"column:sum_distance"`
	SumDurationMs *int64   `gorm:"column:sum_duration_ms"`
	MaxPeak       *float64 `gorm:"column:max_peak"`
}
