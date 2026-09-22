package calendar

// PresencialConflict representa un día presencial de otro grupo que se
// superpone con lo que se está escribiendo. Es bloqueante (409, cross-team)
// si el grupo pertenece a otro equipo, y warning (same_team_warnings) si es
// del mismo equipo.
type PresencialConflict struct {
	GroupID            int64  `json:"group_id"`
	GroupName          string `json:"group_name"`
	TeamID             int64  `json:"team_id"`
	TeamName           string `json:"team_name"`
	Date               string `json:"date"`                 // YYYY-MM-DD
	PresencialTimeFrom string `json:"presencial_time_from"` // HH:MM
	PresencialTimeTo   string `json:"presencial_time_to"`   // HH:MM
}
