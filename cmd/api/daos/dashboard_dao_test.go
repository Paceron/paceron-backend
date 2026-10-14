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

	rows, err := fixture.dao.FindSeries(nil, fixture.filter())

	require.NoError(t, err)
	require.Len(t, rows, 1, "el calentamiento no puede entrar al dashboard")
	assert.Equal(t, fixture.mainEx, rows[0].ExerciseInstanceID)
	assert.Equal(t, string(constants.ExerciseKindRitmoContinuo), rows[0].Kind)
	require.NotNil(t, rows[0].TargetMinutes)
	assert.Equal(t, 30, *rows[0].TargetMinutes, "el objetivo sale de la ExerciseInstance, no del catálogo")
	assert.True(t, rows[0].HasRunnerSession)
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

	rows, err := fixture.dao.FindSeries(nil, fixture.filter())
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

	rows, err := fixture.dao.FindSeries(nil, fixture.filter())

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

	unlinked, err := fixture.dao.UnlinkedExerciseInstances(nil, []int64{fixture.groupID})

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

// emptyMediaURLs inicializa el TextArray que la migración deja NOT NULL: sin esto
// el insert falla con "cannot encode status undefined" y el error no dice nada de
// que le falta.
func emptyMediaURLs(t *testing.T) pgtype.TextArray {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{}))
	return media
}
