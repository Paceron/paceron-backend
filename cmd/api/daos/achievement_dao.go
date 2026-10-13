package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// AchievementDAOInterface reúne el acceso a datos de logros (gamificacion-core
// D5): lectura/escritura idempotente del progreso, la vitrina de estampas y
// las queries de agregación por criterio de los evaluadores.
type AchievementDAOInterface interface {
	// ListProgress devuelve todas las filas de progreso del usuario.
	ListProgress(ctx *gin.Context, userID int64) ([]dbs.UserAchievementProgress, error)
	// InsertUnlock inserta el desbloqueo si (user_id, achievement_id) no
	// existía (ON CONFLICT DO NOTHING); reporta si la fila es NUEVA (el push
	// y las unseen solo aplican al primer desbloqueo).
	InsertUnlock(ctx *gin.Context, userID int64, achievementID string, unlockedAt time.Time) (bool, error)
	// ListStamps devuelve las estampas del usuario (vitrina; vacía hasta E4).
	ListStamps(ctx *gin.Context, userID int64) ([]dbs.GamificationStamp, error)

	// CountFinishedSessions: histórico de runner sessions finished.
	CountFinishedSessions(ctx *gin.Context, userID int64) (int64, error)
	// SumDistanceMeters: suma de distance_meters de feedbacks activos.
	SumDistanceMeters(ctx *gin.Context, userID int64) (float64, error)
	// MaxDistanceSingleSession: MAX del SUM por sesión asignada.
	MaxDistanceSingleSession(ctx *gin.Context, userID int64) (float64, error)
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

func (d *achievementDao) InsertUnlock(ctx *gin.Context, userID int64, achievementID string, unlockedAt time.Time) (bool, error) {
	row := dbs.UserAchievementProgress{
		UserID:        userID,
		AchievementID: achievementID,
		UnlockedAt:    unlockedAt,
	}
	res := d.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if res.Error != nil {
		return false, fmt.Errorf("error inserting achievement unlock: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
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

func (d *achievementDao) CountGPSFinishedSessions(ctx *gin.Context, userID int64) (int64, error) {
	var n int64
	err := d.DB.Raw(`
		SELECT COUNT(DISTINCT points.session_instance_id)
		FROM workout_feedback_points points
		INNER JOIN runner_session rs
			ON rs.session_instance_id = points.session_instance_id
			AND rs.athlete_user_id = ?
			AND rs.status = ?`, userID, runnersession.RunnerSessionStatusFinished).Scan(&n).Error
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
