package invitation

// InviteRunnerRequest es el DTO para invitar un corredor existente a un equipo por email.
type InviteRunnerRequest struct {
	Email   string `json:"email" binding:"required"` // Email del usuario existente a invitar (requerido)
	GroupID *int64 `json:"group_id"`                 // Grupo al que se une al aceptar (opcional, default: grupo principal del equipo)
}
