package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// Intake de intentos de competencias de ruta (competencias-ruta-intake D2):
// mocks de DAO de competencias + DAO de runner sessions. Verifica el orden de
// los 5 guards y que el write va directo al DAO (sin pasar por el service/gate
// presencial de runner sessions).

func routeAttemptDao(instanceID int64) *mockCompetitionDAO {
	return &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) {
			return &dbs.Competition{
				ID:                     id,
				TeamID:                 10,
				Type:                   competition.TypeRoute,
				Status:                 competition.StatusActive,
				RouteSessionInstanceID: &instanceID,
			}, nil
		},
		isParticipantFn: func(competitionID, userID int64) (bool, error) { return true, nil },
	}
}

// TestCompetitionService_CreateAttemptHappy verifica el shape del write: wip,
// atleta = caller, session_instance_id = instancia sintética de la
// competencia, competition_id tagueado. El DAO de runner sessions NO recibe la
// llamada SessionInstanceExists: el intake va directo al Create (el gate
// presencial / día vive solo en runnerSessionService.Create, que no se toca).
func TestCompetitionService_CreateAttemptHappy(t *testing.T) {
	dao := routeAttemptDao(42)
	var existsCalls int
	runnerDao := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) {
			existsCalls++
			return false, nil
		},
	}
	var captured *dbs.RunnerSession
	runnerDao.createFn = func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
		captured = rs
		return true, nil
	}
	svc := NewCompetitionService(dao, nil, nil, runnerDao)

	rs, err := svc.CreateAttempt(nil, 7, 5)
	require.NoError(t, err)
	require.NotNil(t, rs)
	require.NotNil(t, captured)
	assert.Equal(t, int64(42), captured.SessionInstanceID)
	assert.Equal(t, int64(7), captured.AthleteUserID)
	assert.Equal(t, runnersession.RunnerSessionStatusWip, captured.Status)
	require.Equal(t, int64(5), *captured.CompetitionID)
	assert.Equal(t, int64(42), rs.SessionInstanceID)
	assert.Zero(t, existsCalls, "el intake no pasa por el service de runner sessions (gate presencial no aplica)")
}

func TestCompetitionService_CreateAttemptGuardsEnOrden(t *testing.T) {
	runnerDao := &mockRunnerSessionDao{}
	runnerDao.createFn = func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
		t.Fatal("el guard debe cortar antes del write")
		return false, nil
	}

	t.Run("404 no existe", func(t *testing.T) {
		dao := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return nil, daos.ErrCompetitionNotFound
			},
		}
		svc := NewCompetitionService(dao, nil, nil, runnerDao)
		_, err := svc.CreateAttempt(nil, 7, 999)
		require.ErrorIs(t, err, daos.ErrCompetitionNotFound)
	})

	t.Run("400 type metric", func(t *testing.T) {
		dao := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return &dbs.Competition{ID: id, Type: competition.TypeMetric, Status: competition.StatusActive}, nil
			},
		}
		svc := NewCompetitionService(dao, nil, nil, runnerDao)
		_, err := svc.CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
		assert.Contains(t, err.Error(), "solo las competencias de ruta admiten intentos")
	})

	t.Run("400 status no activa", func(t *testing.T) {
		dao := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return &dbs.Competition{ID: id, Type: competition.TypeRoute, Status: competition.StatusPendingApproval}, nil
			},
		}
		svc := NewCompetitionService(dao, nil, nil, runnerDao)
		_, err := svc.CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
		assert.Contains(t, err.Error(), "la competencia no está activa")
	})

	t.Run("403 no participante", func(t *testing.T) {
		dao := routeAttemptDao(42)
		participantCalls := 0
		dao.isParticipantFn = func(competitionID, userID int64) (bool, error) {
			participantCalls++
			return false, nil
		}
		svc := NewCompetitionService(dao, nil, nil, runnerDao)
		_, err := svc.CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionForbidden)
		assert.Equal(t, 1, participantCalls)
	})

	t.Run("400 doble intento via pre-check", func(t *testing.T) {
		dao := routeAttemptDao(42)
		dao.existsAttemptFn = func(competitionID, userID int64) (bool, error) { return true, nil }
		svc := NewCompetitionService(dao, nil, nil, runnerDao)
		_, err := svc.CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
	})

	t.Run("400 doble intento via unique (race, RowsAffected=0)", func(t *testing.T) {
		dao := routeAttemptDao(42)
		runnerDao := &mockRunnerSessionDao{}
		runnerDao.createFn = func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			return false, nil
		}
		svc := NewCompetitionService(dao, nil, nil, runnerDao)
		_, err := svc.CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
	})

	t.Run("403 precede al doble intento", func(t *testing.T) {
		dao := routeAttemptDao(42)
		dao.isParticipantFn = func(competitionID, userID int64) (bool, error) { return false, nil }
		dao.existsAttemptFn = func(competitionID, userID int64) (bool, error) { return true, nil }
		svc := NewCompetitionService(dao, nil, nil, runnerDao)
		_, err := svc.CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionForbidden)
	})
}

func TestCompetitionService_CreateAttemptDAOError500(t *testing.T) {
	dao := routeAttemptDao(42)
	dao.existsAttemptFn = func(competitionID, userID int64) (bool, error) {
		return false, errors.New("db caida")
	}
	svc := NewCompetitionService(dao, nil, nil, &mockRunnerSessionDao{})
	_, err := svc.CreateAttempt(nil, 7, 5)
	require.NotErrorIs(t, err, ErrCompetitionInvalid)
}

// Branch route del Create (D1): route llama CreateWithInstance (la instancia
// sintética nace en el tx del alta); metric sigue llamando Create plano.
func TestCompetitionService_CreateRouteUsaInstancia(t *testing.T) {
	var captured *dbs.Competition
	dao := &mockCompetitionDAO{
		teamOwnerFn:    func(teamID int64) (int64, error) { return 1, nil },
		countMembersFn: func(teamID int64, userIDs []int64) (int64, error) { return int64(len(userIDs)), nil },
		createWithInstFn: func(comp *dbs.Competition, participantIDs []int64) error {
			captured = comp
			return nil
		},
	}
	// mockCompetitionDAO.CreateWithInstance llama a Create: el fn del branch
	// corre igual con el reuso interno; el contador distingue ambos caminos.
	svc := NewCompetitionService(dao, nil, nil)

	_, err := svc.Create(nil, 1, 10, routeCreateReq())
	require.NoError(t, err)
	assert.Equal(t, 1, dao.createWithInstCalls)
	assert.Equal(t, 0, dao.createCalls)
	require.NotNil(t, captured)
	assert.Equal(t, competition.TypeRoute, captured.Type)

	_, err = svc.Create(nil, 1, 10, validMetricCreateReq())
	require.NoError(t, err)
	assert.Equal(t, 1, dao.createWithInstCalls, "metric no usa el branch de instancia")
	assert.Equal(t, 1, dao.createCalls)
}
