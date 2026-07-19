package permission

// CreatePermissionRequest es el DTO para crear un permiso.
type CreatePermissionRequest struct {
	Name        string `json:"name" binding:"required"`        // Nombre único del permiso (requerido)
	Description string `json:"description"`                     // Descripción opcional del permiso
}

// UpdatePermissionRequest es el DTO para actualizar un permiso.
// Todos los campos son opcionales (actualización parcial).
type UpdatePermissionRequest struct {
	Name        *string `json:"name"`        // Nuevo nombre del permiso (opcional)
	Description *string `json:"description"` // Nueva descripción (opcional)
}
