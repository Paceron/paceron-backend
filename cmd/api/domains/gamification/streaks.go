package gamification

// Estados del día en la respuesta de rachas (design.md D2). El 4to valor
// pending es solo para el día de hoy: en curso, no corta ni rompe rachas.
const (
	DayStatusCompleted = "completed"
	DayStatusFailed    = "failed"
	DayStatusSkipped   = "skipped"
	DayStatusPending   = "pending"
)

// GetValidDayStatuses lista los estados válidos de un día en /streaks.
func GetValidDayStatuses() []string {
	return []string{DayStatusCompleted, DayStatusFailed, DayStatusSkipped, DayStatusPending}
}

// StreakSummary es el par current/best de una racha.
type StreakSummary struct {
	Current int `json:"current"`
	Best    int `json:"best"`
}

// StreakDay es un día del array window de /streaks: fecha date-only
// (YYYY-MM-DD en TZ Cordoba) con los DOS estados del día.
type StreakDay struct {
	Date           string `json:"date"`
	StrictStatus   string `json:"strict_status"`
	FlexibleStatus string `json:"flexible_status"`
}

// StreaksResponse es la respuesta de GET /users/{id}/streaks (design.md D2).
// Los rachas se computan sobre TODO el histórico hasta ayer; days solo
// recorta el array devuelto. has_unseen es de la sección streaks.
type StreaksResponse struct {
	Strict    StreakSummary `json:"strict"`
	Flexible  StreakSummary `json:"flexible"`
	Days      []StreakDay   `json:"days"`
	HasUnseen bool          `json:"has_unseen"`
}
