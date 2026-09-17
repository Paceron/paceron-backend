package calendar

import "simple-arq-golang/cmd/api/domains/trainingplan"

// BulkRequest es el body de POST /groups/{id}/calendar/bulk — mismo
// contenido a todas las fechas listadas.
type BulkRequest struct {
	Dates              []string               `json:"dates" binding:"required"`
	Kind               string                 `json:"kind" binding:"required"`
	SessionID          *int64                 `json:"session_id"`
	OtherName          *string                `json:"other_name"`
	IsPresencial       *bool                  `json:"is_presencial"`
	PresencialTime     *string                `json:"presencial_time"`
	PresencialLocation *trainingplan.Location `json:"presencial_location"`
}

// BulkClearRequest es el body de POST /groups/{id}/calendar/bulk-clear.
type BulkClearRequest struct {
	Dates []string `json:"dates" binding:"required"`
}

// ShiftRequest es el body de POST /groups/{id}/calendar/shift.
type ShiftRequest struct {
	FromDate string `json:"from_date" binding:"required"` // "YYYY-MM-DD"
	Days     int    `json:"days" binding:"required"`
}
