package trainingplan

import "time"

type PlanDayResponse struct {
	ID                int64     `json:"id"`
	SequenceNo        int       `json:"sequence_no"`
	Kind              string    `json:"kind"`
	OtherName         *string   `json:"other_name"`
	SessionID         *int64    `json:"session_id"`
	DefaultPresencial bool      `json:"default_presencial"`
	DefaultTime       *string   `json:"default_time"`
	DefaultLocation   *Location `json:"default_location"`
}

type TrainingPlanResponse struct {
	ID          int64             `json:"id"`
	OwnerID     int64             `json:"owner_id"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Days        []PlanDayResponse `json:"days"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
