package dbs

import "time"

// Filas de agregación del leaderboard (gamificacion-core D7). No son tablas:
// son el shape de salida de las queries GROUP BY user_id del leaderboard_dao.
// Los dates escanean como medianoche UTC (DSN TimeZone=UTC) y representan
// fechas de pared de Cordoba cuando vienen de columnas AT TIME ZONE.

// LeaderboardFeedbackAgg agrega las métricas fuente workout_feedback por
// atleta: sums para distance/duration/pace, max para peak_speed; los counts
// alimentan el desempate "menos entrenamientos contribuyentes" y los *_last
// dates el achieved_at del record (period=all).
type LeaderboardFeedbackAgg struct {
	UserID           int64      `gorm:"column:user_id"`
	SumDistance      *float64   `gorm:"column:sum_distance"`
	SumDurationMs    *int64     `gorm:"column:sum_duration_ms"`
	MaxPeak          *float64   `gorm:"column:max_peak"`
	DistanceCount    int64      `gorm:"column:distance_count"`
	DurationCount    int64      `gorm:"column:duration_count"`
	PaceCount        int64      `gorm:"column:pace_count"`
	PeakCount        int64      `gorm:"column:peak_count"`
	DistanceLastDate *time.Time `gorm:"column:distance_last_date"`
	DurationLastDate *time.Time `gorm:"column:duration_last_date"`
	PaceLastDate     *time.Time `gorm:"column:pace_last_date"`
	PeakArgmaxDate   *time.Time `gorm:"column:peak_argmax_date"`
}

// LeaderboardSessionsAgg agrega sessions_count (runner sessions finished de
// instancias de grupos del team) por atleta; last_start alimenta el desempate
// (empezo antes) y el achieved_at del record.
type LeaderboardSessionsAgg struct {
	UserID        int64      `gorm:"column:user_id"`
	SessionsCount int64      `gorm:"column:sessions_count"`
	LastStart     *time.Time `gorm:"column:last_start"`
}

// LeaderboardAttendanceAgg agrega attendance_count por usuario; last
// registered alimenta el desempate (registro primero la última) y el
// achieved_at del record.
type LeaderboardAttendanceAgg struct {
	UserID           int64      `gorm:"column:user_id"`
	AttendanceCount  int64      `gorm:"column:attendance_count"`
	LastRegisteredAt *time.Time `gorm:"column:last_registered_at"`
}
