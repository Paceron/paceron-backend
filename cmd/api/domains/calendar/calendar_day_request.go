package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

// CalendarDayRequest es el body de PUT /groups/{id}/calendar/{date} — upsert
// de un día individual. PresencialTimeFrom/PresencialTimeTo viajan como
// "HH:MM"; TimeTo debe ser posterior a TimeFrom.
type CalendarDayRequest struct {
	Kind               string                 `json:"kind" binding:"required"`
	OtherName          *string                `json:"other_name"`
	SessionID          *int64                 `json:"session_id"`
	CancelledReason    *string                `json:"cancelled_reason"`
	IsPresencial       *bool                  `json:"is_presencial"`
	PresencialTimeFrom *string                `json:"presencial_time_from"`
	PresencialTimeTo   *string                `json:"presencial_time_to"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}

// StampRequest es el body de POST /groups/{id}/calendar/stamp.
type StampRequest struct {
	PlanID    int64  `json:"plan_id" binding:"required"`
	StartDate string `json:"start_date" binding:"required"` // "YYYY-MM-DD"
	Force     bool   `json:"force"`
}
