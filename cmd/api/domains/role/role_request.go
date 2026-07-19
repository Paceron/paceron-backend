package role

// CreateRoleRequest es el DTO para crear un rol.
type CreateRoleRequest struct {
	Name        string `json:"name" binding:"required"`        // Nombre único del rol (requerido)
	Description string `json:"description"`                     // Descripción opcional del rol
}

// UpdateRoleRequest es el DTO para actualizar un rol.
// Todos los campos son opcionales (actualización parcial).
type UpdateRoleRequest struct {
	Name        *string `json:"name"`        // Nuevo nombre del rol (opcional)
	Description *string `json:"description"` // Nueva descripción (opcional)
}
