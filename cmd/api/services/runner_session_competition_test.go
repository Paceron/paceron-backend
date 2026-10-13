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
