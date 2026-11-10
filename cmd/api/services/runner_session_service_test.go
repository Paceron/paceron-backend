package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// mockRunnerSessionDao implementa RunnerSessionDAOInterface delegando a fns
// opcionales; sin fn devuelve el default.
type mockRunnerSessionDao struct {
	sessionInstanceExistsFn   func(ctx *gin.Context, sessionInstanceID int64) (bool, error)
	createFn                  func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error)
	getBySessionAthleteFn     func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error)
	updateStatusFn            func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error
	teamExistsFn              func(ctx *gin.Context, teamID int64) (bool, error)
	existsUserInTeamOwnedByFn func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
}

func (m *mockRunnerSessionDao) SessionInstanceExists(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
	if m.sessionInstanceExistsFn != nil {
		return m.sessionInstanceExistsFn(ctx, sessionInstanceID)
	}
	return true, nil
}

func (m *mockRunnerSessionDao) Create(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
	if m.createFn != nil {
		return m.createFn(ctx, rs)
	}
	return false, nil
}

func (m *mockRunnerSessionDao) GetBySessionAndAthlete(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
	if m.getBySessionAthleteFn != nil {
		return m.getBySessionAthleteFn(ctx, sessionInstanceID, athleteUserID)
	}
	return nil, nil
}

func (m *mockRunnerSessionDao) UpdateStatus(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, runnerSessionID, to, fromStatuses, endDate)
	}
	return nil
}

func (m *mockRunnerSessionDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	if m.teamExistsFn != nil {
		return m.teamExistsFn(ctx, teamID)
	}
	return false, nil
}

func (m *mockRunnerSessionDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	if m.existsUserInTeamOwnedByFn != nil {
		return m.existsUserInTeamOwnedByFn(ctx, targetUserID, ownerUserID)
	}
	return false, nil
}

// allowInstanceAccess es el DAO de instancias por defecto de los tests que no
// ejercitan el guard: todo caller tiene acceso.
func allowInstanceAccess() *mockSessionInstanceDao {
	return &mockSessionInstanceDao{
		hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
			return true, nil
		},
	}
}

var sessionDate = time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
var sessionInstanceDate = time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
var sessionDateRight = time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)

func fixtureRunnerSession() *dbs.RunnerSession {
	return &dbs.RunnerSession{
		Status:    "wip",
		StartDate: sessionDate,
	}
}

func TestRunnerSessionService_Create_Self_Created(t *testing.T) {
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			return true, nil
		},
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			assert.Equal(t, "wip", rs.Status)
			return true, nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, _, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
		StartDate: sessionDate,
	})

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "wip", rs.Status)
}

func TestRunnerSessionService_Create_SessionMissing_NotFound(t *testing.T) {
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			return false, nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
		StartDate: sessionDate,
	})
	require.ErrorIs(t, err, ErrSessionInstanceNotFound)
}

func TestRunnerSessionService_Create_AlreadyExists_Idempotent(t *testing.T) {
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			return true, nil
		},
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			return false, nil // ON CONFLICT DO NOTHING: ya existía
		},
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			return fixtureRunnerSession(), nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, _, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
		StartDate: sessionDate,
	})

	require.NoError(t, err)
	assert.False(t, created, "idempotente: se devolvió la fila existente")
	assert.Equal(t, "wip", rs.Status)
}

func TestRunnerSessionService_Create_TrainerForAthleteInOwnedTeam(t *testing.T) {
	athlete := int64(30)
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			return true, nil
		},
		existsUserInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
			assert.Equal(t, int64(30), targetUserID)
			assert.Equal(t, int64(7), ownerUserID)
			return true, nil
		},
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			assert.Equal(t, int64(30), rs.AthleteUserID)
			return true, nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, _, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
		AthleteUserID: &athlete,
		StartDate:     sessionDate,
	})

	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, int64(30), rs.AthleteUserID)
}

func TestRunnerSessionService_Create_TrainerForOutsider_Forbidden(t *testing.T) {
	athlete := int64(99)
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			return true, nil
		},
		existsUserInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
			return false, nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
		AthleteUserID: &athlete,
		StartDate:     sessionDate,
	})

	require.ErrorIs(t, err, ErrRunnerSessionForbidden)
}

func TestRunnerSessionService_Create_InvalidStartDate(t *testing.T) {
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			return true, nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{})

	require.ErrorIs(t, err, ErrRunnerSessionInvalid)
}

func TestRunnerSessionService_Finish_WipToFinished(t *testing.T) {
	var gotID int64
	var gotTo string
	var gotFrom []string
	var gotEnd time.Time
	firstGet := true
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			if firstGet {
				firstGet = false
				rs := fixtureRunnerSession()
				rs.ID = 42
				return rs, nil
			}
			rs := fixtureRunnerSession()
			rs.ID = 42
			rs.Status = "finished"
			end := gotEnd
			rs.EndDate = &end
			return rs, nil
		},
		updateStatusFn: func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
			gotID = runnerSessionID
			gotTo = to
			gotFrom = fromStatuses
			gotEnd = endDate
			return nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "finished"})

	require.NoError(t, err)
	assert.Equal(t, int64(42), gotID)
	assert.Equal(t, "finished", gotTo)
	assert.Equal(t, []string{"wip", "interrupted"}, gotFrom)
	assert.WithinDuration(t, time.Now(), gotEnd, 2*time.Second)
	assert.Equal(t, "finished", rs.Status)
	require.NotNil(t, rs.EndDate)
	assert.WithinDuration(t, time.Now(), *rs.EndDate, 2*time.Second)
}

func TestRunnerSessionService_Finish_WipToInterrupted(t *testing.T) {
	var gotID int64
	var gotTo string
	var gotFrom []string
	var gotEnd time.Time
	firstGet := true
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			if firstGet {
				firstGet = false
				rs := fixtureRunnerSession()
				rs.ID = 42
				return rs, nil
			}
			rs := fixtureRunnerSession()
			rs.ID = 42
			rs.Status = "interrupted"
			end := gotEnd
			rs.EndDate = &end
			return rs, nil
		},
		updateStatusFn: func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
			gotID = runnerSessionID
			gotTo = to
			gotFrom = fromStatuses
			gotEnd = endDate
			return nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "interrupted"})

	require.NoError(t, err)
	assert.Equal(t, int64(42), gotID)
	assert.Equal(t, "interrupted", gotTo)
	assert.Equal(t, []string{"wip", "interrupted"}, gotFrom)
	assert.WithinDuration(t, time.Now(), gotEnd, 2*time.Second)
	assert.Equal(t, "interrupted", rs.Status)
	require.NotNil(t, rs.EndDate)
	assert.WithinDuration(t, time.Now(), *rs.EndDate, 2*time.Second)
}

func TestRunnerSessionService_Finish_InterruptedToFinished_ResetsEndDate(t *testing.T) {
	oldEnd := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	var gotEnd time.Time
	firstGet := true
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSession()
			rs.ID = 42
			if firstGet {
				firstGet = false
				rs.Status = "interrupted"
				rs.EndDate = &oldEnd
				return rs, nil
			}
			rs.Status = "finished"
			end := gotEnd
			rs.EndDate = &end
			return rs, nil
		},
		updateStatusFn: func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
			assert.Equal(t, int64(42), runnerSessionID)
			assert.Equal(t, "finished", to)
			assert.Equal(t, []string{"wip", "interrupted"}, fromStatuses)
			gotEnd = endDate
			return nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "finished"})

	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), gotEnd, 2*time.Second)
	assert.Equal(t, "finished", rs.Status)
	require.NotNil(t, rs.EndDate)
	assert.WithinDuration(t, time.Now(), *rs.EndDate, 2*time.Second)
}

func TestRunnerSessionService_Finish_AlreadyInterrupted_Idempotent(t *testing.T) {
	oldEnd := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	updateCalled := false
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSession()
			rs.ID = 42
			rs.Status = "interrupted"
			end := oldEnd
			rs.EndDate = &end
			return rs, nil
		},
		updateStatusFn: func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
			updateCalled = true
			return nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "interrupted"})

	require.NoError(t, err)
	assert.False(t, updateCalled, "idempotente: no debe llamar al DAO")
	assert.Equal(t, "interrupted", rs.Status)
	require.NotNil(t, rs.EndDate)
	assert.Equal(t, oldEnd, *rs.EndDate)
}

func TestRunnerSessionService_Finish_AlreadyFinished_Idempotent(t *testing.T) {
	updateCalled := false
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSession()
			rs.Status = "finished"
			return rs, nil
		},
		updateStatusFn: func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
			updateCalled = true
			return nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "finished"})

	require.NoError(t, err)
	assert.False(t, updateCalled, "idempotente: no debe llamar al DAO")
	assert.Equal(t, "finished", rs.Status)
}

func TestRunnerSessionService_Finish_FinishedToInterrupted_Invalid(t *testing.T) {
	updateCalled := false
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSession()
			rs.Status = "finished"
			return rs, nil
		},
		updateStatusFn: func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
			updateCalled = true
			return nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "interrupted"})

	assert.False(t, updateCalled, "transición ilegal: no debe llamar al DAO")
	assert.EqualError(t, err, "datos inválidos: no se puede interrumpir una sesión ya finalizada")
	require.ErrorIs(t, err, ErrRunnerSessionInvalid)
}

func TestRunnerSessionService_Finish_InvalidStatus(t *testing.T) {
	updateCalled := false
	mock := &mockRunnerSessionDao{
		updateStatusFn: func(ctx *gin.Context, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error {
			updateCalled = true
			return nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "abandoned"})

	assert.False(t, updateCalled)
	assert.EqualError(t, err, "datos inválidos: status solo admite 'finished' o 'interrupted'")
	require.ErrorIs(t, err, ErrRunnerSessionInvalid)
}

func TestRunnerSessionService_Finish_NotFound(t *testing.T) {
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			return nil, daos.ErrRunnerSessionNotFound
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "finished"})

	require.ErrorIs(t, err, daos.ErrRunnerSessionNotFound)
}

func TestRunnerSessionService_Finish_TrainerForAthleteInOwnedTeam(t *testing.T) {
	athlete := int64(30)
	var gotAthlete int64
	mock := &mockRunnerSessionDao{
		existsUserInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
			return true, nil
		},
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			gotAthlete = athleteUserID
			return fixtureRunnerSession(), nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{
		AthleteUserID: &athlete,
		Status:        "finished",
	})

	require.NoError(t, err)
	assert.Equal(t, athlete, gotAthlete)
}

func TestRunnerSessionService_Get_Self(t *testing.T) {
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			assert.Equal(t, int64(7), athleteUserID)
			return fixtureRunnerSession(), nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	rs, err := svc.Get(nil, 7, 10, nil)

	require.NoError(t, err)
	assert.Equal(t, "wip", rs.Status)
}

// mockPresencialGate implementa PresencialSessionServiceInterface delegando a
// fns opcionales: nil en el constructor = sin gate ni hooks (comportamiento
// previo, sin cambios).
type mockPresencialGate struct {
	checkFn          func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error)
	onCreatedFn      func(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error)
	onFinishedFn     func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error)
	checkInvoked     bool
	onCreatedInvoked bool
}

func (m *mockPresencialGate) CheckAthleteEntry(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error) {
	m.checkInvoked = true
	if m.checkFn != nil {
		return m.checkFn(ctx, sessionInstanceID, authUserID)
	}
	return nil, nil
}

func (m *mockPresencialGate) OnRunnerCreated(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error) {
	m.onCreatedInvoked = true
	if m.onCreatedFn != nil {
		return m.onCreatedFn(ctx, sessionInstanceID, authUserID, gateDay)
	}
	return nil, false, nil
}

func (m *mockPresencialGate) OnRunnerFinished(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
	if m.onFinishedFn != nil {
		return m.onFinishedFn(ctx, sessionInstanceID, authUserID)
	}
	return nil, false, nil
}

func TestRunnerSessionService_Create_GatesAfterResolveAthlete(t *testing.T) {
	athlete := int64(30)
	// Atleta ajeno: el 403 de resolveAthlete PRECEDE al gate (el gate no corre).
	gate := &mockPresencialGate{}
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), gate)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
		AthleteUserID: &athlete, StartDate: sessionDate,
	})

	require.ErrorIs(t, err, ErrRunnerSessionForbidden)
	assert.False(t, gate.checkInvoked, "el atleta ajeno falla antes del gate")
}

func TestRunnerSessionService_Create_GateAfterResolveAthlete_BeforeWrite(t *testing.T) {
	blockedAtGate := false
	gate := &mockPresencialGate{checkFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error) {
		blockedAtGate = true
		return nil, ErrRunnerSessionClosed
	}}
	writeAttempted := false
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			writeAttempted = true
			return true, nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), gate)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{StartDate: sessionDate})

	require.ErrorIs(t, err, ErrRunnerSessionClosed)
	assert.True(t, blockedAtGate)
	assert.False(t, writeAttempted, "el día cerrado bloquea el write")
}

func TestRunnerSessionService_Create_GatePasses(t *testing.T) {
	gateDay := &dbs.GroupCalendarDay{ID: 9}
	gate := &mockPresencialGate{checkFn: func(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error) {
		return gateDay, nil
	}}
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		createFn:                func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) { return true, nil },
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), gate)

	_, gotDay, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{StartDate: sessionDate})

	require.NoError(t, err)
	assert.True(t, gate.checkInvoked)
	assert.True(t, created)
	assert.Same(t, gateDay, gotDay, "el día del gate sale en la respuesta para el hook de apertura")
}

func TestRunnerSessionService_Create_NilGate_SinGate(t *testing.T) {
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		createFn:                func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) { return true, nil },
	}
	svc := NewRunnerSessionServiceWithAccess(mock, allowInstanceAccess(), nil)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{StartDate: sessionDate})

	require.NoError(t, err)
}

func TestRunnerSessionService_Create_NoInstanceAccess_Forbidden(t *testing.T) {
	writeAttempted := false
	instanceDao := &mockSessionInstanceDao{
		hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
			assert.Equal(t, int64(10), instanceID)
			assert.Equal(t, int64(7), callerID)
			return false, nil
		},
	}
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			writeAttempted = true
			return true, nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, instanceDao, nil)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{StartDate: sessionDate})

	require.ErrorIs(t, err, ErrRunnerSessionForbidden)
	assert.False(t, writeAttempted, "sin acceso no se escribe")
}

func TestRunnerSessionService_Finish_NoInstanceAccess_Forbidden(t *testing.T) {
	readAttempted := false
	instanceDao := &mockSessionInstanceDao{
		hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
			return false, nil
		},
	}
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			readAttempted = true
			return fixtureRunnerSession(), nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, instanceDao, nil)

	_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "finished"})

	require.ErrorIs(t, err, ErrRunnerSessionForbidden)
	assert.False(t, readAttempted, "sin acceso no se lee el estado")
}

func TestRunnerSessionService_Get_NoInstanceAccess_Forbidden(t *testing.T) {
	readAttempted := false
	instanceDao := &mockSessionInstanceDao{
		hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
			return false, nil
		},
	}
	mock := &mockRunnerSessionDao{
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			readAttempted = true
			return fixtureRunnerSession(), nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, instanceDao, nil)

	_, err := svc.Get(nil, 7, 10, nil)

	require.ErrorIs(t, err, ErrRunnerSessionForbidden)
	assert.False(t, readAttempted, "sin acceso no se lee el estado")
}

func TestRunnerSessionService_Access_TrainerForAthleteInOwnedTeam(t *testing.T) {
	athlete := int64(30)
	var checkedCallers []int64
	instanceDao := &mockSessionInstanceDao{
		hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
			checkedCallers = append(checkedCallers, callerID)
			return true, nil
		},
	}
	mock := &mockRunnerSessionDao{
		existsUserInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
			return true, nil
		},
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			return fixtureRunnerSession(), nil
		},
	}
	svc := NewRunnerSessionServiceWithAccess(mock, instanceDao, nil)

	_, err := svc.Get(nil, 7, 10, &athlete)

	require.NoError(t, err)
	assert.Contains(t, checkedCallers, athlete, "el acceso se valida para el atleta efectivo")
	assert.Contains(t, checkedCallers, int64(7), "y también para el caller coach")
}

// Un coach solo puede operar sobre un atleta en una instancia a la que él
// también tiene acceso (el atleta sí tiene acceso, el coach no).
func TestRunnerSessionService_CoachWithoutInstanceAccess_Forbidden(t *testing.T) {
	athlete := int64(30)
	accessDao := func(athleteAccess bool) *mockSessionInstanceDao {
		return &mockSessionInstanceDao{
			hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
				if callerID == athlete {
					return athleteAccess, nil
				}
				return false, nil
			},
		}
	}
	ownedAthlete := func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) { return true, nil }

	t.Run("create", func(t *testing.T) {
		writeAttempted := false
		mock := &mockRunnerSessionDao{
			existsUserInTeamOwnedByFn: ownedAthlete,
			createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
				writeAttempted = true
				return true, nil
			},
		}
		svc := NewRunnerSessionServiceWithAccess(mock, accessDao(true), nil)

		_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
			AthleteUserID: &athlete, StartDate: sessionDate,
		})
		require.ErrorIs(t, err, ErrRunnerSessionForbidden)
		assert.False(t, writeAttempted)
	})

	t.Run("finish", func(t *testing.T) {
		mock := &mockRunnerSessionDao{
			existsUserInTeamOwnedByFn: ownedAthlete,
			getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
				return fixtureRunnerSession(), nil
			},
		}
		svc := NewRunnerSessionServiceWithAccess(mock, accessDao(true), nil)

		_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{
			Status: "finished", AthleteUserID: &athlete,
		})
		require.ErrorIs(t, err, ErrRunnerSessionForbidden)
	})

	t.Run("get", func(t *testing.T) {
		mock := &mockRunnerSessionDao{
			existsUserInTeamOwnedByFn: ownedAthlete,
			getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
				return fixtureRunnerSession(), nil
			},
		}
		svc := NewRunnerSessionServiceWithAccess(mock, accessDao(true), nil)

		_, err := svc.Get(nil, 7, 10, &athlete)
		require.ErrorIs(t, err, ErrRunnerSessionForbidden)
	})
}

// La instancia inexistente gana al guard de acceso: 404, no 403.
func TestRunnerSessionService_InstanceMissing_BeforeAccessGuard(t *testing.T) {
	instanceDao := &mockSessionInstanceDao{
		hasInstanceAccessFn: func(ctx *gin.Context, instanceID, callerID int64) (bool, error) {
			t.Fatal("no se debe consultar acceso de una instancia inexistente")
			return false, nil
		},
	}
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return false, nil },
	}
	svc := NewRunnerSessionServiceWithAccess(mock, instanceDao, nil)

	_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "finished"})
	require.ErrorIs(t, err, ErrSessionInstanceNotFound)

	_, err = svc.Get(nil, 7, 10, nil)
	require.ErrorIs(t, err, ErrSessionInstanceNotFound)
}

func TestRunnerSessionService_NilInstanceDao_FailsClosed(t *testing.T) {
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		createFn:                func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) { return true, nil },
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			return fixtureRunnerSession(), nil
		},
	}
	svc := NewRunnerSessionService(mock, nil)

	_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{StartDate: sessionDate})
	require.Error(t, err)

	_, err = svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "finished"})
	require.Error(t, err)

	_, err = svc.Get(nil, 7, 10, nil)
	require.Error(t, err)
}
