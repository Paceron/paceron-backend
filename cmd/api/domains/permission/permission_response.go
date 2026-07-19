package permission

import "time"

// PermissionResponse es el DTO de respuesta para un permiso.
type PermissionResponse struct {
	ID          int64     `json:"id"`           // ID del permiso
	Name        string    `json:"name"`         // Nombre del permiso
	Description string    `json:"description"`  // Descripción del permiso
	CreatedAt   time.Time `json:"created_at"`   // Fecha de creación
	UpdatedAt   time.Time `json:"updated_at"`   // Fecha de última actualización
}

// DeletePermissionResponse es el DTO de respuesta para eliminación de permiso.
type DeletePermissionResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
