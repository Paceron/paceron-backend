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
	return false, nil
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
	svc := NewRunnerSessionService(mock)

	rs, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
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
	svc := NewRunnerSessionService(mock)

	_, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
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
	svc := NewRunnerSessionService(mock)

	rs, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
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
	svc := NewRunnerSessionService(mock)

	rs, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
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
	svc := NewRunnerSessionService(mock)

	_, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
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
	svc := NewRunnerSessionService(mock)

	_, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{})

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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

	rs, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{Status: "interrupted"})

	require.NoError(t, err)
	assert.False(t, updateCalled, "idempotente: no debe llamar al DAO")
	assert.Equal(t, "interrupted", rs.Status)
	require.NotNil(t, rs.EndDate)
	assert.WithinDuration(t, oldEnd, *rs.EndDate, 0)
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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

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
	svc := NewRunnerSessionService(mock)

	rs, err := svc.Get(nil, 7, 10, nil)

	require.NoError(t, err)
	assert.Equal(t, "wip", rs.Status)
}
