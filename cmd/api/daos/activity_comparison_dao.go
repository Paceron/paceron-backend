package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// ComparisonSessionDay es una sesión con los datos de su día de calendario.
// GroupID es nil cuando la sesión salió del feedback y no tiene un día que el
// atleta haya tenido asignado (design.md D7 de comparacion-planificado-realizado).
type ComparisonSessionDay struct {
	SessionInstanceID  int64      `gorm:"column:session_instance_id"`
	Date               time.Time  `gorm:"column:date"`
	SessionName        string     `gorm:"column:session_name"`
	TeamID             *int64     `gorm:"column:team_id"`
	TeamName           *string    `gorm:"column:team_name"`
	GroupID            *int64     `gorm:"column:group_id"`
	GroupName          *string    `gorm:"column:group_name"`
	IsPresencial       bool       `gorm:"column:is_presencial"`
	PresencialTimeFrom *time.Time `gorm:"column:presencial_time_from"`
	PresencialTimeTo   *time.Time `gorm:"column:presencial_time_to"`
}

// ComparisonPlannedExercise es un ejercicio planificado de una sesión con su
// objetivo (session_exercise_instances + exercise_instances).
type ComparisonPlannedExercise struct {
	LinkID             int64    `gorm:"column:link_id"`
	SessionInstanceID  int64    `gorm:"column:session_instance_id"`
	ExerciseInstanceID int64    `gorm:"column:exercise_instance_id"`
	Role               string   `gorm:"column:role"`
	RepeatCount        int      `gorm:"column:repeat_count"`
	Name               string   `gorm:"column:name"`
	Kind               string   `gorm:"column:kind"`
	Intensity          *string  `gorm:"column:intensity"`
	Minutes            *int     `gorm:"column:minutes"`
	DistanceM          *int     `gorm:"column:distance_m"`
	SpeedKph           *float64 `gorm:"column:speed_kph"`
}

// ComparisonRangeFilter son los filtros de la lista por rango. Las fechas se
// comparan como fechas (sin hora).
type ComparisonRangeFilter struct {
	AthleteUserID int64
	From          time.Time
	To            time.Time
	TeamID        *int64
	GroupID       *int64
}

// ActivityComparisonDaoInterface agrupa las lecturas de la comparación
// planificado vs realizado. Es de solo lectura.
type ActivityComparisonDaoInterface interface {
	// ListPlannedDays devuelve los días de entrenamiento del rango de los grupos
	// donde el atleta era miembro en esa fecha, ordenados por fecha.
	ListPlannedDays(ctx *gin.Context, filter ComparisonRangeFilter) ([]ComparisonSessionDay, error)
	// ListFeedbackSessions devuelve las sesiones con feedback activo del atleta
	// en el rango, con los datos de su día si lo tiene (sin chequear membresía).
	ListFeedbackSessions(ctx *gin.Context, filter ComparisonRangeFilter) ([]ComparisonSessionDay, error)
	// FindSessionDay devuelve la sesión con su día de calendario, o nil si la
	// instancia no existe.
	FindSessionDay(ctx *gin.Context, sessionInstanceID int64) (*ComparisonSessionDay, error)
	// ListPlannedExercises devuelve los ejercicios planificados de varias
	// sesiones, en el orden de la sesión. Lote vacío no toca la DB.
	ListPlannedExercises(ctx *gin.Context, sessionInstanceIDs []int64) ([]ComparisonPlannedExercise, error)
	// ListFeedback devuelve el feedback activo del atleta en varias sesiones.
	ListFeedback(ctx *gin.Context, athleteUserID int64, sessionInstanceIDs []int64) ([]dbs.WorkoutFeedback, error)
	// ListRunnerSessions devuelve los runner_session del atleta en varias sesiones.
	ListRunnerSessions(ctx *gin.Context, athleteUserID int64, sessionInstanceIDs []int64) ([]dbs.RunnerSession, error)
	// IsSessionTeamOwner dice si el usuario es dueño del equipo de la sesión, por
	// su día de calendario o por el equipo de su feedback.
	IsSessionTeamOwner(ctx *gin.Context, sessionInstanceID, userID int64) (bool, error)
	// IsTeamMemberOfOwner dice si el atleta es integrante activo del equipo
	// teamID y ese equipo es del usuario ownerID.
	IsTeamMemberOfOwner(ctx *gin.Context, teamID, athleteUserID, ownerID int64) (bool, error)
}

type activityComparisonDao struct {
	DB *gorm.DB
}

func NewActivityComparisonDao(database *gorm.DB) ActivityComparisonDaoInterface {
	return &activityComparisonDao{DB: database}
}

const comparisonDayColumns = `gcd.session_instance_id, gcd.date, si.name AS session_name,
	t.id AS team_id, t.name AS team_name, g.id AS group_id, g.name AS group_name,
	gcd.is_presencial, gcd.presencial_time_from, gcd.presencial_time_to`

func (d *activityComparisonDao) ListPlannedDays(ctx *gin.Context, filter ComparisonRangeFilter) ([]ComparisonSessionDay, error) {
	var rows []ComparisonSessionDay
	query := d.DB.Table("group_calendar_days gcd").
		Select(comparisonDayColumns).
		Joins("JOIN session_instances si ON si.id = gcd.session_instance_id").
		Joins("JOIN groups g ON g.id = gcd.group_id AND g.deleted_at IS NULL").
		Joins("JOIN teams t ON t.id = g.team_id AND t.deleted_at IS NULL").
		Where("gcd.kind = ? AND gcd.session_instance_id IS NOT NULL", string(constants.GroupCalendarDayKindTraining)).
		Where("gcd.date BETWEEN ?::date AND ?::date", filter.From, filter.To).
		// Miembro en la fecha del día, no hoy: dejar un grupo pone deleted_at.
		Where(`EXISTS (SELECT 1 FROM group_users gu WHERE gu.group_id = g.id AND gu.user_id = ?
			AND gu.date_start::date <= gcd.date
			AND (gu.date_end IS NULL OR gu.date_end::date >= gcd.date)
			AND (gu.deleted_at IS NULL OR gu.deleted_at::date >= gcd.date))`, filter.AthleteUserID)
	if filter.TeamID != nil {
		query = query.Where("t.id = ?", *filter.TeamID)
	}
	if filter.GroupID != nil {
		query = query.Where("g.id = ?", *filter.GroupID)
	}
	if err := query.Order("gcd.date ASC, gcd.session_instance_id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error listing planned days for comparison: %w", err)
	}
	return rows, nil
}

func (d *activityComparisonDao) ListFeedbackSessions(ctx *gin.Context, filter ComparisonRangeFilter) ([]ComparisonSessionDay, error) {
	var rows []ComparisonSessionDay
	// Una fila por sesión: los datos del día si existe y, si no, el equipo del
	// feedback. La fecha es la del día o, sin día, la primera del feedback.
	query := d.DB.Table("workout_feedback wf").
		Select(`wf.assigned_session_id AS session_instance_id,
			COALESCE(MIN(gcd.date), MIN(wf.session_date)) AS date,
			COALESCE(MIN(si.name), '') AS session_name,
			COALESCE(MIN(dt.id), MIN(ft.id)) AS team_id,
			COALESCE(MIN(dt.name), MIN(ft.name)) AS team_name,
			MIN(g.id) AS group_id, MIN(g.name) AS group_name,
			COALESCE(BOOL_OR(gcd.is_presencial), false) AS is_presencial,
			MIN(gcd.presencial_time_from) AS presencial_time_from,
			MIN(gcd.presencial_time_to) AS presencial_time_to`).
		Joins("LEFT JOIN session_instances si ON si.id = wf.assigned_session_id").
		Joins("LEFT JOIN group_calendar_days gcd ON gcd.session_instance_id = wf.assigned_session_id").
		Joins("LEFT JOIN groups g ON g.id = gcd.group_id AND g.deleted_at IS NULL").
		Joins("LEFT JOIN teams dt ON dt.id = g.team_id AND dt.deleted_at IS NULL").
		Joins("LEFT JOIN teams ft ON ft.id = wf.team_id AND ft.deleted_at IS NULL").
		Where("wf.athlete_user_id = ? AND wf.deleted_at IS NULL", filter.AthleteUserID).
		Where("wf.session_date BETWEEN ?::date AND ?::date", filter.From, filter.To)
	if filter.TeamID != nil {
		query = query.Where("(g.team_id = ? OR (gcd.id IS NULL AND wf.team_id = ?))", *filter.TeamID, *filter.TeamID)
	}
	if err := query.Group("wf.assigned_session_id").Order("date ASC, session_instance_id ASC").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error listing feedback sessions for comparison: %w", err)
	}
	return rows, nil
}

func (d *activityComparisonDao) FindSessionDay(ctx *gin.Context, sessionInstanceID int64) (*ComparisonSessionDay, error) {
	var rows []ComparisonSessionDay
	err := d.DB.Table("session_instances si").
		Select(`si.id AS session_instance_id, COALESCE(gcd.date, si.created_at::date) AS date, si.name AS session_name,
			t.id AS team_id, t.name AS team_name, g.id AS group_id, g.name AS group_name,
			COALESCE(gcd.is_presencial, false) AS is_presencial,
			gcd.presencial_time_from, gcd.presencial_time_to`).
		Joins("LEFT JOIN group_calendar_days gcd ON gcd.session_instance_id = si.id").
		Joins("LEFT JOIN groups g ON g.id = gcd.group_id AND g.deleted_at IS NULL").
		Joins("LEFT JOIN teams t ON t.id = g.team_id AND t.deleted_at IS NULL").
		Where("si.id = ?", sessionInstanceID).
		Order("gcd.id ASC").
		Limit(1).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error finding session day for comparison: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (d *activityComparisonDao) ListPlannedExercises(ctx *gin.Context, sessionInstanceIDs []int64) ([]ComparisonPlannedExercise, error) {
	rows := []ComparisonPlannedExercise{}
	if len(sessionInstanceIDs) == 0 {
		return rows, nil
	}
	err := d.DB.Table("session_exercise_instances sei").
		Select(`sei.id AS link_id, sei.session_instance_id, sei.exercise_instance_id, sei.role, sei.repeat_count,
			ei.name, ei.kind, ei.intensity, ei.minutes, ei.distance_m, ei.speed_kph`).
		Joins("JOIN exercise_instances ei ON ei.id = sei.exercise_instance_id").
		Where("sei.session_instance_id IN ?", sessionInstanceIDs).
		Order("sei.session_instance_id ASC, sei.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing planned exercises for comparison: %w", err)
	}
	return rows, nil
}

func (d *activityComparisonDao) ListFeedback(ctx *gin.Context, athleteUserID int64, sessionInstanceIDs []int64) ([]dbs.WorkoutFeedback, error) {
	rows := []dbs.WorkoutFeedback{}
	if len(sessionInstanceIDs) == 0 {
		return rows, nil
	}
	err := d.DB.Where("athlete_user_id = ? AND assigned_session_id IN ? AND deleted_at IS NULL", athleteUserID, sessionInstanceIDs).
		Order("assigned_session_id ASC, assigned_exercise_id ASC, set_number ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing feedback for comparison: %w", err)
	}
	return rows, nil
}

func (d *activityComparisonDao) ListRunnerSessions(ctx *gin.Context, athleteUserID int64, sessionInstanceIDs []int64) ([]dbs.RunnerSession, error) {
	rows := []dbs.RunnerSession{}
	if len(sessionInstanceIDs) == 0 {
		return rows, nil
	}
	err := d.DB.Where("athlete_user_id = ? AND session_instance_id IN ?", athleteUserID, sessionInstanceIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing runner sessions for comparison: %w", err)
	}
	return rows, nil
}

func (d *activityComparisonDao) IsSessionTeamOwner(ctx *gin.Context, sessionInstanceID, userID int64) (bool, error) {
	var owner bool
	err := d.DB.Raw(`SELECT EXISTS (
			SELECT 1 FROM group_calendar_days gcd
			JOIN groups g ON g.id = gcd.group_id AND g.deleted_at IS NULL
			JOIN teams t ON t.id = g.team_id AND t.deleted_at IS NULL
			WHERE gcd.session_instance_id = ? AND t.owner_id = ?
		) OR EXISTS (
			SELECT 1 FROM workout_feedback wf
			JOIN teams t ON t.id = wf.team_id AND t.deleted_at IS NULL
			WHERE wf.assigned_session_id = ? AND wf.deleted_at IS NULL AND t.owner_id = ?
		)`, sessionInstanceID, userID, sessionInstanceID, userID).Scan(&owner).Error
	if err != nil {
		return false, fmt.Errorf("error checking session team owner: %w", err)
	}
	return owner, nil
}

func (d *activityComparisonDao) IsTeamMemberOfOwner(ctx *gin.Context, teamID, athleteUserID, ownerID int64) (bool, error) {
	var member bool
	err := d.DB.Raw(`SELECT EXISTS (
			SELECT 1 FROM team_users tu
			JOIN teams t ON t.id = tu.team_id AND t.deleted_at IS NULL
			WHERE tu.team_id = ? AND tu.user_id = ? AND tu.deleted_at IS NULL AND t.owner_id = ?
		)`, teamID, athleteUserID, ownerID).Scan(&member).Error
	if err != nil {
		return false, fmt.Errorf("error checking team member of owner: %w", err)
	}
	return member, nil
}
