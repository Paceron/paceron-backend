package exercise

import "time"

type ExerciseResponse struct {
	ID          int64     `json:"id"`
	OwnerID     int64     `json:"owner_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Kind        string    `json:"kind"`
	Intensity   *string   `json:"intensity"`
	Minutes     *int      `json:"minutes"`
	DistanceM   *int      `json:"distance_m"`
	SpeedKph    *float64  `json:"speed_kph"`
	MuscleGroup *string   `json:"muscle_group"`
	VideoURL    *string   `json:"video_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
