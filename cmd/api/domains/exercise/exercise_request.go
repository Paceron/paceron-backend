package exercise

// ExerciseRequest es el body de POST/PUT /exercises — mismo shape en ambos,
// el PUT es reemplazo completo.
type ExerciseRequest struct {
	OwnerID     int64    `json:"owner_id" binding:"required"`
	Name        string   `json:"name" binding:"required"`
	Description *string  `json:"description"`
	Kind        string   `json:"kind" binding:"required"`
	Intensity   *string  `json:"intensity"`
	Minutes     *int     `json:"minutes"`
	DistanceM   *int     `json:"distance_m"`
	SpeedKph    *float64 `json:"speed_kph"`
	MuscleGroup *string  `json:"muscle_group"`
}
