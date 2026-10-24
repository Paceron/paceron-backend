package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// Extensión de POST /session-instances/:id/runner con competition_id
// (gamificacion-core E4.1): valida active + participante + rango; sin el
// campo, el flujo queda idéntico al de siempre.

func TestRunnerSessionService_Create_SinCompetitionId_FlujoIntacto(t *testing.T) {
	var received *dbs.RunnerSession
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			received = rs
			return true, nil
		},
	}
	// Sin competitionDAO inyectado: el flujo clásico ni consulta el módulo.
	svc := NewRunnerSessionService(mock, nil)
	_, _, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{StartDate: sessionDate})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Nil(t, received.CompetitionID)
}

func TestRunnerSessionService_Create_CompetitionTageada(t *testing.T) {
	origNow := gametime.Now
	t.Cleanup(func() { gametime.Now = origNow })
	gametime.Now = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, gametime.Location) }

	metric := "distance"
	comp := &dbs.Competition{
		ID: 9, TeamID: 10, Type: competition.TypeMetric, Metric: &metric,
		Status:  competition.StatusActive,
		StartAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		EndAt: func() *time.Time {
			t := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
			return &t
		}(),
	}
	compDAO := &mockCompetitionDAO{
		findByIDFn: func(id int64) (*dbs.Competition, error) { return comp, nil },
		isParticipantFn: func(competitionID, userID int64) (bool, error) {
			return userID == 7, nil
		},
	}
	var captured *int64
	mock := &mockRunnerSessionDao{
		sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
			captured = rs.CompetitionID
			return true, nil
		},
	}
	svc := NewRunnerSessionService(mock, nil, compDAO)
	compID := int64(9)

	// Partida dentro del rango por un participante: taguea.
	_, _, created, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
		StartDate: sessionDate, CompetitionID: &compID,
	})
	require.NoError(t, err)
	assert.True(t, created)
	require.NotNil(t, captured)
	assert.Equal(t, int64(9), *captured)

	// Sin el campo: la sesión no lleva marca.
	captured = nil
	_, _, _, err = svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{StartDate: sessionDate})
	require.NoError(t, err)
	assert.Nil(t, captured)
}

func TestRunnerSessionService_Create_CompetitionRechazos(t *testing.T) {
	origNow := gametime.Now
	t.Cleanup(func() { gametime.Now = origNow })
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, gametime.Location)
	gametime.Now = func() time.Time { return now }

	endBefore := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	endAfter := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	compFor := func(status string, startAt time.Time, endAt *time.Time) *dbs.Competition {
		return &dbs.Competition{ID: 9, TeamID: 10, Status: status, StartAt: startAt, EndAt: endAt}
	}
	cases := []struct {
		name string
		comp *dbs.Competition
	}{
		{"no existe", nil},
		{"no activa", compFor(competition.StatusPendingApproval, now.AddDate(0, 0, -1), &endAfter)},
		{"finished", compFor(competition.StatusFinished, now.AddDate(0, 0, -1), &endAfter)},
		{"no participante", compFor(competition.StatusActive, now.AddDate(0, 0, -1), &endAfter)},
		{"antes de start_at", compFor(competition.StatusActive, now.AddDate(0, 0, 1), &endAfter)},
		{"después de end_at", compFor(competition.StatusActive, now.AddDate(0, 0, -3), &endBefore)},
	}
	for _, tc := range cases {
		compDAO := &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) {
				if tc.comp == nil {
					return nil, daos.ErrCompetitionNotFound
				}
				return tc.comp, nil
			},
			isParticipantFn: func(competitionID, userID int64) (bool, error) {
				return false, nil
			},
		}
		// Un solo participante válido (999): la falla queda pinada a lo que
		// el caso prueba.
		if tc.name == "no participante" {
			compDAO.isParticipantFn = func(competitionID, userID int64) (bool, error) {
				return userID == 999, nil
			}
		}
		mock := &mockRunnerSessionDao{
			sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
		}
		svc := NewRunnerSessionService(mock, nil, compDAO)
		compID := int64(9)
		_, _, _, err := svc.Create(nil, 7, 10, runnersession.CreateRunnerSessionRequest{
			StartDate: sessionDate, CompetitionID: &compID,
		})
		require.ErrorIs(t, err, ErrRunnerSessionInvalid, tc.name)
	}
}

// Anclaje de instancia (competencias-ruta-intake, review extra I1): una
// competencia route solo taguea sobre su propia instancia sintética y una
// instancia sintética solo acepta a su dueña. Cross-metric fuera de alcance:
// el guard cubre también el mixeo de tipos.
func TestRunnerSessionService_Create_CompetenciaRutaAncla(t *testing.T) {
	origNow := gametime.Now
	t.Cleanup(func() { gametime.Now = origNow })
	gametime.Now = func() time.Time { return time.Date(2026, 10, 5, 12, 0, 0, 0, gametime.Location) }

	instA, instB := int64(100), int64(200)
	inWindow := func() *time.Time {
		end := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
		return &end
	}
	routeFor := func(id, instID int64) *dbs.Competition {
		return &dbs.Competition{
			ID: id, TeamID: 10, Type: competition.TypeRoute,
			RouteSessionInstanceID: &instID,
			Status:                 competition.StatusActive,
			StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			EndAt:                  inWindow(),
		}
	}
	compA, compB := routeFor(9, instA), routeFor(11, instB)
	compMetric := &dbs.Competition{
		ID: 5, TeamID: 10, Type: competition.TypeMetric, Metric: strPtr("distance"),
		Status:  competition.StatusActive,
		StartAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		EndAt:   inWindow(),
	}
	byID := map[int64]*dbs.Competition{9: compA, 11: compB, 5: compMetric}
	compDAO := func() *mockCompetitionDAO {
		return &mockCompetitionDAO{
			findByIDFn: func(id int64) (*dbs.Competition, error) { return byID[id], nil },
			isParticipantFn: func(competitionID, userID int64) (bool, error) {
				return userID == 7, nil
			},
			findByRouteInstFn: func(instanceID int64) (*dbs.Competition, error) {
				if instanceID == instA {
					return compA, nil
				}
				if instanceID == instB {
					return compB, nil
				}
				return nil, daos.ErrCompetitionNotFound
			},
		}
	}

	newSvc := func() (*mockCompetitionDAO, *mockRunnerSessionDao, **int64, *int) {
		dao := compDAO()
		var captured *int64
		createCalls := 0
		mock := &mockRunnerSessionDao{
			sessionInstanceExistsFn: func(ctx *gin.Context, sessionInstanceID int64) (bool, error) { return true, nil },
			createFn: func(ctx *gin.Context, rs *dbs.RunnerSession) (bool, error) {
				createCalls++
				captured = rs.CompetitionID
				return true, nil
			},
		}
		return dao, mock, &captured, &createCalls
	}

	comp9, comp11, comp5 := int64(9), int64(11), int64(5)

	// A: instancia sintética de comp A tagueada con comp B (otra route,
	// atleta participante de ambas): 400 y sin fila creada.
	dao, mock, _, createCalls := newSvc()
	svc := NewRunnerSessionService(mock, nil, dao)
	_, _, _, err := svc.Create(nil, 7, instA, runnersession.CreateRunnerSessionRequest{
		StartDate: sessionDate, CompetitionID: &comp11,
	})
	require.ErrorIs(t, err, ErrRunnerSessionInvalid)
	assert.Equal(t, 0, *createCalls, "el rechazo tiene que ser antes del write")

	// B: tagueo type-inconsistente: comp metric sobre la instancia
	// sintética de una route, misma comparación inversa: 400.
	dao, mock, _, createCalls = newSvc()
	svc = NewRunnerSessionService(mock, nil, dao)
	_, _, _, err = svc.Create(nil, 7, instA, runnersession.CreateRunnerSessionRequest{
		StartDate: sessionDate, CompetitionID: &comp5,
	})
	require.ErrorIs(t, err, ErrRunnerSessionInvalid)
	assert.Equal(t, 0, *createCalls)

	// C (happy de regresión): la competencia dueña sobre su propia
	// instancia sintética sigue creando el intento.
	dao, mock, captured, _ := newSvc()
	svc = NewRunnerSessionService(mock, nil, dao)
	_, _, created, err := svc.Create(nil, 7, instA, runnersession.CreateRunnerSessionRequest{
		StartDate: sessionDate, CompetitionID: &comp9,
	})
	require.NoError(t, err)
	assert.True(t, created)
	require.NotNil(t, *captured)
	assert.Equal(t, comp9, **captured)
	assert.Equal(t, 1, dao.findByRouteInstCalls)
}
