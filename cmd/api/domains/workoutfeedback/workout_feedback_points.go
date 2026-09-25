package workoutfeedback

import "time"

// Mensajes de respuesta fijos del submódulo de puntos.
const MsgPointsCreated = "puntos registrados"

// CreatePointsRequest es el body de POST /api/v1/workout-feedback/:id/points.
// El array no puede venir vacío (validado acá con binding y de nuevo en el
// service). "order" es el ordinal 0-based del punto dentro de la serie; los
// ids de instancia viajan denormalizados. Los rangos (lat/lon/order >= 0) se
// validan en el service, no acá — `binding:"required"` rechazaría el order 0.
type CreatePointsRequest struct {
	Points []PointInput `json:"points" binding:"required"`
}

// PointInput es un punto del recorrido GPS de la serie.
type PointInput struct {
	Order              int       `json:"order"`
	SessionInstanceID  int64     `json:"session_instance_id"`
	ExerciseInstanceID int64     `json:"exercise_instance_id"`
	Latitude           float64   `json:"latitude"`
	Longitude          float64   `json:"longitude"`
	RecordedAt         time.Time `json:"recorded_at" binding:"required"`
}

// WorkoutFeedbackPointResponse es el shape plano espejo de
// dbs.WorkoutFeedbackPoint para la respuesta de listado.
type WorkoutFeedbackPointResponse struct {
	ID                 int64     `json:"id"`
	FeedbackID         int64     `json:"feedback_id"`
	SessionInstanceID  int64     `json:"session_instance_id"`
	ExerciseInstanceID int64     `json:"exercise_instance_id"`
	Order              int       `json:"order"`
	Latitude           float64   `json:"latitude"`
	Longitude          float64   `json:"longitude"`
	RecordedAt         time.Time `json:"recorded_at"`
}

// PointsMutationData son los conteos del bulk insert idempotente (ON CONFLICT
// DO NOTHING): created = insertados, skipped = ya existían en (feedback_id, order).
type PointsMutationData struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

// PointsMutationResponse es la respuesta de POST /api/v1/workout-feedback/:id/points.
type PointsMutationResponse struct {
	Message string           `json:"message"`
	Data    PointsMutationData `json:"data"`
}

// PointsListResponse es la respuesta de GET /api/v1/workout-feedback/:id/points.
type PointsListResponse struct {
	Data []WorkoutFeedbackPointResponse `json:"data"`
}