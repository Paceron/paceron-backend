package trainingplan

// PlanDayRequest.DefaultTime viaja como "HH:MM" (string) — el service lo
// parsea con time.Parse("15:04", ...).
type PlanDayRequest struct {
	SequenceNo        int       `json:"sequence_no" binding:"required"`
	Kind              string    `json:"kind" binding:"required"`
	OtherName         *string   `json:"other_name"`
	SessionID         *int64    `json:"session_id"`
	DefaultPresencial *bool     `json:"default_presencial"`
	DefaultTime       *string   `json:"default_time"`
	DefaultLocation   *Location `json:"default_location"`
}

// TrainingPlanRequest es el body de POST /training-plans (reemplazo total).
type TrainingPlanRequest struct {
	OwnerID     int64            `json:"owner_id" binding:"required"`
	Name        string           `json:"name" binding:"required"`
	Description *string          `json:"description"`
	Days        []PlanDayRequest `json:"days" binding:"required"`
}

// TrainingPlanUpdateRequest es el body de PUT /training-plans/{id} — parcial,
// Days (si viene) reemplaza el set entero.
type TrainingPlanUpdateRequest struct {
	Name        *string           `json:"name"`
	Description *string           `json:"description"`
	Days        *[]PlanDayRequest `json:"days"`
}
