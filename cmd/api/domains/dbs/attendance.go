package dbs

import "time"

// Attendance representa la asistencia de un usuario (corredor) a una sesión de
// entrenamiento de un equipo. TrainingSessionID apunta a session_instances.id
// (la copia inmutable de la sesión del catálogo que se crea al asignar contenido
// a un día de calendario) y ya tiene FK real en la base. El índice único
// compuesto garantiza la regla de negocio de no duplicados (un corredor = una
// asistencia por sesión y equipo).
//
// Source registra cómo se cargó la asistencia: "qr" (el corredor escaneó el QR)
// o "manual" (el entrenador la marcó desde el panel). Es NOT NULL porque
// conceptualmente toda fila de asistencia tiene una procedencia; se deja
// nullable en la columna solo durante la migración, que la rellena con 'qr' —
// el único escritor previo a este change era el escaneo QR — y luego la fija
// con SET NOT NULL. RegisteredByUserID queda nullable y sin FK a propósito: el
// actor de un registro histórico puede haber sido dado de baja, y una FK con
// ON DELETE restrictivo rompería borrados de usuario que hoy funcionan.
type Attendance struct {
	ID                 int64     `gorm:"column:id;primaryKey"`                                                                                                                             // ID único de la asistencia (autoincremental)
	TeamID             int64     `gorm:"column:team_id;not null;uniqueIndex:uq_att_team_session_user,priority:1;index:idx_att_team_session,priority:1;index:idx_att_user_team,priority:2"` // ID del equipo
	TrainingSessionID  int64     `gorm:"column:training_session_id;not null;uniqueIndex:uq_att_team_session_user,priority:2;index:idx_att_team_session,priority:2"`                        // ID de la sesión (FK a session_instances.id)
	UserID             int64     `gorm:"column:user_id;not null;uniqueIndex:uq_att_team_session_user,priority:3;index:idx_att_user_team,priority:1"`                                       // ID del usuario que asiste
	Source             string    `gorm:"column:source;not null"`                                                                                                                           // Procedencia: "qr" o "manual"
	RegisteredByUserID *int64    `gorm:"column:registered_by_user_id"`                                                                                                                     // Quién cargó la asistencia (nullable, sin FK)
	CreatedAt          time.Time `gorm:"column:created_at;autoCreateTime"`                                                                                                                 // Fecha de registro
	UpdatedAt          time.Time `gorm:"column:updated_at;autoUpdateTime"`                                                                                                                 // Fecha de última actualización
}

func (Attendance) TableName() string {
	return "attendances"
}
