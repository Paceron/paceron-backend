package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// mockComparisonDao devuelve lo configurado y registra los filtros recibidos.
type mockComparisonDao struct {
	plannedDays      []daos.ComparisonSessionDay
	feedbackSessions []daos.ComparisonSessionDay
	sessionDay       *daos.ComparisonSessionDay
	exercises        []daos.ComparisonPlannedExercise
	feedback         []dbs.WorkoutFeedback
	runnerSessions   []dbs.RunnerSession
	isOwner          bool
	isMember         bool

	errOn string // nombre del método que devuelve error

	lastFilter          daos.ComparisonRangeFilter
	feedbackSessionsHit bool
	feedbackAthlete     int64
}

var errComparisonDB = errors.New("db caída")

func (m *mockComparisonDao) fail(method string) error {
	if m.errOn == method {
		return errComparisonDB
	}
	return nil
}

func (m *mockComparisonDao) ListPlannedDays(_ *gin.Context, filter daos.ComparisonRangeFilter) ([]daos.ComparisonSessionDay, error) {
	m.lastFilter = filter
	return m.plannedDays, m.fail("ListPlannedDays")
}

func (m *mockComparisonDao) ListFeedbackSessions(_ *gin.Context, _ daos.ComparisonRangeFilter) ([]daos.ComparisonSessionDay, error) {
	m.feedbackSessionsHit = true
	return m.feedbackSessions, m.fail("ListFeedbackSessions")
}

func (m *mockComparisonDao) FindSessionDay(_ *gin.Context, _ int64) (*daos.ComparisonSessionDay, error) {
	return m.sessionDay, m.fail("FindSessionDay")
}

func (m *mockComparisonDao) ListPlannedExercises(_ *gin.Context, ids []int64) ([]daos.ComparisonPlannedExercise, error) {
	return m.exercises, m.fail("ListPlannedExercises")
}

func (m *mockComparisonDao) ListFeedback(_ *gin.Context, athleteID int64, _ []int64) ([]dbs.WorkoutFeedback, error) {
	m.feedbackAthlete = athleteID
	return m.feedback, m.fail("ListFeedback")
}

func (m *mockComparisonDao) ListRunnerSessions(_ *gin.Context, _ int64, _ []int64) ([]dbs.RunnerSession, error) {
	return m.runnerSessions, m.fail("ListRunnerSessions")
}

func (m *mockComparisonDao) IsSessionTeamOwner(_ *gin.Context, _, _ int64) (bool, error) {
	return m.isOwner, m.fail("IsSessionTeamOwner")
}

func (m *mockComparisonDao) IsTeamMemberOfOwner(_ *gin.Context, _, _, _ int64) (bool, error) {
	return m.isMember, m.fail("IsTeamMemberOfOwner")
}

// accessDao arma un SessionInstanceDao cuyo HasInstanceAccess responde por usuario.
func accessDao(allowed map[int64]bool, err error) *mockSessionInstanceDao {
	return &mockSessionInstanceDao{hasInstanceAccessFn: func(_ *gin.Context, _ int64, callerID int64) (bool, error) {
		return allowed[callerID], err
	}}
}

func comparisonServiceFor(dao *mockComparisonDao, instanceDao *mockSessionInstanceDao) *activityComparisonService {
	svc := NewActivityComparisonService(dao, instanceDao).(*activityComparisonService)
	svc.now = func() time.Time { return time.Date(2026, 10, 13, 15, 0, 0, 0, time.UTC) }
	return svc
}

func seededDay(sessionID int64, day int, withGroup bool) daos.ComparisonSessionDay {
	teamID, groupID := int64(16), int64(16)
	teamName, groupName := "Los cracks", "Los cracks - group"
	d := daos.ComparisonSessionDay{
		SessionInstanceID: sessionID,
		Date:              time.Date(2026, 10, day, 0, 0, 0, 0, time.UTC),
		SessionName:       "caminar",
		TeamID:            &teamID,
		TeamName:          &teamName,
	}
	if withGroup {
		d.GroupID = &groupID
		d.GroupName = &groupName
	}
	return d
}

func seededExercises(sessionID int64) []daos.ComparisonPlannedExercise {
	ten := 10
	out := []daos.ComparisonPlannedExercise{}
	for i, role := range []string{"warmup", "main", "cooldown"} {
		out = append(out, daos.ComparisonPlannedExercise{
			LinkID: int64(i + 1), SessionInstanceID: sessionID, ExerciseInstanceID: int64(100 + i),
			Role: role, RepeatCount: 1, Name: "caminar 10min", Kind: "liviano", Minutes: &ten,
		})
	}
	return out
}

func seededFeedback(sessionID int64) []dbs.WorkoutFeedback {
	completed, skipped := "completed", "skipped"
	full, half := int64(602340), int64(301120)
	return []dbs.WorkoutFeedback{
		{ID: 181, AssignedSessionID: sessionID, AssignedExerciseID: 100, AthleteUserID: 19, FeedbackOwnerUserID: 19, SetNumber: 1, CompletionStatus: &completed, DurationMs: &full, SessionDate: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
		{ID: 182, AssignedSessionID: sessionID, AssignedExerciseID: 101, AthleteUserID: 19, FeedbackOwnerUserID: 19, SetNumber: 1, CompletionStatus: &completed, DurationMs: &half, SessionDate: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
		{ID: 183, AssignedSessionID: sessionID, AssignedExerciseID: 102, AthleteUserID: 19, FeedbackOwnerUserID: 19, SetNumber: 1, CompletionStatus: &skipped, SessionDate: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
	}
}

// --- GetSessionComparison ---

func TestActivityComparisonService_GetSessionComparison_OwnSession(t *testing.T) {
	day := seededDay(455, 12, true)
	end := time.Date(2026, 10, 12, 13, 16, 40, 0, time.UTC)
	dao := &mockComparisonDao{
		sessionDay: &day, exercises: seededExercises(455), feedback: seededFeedback(455),
		runnerSessions: []dbs.RunnerSession{{SessionInstanceID: 455, AthleteUserID: 19, Status: "finished", StartDate: time.Date(2026, 10, 12, 13, 0, 5, 0, time.UTC), EndDate: &end}},
	}
	svc := comparisonServiceFor(dao, accessDao(map[int64]bool{19: true}, nil))

	resp, err := svc.GetSessionComparison(nil, 19, 455, nil)

	require.NoError(t, err)
	// La segunda caminata duró 5:01 (301120 ms): 50,2% y la sesión 50,1%.
	assert.Equal(t, 50.1, resp.CompliancePct)
	assert.Equal(t, int64(19), resp.AthleteUserID)
	assert.Equal(t, "2026-10-12", resp.Date)
	assert.Equal(t, "finished", resp.Status)
	assert.Equal(t, "2026-10-12T13:00:05Z", *resp.StartedAt)
	assert.Equal(t, "2026-10-12T13:16:40Z", *resp.EndedAt)
	assert.Equal(t, []string{"liviano"}, resp.Kinds)
	require.Len(t, resp.Exercises, 3)
	assert.Equal(t, 50.2, resp.Exercises[1].CompliancePct)
	assert.Equal(t, int64(19), dao.feedbackAthlete)
}

func TestActivityComparisonService_GetSessionComparison_OrphanUsesFeedbackDateAndPresencialSchedule(t *testing.T) {
	day := seededDay(500, 1, false)
	from := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	day.PresencialTimeFrom = &from
	dao := &mockComparisonDao{sessionDay: &day, exercises: seededExercises(500), feedback: seededFeedback(500)}
	svc := comparisonServiceFor(dao, accessDao(map[int64]bool{19: true}, nil))

	resp, err := svc.GetSessionComparison(nil, 19, 500, nil)

	require.NoError(t, err)
	assert.Equal(t, "2026-10-12", resp.Date, "sin día toma la fecha del feedback")
	assert.Equal(t, comparison.SessionStatusNone, resp.Status)
	assert.Equal(t, "2026-10-01T10:00:00Z", *resp.StartedAt)
	assert.Nil(t, resp.EndedAt)
}

func TestActivityComparisonService_GetSessionComparison_TrainerViewsAthlete(t *testing.T) {
	day := seededDay(455, 12, true)
	athlete := int64(19)
	dao := &mockComparisonDao{sessionDay: &day, exercises: seededExercises(455), feedback: seededFeedback(455), isOwner: true}
	svc := comparisonServiceFor(dao, accessDao(map[int64]bool{19: true}, nil))

	resp, err := svc.GetSessionComparison(nil, 7, 455, &athlete)

	require.NoError(t, err)
	assert.Equal(t, int64(19), resp.AthleteUserID)
	assert.Equal(t, int64(19), dao.feedbackAthlete)
}

func TestActivityComparisonService_GetSessionComparison_Errors(t *testing.T) {
	day := seededDay(455, 12, true)
	athlete := int64(19)

	tests := []struct {
		name     string
		dao      *mockComparisonDao
		access   *mockSessionInstanceDao
		caller   int64
		athlete  *int64
		wantErr  error
		wantDBEr bool
	}{
		{name: "instancia inexistente", dao: &mockComparisonDao{}, access: accessDao(nil, nil), caller: 19, wantErr: ErrComparisonNotFound},
		{name: "sesión ajena", dao: &mockComparisonDao{sessionDay: &day}, access: accessDao(map[int64]bool{}, nil), caller: 19, wantErr: ErrComparisonForbidden},
		{name: "no es dueño del equipo", dao: &mockComparisonDao{sessionDay: &day}, access: accessDao(map[int64]bool{19: true}, nil), caller: 7, athlete: &athlete, wantErr: ErrComparisonForbidden},
		{name: "el atleta no tiene acceso", dao: &mockComparisonDao{sessionDay: &day, isOwner: true}, access: accessDao(map[int64]bool{}, nil), caller: 7, athlete: &athlete, wantErr: ErrComparisonForbidden},
		{name: "falla FindSessionDay", dao: &mockComparisonDao{errOn: "FindSessionDay"}, access: accessDao(nil, nil), caller: 19, wantDBEr: true},
		{name: "falla el acceso propio", dao: &mockComparisonDao{sessionDay: &day}, access: accessDao(nil, errComparisonDB), caller: 19, wantDBEr: true},
		{name: "falla IsSessionTeamOwner", dao: &mockComparisonDao{sessionDay: &day, errOn: "IsSessionTeamOwner"}, access: accessDao(nil, nil), caller: 7, athlete: &athlete, wantDBEr: true},
		{name: "falla el acceso del atleta", dao: &mockComparisonDao{sessionDay: &day, isOwner: true}, access: accessDao(nil, errComparisonDB), caller: 7, athlete: &athlete, wantDBEr: true},
		{name: "falla ListPlannedExercises", dao: &mockComparisonDao{sessionDay: &day, errOn: "ListPlannedExercises"}, access: accessDao(map[int64]bool{19: true}, nil), caller: 19, wantDBEr: true},
		{name: "falla ListFeedback", dao: &mockComparisonDao{sessionDay: &day, errOn: "ListFeedback"}, access: accessDao(map[int64]bool{19: true}, nil), caller: 19, wantDBEr: true},
		{name: "falla ListRunnerSessions", dao: &mockComparisonDao{sessionDay: &day, errOn: "ListRunnerSessions"}, access: accessDao(map[int64]bool{19: true}, nil), caller: 19, wantDBEr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := comparisonServiceFor(tt.dao, tt.access)
			_, err := svc.GetSessionComparison(nil, tt.caller, 455, tt.athlete)
			if tt.wantDBEr {
				assert.ErrorIs(t, err, errComparisonDB)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

// --- ListComparisons ---

func TestActivityComparisonService_ListComparisons_MergesPlannedAndFeedbackSessions(t *testing.T) {
	plannedNotDone := seededDay(454, 13, true)
	seeded := seededDay(455, 12, true)
	orphan := seededDay(500, 12, false) // misma fecha que la sembrada: desempata por id
	exercises := append(seededExercises(455), seededExercises(454)...)
	dao := &mockComparisonDao{
		plannedDays:      []daos.ComparisonSessionDay{seeded, plannedNotDone, seeded},
		feedbackSessions: []daos.ComparisonSessionDay{seeded, orphan},
		exercises:        exercises,
		feedback:         seededFeedback(455),
		runnerSessions:   []dbs.RunnerSession{{SessionInstanceID: 455, Status: "finished"}},
	}
	svc := comparisonServiceFor(dao, accessDao(nil, nil))

	resp, err := svc.ListComparisons(nil, 19, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13"})

	require.NoError(t, err)
	assert.Equal(t, "2026-10-01", resp.DateFrom)
	assert.Equal(t, "2026-10-13", resp.DateTo)
	require.Len(t, resp.Sessions, 3)
	// Ordenadas por fecha y, a igual fecha, por id: 455 y 500 del 12, 454 del 13.
	assert.Equal(t, int64(455), resp.Sessions[0].SessionInstanceID)
	assert.Equal(t, 50.1, resp.Sessions[0].CompliancePct)
	assert.Equal(t, "finished", resp.Sessions[0].Status)
	assert.Equal(t, int64(500), resp.Sessions[1].SessionInstanceID)
	assert.Equal(t, int64(454), resp.Sessions[2].SessionInstanceID)
	assert.Equal(t, 0.0, resp.Sessions[2].CompliancePct)
	assert.Equal(t, comparison.SessionStatusNone, resp.Sessions[2].Status)
	assert.Equal(t, 3, resp.Summary.SessionsPlanned)
	assert.Equal(t, 1, resp.Summary.SessionsWithActivity)
	// La 454 es de hoy (13/10) y no se ejecutó: se lista, pero no entra al promedio.
	assert.True(t, resp.Sessions[0].CountsForAverage)
	assert.True(t, resp.Sessions[1].CountsForAverage)
	assert.False(t, resp.Sessions[2].CountsForAverage)
	assert.Equal(t, 2, resp.Summary.SessionsAveraged)
	assert.Equal(t, 25.1, resp.Summary.AvgCompliancePct) // (50,1 + 0) / 2
	assert.Equal(t, int64(19), dao.lastFilter.AthleteUserID)
}

// Hoy cuenta solo si ya terminó; los días anteriores cuentan siempre, aunque
// la sesión siga en curso o no se haya ejecutado.
func TestActivityComparisonService_ListComparisons_TodayCountsOnlyWhenFinished(t *testing.T) {
	pastWip := seededDay(453, 12, true)
	todayFinished := seededDay(455, 13, true)
	todayWip := seededDay(456, 13, true)
	dao := &mockComparisonDao{
		plannedDays:    []daos.ComparisonSessionDay{pastWip, todayFinished, todayWip},
		exercises:      append(seededExercises(455), seededExercises(456)...),
		feedback:       seededFeedback(455),
		runnerSessions: []dbs.RunnerSession{{SessionInstanceID: 453, Status: "wip"}, {SessionInstanceID: 455, Status: "finished"}, {SessionInstanceID: 456, Status: "wip"}},
	}
	svc := comparisonServiceFor(dao, accessDao(nil, nil))

	resp, err := svc.ListComparisons(nil, 19, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13"})

	require.NoError(t, err)
	require.Len(t, resp.Sessions, 3)
	assert.True(t, resp.Sessions[0].CountsForAverage)  // 453: ayer, en curso
	assert.True(t, resp.Sessions[1].CountsForAverage)  // 455: hoy, terminada
	assert.False(t, resp.Sessions[2].CountsForAverage) // 456: hoy, en curso
	assert.Equal(t, 3, resp.Summary.SessionsPlanned)
	assert.Equal(t, 2, resp.Summary.SessionsAveraged)
	assert.Equal(t, 25.1, resp.Summary.AvgCompliancePct) // (0 + 50,1) / 2
}

func TestActivityComparisonService_ListComparisons_GroupFilterSkipsFeedbackSessions(t *testing.T) {
	teamID, groupID := int64(16), int64(16)
	dao := &mockComparisonDao{plannedDays: []daos.ComparisonSessionDay{seededDay(455, 12, true)}}
	svc := comparisonServiceFor(dao, accessDao(nil, nil))

	resp, err := svc.ListComparisons(nil, 19, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", TeamID: &teamID, GroupID: &groupID})

	require.NoError(t, err)
	assert.Len(t, resp.Sessions, 1)
	assert.False(t, dao.feedbackSessionsHit)
	assert.Equal(t, groupID, *dao.lastFilter.GroupID)
}

func TestActivityComparisonService_ListComparisons_KindFilterAndEmpty(t *testing.T) {
	dao := &mockComparisonDao{plannedDays: []daos.ComparisonSessionDay{seededDay(455, 12, true)}, exercises: seededExercises(455)}
	svc := comparisonServiceFor(dao, accessDao(nil, nil))

	matching, err := svc.ListComparisons(nil, 19, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", Kind: "liviano"})
	require.NoError(t, err)
	assert.Len(t, matching.Sessions, 1)

	other, err := svc.ListComparisons(nil, 19, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", Kind: "elongacion"})
	require.NoError(t, err)
	assert.Empty(t, other.Sessions)
	assert.Equal(t, 0, other.Summary.SessionsPlanned)
	assert.Equal(t, 0.0, other.Summary.AvgCompliancePct)
}

func TestActivityComparisonService_ListComparisons_TrainerForAthlete(t *testing.T) {
	teamID, athlete := int64(16), int64(19)
	dao := &mockComparisonDao{isMember: true}
	svc := comparisonServiceFor(dao, accessDao(nil, nil))

	_, err := svc.ListComparisons(nil, 7, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", TeamID: &teamID, AthleteUserID: &athlete})

	require.NoError(t, err)
	assert.Equal(t, int64(19), dao.lastFilter.AthleteUserID)

	self := int64(7)
	_, err = svc.ListComparisons(nil, 7, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", AthleteUserID: &self})
	require.NoError(t, err, "pedirse a sí mismo no exige team_id")
}

func TestActivityComparisonService_ListComparisons_Validation(t *testing.T) {
	teamID, athlete := int64(16), int64(19)
	tests := []struct {
		name    string
		query   comparison.ListQuery
		dao     *mockComparisonDao
		wantErr error
	}{
		{name: "sin fechas", query: comparison.ListQuery{}, wantErr: ErrComparisonInvalidQuery},
		{name: "date_from inválida", query: comparison.ListQuery{DateFrom: "01/10/2026", DateTo: "2026-10-13"}, wantErr: ErrComparisonInvalidQuery},
		{name: "date_to inválida", query: comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "13-10-2026"}, wantErr: ErrComparisonInvalidQuery},
		{name: "rango invertido", query: comparison.ListQuery{DateFrom: "2026-10-13", DateTo: "2026-10-01"}, wantErr: ErrComparisonInvalidQuery},
		{name: "rango de más de 366 días", query: comparison.ListQuery{DateFrom: "2025-01-01", DateTo: "2026-10-13"}, wantErr: ErrComparisonInvalidQuery},
		{name: "termina en el futuro", query: comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-14"}, wantErr: ErrComparisonInvalidQuery},
		{name: "kind desconocido", query: comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", Kind: "volar"}, wantErr: ErrComparisonInvalidQuery},
		{name: "group sin team", query: comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", GroupID: &teamID}, wantErr: ErrComparisonInvalidQuery},
		{name: "otro atleta sin team", query: comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", AthleteUserID: &athlete}, wantErr: ErrComparisonInvalidQuery},
		{name: "otro atleta que no es del equipo", query: comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", TeamID: &teamID, AthleteUserID: &athlete}, dao: &mockComparisonDao{}, wantErr: ErrComparisonForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dao := tt.dao
			if dao == nil {
				dao = &mockComparisonDao{}
			}
			_, err := comparisonServiceFor(dao, accessDao(nil, nil)).ListComparisons(nil, 7, tt.query)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestActivityComparisonService_ListComparisons_DBErrors(t *testing.T) {
	teamID, athlete := int64(16), int64(19)
	for _, method := range []string{"IsTeamMemberOfOwner", "ListPlannedDays", "ListFeedbackSessions", "ListPlannedExercises", "ListFeedback", "ListRunnerSessions"} {
		t.Run(method, func(t *testing.T) {
			dao := &mockComparisonDao{errOn: method, isMember: true}
			_, err := comparisonServiceFor(dao, accessDao(nil, nil)).ListComparisons(nil, 7, comparison.ListQuery{DateFrom: "2026-10-01", DateTo: "2026-10-13", TeamID: &teamID, AthleteUserID: &athlete})
			assert.ErrorIs(t, err, errComparisonDB)
		})
	}
}

func TestActivityComparisonService_ConstructorUsesRealClock(t *testing.T) {
	svc := NewActivityComparisonService(&mockComparisonDao{}, accessDao(nil, nil)).(*activityComparisonService)
	assert.WithinDuration(t, time.Now(), svc.now(), time.Minute)
}
