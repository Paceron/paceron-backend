package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// mockRunnerSessionDao implementa RunnerSessionDAOInterface delegando a fns
// opcionales; sin fn devuelve el default.
type mockRunnerSessionDao struct {
	sessionInstanceExistsFn func(ctx *gin.Context, sessionInstanceID int64) (bool, error)
	createFn                func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error)
	getBySessionAthleteFn   func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error)
	finishFn                func(ctx *gin.Context, rs *dbs.RunnerSession) error
	teamExistsFn            func(ctx *gin.Context, teamID int64) (bool, error)
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

func (m *mockRunnerSessionDao) Finish(ctx *gin.Context, rs *dbs.RunnerSession) error {
	if m.finishFn != nil {
		return m.finishFn(ctx, rs)
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

func TestRunnerSessionService_Finish_Self(t *testing.T) {
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			return true, nil
		},
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			return true, nil
		},
		getBySessionAthleteFn: func(ctx *gin.Context, sessionInstanceID, athleteUserID int64) (*dbs.RunnerSession, error) {
			rs := fixtureRunnerSession()
			rs.Status = "finished"
			return rs, nil
		},
	}
	svc := NewRunnerSessionService(mock)

	rs, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{
		Status: "finished",
	})

	require.NoError(t, err)
	assert.Equal(t, "finished", rs.Status)
}

func TestRunnerSessionService_Finish_InvalidStatus(t *testing.T) {
	mock := &mockRunnerSessionDao{}
	svc := NewRunnerSessionService(mock)

	_, err := svc.Finish(nil, 7, 10, runnersession.RunnerStatusRequest{
		Status: "abandoned",
	})

	require.ErrorIs(t, err, ErrRunnerSessionInvalid)
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
