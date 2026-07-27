package groupuser

import "time"

// GroupUserResponse es el DTO de respuesta para la asociación de un usuario a un grupo.
type GroupUserResponse struct {
	ID        int64      `json:"id"`         // ID de la asociación
	GroupID   int64      `json:"group_id"`   // ID del grupo
	UserID    int64      `json:"user_id"`    // ID del usuario
	DateStart time.Time  `json:"date_start"` // Fecha de inicio de la membresía
	DateEnd   *time.Time `json:"date_end"`   // Fecha de fin de la membresía (nil = activa)
}

// RemoveGroupUserResponse es el DTO de respuesta para quitar un usuario de un grupo.
type RemoveGroupUserResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
