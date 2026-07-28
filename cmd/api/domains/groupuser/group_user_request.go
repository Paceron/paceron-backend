package groupuser

import "time"

// AddGroupUserRequest es el DTO para agregar un usuario a un grupo.
type AddGroupUserRequest struct {
	UserID    int64      `json:"user_id" binding:"required"` // ID del usuario a agregar (requerido)
	DateStart *time.Time `json:"date_start"`                  // Fecha de inicio de la membresía (opcional, default: ahora)
	DateEnd   *time.Time `json:"date_end"`                    // Fecha de fin de la membresía (opcional, nil = activa)
}
