package competition

// DTOs del módulo de competencias (gamificacion-core D9/E4.1).

import "time"

const (
	// Statuses de la competencia (source of truth del ciclo, D8/D9).
	StatusPendingApproval = "pending_approval"
	StatusActive          = "active"
	StatusFinished        = "finished"
	StatusRejected        = "rejected"

	// Tipos de competencia.
	TypeMetric = "metric"
	TypeRoute  = "route"

	// Rol del creador.
	CreatorRoleTrainer = "trainer"
	CreatorRoleRunner  = "runner"

	// Mensajes de respuesta del intake de intentos (competencias-ruta-intake
	// D2). El body data es el shape runner-session (MutationResponse del
	// módulo runnersession).
	MsgCompetitionAttemptCreated = "intento registrado"
)

// GetValidStatuses devuelve la whitelist del filtro status del listado.
func GetValidStatuses() []string {
	return []string{StatusPendingApproval, StatusActive, StatusFinished, StatusRejected}
}

// CreateCompetitionRequest es el body de POST /teams/{id}/competitions.
// El service valida las reglas cruzadas (type ↔ metric/route, rango,
// participantes): binding solo exige lo estructural.
type CreateCompetitionRequest struct {
	GroupID                *int64     `json:"group_id"`
	Type                   string     `json:"type" binding:"required"`
	Metric                 *string    `json:"metric"`
	StartAt                time.Time  `json:"start_at" binding:"required"`
	EndAt                  *time.Time `json:"end_at"`
	RouteGeoJSON           *string    `json:"route_geojson"`
	AcceptanceRadiusMeters *float64   `json:"acceptance_radius_meters"`
	// MaxAttempts es solo de type=route: opcional, >= 1, sin tope. NULL =
	// sin límite; inmutable post-create (no hay edición de competencia).
	MaxAttempts    *int64  `json:"max_attempts"`
	ParticipantIDs []int64 `json:"participants" binding:"required,min=2"`
}

// CompetitionProgressItem es el progreso en vivo de un participante en el
// detalle. value es null para type=route hasta la implementación del matching
// (E4.2): el DTO ya reserva el lugar.
type CompetitionProgressItem struct {
	UserID int64    `json:"user_id"`
	Value  *float64 `json:"value"`
}

// CompetitionResponse es el shape de competencia para detalle y listado
// (spec: {id, team_id, group_id, type, metric?, ...}). progress solo en el
// detalle (omitempty en el listado). session_instance_id = instancia
// sintética del intento (Gap 35): solo type=route, null en metric.
// max_attempts solo vive en route (null = sin límite); metric nunca lo setea.
type CompetitionResponse struct {
	ID                     int64                     `json:"id"`
	TeamID                 int64                     `json:"team_id"`
	GroupID                *int64                    `json:"group_id"`
	Type                   string                    `json:"type"`
	Metric                 *string                   `json:"metric"`
	StartAt                time.Time                 `json:"start_at"`
	EndAt                  *time.Time                `json:"end_at"`
	Route                  *string                   `json:"route,omitempty"`
	AcceptanceRadiusMeters *float64                  `json:"acceptance_radius_meters"`
	SessionInstanceID      *int64                    `json:"session_instance_id"`
	MaxAttempts            *int64                    `json:"max_attempts"`
	Participants           []int64                   `json:"participants"`
	CreatedByUserID        int64                     `json:"created_by_user_id"`
	CreatedByRole          string                    `json:"created_by_role"`
	Status                 string                    `json:"status"`
	ApprovedByUserID       *int64                    `json:"approved_by_user_id"`
	RejectionReason        *string                   `json:"rejection_reason"`
	WinnerUserIDs          []int64                   `json:"winner_user_ids"`
	FinishedAt             *time.Time                `json:"finished_at"`
	Progress               []CompetitionProgressItem `json:"progress,omitempty"`
}

// CompetitionsListResponse es la respuesta de GET /teams/{id}/competitions,
// con has_unseen de la sección competitions embebido por team_id (D11).
type CompetitionsListResponse struct {
	Data      []CompetitionResponse `json:"data"`
	HasUnseen bool                  `json:"has_unseen"`
}

// CompetitionDetailResponse es la respuesta de GET /competitions/{id}: shape
// del listado + progreso en vivo por participante (sin broadcast WS).
type CompetitionDetailResponse struct {
	Data CompetitionResponse `json:"data"`
}

// RejectRequest es el body de PATCH /competitions/{id}/reject: reason opcional.
type RejectRequest struct {
	Reason *string `json:"reason"`
}

// CompetitionCancelResponse es la respuesta de DELETE /competitions/{id}/cancel
// (la competencia ya no existe, no hay data).
type CompetitionCancelResponse struct {
	Message string `json:"message"`
}

const MsgCompetitionCancelled = "competencia cancelada"
