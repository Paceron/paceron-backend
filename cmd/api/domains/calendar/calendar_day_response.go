package calendar

import (
	"time"

	"simple-arq-golang/cmd/api/domains/trainingplan"
)

type CalendarDayResponse struct {
	ID                 int64                  `json:"id"`
	GroupID            int64                  `json:"group_id"`
	Date               string                 `json:"date"` // "YYYY-MM-DD"
	Kind               string                 `json:"kind"`
	OtherName          *string                `json:"other_name"`
	SessionID          *int64                 `json:"session_id"`
	CancelledReason    *string                `json:"cancelled_reason"`
	IsPresencial       bool                   `json:"is_presencial"`
	PresencialTimeFrom *string                `json:"presencial_time_from"`
	PresencialTimeTo   *string                `json:"presencial_time_to"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
	SourcePlanID       *int64                 `json:"source_plan_id"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}
