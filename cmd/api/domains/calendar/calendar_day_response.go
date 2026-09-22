package calendar

import (
	"time"

	"simple-arq-golang/cmd/api/domains/instance"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

type CalendarDayResponse struct {
	ID                 int64                             `json:"id"`
	GroupID            int64                             `json:"group_id"`
	Date               string                            `json:"date"` // "YYYY-MM-DD"
	Kind               string                            `json:"kind"`
	OtherName          *string                           `json:"other_name"`
	SessionInstance    *instance.SessionInstanceResponse `json:"session_instance"`
	CancelledReason    *string                           `json:"cancelled_reason"`
	IsPresencial       bool                              `json:"is_presencial"`
	PresencialTimeFrom *string                           `json:"presencial_time_from"`
	PresencialTimeTo   *string                           `json:"presencial_time_to"`
	PresencialLocation *trainingplan.Location            `json:"presencial_location"`
	SourcePlanID       *int64                            `json:"source_plan_id"`
	CreatedAt          time.Time                         `json:"created_at"`
	UpdatedAt          time.Time                         `json:"updated_at"`
	// SameTeamWarnings solo se completa en la escritura individual (PUT): días
	// presenciales de grupos del MISMO equipo que se superponen con lo
	// guardado. En lecturas y en el wrapper de stamp/bulk/shift queda vacío
	// (omitempty = no viaja en el JSON).
	SameTeamWarnings []PresencialConflict `json:"same_team_warnings,omitempty"`
}
