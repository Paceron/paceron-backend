package daos

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// FinishedSessionStats es el histórico finished y su subconjunto early_N.
type FinishedSessionStats struct {
	Finished int64
	Early    int64
}

// DistanceStats es la distancia acumulada y el máximo por sesión asignada.
type DistanceStats struct {
	Total     float64
	MaxSingle float64
}

// AchievementDAOInterface reúne el acceso a datos de logros (gamificacion-core
// D5): lectura/escritura idempotente del progreso, la vitrina de estampas y
// las queries de agregación por criterio de los evaluadores.
type AchievementDAOInterface interface {
	// ListProgress devuelve todas las filas de progreso del usuario.
	ListProgress(ctx *gin.Context, userID int64) ([]dbs.UserAchievementProgress, error)
	// InsertUnlockWithUnseen inserta el desbloqueo si (user_id, achievement_id)
	// no existía y, solo si es nuevo, su fila de gamification_unseen en la
	// MISMA sentencia atómica. Reporta si la fila es NUEVA.
	InsertUnlockWithUnseen(ctx *gin.Context, userID int64, achievementID string, unlockedAt time.Time) (bool, error)
	// InsertUnlocksWithUnseen hace el alta batch idempotente y registra el
	// unseen de cada fila realmente insertada en la MISMA sentencia atómica:
	// si la DB confirma el unlock, el unseen quedó durable aunque el caller
	// reciba error. Devuelve SOLO los IDs insertados, en el orden recibido.
	InsertUnlocksWithUnseen(ctx *gin.Context, userID int64, achievementIDs []string, unlockedAt time.Time) ([]string, error)
	// ListStamps devuelve las estampas del usuario (vitrina; vacía hasta E4).
	ListStamps(ctx *gin.Context, userID int64) ([]dbs.GamificationStamp, error)

	// CountFinishedSessions: histórico de runner sessions finished.
	CountFinishedSessions(ctx *gin.Context, userID int64) (int64, error)
	// CountEarlyFinishedSessions: sesiones finished con start_date local
	// (Córdoba) antes de las 08:00, agregadas en SQL.
	CountEarlyFinishedSessions(ctx *gin.Context, userID int64) (int64, error)
	// CountFinishedSessionStats combina finished y early_N en una lectura.
	CountFinishedSessionStats(ctx *gin.Context, userID int64) (FinishedSessionStats, error)
	// SumDistanceMeters: suma de distance_meters de feedbacks activos.
	SumDistanceMeters(ctx *gin.Context, userID int64) (float64, error)
	// MaxDistanceSingleSession: MAX del SUM por sesión asignada.
	MaxDistanceSingleSession(ctx *gin.Context, userID int64) (float64, error)
	// SumAndMaxDistance combina total y máximo por sesión en una lectura.
	SumAndMaxDistance(ctx *gin.Context, userID int64) (DistanceStats, error)
	// CountGPSFinishedSessions: sesiones finished con puntos GPS subidos.
	CountGPSFinishedSessions(ctx *gin.Context, userID int64) (int64, error)
	// ListFinishedSessionStarts: start_date de sesiones finished (early_N
	// filtra la hora en Go con la TZ de gamificación).
	ListFinishedSessionStarts(ctx *gin.Context, userID int64) ([]time.Time, error)
	// CountAttendanceDays: días distintos de calendario con asistencia.
	CountAttendanceDays(ctx *gin.Context, userID int64) (int64, error)
}

type achievementDao struct {
	DB *gorm.DB
}

func NewAchievementDao(database *gorm.DB) AchievementDAOInterface {
	return &achievementDao{DB: database}
}

func (d *achievementDao) ListProgress(ctx *gin.Context, userID int64) ([]dbs.UserAchievementProgress, error) {
	var rows []dbs.UserAchievementProgress
	err := d.DB.Where("user_id = ?", userID).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding achievement progress: %w", err)
	}
	return rows, nil
}

func (d *achievementDao) InsertUnlockWithUnseen(ctx *gin.Context, userID int64, achievementID string, unlockedAt time.Time) (bool, error) {
	inserted, err := d.InsertUnlocksWithUnseen(ctx, userID, []string{achievementID}, unlockedAt)
	if err != nil {
		return false, err
	}
	return len(inserted) == 1, nil
}

func (d *achievementDao) InsertUnlocksWithUnseen(ctx *gin.Context, userID int64, achievementIDs []string, unlockedAt time.Time) ([]string, error) {
	if len(achievementIDs) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(achievementIDs))
	args := make([]interface{}, 0, len(achievementIDs)*4+3)
	for i, id := range achievementIDs {
		placeholders[i] = "(?, ?, ?, ?)"
		args = append(args, userID, id, unlockedAt, unlockedAt)
	}
	// Unlock y unseen viajan en una sola sentencia: si Postgres la confirma,
	// ambos quedan durables aunque la respuesta se pierda en el camino.
	args = append(args, userID, constants.GamificationSectionAchievements, unlockedAt)
	query := fmt.Sprintf(`
		WITH inserted AS (
			INSERT INTO user_achievement_progress (user_id, achievement_id, unlocked_at, created_at)
			VALUES %s
			ON CONFLICT (user_id, achievement_id) DO NOTHING
			RETURNING achievement_id
		), unseen AS (
			INSERT INTO gamification_unseen (user_id, section, created_at)
			SELECT ?, ?, ? FROM inserted
		)
		SELECT achievement_id FROM inserted`, strings.Join(placeholders, ", "))

	var returned []string
	if err := d.DB.Raw(query, args...).Scan(&returned).Error; err != nil {
		return nil, fmt.Errorf("error inserting achievement unlocks: %w", err)
	}

	inserted := make(map[string]struct{}, len(returned))
	for _, id := range returned {
		inserted[id] = struct{}{}
	}
	ordered := make([]string, 0, len(returned))
	seen := make(map[string]struct{}, len(returned))
	for _, id := range achievementIDs {
		if _, ok := inserted[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	return ordered, nil
}

func (d *achievementDao) ListStamps(ctx *gin.Context, userID int64) ([]dbs.GamificationStamp, error) {
	var stamps []dbs.GamificationStamp
	err := d.DB.Where("user_id = ?", userID).Order("granted_at").Find(&stamps).Error
	if err != nil {
		return nil, fmt.Errorf("error finding gamification stamps: %w", err)
	}
	return stamps, nil
}

func (d *achievementDao) CountFinishedSessions(ctx *gin.Context, userID int64) (int64, error) {
	var n int64
	err := d.DB.Model(&dbs.RunnerSession{}).
		Where("athlete_user_id = ? AND status = ?", userID, runnersession.RunnerSessionStatusFinished).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("error counting finished runner sessions: %w", err)
	}
	return n, nil
}

// earlySessionStartHour es la frontera de early_N: start_date local antes de
// las 08:00 America/Argentina/Cordoba.
const earlySessionStartHour = 8

func (d *achievementDao) CountEarlyFinishedSessions(ctx *gin.Context, userID int64) (int64, error) {
	var n int64
	err := d.DB.Raw(`
		SELECT COUNT(*)
		FROM runner_session
		WHERE athlete_user_id = ?
			AND status = ?
			AND EXTRACT(HOUR FROM start_date AT TIME ZONE ?) < ?
		`, userID, runnersession.RunnerSessionStatusFinished,
		gametime.Location.String(), earlySessionStartHour).Scan(&n).Error
	if err != nil {
		return 0, fmt.Errorf("error counting early finished runner sessions: %w", err)
	}
	return n, nil
}

func (d *achievementDao) CountFinishedSessionStats(ctx *gin.Context, userID int64) (FinishedSessionStats, error) {
	var stats FinishedSessionStats
	err := d.DB.Raw(`
		SELECT
			COUNT(*) AS finished,
			COUNT(*) FILTER (
				WHERE EXTRACT(HOUR FROM start_date AT TIME ZONE ?) < ?
			) AS early
		FROM runner_session
		WHERE athlete_user_id = ? AND status = ?
		`, gametime.Location.String(), earlySessionStartHour, userID,
		runnersession.RunnerSessionStatusFinished).Scan(&stats).Error
	if err != nil {
		return FinishedSessionStats{}, fmt.Errorf("error counting finished session stats: %w", err)
	}
	return stats, nil
}

func (d *achievementDao) SumDistanceMeters(ctx *gin.Context, userID int64) (float64, error) {
	var total float64
	err := d.DB.Model(&dbs.WorkoutFeedback{}).
		Select("COALESCE(SUM(distance_meters), 0)").
		Where("athlete_user_id = ? AND deleted_at IS NULL AND distance_meters IS NOT NULL", userID).
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("error summing feedback distance: %w", err)
	}
	return total, nil
}

func (d *achievementDao) MaxDistanceSingleSession(ctx *gin.Context, userID int64) (float64, error) {
	var max float64
	err := d.DB.Raw(`
		SELECT COALESCE(MAX(total), 0) FROM (
			SELECT SUM(distance_meters) AS total
			FROM workout_feedback
			WHERE athlete_user_id = ? AND deleted_at IS NULL AND distance_meters IS NOT NULL
			GROUP BY assigned_session_id
		) per_session`, userID).Scan(&max).Error
	if err != nil {
		return 0, fmt.Errorf("error computing max single-session distance: %w", err)
	}
	return max, nil
}

func (d *achievementDao) SumAndMaxDistance(ctx *gin.Context, userID int64) (DistanceStats, error) {
	var stats DistanceStats
	err := d.DB.Raw(`
		SELECT
			COALESCE(SUM(per_session.total), 0) AS total,
			COALESCE(MAX(per_session.total), 0) AS max_single
		FROM (
			SELECT SUM(distance_meters) AS total
			FROM workout_feedback
			WHERE athlete_user_id = ? AND deleted_at IS NULL AND distance_meters IS NOT NULL
			GROUP BY assigned_session_id
		) per_session`, userID).Scan(&stats).Error
	if err != nil {
		return DistanceStats{}, fmt.Errorf("error computing distance stats: %w", err)
	}
	return stats, nil
}

func (d *achievementDao) CountGPSFinishedSessions(ctx *gin.Context, userID int64) (int64, error) {
	var n int64
	err := d.DB.Raw(`
		SELECT COUNT(DISTINCT points.session_instance_id)
		FROM workout_feedback_points points
		INNER JOIN runner_session rs
			ON rs.session_instance_id = points.session_instance_id
			AND rs.athlete_user_id = ?
			AND rs.status = ?
		INNER JOIN workout_feedback wf
			ON wf.id = points.feedback_id
			AND wf.athlete_user_id = rs.athlete_user_id
		`, userID, runnersession.RunnerSessionStatusFinished).Scan(&n).Error
	if err != nil {
		return 0, fmt.Errorf("error counting gps finished sessions: %w", err)
	}
	return n, nil
}

func (d *achievementDao) ListFinishedSessionStarts(ctx *gin.Context, userID int64) ([]time.Time, error) {
	var starts []time.Time
	err := d.DB.Model(&dbs.RunnerSession{}).
		Where("athlete_user_id = ? AND status = ?", userID, runnersession.RunnerSessionStatusFinished).
		Pluck("start_date", &starts).Error
	if err != nil {
		return nil, fmt.Errorf("error listing finished session starts: %w", err)
	}
	return starts, nil
}

func (d *achievementDao) CountAttendanceDays(ctx *gin.Context, userID int64) (int64, error) {
	var n int64
	err := d.DB.Raw(`
		SELECT COUNT(DISTINCT gcd.date)
		FROM attendances att
		INNER JOIN group_calendar_days gcd
			ON gcd.session_instance_id = att.training_session_id
		WHERE att.user_id = ?`, userID).Scan(&n).Error
	if err != nil {
		return 0, fmt.Errorf("error counting attendance days: %w", err)
	}
	return n, nil
}
