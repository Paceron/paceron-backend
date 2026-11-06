package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
)

type TrainerHomeSessionCounts struct {
	Scheduled           int64 `gorm:"column:scheduled"`
	Realized            int64 `gorm:"column:realized"`
	PresencialScheduled int64 `gorm:"column:presencial_scheduled"`
	PresencialRealized  int64 `gorm:"column:presencial_realized"`
}

type TrainerHomeStructureCounts struct {
	ActiveTeams   int64 `gorm:"column:active_teams"`
	ActiveRunners int64 `gorm:"column:active_runners"`
}

type TrainerHomeKpisDaoInterface interface {
	SessionCounts(ctx *gin.Context, ownerID int64, from, to time.Time) (TrainerHomeSessionCounts, error)
	StructureCounts(ctx *gin.Context, ownerID int64) (TrainerHomeStructureCounts, error)
}

type trainerHomeKpisDao struct {
	DB *gorm.DB
}

func NewTrainerHomeKpisDao(database *gorm.DB) TrainerHomeKpisDaoInterface {
	return &trainerHomeKpisDao{DB: database}
}

// SessionCounts agrega días de calendario (no filas de asistencia): varias
// asistencias a una misma sesión cuentan como una sesión realizada.
func (d *trainerHomeKpisDao) SessionCounts(ctx *gin.Context, ownerID int64, from, to time.Time) (TrainerHomeSessionCounts, error) {
	var counts TrainerHomeSessionCounts
	err := d.DB.Raw(`
		SELECT
			COUNT(DISTINCT CASE WHEN gcd.kind = ? AND gcd.session_instance_id IS NOT NULL
				AND gcd.cancelled_reason IS NULL THEN gcd.id END) AS scheduled,
			COUNT(DISTINCT CASE WHEN gcd.kind = ? AND gcd.session_instance_id IS NOT NULL
				AND gcd.cancelled_reason IS NULL AND EXISTS (
					SELECT 1 FROM attendances a
					WHERE a.team_id = t.id AND a.training_session_id = gcd.session_instance_id
				) THEN gcd.id END) AS realized,
			COUNT(DISTINCT CASE WHEN gcd.kind = ? AND gcd.session_instance_id IS NOT NULL
				AND gcd.cancelled_reason IS NULL AND gcd.is_presencial THEN gcd.id END) AS presencial_scheduled,
			COUNT(DISTINCT CASE WHEN gcd.kind = ? AND gcd.session_instance_id IS NOT NULL
				AND gcd.cancelled_reason IS NULL AND gcd.is_presencial AND EXISTS (
					SELECT 1 FROM attendances a
					WHERE a.team_id = t.id AND a.training_session_id = gcd.session_instance_id
				) THEN gcd.id END) AS presencial_realized
		FROM group_calendar_days gcd
		JOIN groups g ON g.id = gcd.group_id AND g.deleted_at IS NULL
		JOIN teams t ON t.id = g.team_id AND t.owner_id = ?
			AND t.deleted_at IS NULL AND t.status = ?
		WHERE gcd.date BETWEEN ? AND ?`,
		string(constants.GroupCalendarDayKindTraining),
		string(constants.GroupCalendarDayKindTraining),
		string(constants.GroupCalendarDayKindTraining),
		string(constants.GroupCalendarDayKindTraining),
		ownerID,
		string(constants.TeamStatusActive),
		from, to,
	).Scan(&counts).Error
	if err != nil {
		return TrainerHomeSessionCounts{}, fmt.Errorf("error counting trainer home sessions: %w", err)
	}
	return counts, nil
}

func (d *trainerHomeKpisDao) StructureCounts(ctx *gin.Context, ownerID int64) (TrainerHomeStructureCounts, error) {
	var counts TrainerHomeStructureCounts
	err := d.DB.Raw(`
		SELECT
			(SELECT COUNT(*) FROM teams t
			 WHERE t.owner_id = ? AND t.deleted_at IS NULL AND t.status = ?) AS active_teams,
			(SELECT COUNT(DISTINCT tu.user_id)
			 FROM team_users tu
			 JOIN teams t ON t.id = tu.team_id AND t.deleted_at IS NULL AND t.status = ?
			 WHERE t.owner_id = ? AND tu.deleted_at IS NULL AND tu.status = ?
				AND tu.role_in_team = ?) AS active_runners`,
		ownerID,
		string(constants.TeamStatusActive),
		string(constants.TeamStatusActive),
		ownerID,
		"active",
		string(constants.TeamUserRoleCorredor),
	).Scan(&counts).Error
	if err != nil {
		return TrainerHomeStructureCounts{}, fmt.Errorf("error counting trainer home structure: %w", err)
	}
	return counts, nil
}
