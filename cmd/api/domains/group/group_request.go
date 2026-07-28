package group

// CreateGroupRequest es el DTO para crear un grupo nuevo.
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required"`   // Nombre del grupo (requerido)
	Description string `json:"description"`                // Descripción del grupo (opcional)
	TeamID      int64  `json:"team_id" binding:"required"` // ID del equipo al que pertenece (requerido)
	IsMain      bool   `json:"is_main"`                    // Si es el grupo principal del equipo (default: false)
}

// UpdateGroupRequest es el DTO para actualizar un grupo existente.
// Todos los campos son opcionales (solo se actualizan los enviados).
type UpdateGroupRequest struct {
	Name        *string `json:"name"`         // Nombre del grupo (opcional)
	Description *string `json:"description"`  // Descripción del grupo (opcional)
	IsMain      *bool   `json:"is_main"`      // Si es el grupo principal (opcional)
}
