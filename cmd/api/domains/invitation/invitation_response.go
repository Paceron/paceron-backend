package invitation

import "time"

// InviteRunnerResponse es el DTO de respuesta para el envío de una invitación.
type InviteRunnerResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}

// InvitationResponse representa una invitación, tanto en listados como en detalle.
type InvitationResponse struct {
	ID           int64     `json:"id"`
	TeamID       int64     `json:"team_id"`
	TeamName     string    `json:"team_name"`
	GroupID      *int64    `json:"group_id"`
	InviteeID    int64     `json:"invitee_id"`
	InviteeName  string    `json:"invitee_name"`
	InviteeEmail string    `json:"invitee_email"`
	Status       string    `json:"status"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// RespondInvitationResponse es el DTO de respuesta para aceptar/rechazar una invitación.
type RespondInvitationResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
