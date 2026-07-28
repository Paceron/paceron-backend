package group

import "time"

// GroupResponse es el DTO de respuesta para un grupo.
type GroupResponse struct {
	ID          int64     `json:"id"`           // ID del grupo
	Name        string    `json:"name"`         // Nombre del grupo
	Description string    `json:"description"`  // Descripción del grupo
	TeamID      int64     `json:"team_id"`      // ID del equipo al que pertenece
	IsMain      bool      `json:"is_main"`      // Si es el grupo principal del equipo
	CreatedAt   time.Time `json:"created_at"`   // Fecha de creación
	UpdatedAt   time.Time `json:"updated_at"`   // Fecha de última actualización
}

// DeleteGroupResponse es el DTO de respuesta para la eliminación de un grupo.
type DeleteGroupResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
