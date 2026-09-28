package attendance

import (
	"time"

	"simple-arq-golang/cmd/api/domains/trainingplan"
)

// DTOs de la gestión de asistencia desde el panel del entrenador.

// SessionAttendanceOption es un elemento del listado de sesiones
// (GET /groups/{group_id}/attendance-sessions): una sesión presencial ya ocurrida
// del grupo, con cuántas asistencias tiene cargadas.
//
// attended_count es el conteo de la tabla de asistencias de ESA sesión, sin
// filtrar por roster: es un dato histórico de la sesión, no un porcentaje. (El
// denominador date-aware es roster_size, que solo vive en la grilla.)
type SessionAttendanceOption struct {
	SessionInstanceID  int64     `json:"session_instance_id"`
	Name               string    `json:"name"`
	Date               time.Time `json:"date"`
	PresencialTimeFrom *string   `json:"presencial_time_from"`
	PresencialTimeTo   *string   `json:"presencial_time_to"`
	AttendedCount      int64     `json:"attended_count"`
}

// SessionAttendanceListResponse es la respuesta del listado de sesiones.
type SessionAttendanceListResponse struct {
	Sessions []SessionAttendanceOption `json:"sessions"`
}

// SessionAttendanceRow es una fila de la grilla: un corredor del roster del grupo
// cruzado con el estado de su asistencia para la sesión.
//
// Invariante: Source es nil exactamente cuando Status es "not_confirmed". Una
// fila "attended" siempre proviene de una fila de attendances persistida, y esa
// fila siempre tiene source poblado (columna NOT NULL).
type SessionAttendanceRow struct {
	UserID       int64      `json:"user_id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	AttendanceID *int64     `json:"attendance_id"`
	Status       string     `json:"status"`
	Source       *string    `json:"source"`
	RegisteredAt *time.Time `json:"registered_at"`
}

// Estados posibles de una fila de la grilla.
const (
	// SessionAttendanceStatusAttended indica que el corredor tiene asistencia
	// registrada para la sesión.
	SessionAttendanceStatusAttended = "attended"
	// SessionAttendanceStatusNotConfirmed indica que el corredor no tiene
	// asistencia registrada para la sesión.
	SessionAttendanceStatusNotConfirmed = "not_confirmed"
)

// AttendanceSummary son los agregados de la grilla.
//
// La tasa se calcula en Go, no en SQL: el redondeo a un decimal es más legible
// y testeable en un solo lugar, y evita el error clásico de dividir por cero
// cuando el roster está vacío.
type AttendanceSummary struct {
	RosterSize        int64   `json:"roster_size"`
	Attended          int64   `json:"attended"`
	NotConfirmed      int64   `json:"not_confirmed"`
	AttendanceRatePct float64 `json:"attendance_rate_pct"`
}

// SessionInfo es el bloque "session" de la respuesta de la grilla: identifica de
// qué sesión es la grilla. Los nombres de grupo y equipo viajan acá para que el
// frontend no tenga que hacer un segundo fetch solo para pintar el header.
type SessionInfo struct {
	SessionInstanceID  int64                  `json:"session_instance_id"`
	Name               string                 `json:"name"`
	Date               time.Time              `json:"date"`
	PresencialTimeFrom *string                `json:"presencial_time_from"`
	PresencialTimeTo   *string                `json:"presencial_time_to"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
	GroupID            int64                  `json:"group_id"`
	GroupName          string                 `json:"group_name"`
	TeamID             int64                  `json:"team_id"`
	TeamName           string                 `json:"team_name"`
}

// SessionAttendanceResponse es la respuesta de la grilla de una sesión
// (GET /attendance/session/{session_instance_id}).
type SessionAttendanceResponse struct {
	Session SessionInfo            `json:"session"`
	Summary AttendanceSummary      `json:"summary"`
	Roster  []SessionAttendanceRow `json:"roster"`
}

// BulkSaveResult son los contadores de la carga masiva
// (POST /attendance/bulk). Son excluyentes: created + updated == filas del lote.
type BulkSaveResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
}

// BulkSaveRequest es el body de POST /attendance/bulk.
type BulkSaveRequest struct {
	TeamID            int64               `json:"team_id"`
	TrainingSessionID int64               `json:"training_session_id"`
	Entries           []BulkSaveEntryItem `json:"entries"`
}

// BulkSaveEntryItem es un corredor a marcar en la carga masiva.
type BulkSaveEntryItem struct {
	UserID int64 `json:"user_id"`
}
