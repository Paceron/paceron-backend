package gamification

// Shape del GET /teams/{id}/leaderboard (gamificacion-core design.md D7).

// Métricas y períodos válidos del query string (whitelist validada por el
// service, 400 fuera de estas listas).
const (
	LeaderboardMetricDistance   = "distance"
	LeaderboardMetricDuration   = "duration"
	LeaderboardMetricPace       = "pace"
	LeaderboardMetricPeakSpeed  = "peak_speed"
	LeaderboardMetricSessions   = "sessions_count"
	LeaderboardMetricAttendance = "attendance_count"

	LeaderboardPeriodWeek   = "week"
	LeaderboardPeriodMonth  = "month"
	LeaderboardPeriodYear   = "year"
	LeaderboardPeriodCustom = "custom"
	LeaderboardPeriodAll    = "all"

	LeaderboardTopSize = 5
)

func GetValidLeaderboardMetrics() []string {
	return []string{
		LeaderboardMetricDistance, LeaderboardMetricDuration, LeaderboardMetricPace,
		LeaderboardMetricPeakSpeed, LeaderboardMetricSessions, LeaderboardMetricAttendance,
	}
}

func GetValidLeaderboardPeriods() []string {
	return []string{LeaderboardPeriodWeek, LeaderboardPeriodMonth, LeaderboardPeriodYear, LeaderboardPeriodCustom, LeaderboardPeriodAll}
}

// LeaderboardRow es una fila del top 5: value es el agregado bruto del
// período (metros, ms, s/km, km/h o conteo) y rank se comparte a igual valor
// exacto (la siguiente posición saltea N).
type LeaderboardRow struct {
	UserID    int64   `json:"user_id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
	Value     float64 `json:"value"`
	Rank      int     `json:"rank"`
}

// LeaderboardViewerRank es la posición del caller en el ranking completo,
// null si no contribuyó al período (incluye al owner, que no puntúa).
type LeaderboardViewerRank struct {
	UserID int64   `json:"user_id"`
	Value  float64 `json:"value"`
	Rank   int     `json:"rank"`
}

// LeaderboardRecord es el máximo histórico del team (solo period=all). Para
// pace el record es el MEJOR ritmo (el más bajo). achieved_at es la fecha de
// sesión de la fila que alcanzó el valor; a empate entre corredores, el más
// reciente.
type LeaderboardRecord struct {
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Value      float64 `json:"value"`
	AchievedAt string  `json:"achieved_at"`
}

type LeaderboardResponse struct {
	Rows       []LeaderboardRow       `json:"rows"`
	ViewerRank *LeaderboardViewerRank `json:"viewer_rank"`
	Record     *LeaderboardRecord     `json:"record,omitempty"`
}
