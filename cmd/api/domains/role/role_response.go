package role

import "time"

// RoleResponse es el DTO de respuesta para un rol.
type RoleResponse struct {
	ID          int64     `json:"id"`           // ID del rol
	Name        string    `json:"name"`         // Nombre del rol
	Description string    `json:"description"`  // Descripción del rol
	CreatedAt   time.Time `json:"created_at"`   // Fecha de creación
	UpdatedAt   time.Time `json:"updated_at"`   // Fecha de última actualización
}

// DeleteRoleResponse es el DTO de respuesta para eliminación de rol.
type DeleteRoleResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
