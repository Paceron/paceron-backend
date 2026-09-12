package session

import "time"

type SessionExerciseResponse struct {
	ID          int64  `json:"id"`
	ExerciseID  int64  `json:"exercise_id"`
	Role        string `json:"role"`
	RepeatCount int    `json:"repeat_count"`
	RestMinutes int    `json:"rest_minutes"`
}

type SessionResponse struct {
	ID          int64                      `json:"id"`
	OwnerID     int64                      `json:"owner_id"`
	Name        string                     `json:"name"`
	Description *string                    `json:"description"`
	Exercises   []SessionExerciseResponse `json:"exercises"`
	CreatedAt   time.Time                  `json:"created_at"`
	UpdatedAt   time.Time                  `json:"updated_at"`
}
