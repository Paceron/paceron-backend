package services

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
)

type historyDAOCalls struct {
	searchFilters daos.WorkoutFeedbackHistoryFilters
	sortCol       string
	order         string
	limit         int
	offset        int
	searchCalled  bool
	teamExists    bool
	isTeamOwner   bool
	userIDs       []int64
	teamIDs       []int64
	groupIDs      []int64
}

func newHistoryMock(calls *historyDAOCalls, rows []dbs.WorkoutFeedbackHistoryRow) *mockWorkoutFeedbackDao {
	return &mockWorkoutFeedbackDao{
		historySearchFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackHistoryFilters, sortCol, order string, limit, offset int) ([]dbs.WorkoutFeedbackHistoryRow, error) {
			calls.searchCalled = true
			calls.searchFilters = filters
			calls.sortCol = sortCol
			calls.order = order
			calls.limit = limit
			calls.offset = offset
			return rows, nil
		},
		historyCountFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackHistoryFilters) (int64, error) {
			return 42, nil
		},
		historyAthletesFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackHistoryFilters) ([]dbs.IDName, error) {
			return []dbs.IDName{{ID: 7, Name: "Juan"}}, nil
		},
		historyExercisesFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackHistoryFilters) ([]dbs.IDName, error) {
			return []dbs.IDName{{ID: 3, Name: "Sentadilla"}}, nil
		},
		usersByIDsFn: func(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
			calls.userIDs = userIDs
			users := []*dbs.User{}
			for _, id := range userIDs {
				users = append(users, &dbs.User{ID: id, Name: "Atleta" + strconv.FormatInt(id, 10)})
			}
			return users, nil
		},
		teamsByIDsFn: func(ctx *gin.Context, teamIDs []int64) ([]dbs.Team, error) {
			calls.teamIDs = teamIDs
			teams := []dbs.Team{}
			for _, id := range teamIDs {
				teams = append(teams, dbs.Team{ID: id, Name: "Equipo"})
			}
			return teams, nil
		},
		groupsByIDsFn: func(ctx *gin.Context, groupIDs []int64) ([]dbs.Group, error) {
			calls.groupIDs = groupIDs
			groups := []dbs.Group{}
			for _, id := range groupIDs {
				groups = append(groups, dbs.Group{ID: id, Name: "Grupo"})
			}
			return groups, nil
		},
		teamExistsFn: func(ctx *gin.Context, teamID int64) (bool, error) {
			calls.teamExists = true
			return true, nil
		},
		isTeamOwnerFn: func(ctx *gin.Context, teamID, userID int64) (bool, error) {
			calls.isTeamOwner = true
			return true, nil
		},
	}
}

func TestWorkoutFeedbackService_AthleteHistory_TargetDistinto_403(t *testing.T) {
	calls := &historyDAOCalls{}
	svc := NewWorkoutFeedbackService(newHistoryMock(calls, nil))

	_, err := svc.AthleteHistory(nil, 7, 99, workoutfeedback.HistoryQuery{})

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
	assert.False(t, calls.searchCalled, "no debe consultar el DAO si el target no es el caller")
}

func TestWorkoutFeedbackService_AthleteHistory_Self_ScopeYDefaults(t *testing.T) {
	calls := &historyDAOCalls{}
	svc := NewWorkoutFeedbackService(newHistoryMock(calls, nil))

	resp, err := svc.AthleteHistory(nil, 7, 7, workoutfeedback.HistoryQuery{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.True(t, calls.searchCalled)
	require.NotNil(t, calls.searchFilters.AthleteUserID)
	assert.Equal(t, int64(7), *calls.searchFilters.AthleteUserID)
	assert.Nil(t, calls.searchFilters.TeamID)
	assert.Nil(t, calls.searchFilters.AthleteFilterUserID, "el endpoint del atleta no soporta filtro de atleta")
	assert.Equal(t, "feedback_date", calls.sortCol)
	assert.Equal(t, "desc", calls.order)
	assert.Equal(t, 20, calls.limit)
	assert.Equal(t, 0, calls.offset)
	assert.Equal(t, 1, resp.Page)
	assert.Equal(t, 20, resp.PageSize)
	assert.Equal(t, int64(42), resp.Total)
	assert.Equal(t, []workoutfeedback.WorkoutFeedbackHistoryItem{}, resp.Items)
	assert.Equal(t, []dbs.IDName{{ID: 7, Name: "Juan"}}, resp.AvailableAthletes)
	assert.Equal(t, []dbs.IDName{{ID: 3, Name: "Sentadilla"}}, resp.AvailableExercises)
}

func TestWorkoutFeedbackService_AthleteHistory_FiltrosYPaginacion(t *testing.T) {
	calls := &historyDAOCalls{}
	svc := NewWorkoutFeedbackService(newHistoryMock(calls, nil))

	teamID := int64(4)
	groupID := int64(6)
	exerciseID := int64(3)
	setNumber := 2
	athleteFilter := int64(88)
	page := 3
	pageSize := 10
	_, err := svc.AthleteHistory(nil, 7, 7, workoutfeedback.HistoryQuery{
		TeamID:        &teamID,
		GroupID:       &groupID,
		ExerciseID:    &exerciseID,
		SetNumber:     &setNumber,
		AthleteUserID: &athleteFilter,
		DateFrom:      histStrPtr("2026-01-01"),
		DateTo:        histStrPtr("2026-01-31"),
		Page:          &page,
		PageSize:      &pageSize,
		Sort:          "set_number",
		Order:         "asc",
	})

	require.NoError(t, err)
	f := calls.searchFilters
	require.NotNil(t, f.TeamID)
	assert.Equal(t, int64(4), *f.TeamID)
	require.NotNil(t, f.GroupID)
	assert.Equal(t, int64(6), *f.GroupID)
	require.NotNil(t, f.ExerciseInstanceID)
	assert.Equal(t, int64(3), *f.ExerciseInstanceID)
	require.NotNil(t, f.SetNumber)
	assert.Equal(t, 2, *f.SetNumber)
	assert.Nil(t, f.AthleteFilterUserID)
	require.NotNil(t, f.DateFrom)
	assert.Equal(t, "2026-01-01", f.DateFrom.Format("2006-01-02"))
	require.NotNil(t, f.DateTo)
	assert.Equal(t, "2026-01-31", f.DateTo.Format("2006-01-02"))
	assert.Equal(t, "set_number", calls.sortCol)
	assert.Equal(t, "asc", calls.order)
	assert.Equal(t, 10, calls.limit)
	assert.Equal(t, 20, calls.offset)
}

func TestWorkoutFeedbackService_AthleteHistory_ResponseShapeNombresBatch(t *testing.T) {
	sessionDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	started := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	ended := time.Date(2026, 3, 10, 8, 45, 0, 0, time.UTC)
	teamID := int64(4)
	groupID := int64(6)
	orphanTeam := int64(999)
	status := "completa"
	duration := int64(2700000)
	active := int64(2400000)
	distance := 5.5
	exName := "Sentadilla"
	sessionName := "Fuerza piernas"
	catalog := int64(12)

	rows := []dbs.WorkoutFeedbackHistoryRow{
		{ID: 1, AthleteUserID: 7, TeamID: &teamID, GroupID: &groupID, SessionDate: sessionDate,
			SetNumber: 1, CompletionStatus: &status, DurationMs: &duration, ActiveDurationMs: &active,
			DistanceMeters: &distance, StartedAt: &started, EndedAt: &ended,
			ExerciseID: 3, ExerciseName: &exName, CatalogExerciseID: &catalog, SessionName: &sessionName,
			SessionInstanceID: 404},
		// mismo atleta de nuevo: el batch de users debe deduplicar
		{ID: 2, AthleteUserID: 7, TeamID: &teamID, SessionDate: sessionDate, SetNumber: 2, ExerciseID: 3},
		// huérfano: sin team, sin group
		{ID: 3, AthleteUserID: 8, SessionDate: sessionDate, SetNumber: 1, ExerciseID: 5},
		// team inexistente en el batch (soft-delete): id presente, nombre no resuelve
		{ID: 4, AthleteUserID: 8, TeamID: &orphanTeam, SessionDate: sessionDate, SetNumber: 1, ExerciseID: 5},
	}

	calls := &historyDAOCalls{}
	mock := newHistoryMock(calls, rows)
	mock.teamsByIDsFn = func(ctx *gin.Context, teamIDs []int64) ([]dbs.Team, error) {
		calls.teamIDs = teamIDs
		return []dbs.Team{{ID: 4, Name: "Equipo"}}, nil
	}
	svc := NewWorkoutFeedbackService(mock)

	resp, err := svc.AthleteHistory(nil, 7, 7, workoutfeedback.HistoryQuery{})
	require.NoError(t, err)
	require.Len(t, resp.Items, 4)

	first := resp.Items[0]
	assert.Equal(t, int64(1), first.ID)
	assert.Equal(t, int64(7), first.AthleteUserID)
	assert.Equal(t, "Atleta7", first.AthleteName)
	require.NotNil(t, first.TeamID)
	assert.Equal(t, int64(4), *first.TeamID)
	require.NotNil(t, first.TeamName)
	assert.Equal(t, "Equipo", *first.TeamName)
	require.NotNil(t, first.GroupID)
	assert.Equal(t, int64(6), *first.GroupID)
	require.NotNil(t, first.GroupName)
	assert.Equal(t, "Grupo", *first.GroupName)
	assert.Equal(t, "2026-03-10", first.Date)
	require.NotNil(t, first.SessionName)
	assert.Equal(t, "Fuerza piernas", *first.SessionName)
	assert.Equal(t, int64(404), first.SessionInstanceID)
	assert.Equal(t, int64(3), first.ExerciseID)
	require.NotNil(t, first.ExerciseName)
	assert.Equal(t, "Sentadilla", *first.ExerciseName)
	require.NotNil(t, first.CatalogExerciseID)
	assert.Equal(t, int64(12), *first.CatalogExerciseID)
	assert.Equal(t, 1, first.SetNumber)
	require.NotNil(t, first.CompletionStatus)
	assert.Equal(t, "completa", *first.CompletionStatus)
	require.NotNil(t, first.DurationMs)
	assert.Equal(t, int64(2700000), *first.DurationMs)
	require.NotNil(t, first.ActiveDurationMs)
	assert.Equal(t, int64(2400000), *first.ActiveDurationMs)
	require.NotNil(t, first.DistanceMeters)
	assert.Equal(t, 5.5, *first.DistanceMeters)
	require.NotNil(t, first.StartedAt)
	require.NotNil(t, first.EndedAt)

	// huérfano
	assert.Equal(t, "Atleta8", resp.Items[2].AthleteName)
	assert.Nil(t, resp.Items[2].TeamID)
	assert.Nil(t, resp.Items[2].TeamName)
	assert.Nil(t, resp.Items[2].GroupID)
	assert.Nil(t, resp.Items[2].GroupName)
	assert.Equal(t, int64(5), resp.Items[2].ExerciseID)
	assert.Nil(t, resp.Items[2].ExerciseName)
	assert.Nil(t, resp.Items[2].SessionName)
	assert.Equal(t, int64(0), resp.Items[2].SessionInstanceID)

	// team no resuelto: team_id presente, team_name null
	require.NotNil(t, resp.Items[3].TeamID)
	assert.Equal(t, int64(999), *resp.Items[3].TeamID)
	assert.Nil(t, resp.Items[3].TeamName)

	// batches deduplicados
	assert.Equal(t, []int64{7, 8}, calls.userIDs)
	assert.Equal(t, []int64{4, 999}, calls.teamIDs)
	assert.Equal(t, []int64{6}, calls.groupIDs)
}

func TestWorkoutFeedbackService_AthleteHistory_Validaciones_400(t *testing.T) {
	cases := []struct {
		name  string
		query workoutfeedback.HistoryQuery
	}{
		{"group sin team", workoutfeedback.HistoryQuery{GroupID: histInt64Ptr(6)}},
		{"team_id cero", workoutfeedback.HistoryQuery{TeamID: histInt64Ptr(0)}},
		{"group_id cero", workoutfeedback.HistoryQuery{TeamID: histInt64Ptr(4), GroupID: histInt64Ptr(0)}},
		{"exercise_id cero", workoutfeedback.HistoryQuery{ExerciseID: histInt64Ptr(0)}},
		{"set_number negativo", workoutfeedback.HistoryQuery{SetNumber: histIntPtr(-1)}},
		{"solo date_from", workoutfeedback.HistoryQuery{DateFrom: histStrPtr("2026-01-01")}},
		{"solo date_to", workoutfeedback.HistoryQuery{DateTo: histStrPtr("2026-01-31")}},
		{"from mayor a to", workoutfeedback.HistoryQuery{DateFrom: histStrPtr("2026-02-01"), DateTo: histStrPtr("2026-01-01")}},
		{"formato de fecha invalido", workoutfeedback.HistoryQuery{DateFrom: histStrPtr("01/02/2026"), DateTo: histStrPtr("2026-01-03")}},
		{"page cero", workoutfeedback.HistoryQuery{Page: histIntPtr(0)}},
		{"page_size cero", workoutfeedback.HistoryQuery{PageSize: histIntPtr(0)}},
		{"page_size sobre tope", workoutfeedback.HistoryQuery{PageSize: histIntPtr(101)}},
		{"sort fuera de whitelist", workoutfeedback.HistoryQuery{Sort: "foo"}},
		{"order fuera de whitelist", workoutfeedback.HistoryQuery{Order: "ASCENDING"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := &historyDAOCalls{}
			svc := NewWorkoutFeedbackService(newHistoryMock(calls, nil))

			_, err := svc.AthleteHistory(nil, 7, 7, tc.query)

			require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
			assert.False(t, calls.searchCalled, "la validación debe cortar antes de consultar el DAO")
		})
	}
}

func TestWorkoutFeedbackService_AthleteHistory_FechasIguales_UnDia_OK(t *testing.T) {
	calls := &historyDAOCalls{}
	svc := NewWorkoutFeedbackService(newHistoryMock(calls, nil))

	_, err := svc.AthleteHistory(nil, 7, 7, workoutfeedback.HistoryQuery{
		DateFrom: histStrPtr("2026-01-15"),
		DateTo:   histStrPtr("2026-01-15"),
	})

	require.NoError(t, err)
	require.NotNil(t, calls.searchFilters.DateFrom)
	require.NotNil(t, calls.searchFilters.DateTo)
	assert.True(t, calls.searchFilters.DateFrom.Equal(*calls.searchFilters.DateTo))
}

func TestWorkoutFeedbackService_AdministeredHistory_TargetDistinto_403(t *testing.T) {
	calls := &historyDAOCalls{}
	svc := NewWorkoutFeedbackService(newHistoryMock(calls, nil))

	_, err := svc.AdministeredHistory(nil, 7, 99, workoutfeedback.HistoryQuery{TeamID: histInt64Ptr(4)})

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
	assert.False(t, calls.searchCalled)
	assert.False(t, calls.teamExists, "ni siquiera debe chequear el equipo si el target no es el caller")
}

func TestWorkoutFeedbackService_AdministeredHistory_SinTeam_400(t *testing.T) {
	calls := &historyDAOCalls{}
	svc := NewWorkoutFeedbackService(newHistoryMock(calls, nil))

	_, err := svc.AdministeredHistory(nil, 7, 7, workoutfeedback.HistoryQuery{})

	require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
	assert.False(t, calls.searchCalled)
	assert.False(t, calls.teamExists)
}

func TestWorkoutFeedbackService_AdministeredHistory_TeamInexistente_404(t *testing.T) {
	calls := &historyDAOCalls{}
	mock := newHistoryMock(calls, nil)
	mock.teamExistsFn = func(ctx *gin.Context, teamID int64) (bool, error) {
		calls.teamExists = true
		return false, nil
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.AdministeredHistory(nil, 7, 7, workoutfeedback.HistoryQuery{TeamID: histInt64Ptr(4)})

	require.ErrorIs(t, err, ErrTeamNotFound)
	assert.False(t, calls.searchCalled)
}

func TestWorkoutFeedbackService_AdministeredHistory_NoOwner_403(t *testing.T) {
	calls := &historyDAOCalls{}
	mock := newHistoryMock(calls, nil)
	mock.isTeamOwnerFn = func(ctx *gin.Context, teamID, userID int64) (bool, error) {
		calls.isTeamOwner = true
		return false, nil
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.AdministeredHistory(nil, 7, 7, workoutfeedback.HistoryQuery{TeamID: histInt64Ptr(4)})

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
	assert.False(t, calls.searchCalled)
}

func TestWorkoutFeedbackService_AdministeredHistory_OK_AthleteFilterSegundoNivel(t *testing.T) {
	calls := &historyDAOCalls{}
	svc := NewWorkoutFeedbackService(newHistoryMock(calls, []dbs.WorkoutFeedbackHistoryRow{
		{ID: 1, AthleteUserID: 9, SessionDate: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), SetNumber: 1, ExerciseID: 3},
	}))

	athleteFilter := int64(9)
	resp, err := svc.AdministeredHistory(nil, 7, 7, workoutfeedback.HistoryQuery{
		TeamID:        histInt64Ptr(4),
		AthleteUserID: &athleteFilter,
		Sort:          "exercise_name",
		Order:         "desc",
	})

	require.NoError(t, err)
	require.True(t, calls.searchCalled)
	f := calls.searchFilters
	require.NotNil(t, f.TeamID)
	assert.Equal(t, int64(4), *f.TeamID)
	require.NotNil(t, f.AthleteFilterUserID)
	assert.Equal(t, int64(9), *f.AthleteFilterUserID)
	assert.Equal(t, "exercise_name", calls.sortCol)
	assert.Equal(t, "desc", calls.order)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "Atleta9", resp.Items[0].AthleteName)
	// pools passthrough: el service no los recorta por el filtro de 2do nivel
	assert.Equal(t, []dbs.IDName{{ID: 7, Name: "Juan"}}, resp.AvailableAthletes)
	assert.Equal(t, []dbs.IDName{{ID: 3, Name: "Sentadilla"}}, resp.AvailableExercises)
}

func TestWorkoutFeedbackService_AdministeredHistory_DAOError_Propaga(t *testing.T) {
	calls := &historyDAOCalls{}
	mock := newHistoryMock(calls, nil)
	mock.historySearchFn = func(ctx *gin.Context, filters daos.WorkoutFeedbackHistoryFilters, sortCol, order string, limit, offset int) ([]dbs.WorkoutFeedbackHistoryRow, error) {
		return nil, errors.New("db caida")
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.AdministeredHistory(nil, 7, 7, workoutfeedback.HistoryQuery{TeamID: histInt64Ptr(4)})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db caida")
}

func histInt64Ptr(v int64) *int64 {
	return &v
}

func histIntPtr(v int) *int {
	return &v
}

func histStrPtr(v string) *string {
	return &v
}
