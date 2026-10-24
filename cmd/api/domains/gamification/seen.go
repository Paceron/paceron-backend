package gamification

// SeenRequest es el body de POST /users/{id}/gamification/seen (design.md D3).
// Section es streaks/achievements/competitions; team_id solo se admite en
// competitions.
type SeenRequest struct {
	Section string `json:"section" binding:"required"`
	TeamID  *int64 `json:"team_id"`
}

// SeenResponse es la respuesta del POST seen: marcar visto es no-op exitoso
// incluso con 0 filas por borrar.
type SeenResponse struct {
	Message string `json:"message"`
}
