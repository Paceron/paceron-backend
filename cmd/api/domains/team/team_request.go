package team

// CreateTeamRequest es el DTO para crear un equipo nuevo.
type CreateTeamRequest struct {
	Name               string `json:"name" binding:"required"`          // Nombre del equipo (requerido)
	Description        string `json:"description"`                      // Descripción del equipo (opcional)
	Level              string `json:"level"`                            // Nivel del equipo (opcional)
	MaxMembers         int64  `json:"max_members" binding:"required"`   // Cantidad máxima de integrantes (requerido)
	Requirements       string `json:"requirements"`                     // Requerimientos para entrar (opcional)
	OwnerID            int64  `json:"owner_id" binding:"required"`      // ID del usuario owner (requerido, debe tener rol entrenador)
	CreateDefaultGroup *bool  `json:"create_default_group"`             // Si es true, crea un grupo principal con nombre "{name} - group" (opcional)
}
