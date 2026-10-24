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

// Intake de intentos de competencias de ruta (Gap 36, intentos múltiples):
// mocks de DAO de competencias. Verifica el orden de los guards de cabeza y
// que el límite/la materialización de instancia vivan en el tx del DAO de
// competencias (el service delega sin escribir intentos por su cuenta).

func routeAttemptDao(instanceID int64) *mockCompetitionDAO {
	attempts := int64(3)
	return &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) {
			return &dbs.Competition{
				ID:                     id,
				TeamID:                 10,
				Type:                   competition.TypeRoute,
				Status:                 competition.StatusActive,
				RouteSessionInstanceID: &instanceID,
				MaxAttempts:            &attempts,
			}, nil
		},
		isParticipantFn: func(competitionID, userID int64) (bool, error) { return true, nil },
	}
}

// TestCompetitionService_CreateAttemptHappy verifica la delegación: el service
// pasa comp + atleta al DAO (el límite lo decide el DAO con la fila lockeada)
// y devuelve la runner session que el
// DAO materializó. El DAO de runner sessions no recibe ninguna llamada: la tx
// del intento vive entera en el DAO de competencias.
func TestCompetitionService_CreateAttemptHappy(t *testing.T) {
	dao := routeAttemptDao(42)
	var capturedComp *dbs.Competition
	var capturedAthlete int64
	dao.createAttemptSessionFn = func(comp *dbs.Competition, athleteUserID int64) (*dbs.RunnerSession, error) {
		capturedComp = comp
		capturedAthlete = athleteUserID
		return &dbs.RunnerSession{ID: 123, SessionInstanceID: 42, AthleteUserID: athleteUserID,
			Status: runnersession.RunnerSessionStatusWip, CompetitionID: &comp.ID}, nil
	}
	var runnerCreateCalls int
	runnerDao := &mockRunnerSessionDao{}
	runnerDao.createFn = func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
		runnerCreateCalls++
		return true, nil
	}
	svc := NewCompetitionService(dao, nil, nil, runnerDao)

	rs, err := svc.CreateAttempt(nil, 7, 5)
	require.NoError(t, err)
	require.NotNil(t, rs)
	assert.Equal(t, int64(123), rs.ID)
	assert.Equal(t, int64(42), rs.SessionInstanceID, "la runner session la devuelve el DAO, sin remap del service")
	assert.Equal(t, int64(5), capturedComp.ID)
	assert.Equal(t, int64(7), capturedAthlete)
	assert.Equal(t, 0, runnerCreateCalls, "el intake no escribe por runnerSessionDao")
}

func TestCompetitionService_CreateAttemptGuardsEnOrden(t *testing.T) {
	// daoWriteBlocked vigila el DAO de runner sessions (que ya no participa):
	// ningún guard debe llegar a escribir.
	daoWriteBlocked := func(t *testing.T) *mockRunnerSessionDao {
		runnerDao := &mockRunnerSessionDao{}
		runnerDao.createFn = func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			t.Fatal("el guard debe cortar antes de escribir (y el write ya vive en el DAO)")
			return false, nil
		}
		return runnerDao
	}

	writeBlockedSvc := func(t *testing.T, dao *mockCompetitionDAO) CompetitionServiceInterface {
		return NewCompetitionService(dao, nil, nil, daoWriteBlocked(t))
	}

	t.Run("404 no existe", func(t *testing.T) {
		dao := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return nil, daos.ErrCompetitionNotFound
			},
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 999)
		require.ErrorIs(t, err, daos.ErrCompetitionNotFound)
		assert.Equal(t, 0, dao.createAttemptSessionCalls)
	})

	t.Run("400 type metric", func(t *testing.T) {
		dao := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return &dbs.Competition{ID: id, Type: competition.TypeMetric, Status: competition.StatusActive}, nil
			},
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
		assert.Contains(t, err.Error(), "solo las competencias de ruta admiten intentos")
		assert.Equal(t, 0, dao.createAttemptSessionCalls)
	})

	t.Run("400 status no activa", func(t *testing.T) {
		dao := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return &dbs.Competition{ID: id, Type: competition.TypeRoute, Status: competition.StatusPendingApproval}, nil
			},
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
		assert.Contains(t, err.Error(), "la competencia no está activa")
		assert.Equal(t, 0, dao.createAttemptSessionCalls)
	})

	t.Run("403 no participante", func(t *testing.T) {
		dao := routeAttemptDao(42)
		participantCalls := 0
		dao.isParticipantFn = func(competitionID, userID int64) (bool, error) {
			participantCalls++
			return false, nil
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionForbidden)
		assert.Equal(t, 1, participantCalls)
		assert.Equal(t, 0, dao.createAttemptSessionCalls)
	})

	t.Run("400 sin instancia compartida", func(t *testing.T) {
		dao := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				return &dbs.Competition{ID: id, TeamID: 10, Type: competition.TypeRoute, Status: competition.StatusActive}, nil
			},
			isParticipantFn: func(competitionID, userID int64) (bool, error) { return true, nil },
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
		assert.Contains(t, err.Error(), "no tiene instancia de ruta asignada")
		assert.Equal(t, 0, dao.createAttemptSessionCalls)
	})

	t.Run("400 límite de intentos", func(t *testing.T) {
		dao := routeAttemptDao(42)
		dao.createAttemptSessionFn = func(comp *dbs.Competition, athleteUserID int64) (*dbs.RunnerSession, error) {
			return nil, daos.ErrCompetitionAttemptLimit
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
		assert.Contains(t, err.Error(), "llegaste al límite de intentos de esta competencia")
	})

	t.Run("400 conflict de instancia compartida (ON CONFLICT, DAO lo decide)", func(t *testing.T) {
		dao := routeAttemptDao(42)
		dao.createAttemptSessionFn = func(comp *dbs.Competition, athleteUserID int64) (*dbs.RunnerSession, error) {
			return nil, daos.ErrCompetitionAttemptConflict
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionInvalid)
		assert.Contains(t, err.Error(), "ya tenés un intento registrado en esta competencia")
	})

	t.Run("403 precede al límite", func(t *testing.T) {
		dao := routeAttemptDao(42)
		dao.isParticipantFn = func(competitionID, userID int64) (bool, error) { return false, nil }
		dao.createAttemptSessionFn = func(comp *dbs.Competition, athleteUserID int64) (*dbs.RunnerSession, error) {
			t.Fatal("el guard de participante corta antes del DAO")
			return nil, nil
		}
		_, err := writeBlockedSvc(t, dao).CreateAttempt(nil, 7, 5)
		require.ErrorIs(t, err, ErrCompetitionForbidden)
	})
}

func TestCompetitionService_CreateAttemptDAOError500(t *testing.T) {
	dao := routeAttemptDao(42)
	dao.createAttemptSessionFn = func(comp *dbs.Competition, athleteUserID int64) (*dbs.RunnerSession, error) {
		return nil, errors.New("db caida")
	}
	svc := NewCompetitionService(dao, nil, nil, &mockRunnerSessionDao{})
	_, err := svc.CreateAttempt(nil, 7, 5)
	require.EqualError(t, err, "error al registrar el intento")
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
