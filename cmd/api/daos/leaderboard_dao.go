package daos

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// Leaderboard por team (gamificacion-core D7). Cada agregación es UNA query
// GROUP BY user_id sobre su tabla fuente; los nombres/avatar se resuelven en
// batch con el DAO de usuarios existente. El owner del team nunca genera fila
// propia (exclusión por subquery sobre teams.owner_id).
type LeaderboardDAOInterface interface {
	// IsTeamGroupMember indica si userID es miembro activo de ALGUNO de los
	// grupos del team (mismo criterio de membresía activa que el resto del
	// módulo: deleted_at IS NULL y date_end vigente).
	IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error)
	// IsTeamOwner indica si userID es el owner del team.
	IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error)
	// AggregateFeedback agrega las métricas fuente workout_feedback
	// (distance/duration/pace/peak_speed) por atleta del team en el período.
	AggregateFeedback(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error)
	// AggregateRunnerSessions agrega sessions_count (runner sessions finished
	// sobre instancias de grupos del team) por atleta en el período.
	AggregateRunnerSessions(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardSessionsAgg, error)
	// AggregateAttendances agrega attendance_count (asistencias registradas del
	// team) por usuario en el período.
	AggregateAttendances(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardAttendanceAgg, error)
	// UsersByIDs resuelve nombre/avatar_url en batch (1 query IN, sin N+1).
	UsersByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error)
}

type leaderboardDao struct {
	DB    *gorm.DB
	users UserDaoInterface
}

func NewLeaderboardDao(database *gorm.DB) LeaderboardDAOInterface {
	return &leaderboardDao{DB: database, users: NewUserDao(database)}
}

// leaderboardTZ es la TZ del spec (D1): los timestamps (start_date/created_at)
// se proyectan a fecha de pared de Cordoba antes de compararlos con los bordes
// del período. Interpolada en el SQL a propósito: es una constante del spec,
// nunca input del cliente.
var leaderboardTZ = gametime.Location.String()

func (d *leaderboardDao) IsTeamGroupMember(ctx *gin.Context, teamID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.GroupUser{}).
		Joins("JOIN groups ON groups.id = group_users.group_id AND groups.deleted_at IS NULL").
		Where("groups.team_id = ? AND group_users.user_id = ? AND group_users.deleted_at IS NULL AND (group_users.date_end IS NULL OR group_users.date_end > NOW())", teamID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking leaderboard group membership: %w", err)
	}
	return count > 0, nil
}

func (d *leaderboardDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	var count int64
	err := d.DB.Model(&dbs.Team{}).
		Where("id = ? AND owner_id = ? AND deleted_at IS NULL", teamID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("error checking leaderboard team owner: %w", err)
	}
	return count > 0, nil
}

func (d *leaderboardDao) AggregateFeedback(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardFeedbackAgg, error) {
	// Filtros FILTER por métrica: el desempate general cuenta actividades con
	// métrica > 0; peak_argmax_date trae la session_date de la fila del max
	// (a empate la más reciente) para el record de period=all.
	query := d.DB.Table("workout_feedback AS wf").
		Select(`wf.athlete_user_id AS user_id,
			SUM(wf.distance_meters) AS sum_distance,
			SUM(wf.duration_ms) AS sum_duration_ms,
			MAX(wf.peak_speed_kph) AS max_peak,
			COUNT(*) FILTER (WHERE wf.distance_meters > 0) AS distance_count,
			COUNT(*) FILTER (WHERE wf.duration_ms > 0) AS duration_count,
			COUNT(*) FILTER (WHERE wf.duration_ms > 0 AND wf.distance_meters > 0) AS pace_count,
			COUNT(*) FILTER (WHERE wf.peak_speed_kph IS NOT NULL) AS peak_count,
			MAX(wf.session_date) FILTER (WHERE wf.distance_meters > 0) AS distance_last_date,
			MAX(wf.session_date) FILTER (WHERE wf.duration_ms > 0) AS duration_last_date,
			MAX(wf.session_date) FILTER (WHERE wf.duration_ms > 0 AND wf.distance_meters > 0) AS pace_last_date,
			(ARRAY_AGG(wf.session_date ORDER BY wf.peak_speed_kph DESC NULLS LAST, wf.session_date DESC))[1] AS peak_argmax_date`).
		Where("wf.deleted_at IS NULL AND wf.team_id = ?", teamID).
		Where("wf.athlete_user_id <> (SELECT owner_id FROM teams WHERE id = ? AND deleted_at IS NULL)", teamID)
	query = applyLeaderboardDateRange(query, "wf.session_date", from, to)

	var rows []dbs.LeaderboardFeedbackAgg
	if err := query.Group("wf.athlete_user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error aggregating leaderboard feedback: %w", err)
	}
	return rows, nil
}

func (d *leaderboardDao) AggregateRunnerSessions(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardSessionsAgg, error) {
	query := d.DB.Table("runner_session AS rs").
		Select(fmt.Sprintf(`rs.athlete_user_id AS user_id,
			COUNT(DISTINCT rs.id) AS sessions_count,
			MAX(rs.start_date AT TIME ZONE '%s') AS last_start`, leaderboardTZ)).
		Joins("JOIN group_calendar_days gcd ON gcd.session_instance_id = rs.session_instance_id").
		Joins("JOIN groups g ON g.id = gcd.group_id AND g.deleted_at IS NULL").
		Where("g.team_id = ? AND rs.status = 'finished'", teamID).
		Where("rs.athlete_user_id <> (SELECT owner_id FROM teams WHERE id = ? AND deleted_at IS NULL)", teamID)
	query = applyLeaderboardDateRange(query, fmt.Sprintf("(rs.start_date AT TIME ZONE '%s')::date", leaderboardTZ), from, to)

	var rows []dbs.LeaderboardSessionsAgg
	if err := query.Group("rs.athlete_user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error aggregating leaderboard sessions: %w", err)
	}
	return rows, nil
}

func (d *leaderboardDao) AggregateAttendances(ctx *gin.Context, teamID int64, from, to *string) ([]dbs.LeaderboardAttendanceAgg, error) {
	query := d.DB.Table("attendances AS att").
		Select(fmt.Sprintf(`att.user_id AS user_id,
			COUNT(*) AS attendance_count,
			MAX(att.created_at AT TIME ZONE '%s') AS last_registered_at`, leaderboardTZ)).
		Where("att.team_id = ?", teamID).
		Where("att.user_id <> (SELECT owner_id FROM teams WHERE id = ? AND deleted_at IS NULL)", teamID)
	query = applyLeaderboardDateRange(query, fmt.Sprintf("(att.created_at AT TIME ZONE '%s')::date", leaderboardTZ), from, to)

	var rows []dbs.LeaderboardAttendanceAgg
	if err := query.Group("att.user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error aggregating leaderboard attendances: %w", err)
	}
	return rows, nil
}

func (d *leaderboardDao) UsersByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	return d.users.FindByIDs(ctx, userIDs)
}

// applyLeaderboardDateRange agrega los bordes del período. Los bordes viajan
// como string YYYY-MM-DD y se castean a ::date: comparar una columna date
// contra un time.Time con TZ dependiente del DSN es una trampa (medianoche
// Cordoba = 03:00 UTC excluiría el primer día).
func applyLeaderboardDateRange(query *gorm.DB, dateExpr string, from, to *string) *gorm.DB {
	if from != nil {
		query = query.Where(dateExpr+" >= ?::date", *from)
	}
	if to != nil {
		query = query.Where(dateExpr+" <= ?::date", *to)
	}
	return query
}
