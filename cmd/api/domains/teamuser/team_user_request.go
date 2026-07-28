package teamuser

// AddTeamUserRequest es el DTO para agregar un usuario a un equipo.
type AddTeamUserRequest struct {
	UserID     int64  `json:"user_id" binding:"required"`      // ID del usuario a agregar (requerido)
	RoleInTeam string `json:"role_in_team" binding:"required"` // Rol del usuario en el equipo (requerido: corredor)
}
