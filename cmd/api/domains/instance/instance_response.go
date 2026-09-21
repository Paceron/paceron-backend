package instance

import (
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// InstanceExerciseResponse es un ejercicio congelado dentro de una
// SessionInstance, embebido en la respuesta de calendario (design.md D9):
// combina el detalle de la ExerciseInstance con el vínculo
// role/repeat_count/rest_minutes de SessionExerciseInstance.
type InstanceExerciseResponse struct {
	ID          int64    `json:"id"`
	ExerciseID  *int64   `json:"exercise_id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Description *string  `json:"description"`
	Intensity   *string  `json:"intensity"`
	Minutes     *int     `json:"minutes"`
	DistanceM   *int     `json:"distance_m"`
	SpeedKph    *float64 `json:"speed_kph"`
	MuscleGroup *string  `json:"muscle_group"`
	VideoURL    *string  `json:"video_url"`
	Role        string   `json:"role"`
	RepeatCount int      `json:"repeat_count"`
	RestMinutes int      `json:"rest_minutes"`
}

// SessionInstanceResponse es el detalle completo congelado de una
// SessionInstance (design.md D9): `{id, name, description, exercises: [...]}`.
// La sesión de calendario lo embebe como `session_instance` (o null si
// kind != training).
type SessionInstanceResponse struct {
	ID          int64                      `json:"id"`
	SessionID   *int64                     `json:"session_id"`
	Name        string                     `json:"name"`
	Description *string                    `json:"description"`
	CreatedAt   time.Time                  `json:"created_at"`
	Exercises   []InstanceExerciseResponse `json:"exercises"`
}

// NewExerciseResponse mapea una fila de SessionExerciseInstance junto a su
// ExerciseInstance al shape embebido de D9. Los rows deben venir ordenados
// (el DAO ordena por id).
func NewExerciseResponse(link dbs.SessionExerciseInstance, ex dbs.ExerciseInstance) InstanceExerciseResponse {
	return InstanceExerciseResponse{
		ID:          ex.ID,
		Name:        ex.Name,
		Kind:        ex.Kind,
		Description: ex.Description,
		Intensity:   ex.Intensity,
		Minutes:     ex.Minutes,
		DistanceM:   ex.DistanceM,
		SpeedKph:    ex.SpeedKph,
		MuscleGroup: ex.MuscleGroup,
		VideoURL:    ex.VideoURL,
		ExerciseID:  ex.SourceExerciseID,
		Role:        link.Role,
		RepeatCount: link.RepeatCount,
		RestMinutes: link.RestMinutes,
	}
}

// NewSessionResponse mapea una SessionInstance más sus links y ejercicios
// al shape embebido de D9. Los links y ejercicios se resuelven por ID (no se
// requiere orden paralelo); si algún link referencia un ExerciseInstance que
// no vino en la carga, devuelve error en vez de producir un D9 incompleto.
func NewSessionResponse(sess dbs.SessionInstance, links []dbs.SessionExerciseInstance, exercises []dbs.ExerciseInstance) (SessionInstanceResponse, error) {
	result := SessionInstanceResponse{
		ID:          sess.ID,
		SessionID:   sess.SourceSessionID,
		Name:        sess.Name,
		Description: sess.Description,
		CreatedAt:   sess.CreatedAt,
		Exercises:   make([]InstanceExerciseResponse, 0, len(links)),
	}
	byID := make(map[int64]dbs.ExerciseInstance, len(exercises))
	for _, ex := range exercises {
		byID[ex.ID] = ex
	}
	for _, link := range links {
		ex, ok := byID[link.ExerciseInstanceID]
		if !ok {
			return SessionInstanceResponse{}, fmt.Errorf("session instance %d: ejercicio instancia %d faltante (link %d)", sess.ID, link.ExerciseInstanceID, link.ID)
		}
		result.Exercises = append(result.Exercises, NewExerciseResponse(link, ex))
	}
	return result, nil
}
