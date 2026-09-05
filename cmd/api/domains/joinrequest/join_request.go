package joinrequest

import "time"

// JoinRequestResponse es el DTO de respuesta para una solicitud de ingreso.
type JoinRequestResponse struct {
	ID         int64     `json:"id"`
	TeamID     int64     `json:"team_id"`
	TeamName   string    `json:"team_name"`
	RunnerID   int64     `json:"runner_id"`
	RunnerName string    `json:"runner_name"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// PendingCountResponse es la respuesta de GET /api/v1/join-requests/pending-count.
type PendingCountResponse struct {
	Count int64 `json:"count"`
}
