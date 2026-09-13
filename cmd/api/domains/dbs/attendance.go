package dbs

import "time"

// Attendance representa la asistencia de un usuario (corredor) a una sesión de
// entrenamiento de un equipo. La tabla se creó con training_session_id como FK
// opaca: la tabla training_sessions no existe aún y la constraint se agregará en
// el change que la introduzca. El índice único compuesto garantiza la regla de
// negocio de no duplicados (un corredor = una asistencia por sesión y equipo).
type Attendance struct {
	ID                int64     `gorm:"column:id;primaryKey"`                                                                                                                             // ID único de la asistencia (autoincremental)
	TeamID            int64     `gorm:"column:team_id;not null;uniqueIndex:uq_att_team_session_user,priority:1;index:idx_att_team_session,priority:1;index:idx_att_user_team,priority:2"` // ID del equipo
	TrainingSessionID int64     `gorm:"column:training_session_id;not null;uniqueIndex:uq_att_team_session_user,priority:2;index:idx_att_team_session,priority:2"`                        // ID de la sesión de entrenamiento (FK opaca, ver doc)
	UserID            int64     `gorm:"column:user_id;not null;uniqueIndex:uq_att_team_session_user,priority:3;index:idx_att_user_team,priority:1"`                                       // ID del usuario que asiste
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime"`                                                                                                                 // Fecha de registro
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`                                                                                                                 // Fecha de última actualización
}

func (Attendance) TableName() string {
	return "attendances"
}
