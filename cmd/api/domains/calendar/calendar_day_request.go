package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

// CalendarDayRequest es el body de PUT /groups/{id}/calendar/{date} — upsert
// de un día individual. PresencialTime viaja como "HH:MM".
type CalendarDayRequest struct {
	Kind               string                 `json:"kind" binding:"required"`
	OtherName          *string                `json:"other_name"`
	SessionID          *int64                 `json:"session_id"`
	CancelledReason    *string                `json:"cancelled_reason"`
	IsPresencial       *bool                  `json:"is_presencial"`
	PresencialTime     *string                `json:"presencial_time"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}

// StampRequest es el body de POST /groups/{id}/calendar/stamp.
type StampRequest struct {
	PlanID    int64  `json:"plan_id" binding:"required"`
	StartDate string `json:"start_date" binding:"required"` // "YYYY-MM-DD"
	Force     bool   `json:"force"`
}
