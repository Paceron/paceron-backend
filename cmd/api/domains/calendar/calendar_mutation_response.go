package calendar

// CalendarMutationResponse es el wrapper de respuesta de las escrituras por
// lote (stamp/bulk/shift): los días resultantes más los avisos no bloqueantes
// de superposición presencial same-team. Reemplaza al array crudo de
// CalendarDayResponse (breaking coordinado con frontend, design.md D8).
type CalendarMutationResponse struct {
	Days             []CalendarDayResponse `json:"days"`
	SameTeamWarnings []PresencialConflict  `json:"same_team_warnings,omitempty"`
}
