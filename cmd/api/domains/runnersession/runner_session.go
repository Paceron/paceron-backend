package runnersession

import "time"

// Mensajes de respuesta fijos del módulo de estado de sesión del corredor.
const (
	MsgRunnerSessionCreated = "estado de sesión creado"
	MsgRunnerSessionExisted = "el estado de sesión ya existía"
	MsgRunnerSessionFinished = "sesión marcada como completada"
)

// CreateRunnerSessionRequest es el body de POST /api/v1/session-instances/:id/runner.
// start_date es obligatorio (RFC3339, el instante del primer Play o del primer
// ingreso al registro manual, timezone del cliente). athlete_user_id es
// opcional: si no viene, el atleta es el auth_user_id; si viene ajeno, el auth
// debe ser owner de un equipo al que pertenezca ese atleta.
type CreateRunnerSessionRequest struct {
	AthleteUserID *int64    `json:"athlete_user_id"`
	StartDate     time.Time `json:"start_date" binding:"required"`
}

// RunnerStatusRequest es el body de PATCH /api/v1/session-instances/:id/runner.
// Único estado válido en esta versión: "finished". athlete_user_id es opcional:
// si no viene opera sobre el auth (self), si viene ajeno requiere trainer.
type RunnerStatusRequest struct {
	AthleteUserID *int64 `json:"athlete_user_id"`
	Status        string `json:"status" binding:"required"`
}

// RunnerSessionResponse es el shape plano espejo de dbs.RunnerSession para las
// respuestas de estado (create/update/get).
type RunnerSessionResponse struct {
	ID                int64      `json:"id"`
	SessionInstanceID int64      `json:"session_instance_id"`
	AthleteUserID     int64      `json:"athlete_user_id"`
	Status            string     `json:"status"`
	StartDate         time.Time  `json:"start_date"`
	EndDate           *time.Time `json:"end_date"`
}

// MutationResponse es la respuesta de create/finish (mensaje + recurso).
type MutationResponse struct {
	Message string                 `json:"message"`
	Data    *RunnerSessionResponse `json:"data"`
}

// RunnerSessionListResponse es la respuesta de GET /api/v1/session-instances/:id/runner.
type RunnerSessionListResponse struct {
	Data RunnerSessionResponse `json:"data"`
}