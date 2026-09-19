package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

type NextSessionResponse struct {
	GroupID            int64                  `json:"group_id"`
	Date               string                 `json:"date"`
	SessionID          *int64                 `json:"session_id"`
	IsPresencial       bool                   `json:"is_presencial"`
	PresencialTimeFrom *string                `json:"presencial_time_from"`
	PresencialTimeTo   *string                `json:"presencial_time_to"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}
