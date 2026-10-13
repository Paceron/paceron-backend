package daos

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/runnersession"
)

// DashboardScopeFilter acota una consulta de métricas. El scope llega YA
// resuelto desde el service: no hay forma de pasar un group_id crudo, y por eso
// el filtro viaja en el WHERE y no sobre el resultado (design.md, spec "El scope
// se aplica en la consulta, no sobre el resultado").
type DashboardScopeFilter struct {
	GroupIDs []int64
	From     time.Time
	To       time.Time
	// Granularity elige el anclaje de calendario del bucket. Vacío = session.
	Granularity string
	// ExerciseIDs son ids del CATÁLOGO contra los que se filtra por
	// source_exercise_id.
	ExerciseIDs []int64
	// LegacyExerciseInstanceIDs son ids de exercise_instances que se resolvieron
	// por nombre normalizado contra el catálogo (instancias anteriores al change
	// de instancia-referencia-catálogo). Descartarlas perdería ese histórico en
	// silencio, que es el riesgo principal de /exercises/{id}/series.
	LegacyExerciseInstanceIDs []int64
	// AthleteUserIDs acota a ciertos corredores (leaderboard, comparisons).
	AthleteUserIDs []int64
}

// DashboardSeriesRow es una fila de feedback con el contexto que las fórmulas del
// KPI necesitan y que no están en la fila de workout_feedback: el kind y el
// objetivo congelado de la ExerciseInstance (design.md D1/D3) y si su sesión tuvo
// fila en runner_session.
type DashboardSeriesRow struct {
	FeedbackID          int64
	SessionInstanceID   int64
	ExerciseInstanceID  int64
	SessionDate         time.Time
	BucketStart         time.Time
	AthleteUserID       int64
	FeedbackOwnerUserID int64
	SetNumber           int
	Kind                string
	TargetMinutes       *int
	TargetDistanceM     *int
	DurationMs          *int64
	ActiveDurationMs    *int64
	DistanceMeters      *float64
	CompletionStatus    *string
	HasRunnerSession    bool
}

// DashboardOpenSessions cuenta las sesiones en estado wip del scope. Es una
// SEGUNDA query a propósito: el WHERE de las series excluye las wip, así que
// contarlas en el mismo result set daría siempre cero (design.md, tarea 3.4d).
type DashboardOpenSessions struct {
	Sessions int
}

// bucketStartExpressions mapea granularidad al anclaje de calendario de
// Postgres. Todos son anclajes de calendario, no relativos al from (spec "Los
// aggregates se agrupan por fecha lógica, no por instante"): date_trunc('week')
// ya es lunes ISO 8601 y 'quarter' corta en enero/abril/julio/octubre.
var bucketStartExpressions = map[string]string{
	"session": "wf.session_date::date",
	"week":    "date_trunc('week', wf.session_date::date)::date",
	"month":   "date_trunc('month', wf.session_date::date)::date",
	"quarter": "date_trunc('quarter', wf.session_date::date)::date",
}

// seriesFromClause es la query core del dashboard (design.md Context). Los cuatro
// joins son obligatorios:
//
//   - workout_feedback por assigned_session_id: las series del día.
//   - exercise_instances por id = assigned_exercise_id: el kind y el objetivo
//     CONGELADO de esa sesión (D3), no el vigente del catálogo.
//   - session_exercise_instances por (session_instance_id, exercise_instance_id):
//     el rol vive en el link, no en feedback ni en la instancia (D9). Sin este
//     JOIN un calentamiento entra al dashboard como principal.
//   - runner_session por (session_instance_id, athlete_user_id): es LEFT, y no
//     negociable. El feedback se crea SIN fila runner_session; con INNER JOIN todo
//     el feedback previo al feature de registro de sesión desaparecería sin que
//     ningún contador lo señale.
//
// role = 'main', el filtro de status y la ventana de membresía van en el WHERE,
// no en un paso posterior sobre las filas.
const seriesFromClause = `
FROM group_calendar_days gcd
JOIN workout_feedback wf
	ON wf.assigned_session_id = gcd.session_instance_id
JOIN exercise_instances ei
	ON ei.id = wf.assigned_exercise_id
JOIN session_exercise_instances sei
	ON sei.session_instance_id = wf.assigned_session_id
	AND sei.exercise_instance_id = wf.assigned_exercise_id
LEFT JOIN runner_session rs
	ON rs.session_instance_id = wf.assigned_session_id
	AND rs.athlete_user_id = wf.athlete_user_id
WHERE gcd.group_id IN ?
  AND gcd.date BETWEEN ? AND ?
  AND sei.role = 'main'
  AND wf.deleted_at IS NULL
  AND (rs.status IN (?, ?) OR rs.status IS NULL)`

// membershipWindowEXISTS acota las series a los corredores que eran miembros del
// grupo en la fecha de la sesión (tarea 6.2): el roster del leaderboard tiene que
// respetar la fecha de la sesión, no la membresía actual.
//
// EXISTS y no un JOIN: un corredor con dos membresías (reingreso) duplicaría sus
// series. Y NO filtra por group_users.deleted_at a propósito: quitar a alguien
// del grupo hace soft-delete de la fila sin setear date_end, así que filtrar por
// deleted_at borraría de un plumazo todo su histórico del dashboard — la peor
// forma de perder datos es la silenciosa.
const membershipWindowEXISTS = `
  AND EXISTS (
	SELECT 1 FROM group_users gu
	WHERE gu.group_id = gcd.group_id
	  AND gu.user_id = wf.athlete_user_id
	  AND gu.date_start::date <= gcd.date::date
	  AND (gu.date_end IS NULL OR gu.date_end::date >= gcd.date::date)
  )`

// countOpenSessionsSQL cuenta sesiones wip del mismo scope. Los mismos cuatro
// joins y el mismo filtro de rol: una sesión a medias no es un dato que el
// dashboard pueda usar, pero su existencia tiene que quedar visible.
const countOpenSessionsSQL = `
SELECT COUNT(DISTINCT (wf.assigned_session_id, wf.athlete_user_id)) AS sessions
FROM group_calendar_days gcd
JOIN workout_feedback wf
	ON wf.assigned_session_id = gcd.session_instance_id
JOIN exercise_instances ei
	ON ei.id = wf.assigned_exercise_id
JOIN session_exercise_instances sei
	ON sei.session_instance_id = wf.assigned_session_id
	AND sei.exercise_instance_id = wf.assigned_exercise_id
JOIN runner_session rs
	ON rs.session_instance_id = wf.assigned_session_id
	AND rs.athlete_user_id = wf.athlete_user_id
WHERE gcd.group_id IN ?
  AND gcd.date BETWEEN ? AND ?
  AND sei.role = 'main'
  AND wf.deleted_at IS NULL
  AND rs.status = ?` + membershipWindowEXISTS

// scopeDateRangeSQL trae MIN/MAX de session_date dentro del scope. Aplica los
// mismos filtros que las series —role main y estado de runner_session— para que
// el rango no prometa datos que ninguna métrica puede mostrar.
const scopeDateRangeSQL = `
SELECT MIN(wf.session_date) AS min_date, MAX(wf.session_date) AS max_date
FROM group_calendar_days gcd
JOIN workout_feedback wf
	ON wf.assigned_session_id = gcd.session_instance_id
JOIN exercise_instances ei
	ON ei.id = wf.assigned_exercise_id
JOIN session_exercise_instances sei
	ON sei.session_instance_id = wf.assigned_session_id
	AND sei.exercise_instance_id = wf.assigned_exercise_id
LEFT JOIN runner_session rs
	ON rs.session_instance_id = wf.assigned_session_id
	AND rs.athlete_user_id = wf.athlete_user_id
WHERE gcd.group_id IN ?
  AND sei.role = 'main'
  AND wf.deleted_at IS NULL
  AND (rs.status IN (?, ?) OR rs.status IS NULL)` + membershipWindowEXISTS

// unlinkedExerciseInstancesSQL lista las ExerciseInstance del scope sin
// source_exercise_id, con el dueño del equipo al que pertenecen. Es la entrada
// del fallback por nombre: sin esto, /exercises/{id}/series perdería en silencio
// todo el histórico anterior al change de instancia-referencia-catálogo.
const unlinkedExerciseInstancesSQL = `
SELECT DISTINCT
	ei.id AS exercise_instance_id,
	ei.name AS name,
	t.owner_id AS owner_id
FROM group_calendar_days gcd
JOIN session_instances si
	ON si.id = gcd.session_instance_id
JOIN session_exercise_instances sei
	ON sei.session_instance_id = si.id
JOIN exercise_instances ei
	ON ei.id = sei.exercise_instance_id
JOIN groups g
	ON g.id = gcd.group_id AND g.deleted_at IS NULL
JOIN teams t
	ON t.id = g.team_id AND t.deleted_at IS NULL
WHERE gcd.group_id IN ?
  AND ei.source_exercise_id IS NULL`

// DashboardDaoInterface define el acceso a datos de métricas del dashboard.
type DashboardDaoInterface interface {
	FindSeries(ctx *gin.Context, filter DashboardScopeFilter) ([]DashboardSeriesRow, error)
	CountOpenSessions(ctx *gin.Context, filter DashboardScopeFilter) (DashboardOpenSessions, error)
	ScopeDateRange(ctx *gin.Context, groupIDs []int64) (minDate, maxDate *time.Time, err error)
	UnlinkedExerciseInstances(ctx *gin.Context, groupIDs []int64) ([]UnlinkedExerciseInstance, error)
	ScopeAthletes(ctx *gin.Context, groupIDs []int64) ([]int64, error)
	FindCatalogExercisesByOwner(ctx *gin.Context, ownerID int64, kinds []string) ([]DashboardCatalogExercises, error)
	FindDashboardCatalogExerciseByID(ctx *gin.Context, id int64) (*DashboardCatalogExercises, error)
	FindDashboardCatalogExercisesByIDs(ctx *gin.Context, ids []int64) ([]DashboardCatalogExercises, error)
}

type dashboardDao struct {
	DB *gorm.DB
}

// NewDashboardDao crea una nueva instancia de DashboardDao.
func NewDashboardDao(database *gorm.DB) DashboardDaoInterface {
	return &dashboardDao{DB: database}
}

// UnlinkedExerciseInstance es una ExerciseInstance sin source_exercise_id junto
// al dueño del equipo al que pertenece (el que define a qué catálogo se resuelve
// por nombre).
type UnlinkedExerciseInstance struct {
	ExerciseInstanceID int64
	Name               string
	OwnerID            int64
}

// FindSeries devuelve las filas crudas que alimentan TODOS los KPI del
// dashboard: resumen, timeseries, leaderboard, comparaciones y detalle de series.
// La agregación vive en el dominio (functions puras ya testeadas), no acá: traer
// las filas crudas es lo que permite que un mismo cálculo se use en los seis
// endpoints sin duplicar fórmulas.
func (d *dashboardDao) FindSeries(ctx *gin.Context, filter DashboardScopeFilter) ([]DashboardSeriesRow, error) {
	if len(filter.GroupIDs) == 0 {
		return nil, nil
	}
	bucket, ok := bucketStartExpressions[filter.Granularity]
	if !ok {
		bucket = bucketStartExpressions["session"]
	}

	selected := []string{
		"wf.id AS feedback_id",
		"wf.assigned_session_id AS session_instance_id",
		"wf.assigned_exercise_id AS exercise_instance_id",
		"wf.session_date AS session_date",
		bucket + " AS bucket_start",
		"wf.athlete_user_id AS athlete_user_id",
		"wf.feedback_owner_user_id AS feedback_owner_user_id",
		"wf.set_number AS set_number",
		"ei.kind AS kind",
		"ei.minutes AS target_minutes",
		"ei.distance_m AS target_distance_m",
		"wf.duration_ms AS duration_ms",
		"wf.active_duration_ms AS active_duration_ms",
		"wf.distance_meters AS distance_meters",
		"wf.completion_status AS completion_status",
		"(rs.id IS NOT NULL) AS has_runner_session",
	}

	args := []interface{}{filter.GroupIDs, filter.From, filter.To,
		runnersession.RunnerSessionStatusFinished, runnersession.RunnerSessionStatusInterrupted}
	conditions := []string{}

	if len(filter.AthleteUserIDs) > 0 {
		conditions = append(conditions, "wf.athlete_user_id IN ?")
		args = append(args, filter.AthleteUserIDs)
	}
	if len(filter.ExerciseIDs) > 0 || len(filter.LegacyExerciseInstanceIDs) > 0 {
		catalogRefs := make([]string, 0, 2)
		if len(filter.ExerciseIDs) > 0 {
			catalogRefs = append(catalogRefs, "ei.source_exercise_id IN ?")
			args = append(args, filter.ExerciseIDs)
		}
		if len(filter.LegacyExerciseInstanceIDs) > 0 {
			catalogRefs = append(catalogRefs, "ei.id IN ?")
			args = append(args, filter.LegacyExerciseInstanceIDs)
		}
		conditions = append(conditions, "("+strings.Join(catalogRefs, " OR ")+")")
	}

	query := "SELECT " + strings.Join(selected, ", ") + seriesFromClause + membershipWindowEXISTS
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY wf.session_date, wf.athlete_user_id, wf.set_number, wf.id"

	var rows []struct {
		FeedbackID          int64     `gorm:"column:feedback_id"`
		SessionInstanceID   int64     `gorm:"column:session_instance_id"`
		ExerciseInstanceID  int64     `gorm:"column:exercise_instance_id"`
		SessionDate         time.Time `gorm:"column:session_date"`
		BucketStart         time.Time `gorm:"column:bucket_start"`
		AthleteUserID       int64     `gorm:"column:athlete_user_id"`
		FeedbackOwnerUserID int64     `gorm:"column:feedback_owner_user_id"`
		SetNumber           int       `gorm:"column:set_number"`
		Kind                string    `gorm:"column:kind"`
		TargetMinutes       *int      `gorm:"column:target_minutes"`
		TargetDistanceM     *int      `gorm:"column:target_distance_m"`
		DurationMs          *int64    `gorm:"column:duration_ms"`
		ActiveDurationMs    *int64    `gorm:"column:active_duration_ms"`
		DistanceMeters      *float64  `gorm:"column:distance_meters"`
		CompletionStatus    *string   `gorm:"column:completion_status"`
		HasRunnerSession    bool      `gorm:"column:has_runner_session"`
	}
	if err := d.DB.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error finding dashboard series: %w", err)
	}

	out := make([]DashboardSeriesRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, DashboardSeriesRow{
			FeedbackID:          row.FeedbackID,
			SessionInstanceID:   row.SessionInstanceID,
			ExerciseInstanceID:  row.ExerciseInstanceID,
			SessionDate:         row.SessionDate,
			BucketStart:         row.BucketStart,
			AthleteUserID:       row.AthleteUserID,
			FeedbackOwnerUserID: row.FeedbackOwnerUserID,
			SetNumber:           row.SetNumber,
			Kind:                row.Kind,
			TargetMinutes:       row.TargetMinutes,
			TargetDistanceM:     row.TargetDistanceM,
			DurationMs:          row.DurationMs,
			ActiveDurationMs:    row.ActiveDurationMs,
			DistanceMeters:      row.DistanceMeters,
			CompletionStatus:    row.CompletionStatus,
			HasRunnerSession:    row.HasRunnerSession,
		})
	}
	return out, nil
}

// CountOpenSessions cuenta las sesiones wip del scope. Van aparte de FindSeries
// porque el WHERE de las series ya las saca del result set.
func (d *dashboardDao) CountOpenSessions(ctx *gin.Context, filter DashboardScopeFilter) (DashboardOpenSessions, error) {
	if len(filter.GroupIDs) == 0 {
		return DashboardOpenSessions{}, nil
	}
	var out struct {
		Sessions int `gorm:"column:sessions"`
	}
	args := []interface{}{filter.GroupIDs, filter.From, filter.To, runnersession.RunnerSessionStatusWip}
	if len(filter.AthleteUserIDs) > 0 {
		args = append(args, filter.AthleteUserIDs)
	}
	query := countOpenSessionsSQL
	if len(filter.AthleteUserIDs) > 0 {
		query += " AND wf.athlete_user_id IN ?"
	}
	if err := d.DB.Raw(query, args...).Scan(&out).Error; err != nil {
		return DashboardOpenSessions{}, fmt.Errorf("error counting dashboard open sessions: %w", err)
	}
	return DashboardOpenSessions{Sessions: out.Sessions}, nil
}

// ScopeDateRange devuelve MIN/MAX de session_date dentro del scope, o nil/nil si
// el scope no tiene ninguna serie registrada. El front usa ese nil para
// distinguir "nunca hubo datos" de "hay datos acotados".
func (d *dashboardDao) ScopeDateRange(ctx *gin.Context, groupIDs []int64) (minDate, maxDate *time.Time, err error) {
	if len(groupIDs) == 0 {
		return nil, nil, nil
	}
	var out struct {
		MinDate *time.Time `gorm:"column:min_date"`
		MaxDate *time.Time `gorm:"column:max_date"`
	}
	args := []interface{}{groupIDs, runnersession.RunnerSessionStatusFinished, runnersession.RunnerSessionStatusInterrupted}
	if err := d.DB.Raw(scopeDateRangeSQL, args...).Scan(&out).Error; err != nil {
		return nil, nil, fmt.Errorf("error finding dashboard date range: %w", err)
	}
	return out.MinDate, out.MaxDate, nil
}

// UnlinkedExerciseInstances devuelve las instancias sin source_exercise_id del
// scope, con el dueño de su equipo. La resolución por nombre normalizado vive en
// el dominio (dashboard.NewNameIndex); el DAO solo trae los candidatos.
func (d *dashboardDao) UnlinkedExerciseInstances(ctx *gin.Context, groupIDs []int64) ([]UnlinkedExerciseInstance, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var rows []struct {
		ExerciseInstanceID int64  `gorm:"column:exercise_instance_id"`
		Name               string `gorm:"column:name"`
		OwnerID            int64  `gorm:"column:owner_id"`
	}
	if err := d.DB.Raw(unlinkedExerciseInstancesSQL, groupIDs).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error finding unlinked exercise instances: %w", err)
	}
	out := make([]UnlinkedExerciseInstance, 0, len(rows))
	for _, row := range rows {
		out = append(out, UnlinkedExerciseInstance{
			ExerciseInstanceID: row.ExerciseInstanceID,
			Name:               row.Name,
			OwnerID:            row.OwnerID,
		})
	}
	return out, nil
}

// ScopeAthletes son los corredores con membresía activa en los grupos del scope.
// En la vista de entrenador es el roster que necesitan leaderboard y
// comparaciones; el filtrado por ventana de fecha de sesión ocurre en la query de
// series, no acá.
func (d *dashboardDao) ScopeAthletes(ctx *gin.Context, groupIDs []int64) ([]int64, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}
	var rows []int64
	if err := d.DB.Raw(`SELECT DISTINCT gu.user_id
FROM group_users gu
WHERE gu.group_id IN ? AND gu.deleted_at IS NULL
ORDER BY gu.user_id`, groupIDs).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error finding dashboard athletes: %w", err)
	}
	return rows, nil
}

// DashboardCatalogExercises son los ejercicios de catálogo del dueño pedido, con
// el objetivo VIGENTE. No es el objetivo con el que se evalúa una sesión histórica
// —ese va congelado en la ExerciseInstance—, y /filters lo rotula explícitamente.
type DashboardCatalogExercises struct {
	ID        int64
	OwnerID   int64
	Name      string
	Kind      string
	Minutes   *int
	DistanceM *int
}

// FindCatalogExercisesByOwner devuelve el catálogo vigente del dueño, limitado a
// los tipos que existen hoy: reusar GetValidExerciseKinds() devolvería los 10
// valores válidos (5 vigentes + 5 legacy) y el tablist del front mostraría tipos
// que ya no están en el catálogo (tarea 6.2b).
func (d *dashboardDao) FindCatalogExercisesByOwner(ctx *gin.Context, ownerID int64, kinds []string) ([]DashboardCatalogExercises, error) {
	if len(kinds) == 0 {
		return nil, nil
	}
	var rows []struct {
		ID        int64  `gorm:"column:id"`
		OwnerID   int64  `gorm:"column:owner_id"`
		Name      string `gorm:"column:name"`
		Kind      string `gorm:"column:kind"`
		Minutes   *int   `gorm:"column:minutes"`
		DistanceM *int   `gorm:"column:distance_m"`
	}
	if err := d.DB.Raw(`SELECT id, owner_id, name, kind, minutes, distance_m
FROM exercises
WHERE owner_id = ? AND deleted_at IS NULL AND kind IN ?
ORDER BY name, id`, ownerID, kinds).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error finding dashboard catalog exercises: %w", err)
	}
	out := make([]DashboardCatalogExercises, 0, len(rows))
	for _, row := range rows {
		out = append(out, DashboardCatalogExercises{
			ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Kind: row.Kind,
			Minutes: row.Minutes, DistanceM: row.DistanceM,
		})
	}
	return out, nil
}

// FindDashboardCatalogExerciseByID trae un ejercicio de catálogo por id.
func (d *dashboardDao) FindDashboardCatalogExerciseByID(ctx *gin.Context, id int64) (*DashboardCatalogExercises, error) {
	var row struct {
		ID        int64  `gorm:"column:id"`
		OwnerID   int64  `gorm:"column:owner_id"`
		Name      string `gorm:"column:name"`
		Kind      string `gorm:"column:kind"`
		Minutes   *int   `gorm:"column:minutes"`
		DistanceM *int   `gorm:"column:distance_m"`
	}
	if err := d.DB.Raw(`SELECT id, owner_id, name, kind, minutes, distance_m
FROM exercises
WHERE id = ? AND deleted_at IS NULL`, id).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("error finding dashboard catalog exercise: %w", err)
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &DashboardCatalogExercises{
		ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Kind: row.Kind,
		Minutes: row.Minutes, DistanceM: row.DistanceM,
	}, nil
}

// FindDashboardCatalogExercisesByIDs trae varios ejercicios de catálogo por id,
// para resolver el filtro `exercise_ids[]` en una sola query.
func (d *dashboardDao) FindDashboardCatalogExercisesByIDs(ctx *gin.Context, ids []int64) ([]DashboardCatalogExercises, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []struct {
		ID        int64  `gorm:"column:id"`
		OwnerID   int64  `gorm:"column:owner_id"`
		Name      string `gorm:"column:name"`
		Kind      string `gorm:"column:kind"`
		Minutes   *int   `gorm:"column:minutes"`
		DistanceM *int   `gorm:"column:distance_m"`
	}
	if err := d.DB.Raw(`SELECT id, owner_id, name, kind, minutes, distance_m
FROM exercises
WHERE id IN ? AND deleted_at IS NULL
ORDER BY id`, ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error finding dashboard catalog exercises by ids: %w", err)
	}
	out := make([]DashboardCatalogExercises, 0, len(rows))
	for _, row := range rows {
		out = append(out, DashboardCatalogExercises{
			ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Kind: row.Kind,
			Minutes: row.Minutes, DistanceM: row.DistanceM,
		})
	}
	return out, nil
}
