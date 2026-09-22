package calendar

// PresencialCollision es el marcado de colisión de administered-calendar
// (design.md D8): un día presencial del calendario del entrenador que se
// superpone con otro día presencial de otro grupo administrado.
// Type es "cross_team" si algún colisionante es de otro equipo (gana sobre
// same_team), "same_team" si todos son del mismo equipo. Conflicts lista
// todos los colisionantes.
type PresencialCollision struct {
	Type      string               `json:"type"` // "same_team" | "cross_team"
	Conflicts []PresencialConflict `json:"conflicts"`
}
