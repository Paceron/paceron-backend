package session

type SessionExerciseRequest struct {
	ExerciseID  int64  `json:"exercise_id" binding:"required"`
	Role        string `json:"role" binding:"required"`
	RepeatCount *int   `json:"repeat_count"`
	RestMinutes *int   `json:"rest_minutes"`
}

// SessionRequest es el body de POST/PUT /sessions — mismo shape en ambos,
// el PUT reemplaza Exercises entero. exclude_group_ids/clone_name/
// clone_description los agrega el change de calendario (no tocar acá).
type SessionRequest struct {
	OwnerID     int64                    `json:"owner_id" binding:"required"`
	Name        string                   `json:"name" binding:"required"`
	Description *string                  `json:"description"`
	Exercises   []SessionExerciseRequest `json:"exercises" binding:"required"`
}
