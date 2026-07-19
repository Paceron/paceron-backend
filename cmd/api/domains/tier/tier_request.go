package tier

// CreateTierRequest es el DTO para crear un tier.
type CreateTierRequest struct {
	Name        string `json:"name" binding:"required"`        // Nombre del tier (requerido)
	Description string `json:"description"`                     // Descripción opcional del tier
	RoleID      int64  `json:"role_id" binding:"required"`     // ID del rol al que pertenece (requerido)
}

// UpdateTierRequest es el DTO para actualizar un tier.
// Todos los campos son opcionales (actualización parcial).
type UpdateTierRequest struct {
	Name        *string `json:"name"`        // Nuevo nombre del tier (opcional)
	Description *string `json:"description"` // Nueva descripción (opcional)
}
