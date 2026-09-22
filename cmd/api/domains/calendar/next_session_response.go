package calendar

import (
	"simple-arq-golang/cmd/api/domains/instance"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

type NextSessionResponse struct {
	GroupID            int64                             `json:"group_id"`
	Date               string                            `json:"date"`
	SessionInstance    *instance.SessionInstanceResponse `json:"session_instance"`
	IsPresencial       bool                              `json:"is_presencial"`
	PresencialTimeFrom *string                           `json:"presencial_time_from"`
	PresencialTimeTo   *string                           `json:"presencial_time_to"`
	PresencialLocation *trainingplan.Location            `json:"presencial_location"`
}
