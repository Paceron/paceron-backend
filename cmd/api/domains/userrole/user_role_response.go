package userrole

import "time"

// UserRoleResponse es el DTO de respuesta para una asignación de rol a usuario.
type UserRoleResponse struct {
	ID             int64     `json:"id"`              // ID de la asignación
	UserID         int64     `json:"user_id"`         // ID del usuario
	RoleID         int64     `json:"role_id"`         // ID del rol asignado
	TierID         int64     `json:"tier_id"`         // ID del tier asignado
	AssignmentDate time.Time `json:"assignment_date"` // Fecha de asignación
	Status         string    `json:"status"`          // Estado de la asignación
}

// RemoveRoleResponse es el DTO de respuesta para la baja de un rol asignado.
type RemoveRoleResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
