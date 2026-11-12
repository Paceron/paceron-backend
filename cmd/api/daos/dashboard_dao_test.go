package daos

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/testutils"
)

// dashboardFixture arma el escenario mínimo que la query core del dashboard
// necesita: un entrenador con su equipo, su grupo, un día de calendario con su
// SessionInstance, un ExerciseInstance de bloque principal y uno de calentamiento,
// y un corredor con membresía.
type dashboardFixture struct {
	db          *gorm.DB
	dao         DashboardDaoInterface
	ownerID     int64
	teamID      int64
	groupID     int64
	athleteID   int64
	mainEx      int64
	warmupEx    int64
	sessionInst int64
	calendarDay time.Time
	catalogEx   int64
}

func newDashboardFixture(t *testing.T, db *gorm.DB, tag string) *dashboardFixture {
	t.Helper()
	owner := persistUser(db, "dash-owner-"+tag+"@test.com", "6"+tag)
	athlete := persistUser(db, "dash-athlete-"+tag+"@test.com", "7"+tag)
	team := testTeam(db, "equipo_dash_"+tag, owner.ID)
	group := testGroup(db, "grupo_dash_"+tag, team.ID)

	sessionDate := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	sessionInst := testSessionInstance(db, "sesion-dash-"+tag)
	require.NoError(t, NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{
		GroupID: group.ID, Date: sessionDate, Kind: "training", SessionInstanceID: &sessionInst,
	}))

	catalog := &dbs.Exercise{OwnerID: owner.ID, Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtrDAO(30), DistanceM: intPtrDAO(4000)}
	require.NoError(t, db.Create(catalog).Error)
	catalogEx := catalog.ID

	mainEx := persistExerciseInstance(t, db, sessionInst, "Distancia 4 km", string(constants.ExerciseKindRitmoContinuo),
		intPtrDAO(30), intPtrDAO(4000), &catalogEx, string(constants.SessionExerciseRoleMain))
	warmupEx := persistExerciseInstance(t, db, sessionInst, "Sparring", string(constants.ExerciseKindLiviano),
		intPtrDAO(10), nil, nil, string(constants.SessionExerciseRoleWarmup))

	require.NoError(t, NewGroupUserDao(db).Create(nil, &dbs.GroupUser{
		GroupID: group.ID, UserID: athlete.ID, DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}))

	return &dashboardFixture{
		db: db, dao: NewDashboardDao(db), ownerID: owner.ID, teamID: team.ID, groupID: group.ID,
		athleteID: athlete.ID, mainEx: mainEx, warmupEx: warmupEx, sessionInst: sessionInst,
		calendarDay: sessionDate, catalogEx: catalog.ID,
	}
}

func persistExerciseInstance(t *testing.T, db *gorm.DB, sessionInstanceID int64, name, kind string, minutes, distanceM *int, sourceEx *int64, role string) int64 {
	t.Helper()
	instance := &dbs.ExerciseInstance{Name: name, Kind: kind, Minutes: minutes, DistanceM: distanceM, SourceExerciseID: sourceEx}
	require.NoError(t, db.Create(instance).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: sessionInstanceID, ExerciseInstanceID: instance.ID, Role: role,
	}).Error)
	return instance.ID
}

func intPtrDAO(v int) *int { return &v }

// persistFeedback crea una fila de workout_feedback para el día del fixture.
func (f *dashboardFixture) persistFeedback(t *testing.T, exerciseInstanceID, athleteID int64, setNumber int, timeMs int64, distanceM *float64) {
	t.Helper()
	require.NoError(t, f.db.Create(&dbs.WorkoutFeedback{
		TeamID:              &f.teamID,
		AssignedSessionID:   f.sessionInst,
		AssignedExerciseID:  exerciseInstanceID,
		AthleteUserID:       athleteID,
		FeedbackOwnerUserID: athleteID,
		ReportSource:        "atleta",
		SessionDate:         f.calendarDay,
		SetNumber:           setNumber,
		DurationMs:          &timeMs,
		ActiveDurationMs:    &timeMs,
		DistanceMeters:      distanceM,
		CompletionStatus:    strPtrDAO("completa"),
		MediaURLs:           emptyMediaURLs(t),
	}).Error)
}

func (f *dashboardFixture) persistRunnerSession(t *testing.T, athleteID int64, status string) {
	t.Helper()
	require.NoError(t, f.db.Create(&dbs.RunnerSession{
		SessionInstanceID: f.sessionInst,
		AthleteUserID:     athleteID,
		Status:            status,
		StartDate:         f.calendarDay,
	}).Error)
}

func (f *dashboardFixture) filter() DashboardScopeFilter {
	return DashboardScopeFilter{
		GroupIDs:    []int64{f.groupID},
		From:        f.calendarDay.AddDate(0, 0, -1),
		To:          f.calendarDay.AddDate(0, 0, 1),
		Granularity: "session",
	}
}

func strPtrDAO(v string) *string     { return &v }
func floatPtrDAO(v float64) *float64 { return &v }

// 5.5: el filtro `sei.role = 'main'` tiene que estar en la consulta. Sin él, un
// calentamiento con series cargadas entra al dashboard como bloque principal.
func TestDashboardDao_FindSeries_SoloRolMain(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "rolemain")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistFeedback(t, fixture.warmupEx, fixture.athleteID, 1, 600000, nil)
	fixture.persistRunnerSession(t, fixture.athleteID, runnersession.RunnerSessionStatusFinished)

	filter := fixture.filter()
	filter.SessionInstanceID = &fixture.sessionInst
	rows, err := fixture.dao.FindSeries(nil, filter)

	require.NoError(t, err)
	require.Len(t, rows, 1, "el calentamiento no puede entrar al dashboard")
	assert.Equal(t, fixture.mainEx, rows[0].ExerciseInstanceID)
	assert.Equal(t, string(constants.ExerciseKindRitmoContinuo), rows[0].Kind)
	require.NotNil(t, rows[0].TargetMinutes)
	assert.Equal(t, 30, *rows[0].TargetMinutes, "el objetivo sale de la ExerciseInstance, no del catálogo")
	assert.True(t, rows[0].HasRunnerSession)
}

func TestDashboardDao_FindSeries_AcotaUnaSesionAtletaEjercicioYRPE(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "session-comparison")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistFeedback(t, fixture.warmupEx, fixture.athleteID, 2, 600000, floatPtrDAO(1000))
	rpe := int16(8)
	require.NoError(t, db.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_session_id = ? AND assigned_exercise_id = ? AND athlete_user_id = ? AND set_number = ?",
			fixture.sessionInst, fixture.mainEx, fixture.athleteID, 1).
		Update("rpe", rpe).Error)
	sessionID := fixture.sessionInst

	filter := fixture.filter()
	filter.SessionInstanceID = &sessionID
	filter.AthleteUserIDs = []int64{fixture.athleteID}
	filter.ExerciseIDs = []int64{fixture.catalogEx}
	rows, err := fixture.dao.FindSeries(nil, filter)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, fixture.mainEx, rows[0].ExerciseInstanceID)
	assert.Equal(t, fixture.sessionInst, rows[0].SessionInstanceID)
	require.NotNil(t, rows[0].RPE)
	assert.Equal(t, rpe, *rows[0].RPE)

	otherSessionID := fixture.sessionInst + 999
	filter.SessionInstanceID = &otherSessionID
	rows, err = fixture.dao.FindSeries(nil, filter)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// El LEFT JOIN a runner_session es lo que mantiene el histórico previo al feature
// de registro de sesión: con INNER JOIN ese feedback desaparecería sin que ningún
// contador lo señale.
func TestDashboardDao_FindSeries_FeedbackSinRunnerSessionEntra(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "legacy")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 2, 250000, floatPtrDAO(4000))
	// Sin ninguna fila en runner_session: es el caso del feedback cargado antes del
	// feature de registro de sesión.

	rows, err := fixture.dao.FindSeries(nil, fixture.filter())

	require.NoError(t, err)
	require.Len(t, rows, 2, "con INNER JOIN esto quedaría vacío sin que nada lo señale")
	assert.False(t, rows[0].HasRunnerSession)
	assert.False(t, rows[1].HasRunnerSession)
}

// Las sesiones wip quedan fuera del KPI y se cuentan aparte — y para eso hace
// falta una segunda query, porque el WHERE de las series ya las sacó.
func TestDashboardDao_WipNoEntraYSeCuentaEnOpenSessions(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "wip")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 2, 250000, floatPtrDAO(4000))
	fixture.persistRunnerSession(t, fixture.athleteID, runnersession.RunnerSessionStatusWip)

	filter := fixture.filter()
	filter.SessionInstanceID = &fixture.sessionInst
	rows, err := fixture.dao.FindSeries(nil, filter)
	require.NoError(t, err)
	assert.Empty(t, rows, "una sesión a medias no alimenta los indicadores")

	open, err := fixture.dao.CountOpenSessions(nil, fixture.filter())
	require.NoError(t, err)
	assert.Equal(t, 1, open.Sessions, "y queda visible en open_sessions")
}

func TestDashboardDao_InterruptedEntraAlKPI(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "interr")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistRunnerSession(t, fixture.athleteID, runnersession.RunnerSessionStatusInterrupted)

	filter := fixture.filter()
	filter.SessionInstanceID = &fixture.sessionInst
	rows, err := fixture.dao.FindSeries(nil, filter)

	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func TestDashboardDao_FeedbackSoftDeletedNoEntra(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "softdel")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	require.NoError(t, db.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_session_id = ?", fixture.sessionInst).
		Update("deleted_at", gorm.Expr("NOW()")).Error)

	rows, err := fixture.dao.FindSeries(nil, fixture.filter())

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestDashboardDao_ScopeTravelsEnElWhere(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "scope")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))

	otherGroup := testGroup(db, "grupo_dash_scope_otro", fixture.teamID)

	rows, err := fixture.dao.FindSeries(nil, DashboardScopeFilter{
		GroupIDs: []int64{otherGroup.ID},
		From:     fixture.calendarDay.AddDate(0, 0, -1),
		To:       fixture.calendarDay.AddDate(0, 0, 1),
	})

	require.NoError(t, err)
	assert.Empty(t, rows, "otro grupo del mismo equipo no ve las series de este")
}

// La ventana de membresía se evalúa contra la fecha de la sesión: un corredor que
// se sumó después no aparece, y uno que se fue antes tampoco.
func TestDashboardDao_VentanaDeMembresia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "membresia")
	late := persistUser(db, fmt.Sprintf("dash-late-%d@test.com", fixture.groupID), fmt.Sprintf("8%09d", fixture.groupID%100000000))
	require.NoError(t, NewGroupUserDao(db).Create(nil, &dbs.GroupUser{
		GroupID: fixture.groupID, UserID: late.ID, DateStart: fixture.calendarDay.AddDate(0, 0, 30),
	}))
	fixture.persistFeedback(t, fixture.mainEx, late.ID, 1, 240000, floatPtrDAO(4000))

	rows, err := fixture.dao.FindSeries(nil, fixture.filter())

	require.NoError(t, err)
	assert.Empty(t, rows, "un corredor que se sumó después de la sesión no tiene series esa fecha")

	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	rows, err = fixture.dao.FindSeries(nil, fixture.filter())
	require.NoError(t, err)
	require.Len(t, rows, 1, "el que era miembro en la fecha de la sesión entra")

	// Con membership soft-deleted la serie sigue entrando: el histórico de quien
	// se va del grupo no puede desaparecer en silencio.
	require.NoError(t, db.Model(&dbs.GroupUser{}).
		Where("group_id = ? AND user_id = ?", fixture.groupID, fixture.athleteID).
		Update("deleted_at", gorm.Expr("NOW()")).Error)

	rows, err = fixture.dao.FindSeries(nil, fixture.filter())
	require.NoError(t, err)
	assert.Len(t, rows, 1, "el soft-delete de la membresía no borra el historial")
}

func TestDashboardDao_BucketsDeCalendario(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "buckets")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))

	// 2026-09-15 es martes: la semana arranca el lunes 14 y el mes, el día 1.
	for _, tc := range []struct {
		granularity string
		expected    string
	}{
		{"session", "2026-09-15"},
		{"week", "2026-09-14"},
		{"month", "2026-09-01"},
		{"quarter", "2026-07-01"},
	} {
		filter := fixture.filter()
		filter.Granularity = tc.granularity

		rows, err := fixture.dao.FindSeries(nil, filter)

		require.NoError(t, err, "granularity=%s", tc.granularity)
		require.Len(t, rows, 1)
		assert.Equal(t, tc.expected, rows[0].BucketStart.Format("2006-01-02"), "granularity=%s", tc.granularity)
	}
}

// Una granularidad desconocida no puede colarse en la query: degrada a session.
func TestDashboardDao_GranularidadDesconocidaDegradaASession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "granon")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))

	filter := fixture.filter()
	filter.Granularity = "hour; DROP TABLE exercises"

	rows, err := fixture.dao.FindSeries(nil, filter)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "2026-09-15", rows[0].BucketStart.Format("2006-01-02"))
}

func TestDashboardDao_ScopeDateRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "rango")

	minDate, maxDate, err := fixture.dao.ScopeDateRange(nil, []int64{fixture.groupID})
	require.NoError(t, err)
	assert.Nil(t, minDate, "sin datos el rango viene null, no {min: null, max: null}")
	assert.Nil(t, maxDate)

	second := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	require.NoError(t, fixture.db.Create(&dbs.GroupCalendarDay{
		GroupID: fixture.groupID, Date: second, Kind: "training", SessionInstanceID: &fixture.sessionInst,
	}).Error)
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	require.NoError(t, fixture.db.Model(&dbs.WorkoutFeedback{}).
		Where("assigned_session_id = ?", fixture.sessionInst).
		Update("session_date", second).Error)

	minDate, maxDate, err = fixture.dao.ScopeDateRange(nil, []int64{fixture.groupID})
	require.NoError(t, err)
	require.NotNil(t, minDate)
	require.NotNil(t, maxDate)
	assert.Equal(t, "2026-09-20", minDate.Format("2006-01-02"))
	assert.Equal(t, "2026-09-20", maxDate.Format("2006-01-02"))
}

// scopeDateRangeFilterFixture cubre los filtros de ScopeDateRange: rol main,
// runner status permitidos, membresía por ventana y feedback sin runner_session.
// expectedMin/Max son las fechas que sí cumplen todos los filtros en groupID.
type scopeDateRangeFilterFixture struct {
	db              *gorm.DB
	dao             DashboardDaoInterface
	groupID         int64
	otherGroupID    int64
	excludedGroupID int64
	expectedMin     time.Time
	expectedMax     time.Time
	otherGroupDate  time.Time
}

func newScopeDateRangeFilterFixture(t *testing.T, db *gorm.DB, tag string) *scopeDateRangeFilterFixture {
	t.Helper()
	owner := persistUser(db, "scope-owner-"+tag+"@test.com", "5"+tag)
	team := testTeam(db, "equipo_scope_"+tag, owner.ID)
	group := testGroup(db, "grupo_scope_"+tag, team.ID)
	otherGroup := testGroup(db, "grupo_scope_other_"+tag, team.ID)
	excludedGroup := testGroup(db, "grupo_scope_excluded_"+tag, team.ID)

	active := persistUser(db, "scope-active-"+tag+"@test.com", "6"+tag+"01")
	closed := persistUser(db, "scope-closed-"+tag+"@test.com", "6"+tag+"02")
	future := persistUser(db, "scope-future-"+tag+"@test.com", "6"+tag+"03")
	orphan := persistUser(db, "scope-orphan-"+tag+"@test.com", "6"+tag+"04")
	softMembership := persistUser(db, "scope-soft-"+tag+"@test.com", "6"+tag+"05")

	day := func(d int) time.Time { return time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC) }
	membershipStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	closedEnd := day(8)
	futureStart := day(7)
	deletedAt := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	addMembership := func(groupID, userID int64, start time.Time, end, deleted *time.Time) {
		require.NoError(t, db.Create(&dbs.GroupUser{
			GroupID: groupID, UserID: userID, DateStart: start, DateEnd: end, DeletedAt: deleted,
		}).Error)
	}
	addMembership(group.ID, active.ID, membershipStart, nil, nil)
	addMembership(group.ID, closed.ID, membershipStart, &closedEnd, nil)
	addMembership(group.ID, future.ID, futureStart, nil, nil)
	addMembership(group.ID, orphan.ID, membershipStart, nil, nil)
	addMembership(group.ID, softMembership.ID, membershipStart, nil, &deletedAt)
	addMembership(otherGroup.ID, active.ID, membershipStart, nil, nil)
	addMembership(excludedGroup.ID, active.ID, membershipStart, nil, nil)

	addSession := func(groupID int64, date time.Time, name string) (siID, mainEx, warmupEx, cooldownEx int64) {
		siID = testSessionInstance(db, name)
		require.NoError(t, db.Create(&dbs.GroupCalendarDay{
			GroupID: groupID, Date: date, Kind: "training", SessionInstanceID: &siID,
		}).Error)
		mainEx = persistExerciseInstance(t, db, siID, "Distancia", string(constants.ExerciseKindRitmoContinuo),
			intPtrDAO(30), intPtrDAO(4000), nil, string(constants.SessionExerciseRoleMain))
		warmupEx = persistExerciseInstance(t, db, siID, "Sparring", string(constants.ExerciseKindLiviano),
			intPtrDAO(10), nil, nil, string(constants.SessionExerciseRoleWarmup))
		cooldownEx = persistExerciseInstance(t, db, siID, "Vuelta", string(constants.ExerciseKindLiviano),
			intPtrDAO(5), nil, nil, string(constants.SessionExerciseRoleCooldown))
		return siID, mainEx, warmupEx, cooldownEx
	}

	addFeedback := func(siID, exerciseID, athleteID int64, date time.Time, deleted bool) {
		var deletedAt *time.Time
		if deleted {
			at := date.Add(time.Hour)
			deletedAt = &at
		}
		require.NoError(t, db.Create(&dbs.WorkoutFeedback{
			TeamID: &team.ID, AssignedSessionID: siID, AssignedExerciseID: exerciseID,
			AthleteUserID: athleteID, FeedbackOwnerUserID: athleteID, ReportSource: "atleta",
			SessionDate: date, SetNumber: 1, DurationMs: intPtr64(240000),
			ActiveDurationMs: intPtr64(240000), DistanceMeters: floatPtrDAO(4000),
			CompletionStatus: strPtrDAO("completa"), MediaURLs: emptyMediaURLs(t), DeletedAt: deletedAt,
		}).Error)
	}
	addRunner := func(siID, athleteID int64, status string, date time.Time) {
		require.NoError(t, db.Create(&dbs.RunnerSession{
			SessionInstanceID: siID, AthleteUserID: athleteID, Status: status, StartDate: date,
		}).Error)
	}

	// 2026-09-01: finished + main + membresía activa -> mínimo esperado.
	si, mainEx, _, _ := addSession(group.ID, day(1), "scope-"+tag+"-01")
	addFeedback(si, mainEx, active.ID, day(1), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusFinished, day(1))

	// 2026-09-02: interrupted también es un status permitido.
	si, mainEx, _, _ = addSession(group.ID, day(2), "scope-"+tag+"-02")
	addFeedback(si, mainEx, active.ID, day(2), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusInterrupted, day(2))

	// 2026-09-03: wip no entra al rango.
	si, mainEx, _, _ = addSession(group.ID, day(3), "scope-"+tag+"-03")
	addFeedback(si, mainEx, active.ID, day(3), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusWip, day(3))

	// 2026-09-04: feedback huérfano (sin runner_session) entra por rs.status IS NULL.
	si, mainEx, _, _ = addSession(group.ID, day(4), "scope-"+tag+"-04")
	addFeedback(si, mainEx, orphan.ID, day(4), false)

	// 2026-09-05: solo roles no main, aunque el runner esté finished.
	si, _, warmupEx, cooldownEx := addSession(group.ID, day(5), "scope-"+tag+"-05")
	addFeedback(si, warmupEx, active.ID, day(5), false)
	addFeedback(si, cooldownEx, active.ID, day(5), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusFinished, day(5))

	// 2026-09-06: feedback borrado y feedback de una membresía que empieza al día siguiente.
	si, mainEx, _, _ = addSession(group.ID, day(6), "scope-"+tag+"-06")
	addFeedback(si, mainEx, active.ID, day(6), true)
	addFeedback(si, mainEx, future.ID, day(6), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusFinished, day(6))
	addRunner(si, future.ID, runnersession.RunnerSessionStatusFinished, day(6))

	// 2026-09-07: la membresía arranca el mismo día -> entra (borde inclusivo).
	si, mainEx, _, _ = addSession(group.ID, day(7), "scope-"+tag+"-07")
	addFeedback(si, mainEx, future.ID, day(7), false)
	addRunner(si, future.ID, runnersession.RunnerSessionStatusFinished, day(7))

	// 2026-09-08: la membresía termina el mismo día -> entra (borde inclusivo).
	si, mainEx, _, _ = addSession(group.ID, day(8), "scope-"+tag+"-08")
	addFeedback(si, mainEx, closed.ID, day(8), false)
	addRunner(si, closed.ID, runnersession.RunnerSessionStatusFinished, day(8))

	// 2026-09-09: membresía terminada -> no entra.
	si, mainEx, _, _ = addSession(group.ID, day(9), "scope-"+tag+"-09")
	addFeedback(si, mainEx, closed.ID, day(9), false)
	addRunner(si, closed.ID, runnersession.RunnerSessionStatusFinished, day(9))

	// 2026-09-11: group_users.deleted_at no filtra el histórico (misma regla que series).
	si, mainEx, _, _ = addSession(group.ID, day(11), "scope-"+tag+"-11")
	addFeedback(si, mainEx, softMembership.ID, day(11), false)
	addRunner(si, softMembership.ID, runnersession.RunnerSessionStatusFinished, day(11))

	// 2026-09-13: semijoin a exercise_instances conserva el feedback sin instancia.
	si, _, _, _ = addSession(group.ID, day(13), "scope-"+tag+"-13")
	const missingExerciseID int64 = 9000000001
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: si, ExerciseInstanceID: missingExerciseID, Role: string(constants.SessionExerciseRoleMain),
	}).Error)
	addFeedback(si, missingExerciseID, active.ID, day(13), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusFinished, day(13))

	// 2026-09-30: máximo esperado.
	si, mainEx, _, _ = addSession(group.ID, day(30), "scope-"+tag+"-30")
	addFeedback(si, mainEx, active.ID, day(30), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusInterrupted, day(30))

	otherDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	si, mainEx, _, _ = addSession(otherGroup.ID, otherDate, "scope-"+tag+"-other")
	addFeedback(si, mainEx, active.ID, otherDate, false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusFinished, otherDate)

	si, mainEx, _, _ = addSession(excludedGroup.ID, day(15), "scope-"+tag+"-excluded")
	addFeedback(si, mainEx, active.ID, day(15), false)
	addRunner(si, active.ID, runnersession.RunnerSessionStatusWip, day(15))

	return &scopeDateRangeFilterFixture{
		db: db, dao: NewDashboardDao(db), groupID: group.ID, otherGroupID: otherGroup.ID,
		excludedGroupID: excludedGroup.ID, expectedMin: day(1), expectedMax: day(30), otherGroupDate: otherDate,
	}
}

func intPtr64(v int64) *int64 { return &v }

func assertSameRange(t *testing.T, wantMin, wantMax, gotMin, gotMax *time.Time) {
	t.Helper()
	if wantMin == nil {
		require.Nil(t, gotMin)
	} else {
		require.NotNil(t, gotMin)
		assert.Equal(t, wantMin.Format("2006-01-02"), gotMin.Format("2006-01-02"))
	}
	if wantMax == nil {
		require.Nil(t, gotMax)
	} else {
		require.NotNil(t, gotMax)
		assert.Equal(t, wantMax.Format("2006-01-02"), gotMax.Format("2006-01-02"))
	}
}

// D4: la prueba de referencia fija el rango con todos los filtros de series,
// incluidos feedback huérfano y nulos cuando el scope no tiene filas válidas.
func TestDashboardDao_ScopeDateRangeFiltros(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newScopeDateRangeFilterFixture(t, db, "filtros")

	minDate, maxDate, err := fixture.dao.ScopeDateRange(nil, []int64{fixture.groupID})
	require.NoError(t, err)
	require.NotNil(t, minDate)
	require.NotNil(t, maxDate)
	assert.Equal(t, fixture.expectedMin.Format("2006-01-02"), minDate.Format("2006-01-02"))
	assert.Equal(t, fixture.expectedMax.Format("2006-01-02"), maxDate.Format("2006-01-02"))

	minDate, maxDate, err = fixture.dao.ScopeDateRange(nil, []int64{fixture.otherGroupID})
	require.NoError(t, err)
	require.NotNil(t, minDate)
	require.NotNil(t, maxDate)
	assert.Equal(t, fixture.otherGroupDate.Format("2006-01-02"), minDate.Format("2006-01-02"))
	assert.Equal(t, fixture.otherGroupDate.Format("2006-01-02"), maxDate.Format("2006-01-02"))

	minDate, maxDate, err = fixture.dao.ScopeDateRange(nil, []int64{fixture.excludedGroupID})
	require.NoError(t, err)
	assert.Nil(t, minDate)
	assert.Nil(t, maxDate)

	minDate, maxDate, err = fixture.dao.ScopeDateRange(nil, []int64{fixture.groupID, fixture.otherGroupID})
	require.NoError(t, err)
	require.NotNil(t, minDate)
	require.NotNil(t, maxDate)
	assert.Equal(t, fixture.otherGroupDate.Format("2006-01-02"), minDate.Format("2006-01-02"))
	assert.Equal(t, fixture.expectedMax.Format("2006-01-02"), maxDate.Format("2006-01-02"))

	minDate, maxDate, err = fixture.dao.ScopeDateRange(nil, nil)
	require.NoError(t, err)
	assert.Nil(t, minDate)
	assert.Nil(t, maxDate)
}

const legacyScopeDateRangeSQL = `
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

// D4: la reescritura con semijoins solo se adoptó con paridad exacta contra la
// consulta anterior sobre el mismo fixture.
func TestDashboardDao_ScopeDateRangeParidad(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newScopeDateRangeFilterFixture(t, db, "paridad")

	for _, groupIDs := range [][]int64{
		{fixture.groupID},
		{fixture.otherGroupID},
		{fixture.excludedGroupID},
		{fixture.groupID, fixture.otherGroupID},
		{fixture.groupID, fixture.excludedGroupID},
	} {
		gotMin, gotMax, err := fixture.dao.ScopeDateRange(nil, groupIDs)
		require.NoError(t, err)

		var legacy struct {
			MinDate *time.Time `gorm:"column:min_date"`
			MaxDate *time.Time `gorm:"column:max_date"`
		}
		require.NoError(t, db.Raw(legacyScopeDateRangeSQL, groupIDs,
			runnersession.RunnerSessionStatusFinished, runnersession.RunnerSessionStatusInterrupted).Scan(&legacy).Error)

		assertSameRange(t, legacy.MinDate, legacy.MaxDate, gotMin, gotMax)
	}
}

// 4.1b: las instancias sin source_exercise_id del scope llegan con el dueño de su
// equipo, que es lo que acota el fallback por nombre.
func TestDashboardDao_UnlinkedExerciseInstances(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "unlinked")
	orphan := &dbs.ExerciseInstance{Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtrDAO(30), DistanceM: intPtrDAO(4000)}
	require.NoError(t, db.Create(orphan).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: fixture.sessionInst, ExerciseInstanceID: orphan.ID, Role: "main",
	}).Error)

	from, to := fixture.calendarDay.AddDate(0, 0, -1), fixture.calendarDay.AddDate(0, 0, 1)
	unlinked, err := fixture.dao.UnlinkedExerciseInstances(nil, []int64{fixture.groupID}, from, to)

	require.NoError(t, err)
	ids := make([]int64, 0, len(unlinked))
	for _, instance := range unlinked {
		ids = append(ids, instance.ExerciseInstanceID)
		if instance.ExerciseInstanceID == orphan.ID {
			assert.Equal(t, fixture.ownerID, instance.OwnerID, "el dueño del equipo es el que acota el fallback")
			assert.Equal(t, "Distancia 4 km", instance.Name)
		}
	}
	assert.Contains(t, ids, orphan.ID)
	// La instancia con source_exercise_id no aparece: ya tiene su vínculo.
	assert.NotContains(t, ids, fixture.mainEx)
}

// D3: el rango de la consulta acota la SQL. Una instancia legacy fuera del rango
// no se trae, porque ya no puede aportar series al agregado.
func TestDashboardDao_UnlinkedExerciseInstancesAcotaAlRango(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "unlinkedrange")
	inRange := &dbs.ExerciseInstance{Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtrDAO(30), DistanceM: intPtrDAO(4000)}
	require.NoError(t, db.Create(inRange).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: fixture.sessionInst, ExerciseInstanceID: inRange.ID, Role: "main",
	}).Error)

	oldSession := testSessionInstance(db, "sesion-dash-unlinkedrange-old")
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: fixture.groupID, Date: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
		Kind: "training", SessionInstanceID: &oldSession,
	}).Error)
	outOfRange := &dbs.ExerciseInstance{Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtrDAO(30), DistanceM: intPtrDAO(4000)}
	require.NoError(t, db.Create(outOfRange).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: oldSession, ExerciseInstanceID: outOfRange.ID, Role: "main",
	}).Error)

	from, to := fixture.calendarDay.AddDate(0, 0, -1), fixture.calendarDay.AddDate(0, 0, 1)
	unlinked, err := fixture.dao.UnlinkedExerciseInstances(nil, []int64{fixture.groupID}, from, to)

	require.NoError(t, err)
	ids := make([]int64, 0, len(unlinked))
	for _, instance := range unlinked {
		ids = append(ids, instance.ExerciseInstanceID)
	}
	assert.Contains(t, ids, inRange.ID, "la instancia dentro del rango sigue apareciendo")
	assert.NotContains(t, ids, outOfRange.ID, "la instancia fuera del rango no se escanea")
}

func TestDashboardDao_UnlinkedExerciseInstancesSinGruposORangoVacio(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "unlinkedempty")

	noGroups, err := fixture.dao.UnlinkedExerciseInstances(nil, nil,
		fixture.calendarDay, fixture.calendarDay)
	require.NoError(t, err)
	assert.Empty(t, noGroups)

	emptyRange, err := fixture.dao.UnlinkedExerciseInstances(nil, []int64{fixture.groupID},
		time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, emptyRange)
}

func TestDashboardDao_FindSeriesPorEjercicio(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "filtroexe")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistFeedback(t, fixture.warmupEx, fixture.athleteID, 1, 600000, nil)

	filter := fixture.filter()
	filter.ExerciseIDs = []int64{fixture.catalogEx}

	rows, err := fixture.dao.FindSeries(nil, filter)

	require.NoError(t, err)
	require.Len(t, rows, 1, "el filtro por exercise_ids[] es por source_exercise_id")
	assert.Equal(t, fixture.mainEx, rows[0].ExerciseInstanceID)
}

// El fallback por nombre entra por ExerciseInstanceIDs: las instancias huérfanas no
// se descartan.
func TestDashboardDao_FindSeriesIncluyeInstanciasResueltas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "legacyid")
	orphan := &dbs.ExerciseInstance{Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtrDAO(30), DistanceM: intPtrDAO(4000)}
	require.NoError(t, db.Create(orphan).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: fixture.sessionInst, ExerciseInstanceID: orphan.ID, Role: "main",
	}).Error)
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		TeamID: &fixture.teamID, AssignedSessionID: fixture.sessionInst, AssignedExerciseID: orphan.ID,
		AthleteUserID: fixture.athleteID, FeedbackOwnerUserID: fixture.athleteID, ReportSource: "atleta",
		SessionDate: fixture.calendarDay, SetNumber: 1, DurationMs: int64PtrDAO(240000),
		ActiveDurationMs: int64PtrDAO(240000), DistanceMeters: floatPtrDAO(4000),
		MediaURLs: emptyMediaURLs(t),
	}).Error)

	filter := fixture.filter()
	filter.ExerciseIDs = []int64{fixture.catalogEx}
	filter.LegacyExerciseInstanceIDs = []int64{orphan.ID}

	rows, err := fixture.dao.FindSeries(nil, filter)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, orphan.ID, rows[0].ExerciseInstanceID)
}

// 7.2b: una instancia sin objetivo (minutes y distance_m en NULL) tiene que
// llegar a la fila con los dos objetivos en nil, para que el dominio la marque
// has_target = false en vez de inventar un cero. El catálogo del seed SIEMPRE
// tiene objetivo, así que el caso no se puede cubrir con datos sembrados: vive
// acá, con una fila de prueba.
func TestDashboardDao_FindSeries_InstanciaSinObjetivo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "sinojetivo")
	sinObjetivo := &dbs.ExerciseInstance{Name: "Ritmo sin objetivo", Kind: string(constants.ExerciseKindRitmoContinuo)}
	require.NoError(t, db.Create(sinObjetivo).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: fixture.sessionInst, ExerciseInstanceID: sinObjetivo.ID, Role: "main",
	}).Error)
	require.NoError(t, db.Create(&dbs.WorkoutFeedback{
		TeamID: &fixture.teamID, AssignedSessionID: fixture.sessionInst, AssignedExerciseID: sinObjetivo.ID,
		AthleteUserID: fixture.athleteID, FeedbackOwnerUserID: fixture.athleteID, ReportSource: "corredor",
		SessionDate: fixture.calendarDay, SetNumber: 1, DurationMs: int64PtrDAO(1500000),
		ActiveDurationMs: int64PtrDAO(1500000), DistanceMeters: floatPtrDAO(6000),
		MediaURLs: emptyMediaURLs(t),
	}).Error)

	rows, err := fixture.dao.FindSeries(nil, fixture.filter())

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].TargetMinutes, "un objetivo ausente tiene que venir nil y no 0")
	assert.Nil(t, rows[0].TargetDistanceM, "un objetivo ausente tiene que venir nil y no 0")
	assert.NotNil(t, rows[0].DistanceMeters, "lo logrado sí viene: la serie es medible aunque no tenga con qué compararse")
}

func TestDashboardDao_CatalogoPorDuenoYTipo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewDashboardDao(db)
	owner := persistUser(db, "dash-cat-owner@test.com", "61000001")
	other := persistUser(db, "dash-cat-other@test.com", "61000002")
	require.NoError(t, db.Create(&dbs.Exercise{OwnerID: owner.ID, Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: intPtrDAO(30), DistanceM: intPtrDAO(4000)}).Error)
	require.NoError(t, db.Create(&dbs.Exercise{OwnerID: owner.ID, Name: "Trote viejo", Kind: string(constants.ExerciseKindRunning)}).Error)
	require.NoError(t, db.Create(&dbs.Exercise{OwnerID: other.ID, Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo)}).Error)

	exercises, err := dao.FindCatalogExercisesByOwner(nil, owner.ID, constants.CurrentExerciseKinds())

	require.NoError(t, err)
	require.Len(t, exercises, 1, "el catálogo es por dueño y solo de los tipos vigentes")
	assert.Equal(t, "Distancia 4 km", exercises[0].Name)
	require.NotNil(t, exercises[0].Minutes)
	assert.Equal(t, 30, *exercises[0].Minutes, "el objetivo vigente del catálogo")
}

func TestDashboardDao_CatalogoPorID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewDashboardDao(db)
	owner := persistUser(db, "dash-catid-owner@test.com", "62000001")
	minutes := 30
	distance := 4000
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo), Minutes: &minutes, DistanceM: &distance}
	require.NoError(t, db.Create(exercise).Error)
	// El catálogo es por dueño: dos entrenadores pueden tener el mismo nombre.
	other := &dbs.Exercise{OwnerID: owner.ID, Name: "Distancia 21 km", Kind: string(constants.ExerciseKindRitmoContinuo)}
	require.NoError(t, db.Create(other).Error)

	found, err := dao.FindDashboardCatalogExerciseByID(nil, exercise.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, exercise.ID, found.ID)
	assert.Equal(t, owner.ID, found.OwnerID)
	assert.Equal(t, "Distancia 4 km", found.Name)
	require.NotNil(t, found.DistanceM)
	assert.Equal(t, 4000, *found.DistanceM)

	missing, err := dao.FindDashboardCatalogExerciseByID(nil, exercise.ID+999999)
	require.NoError(t, err)
	assert.Nil(t, missing, "inexistente es nil, para que el service responda 404")

	byIDs, err := dao.FindDashboardCatalogExercisesByIDs(nil, []int64{other.ID, exercise.ID})
	require.NoError(t, err)
	require.Len(t, byIDs, 2)
	assert.Equal(t, exercise.ID, byIDs[0].ID, "ordenado por id")

	empty, err := dao.FindDashboardCatalogExercisesByIDs(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// Un ejercicio soft-deleted no aparece: el catálogo vigente es el que está vivo.
func TestDashboardDao_CatalogoExcluyeSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewDashboardDao(db)
	owner := persistUser(db, "dash-del-owner@test.com", "63000001")
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "Borrado", Kind: string(constants.ExerciseKindLiviano)}
	require.NoError(t, db.Create(exercise).Error)
	require.NoError(t, db.Model(&dbs.Exercise{}).Where("id = ?", exercise.ID).
		Update("deleted_at", gorm.Expr("NOW()")).Error)

	byOwner, err := dao.FindCatalogExercisesByOwner(nil, owner.ID, constants.CurrentExerciseKinds())
	require.NoError(t, err)
	assert.Empty(t, byOwner)

	byID, err := dao.FindDashboardCatalogExerciseByID(nil, exercise.ID)
	require.NoError(t, err)
	assert.Nil(t, byID)
}

// CountOpenSessions con filtro de corredor: cuenta las wip de ese corredor, no las
// de todo el scope.
func TestDashboardDao_OpenSessionsPorCorredor(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "openpor")
	other := persistUser(db, fmt.Sprintf("dash-open-other-%d@test.com", fixture.groupID), fmt.Sprintf("9%09d", fixture.groupID%100000000))
	require.NoError(t, NewGroupUserDao(db).Create(nil, &dbs.GroupUser{
		GroupID: fixture.groupID, UserID: other.ID, DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}))
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistFeedback(t, fixture.mainEx, other.ID, 1, 240000, floatPtrDAO(4000))
	fixture.persistRunnerSession(t, fixture.athleteID, runnersession.RunnerSessionStatusWip)
	fixture.persistRunnerSession(t, other.ID, runnersession.RunnerSessionStatusWip)

	all, err := fixture.dao.CountOpenSessions(nil, fixture.filter())
	require.NoError(t, err)
	assert.Equal(t, 2, all.Sessions)

	filter := fixture.filter()
	filter.AthleteUserIDs = []int64{fixture.athleteID}
	mine, err := fixture.dao.CountOpenSessions(nil, filter)
	require.NoError(t, err)
	assert.Equal(t, 1, mine.Sessions)
}

func TestDashboardDao_ScopeAthletes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "atletas")

	athletes, err := fixture.dao.ScopeAthletes(nil, []int64{fixture.groupID})

	require.NoError(t, err)
	assert.Equal(t, []int64{fixture.athleteID}, athletes)
}

func TestDashboardDao_ScopeVacioNoConsulta(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewDashboardDao(db)

	rows, err := dao.FindSeries(nil, DashboardScopeFilter{})
	require.NoError(t, err)
	assert.Empty(t, rows, "sin scope no hay nada que consultar")

	athletes, err := dao.ScopeAthletes(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, athletes)
}

// El rango de datos acota qué series entran, y lo hace en la consulta: dentro y fuera
// de la ventana, con las dos magnitudes a la vez.
func TestDashboardDao_FindSeries_RangoDeDistanciaYTiempo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "rango")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 1200000, floatPtrDAO(3000))
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 2, 420000, floatPtrDAO(6000))
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 3, 600000, floatPtrDAO(50000))

	all, err := fixture.dao.FindSeries(nil, fixture.filter())
	require.NoError(t, err)
	require.Len(t, all, 3, "sin rango entran las tres")

	filter := fixture.filter()
	filter.DistanceFromM, filter.DistanceToM = float64PtrDAO(5000), float64PtrDAO(42000)
	filter.TimeFromMs, filter.TimeToMs = int64PtrDAO(300000), int64PtrDAO(900000)

	rows, err := fixture.dao.FindSeries(nil, filter)

	require.NoError(t, err)
	require.Len(t, rows, 1, "solo la de 6 km en 7 min cae en las dos ventanas")
	assert.Equal(t, 2, rows[0].SetNumber)
}

// El tiempo del rango se compara sobre el TIEMPO LOGRADO —COALESCE(active,
// duration)— y las unidades del contrato son minutos. Los dos errores posibles se
// comen con estas tres series: uno compara el crudo y deja afuera la que cuenta, el
// otro trata los minutos como milisegundos y deja entrar todo.
func TestDashboardDao_FindSeries_FiltroDeTiempoUsaElTiempoLogrado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "tiemofiltro")
	// 260 s sin tiempo activo: 4,33 min. Entra en "hasta 5 min" y queda afuera de
	// "desde 5 min".
	persistFeedbackWithTimes(t, fixture, 1, nil, int64PtrDAO(260000), floatPtrDAO(6000))
	// Tiempo activo de 60 s con duration_ms de una hora: el logrado es el activo, así
	// que entra en "hasta 2 min". Filtrando por duration_ms en crudo quedaría afuera.
	persistFeedbackWithTimes(t, fixture, 2, int64PtrDAO(60000), int64PtrDAO(3600000), floatPtrDAO(1000))

	atMost, err := fixture.dao.FindSeries(nil, filterWithTimeRange(fixture.filter(), nil, int64PtrDAO(300000)))
	require.NoError(t, err)
	require.Len(t, atMost, 2, "los 260 s entran en una ventana de hasta 5 min")

	atLeast, err := fixture.dao.FindSeries(nil, filterWithTimeRange(fixture.filter(), int64PtrDAO(300000), nil))
	require.NoError(t, err)
	assert.Empty(t, atLeast, "ninguna de las dos lasted 5 min: 260 s son 4,33 y el tiempo activo es de 60 s")

	// El tiempo activo manda sobre duration_ms: esta serie tiene una hora de duración
	// en crudo, así que entrarla o no dice de qué columna leyó el filtro.
	fromTwoMinutes, err := fixture.dao.FindSeries(nil, filterWithTimeRange(fixture.filter(), int64PtrDAO(120000), nil))
	require.NoError(t, err)
	require.Len(t, fromTwoMinutes, 1, "solo los 260 s alcanzan los 2 min")
	assert.Equal(t, 1, fromTwoMinutes[0].SetNumber)

	// Una serie sin ningún tiempo no se puede comparar contra una ventana de tiempo:
	// queda afuera, no entra como inválida.
	persistFeedbackWithTimes(t, fixture, 3, nil, nil, floatPtrDAO(6000))
	withNoTime, err := fixture.dao.FindSeries(nil, filterWithTimeRange(fixture.filter(), nil, int64PtrDAO(300000)))
	require.NoError(t, err)
	assert.Len(t, withNoTime, 2, "sin tiempo no hay contra qué comparar el rango")
}

// El rango se arma como WHERE: las condiciones y sus argumentos tienen que venir en el
// mismo orden, y sin rango no hay nada que agregar.
func TestDashboardDao_RangoDeSerieArmaElWhere(t *testing.T) {
	conditions, args := DashboardScopeFilter{
		DistanceFromM: float64PtrDAO(5000),
		DistanceToM:   float64PtrDAO(42000),
		TimeFromMs:    int64PtrDAO(240000),
		TimeToMs:      int64PtrDAO(5400000),
	}.rangeConditions()

	assert.Equal(t, []string{
		"wf.distance_meters >= ?",
		"wf.distance_meters <= ?",
		achievedTimeSQL + " >= ?",
		achievedTimeSQL + " <= ?",
	}, conditions)
	assert.Equal(t, []interface{}{float64(5000), float64(42000), int64(240000), int64(5400000)}, args)
	assert.Contains(t, achievedTimeSQL, "active_duration_ms", "el tiempo del rango es el logrado, no el crudo")

	empty, emptyArgs := DashboardScopeFilter{}.rangeConditions()
	assert.Empty(t, empty)
	assert.Empty(t, emptyArgs, "sin rango no se agrega nada a la consulta")
}

// CountOpenSessions cuenta sesiones, no series: el rango de datos no lo toca.
func TestDashboardDao_OpenSessionsIgnoraElRangoDeSerie(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "openrango")
	fixture.persistFeedback(t, fixture.mainEx, fixture.athleteID, 1, 240000, floatPtrDAO(4000))
	fixture.persistRunnerSession(t, fixture.athleteID, runnersession.RunnerSessionStatusWip)

	filter := fixture.filter()
	filter.DistanceFromM = float64PtrDAO(9000000)

	open, err := fixture.dao.CountOpenSessions(nil, filter)

	require.NoError(t, err)
	assert.Equal(t, 1, open.Sessions)
}

// Prohibido SELECT *: el SQL de la query core nombra columnas una por una.
func TestDashboardDao_QueryCoreNoUsaSelectAsterisk(t *testing.T) {
	for name, query := range map[string]string{
		"series":    seriesFromClause,
		"open":      countOpenSessionsSQL,
		"dateRange": scopeDateRangeSQL,
		"unlinked":  unlinkedExerciseInstancesSQL,
	} {
		assert.False(t, strings.Contains(query, "SELECT *"), "%s no puede usar SELECT *", name)
	}
	// Y el filtro por rol main está en la query, no en un paso posterior.
	assert.Contains(t, seriesFromClause, "sei.role = 'main'")
	assert.Contains(t, countOpenSessionsSQL, "sei.role = 'main'")
	assert.Contains(t, scopeDateRangeSQL, "sei.role = 'main'")
	assert.Contains(t, seriesFromClause, "rs.status IS NULL", "el LEFT JOIN tiene que admitir feedback sin runner_session")
}

func int64PtrDAO(v int64) *int64 { return &v }
func float64PtrDAO(v float64) *float64 {
	return &v
}

// persistFeedbackWithTimes crea una serie con tiempo activo y duration_ms por
// separado, o con cualquiera de los dos en null, para poder probar el tiempo LOGRADO.
func persistFeedbackWithTimes(t *testing.T, f *dashboardFixture, setNumber int, active, duration *int64, distanceM *float64) {
	t.Helper()
	require.NoError(t, f.db.Create(&dbs.WorkoutFeedback{
		TeamID:              &f.teamID,
		AssignedSessionID:   f.sessionInst,
		AssignedExerciseID:  f.mainEx,
		AthleteUserID:       f.athleteID,
		FeedbackOwnerUserID: f.athleteID,
		ReportSource:        "atleta",
		SessionDate:         f.calendarDay,
		SetNumber:           setNumber,
		DurationMs:          duration,
		ActiveDurationMs:    active,
		DistanceMeters:      distanceM,
		CompletionStatus:    strPtrDAO("completa"),
		MediaURLs:           emptyMediaURLs(t),
	}).Error)
}

func filterWithTimeRange(filter DashboardScopeFilter, from, to *int64) DashboardScopeFilter {
	filter.TimeFromMs = from
	filter.TimeToMs = to
	return filter
}

// emptyMediaURLs inicializa el TextArray que la migración deja NOT NULL: sin esto
// el insert falla con "cannot encode status undefined" y el error no dice nada de
// que le falta.
func emptyMediaURLs(t *testing.T) pgtype.TextArray {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{}))
	return media
}

// El rango de UnlinkedExerciseInstances es inclusivo en ambos extremos: un día
// en el borde entra, el día siguiente al to queda afuera.
func TestDashboardDao_UnlinkedExerciseInstancesRangoInclusivo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixture := newDashboardFixture(t, db, "unlinkedbounds")
	orphan := &dbs.ExerciseInstance{Name: "Distancia 4 km", Kind: string(constants.ExerciseKindRitmoContinuo)}
	require.NoError(t, db.Create(orphan).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: fixture.sessionInst, ExerciseInstanceID: orphan.ID, Role: "main",
	}).Error)

	day := fixture.calendarDay
	fromDay := func(d time.Time) []int64 {
		t.Helper()
		rows, err := fixture.dao.UnlinkedExerciseInstances(nil, []int64{fixture.groupID}, d, d)
		require.NoError(t, err)
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ExerciseInstanceID)
		}
		return ids
	}

	assert.Contains(t, fromDay(day), orphan.ID, "from=to=el día del calendario incluye")
	assert.NotContains(t, fromDay(day.AddDate(0, 0, 1)), orphan.ID, "el día posterior al to queda afuera")
	assert.NotContains(t, fromDay(day.AddDate(0, 0, -1)), orphan.ID, "el día anterior al from queda afuera")
}

func TestDashboardDao_UnlinkedExerciseInstances_DBError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewDashboardDao(failing)

	_, err := dao.UnlinkedExerciseInstances(nil, []int64{1}, time.Now(), time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding unlinked exercise instances")
}

// Todas las lecturas de scope del dashboard envuelven el error de DB.
func TestDashboardDao_LecturasDeScope_DBError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	filter := DashboardScopeFilter{GroupIDs: []int64{1}, From: time.Now(), To: time.Now()}

	cases := []struct {
		name string
		run  func(DashboardDaoInterface) error
		want string
	}{
		{"CountOpenSessions", func(d DashboardDaoInterface) error {
			_, err := d.CountOpenSessions(nil, filter)
			return err
		}, "error counting dashboard open sessions"},
		{"ScopeDateRange", func(d DashboardDaoInterface) error {
			_, _, err := d.ScopeDateRange(nil, filter.GroupIDs)
			return err
		}, "error finding dashboard date range"},
		{"ScopeAthletes", func(d DashboardDaoInterface) error {
			_, err := d.ScopeAthletes(nil, filter.GroupIDs)
			return err
		}, "error finding dashboard athletes"},
		{"FindCatalogExercisesByOwner", func(d DashboardDaoInterface) error {
			_, err := d.FindCatalogExercisesByOwner(nil, 1, []string{"ritmo_continuo"})
			return err
		}, "error finding dashboard catalog exercises"},
		{"FindDashboardCatalogExerciseByID", func(d DashboardDaoInterface) error {
			_, err := d.FindDashboardCatalogExerciseByID(nil, 1)
			return err
		}, "error finding dashboard catalog exercise"},
		{"FindDashboardCatalogExercisesByIDs", func(d DashboardDaoInterface) error {
			_, err := d.FindDashboardCatalogExercisesByIDs(nil, []int64{1})
			return err
		}, "error finding dashboard catalog exercises"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			failing := testutils.FailingDB(t, db, nthFail("select", 1))
			err := tc.run(NewDashboardDao(failing))

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
