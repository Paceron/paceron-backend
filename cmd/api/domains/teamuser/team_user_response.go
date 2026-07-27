package teamuser

import "time"

// TeamUserResponse es el DTO de respuesta para la asociación de un usuario a un equipo.
type TeamUserResponse struct {
	ID             int64     `json:"id"`               // ID de la asociación
	TeamID         int64     `json:"team_id"`          // ID del equipo
	UserID         int64     `json:"user_id"`          // ID del usuario
	RoleInTeam     string    `json:"role_in_team"`     // Rol del usuario en el equipo
	Status         string    `json:"status"`           // Estado de la asociación
	AssignmentDate time.Time `json:"assignment_date"`  // Fecha de asociación
}

// RemoveTeamUserResponse es el DTO de respuesta para quitar un usuario de un equipo.
type RemoveTeamUserResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
