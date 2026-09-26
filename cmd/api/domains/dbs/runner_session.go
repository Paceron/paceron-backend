package dbs

import "time"

// RunnerSession es el estado de una sesión asignada por atleta: wip (se dio
// Play o se ingresó al registro manual) o finished (se completaron todas las
// series). session_instance_id / athlete_user_id son FK opacas (sin constraint
// de DB, integridad gobernada por el service — mismo patrón que
// workout_feedback.assigned_session_id). El constraint único real
// uq_runner_session_session_athlete (UNIQUE plano, no parcial) lo emite
// AutoMigrate desde los tags: un corredor tiene un solo estado por sesión, y
// dos atletas pueden compartir la misma sesión (día de grupo). start_date es el
// instante del primer Play / primer ingreso manual; end_date lo setea el
// servidor al pasar a finished. No hay soft-delete (lifecycle wip -> finished).
type RunnerSession struct {
	ID                int64     `gorm:"column:id;primaryKey"`
	SessionInstanceID int64     `gorm:"column:session_instance_id;not null;uniqueIndex:uq_runner_session_session_athlete,priority:1"`
	AthleteUserID     int64     `gorm:"column:athlete_user_id;not null;uniqueIndex:uq_runner_session_session_athlete,priority:2"`
	Status            string    `gorm:"column:status;not null;default:'wip'"`
	StartDate         time.Time `gorm:"column:start_date;not null"`
	EndDate           *time.Time `gorm:"column:end_date"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (RunnerSession) TableName() string {
	return "runner_session"
}