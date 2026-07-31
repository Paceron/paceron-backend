package invitation

// InviteRunnerRequest es el DTO para invitar un corredor existente a un equipo por email.
type InviteRunnerRequest struct {
	Email   string `json:"email" binding:"required"` // Email del usuario existente a invitar (requerido)
	GroupID *int64 `json:"group_id"`                 // Grupo al que se une al aceptar (opcional, default: grupo principal del equipo)
}

// RespondInvitationRequest es el DTO compartido para aceptar/rechazar una invitación.
type RespondInvitationRequest struct {
	UserID int64 `json:"user_id" binding:"required"` // ID del usuario invitado que responde (requerido)
}
