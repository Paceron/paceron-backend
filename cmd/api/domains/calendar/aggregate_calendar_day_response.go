package calendar

// AggregateCalendarDayResponse es el item del calendario agregado
// (member-calendar/administered-calendar, design.md D8): los campos de
// CalendarDayResponse embebidos, más group_id/group_name/team_id/team_name
// embebidos planos, resueltos server-side. El GroupID propio sombra al del
// struct embebido en el JSON (regla de profundidad de encoding/json).
type AggregateCalendarDayResponse struct {
	CalendarDayResponse

	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
	TeamID    int64  `json:"team_id"`
	TeamName  string `json:"team_name"`

	// PresencialCollision solo se completa en administered-calendar: días
	// presenciales que se superponen con otro día presencial de otro grupo
	// administrado (member-calendar nunca lo trae). nil = sin colisión,
	// omitido en el JSON.
	PresencialCollision *PresencialCollision `json:"presencial_collision,omitempty"`
}
