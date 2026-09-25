package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
)

type mockWorkoutFeedbackDao struct {
	createFn            func(ctx *gin.Context, feedback *dbs.WorkoutFeedback) error
	getByIDFn           func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error)
	updateFn            func(ctx *gin.Context, id int64, updates map[string]interface{}) (*dbs.WorkoutFeedback, error)
	softDeleteFn        func(ctx *gin.Context, id int64) error
	searchFn            func(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error)
	teamExistsFn        func(ctx *gin.Context, teamID int64) (bool, error)
	isTeamOwnerFn       func(ctx *gin.Context, teamID, userID int64) (bool, error)
	userInTeamOwnedByFn func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error)
	bulkCreatePointsFn   func(ctx *gin.Context, feedbackID int64, points []dbs.WorkoutFeedbackPoint) (int64, error)
	getPointsByFeedbackFn func(ctx *gin.Context, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error)
	getBySessionFn       func(ctx *gin.Context, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error)
}

func (m *mockWorkoutFeedbackDao) Create(ctx *gin.Context, feedback *dbs.WorkoutFeedback) error {
	if m.createFn != nil {
		return m.createFn(ctx, feedback)
	}
	return nil
}

func (m *mockWorkoutFeedbackDao) GetByID(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackDao) Update(ctx *gin.Context, id int64, updates map[string]interface{}) (*dbs.WorkoutFeedback, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, updates)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func (m *mockWorkoutFeedbackDao) Search(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, filters)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackDao) TeamExists(ctx *gin.Context, teamID int64) (bool, error) {
	if m.teamExistsFn != nil {
		return m.teamExistsFn(ctx, teamID)
	}
	return false, nil
}

func (m *mockWorkoutFeedbackDao) IsTeamOwner(ctx *gin.Context, teamID, userID int64) (bool, error) {
	if m.isTeamOwnerFn != nil {
		return m.isTeamOwnerFn(ctx, teamID, userID)
	}
	return false, nil
}

func (m *mockWorkoutFeedbackDao) ExistsUserInTeamOwnedBy(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
	if m.userInTeamOwnedByFn != nil {
		return m.userInTeamOwnedByFn(ctx, targetUserID, ownerUserID)
	}
	return false, nil
}

func (m *mockWorkoutFeedbackDao) BulkCreatePoints(ctx *gin.Context, feedbackID int64, points []dbs.WorkoutFeedbackPoint) (int64, error) {
	if m.bulkCreatePointsFn != nil {
		return m.bulkCreatePointsFn(ctx, feedbackID, points)
	}
	return 0, nil
}

func (m *mockWorkoutFeedbackDao) GetPointsByFeedback(ctx *gin.Context, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error) {
	if m.getPointsByFeedbackFn != nil {
		return m.getPointsByFeedbackFn(ctx, feedbackID)
	}
	return nil, nil
}

func (m *mockWorkoutFeedbackDao) GetBySession(ctx *gin.Context, sessionInstanceID int64, athleteUserID *int64) ([]dbs.WorkoutFeedback, error) {
	if m.getBySessionFn != nil {
		return m.getBySessionFn(ctx, sessionInstanceID, athleteUserID)
	}
	return nil, nil
}

func validCreateRequest() workoutfeedback.CreateFeedbackRequest {
	return workoutfeedback.CreateFeedbackRequest{
		AssignedSessionID:  1,
		AssignedExerciseID: 2,
		ReportSource:       "corredor",
		SessionDate:        "2026-01-15",
		SetNumber:          1,
	}
}

func TestWorkoutFeedbackService_Create_Self(t *testing.T) {
	var captured *dbs.WorkoutFeedback
	mock := &mockWorkoutFeedbackDao{
		createFn: func(ctx *gin.Context, feedback *dbs.WorkoutFeedback) error {
			captured = feedback
			feedback.ID = 10
			return nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	feedback, err := svc.Create(nil, 7, validCreateRequest())

	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, int64(7), feedback.AthleteUserID)              // atleta = auth por default
	assert.Equal(t, int64(7), feedback.FeedbackOwnerUserID)        // reportante = auth
	assert.Equal(t, "corredor", feedback.ReportSource)
	assert.Equal(t, int64(10), feedback.ID)
}

func TestWorkoutFeedbackService_Create_TrainerForAthlete_Owned(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		userInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
			assert.Equal(t, int64(5), targetUserID)
			assert.Equal(t, int64(7), ownerUserID)
			return true, nil
		},
		createFn: func(ctx *gin.Context, feedback *dbs.WorkoutFeedback) error {
			return nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	req := validCreateRequest()
	athlete := int64(5)
	req.AthleteUserID = &athlete

	feedback, err := svc.Create(nil, 7, req)

	require.NoError(t, err)
	assert.Equal(t, int64(5), feedback.AthleteUserID)
	assert.Equal(t, int64(7), feedback.FeedbackOwnerUserID)
}

func TestWorkoutFeedbackService_Create_TrainerForAthlete_NotOwned(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		userInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) {
			return false, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	req := validCreateRequest()
	athlete := int64(5)
	req.AthleteUserID = &athlete

	_, err := svc.Create(nil, 7, req)

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_Create_Duplicate(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		createFn: func(ctx *gin.Context, feedback *dbs.WorkoutFeedback) error {
			return daos.ErrWorkoutFeedbackDuplicate
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Create(nil, 7, validCreateRequest())

	require.ErrorIs(t, err, daos.ErrWorkoutFeedbackDuplicate)
}

func TestWorkoutFeedbackService_Create_Validation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*workoutfeedback.CreateFeedbackRequest)
	}{
		{
			name: "session of assignment 0",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.AssignedSessionID = 0
			},
		},
		{
			name: "exercise of assignment 0",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.AssignedExerciseID = 0
			},
		},
		{
			name: "empty report source",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.ReportSource = ""
			},
		},
		{
			name: "invalid session date format",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.SessionDate = "15-01-2026"
			},
		},
		{
			name: "negative set number",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.SetNumber = -1
			},
		},
		{
			name: "rpe out of range",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.RPE = fbInt16Ptr(0)
			},
		},
		{
			name: "rpe above range",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.RPE = fbInt16Ptr(11)
			},
		},
		{
			name: "negative duration",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.DurationMs = fbInt64Ptr(-5)
			},
		},
		{
			name: "negative weight",
			mutate: func(r *workoutfeedback.CreateFeedbackRequest) {
				r.WeightKg = fbFloat64Ptr(-1)
			},
		},
	}

	svc := NewWorkoutFeedbackService(&mockWorkoutFeedbackDao{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := validCreateRequest()
			tc.mutate(&req)
			_, err := svc.Create(nil, 7, req)
			require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
		})
	}
}

func TestWorkoutFeedbackService_Create_EndedBeforeStarted(t *testing.T) {
	svc := NewWorkoutFeedbackService(&mockWorkoutFeedbackDao{})
	req := validCreateRequest()
	req.StartedAt = fbTimePtr(past)
	req.EndedAt = fbTimePtr(earlier)

	_, err := svc.Create(nil, 7, req)

	require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
}

func TestWorkoutFeedbackService_GetByID_AsAthlete(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 7, FeedbackOwnerUserID: 8}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
			assert.Equal(t, int64(1), id)
			return fb, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	got, err := svc.GetByID(nil, 7, 1)

	require.NoError(t, err)
	assert.Equal(t, fb, got)
}

func TestWorkoutFeedbackService_GetByID_AsReporter(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 7}
	mock := &mockWorkoutFeedbackDao{getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil }}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.GetByID(nil, 7, 1)

	require.NoError(t, err)
}

func TestWorkoutFeedbackService_GetByID_AsTeamOwner(t *testing.T) {
	teamID := int64(3)
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 9, TeamID: &teamID}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		isTeamOwnerFn: func(ctx *gin.Context, teamID, userID int64) (bool, error) {
			assert.Equal(t, int64(3), teamID)
			assert.Equal(t, int64(7), userID)
			return true, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.GetByID(nil, 7, 1)

	require.NoError(t, err)
}

func TestWorkoutFeedbackService_GetByID_Forbidden(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 9}
	mock := &mockWorkoutFeedbackDao{getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil }}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.GetByID(nil, 7, 1)

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_GetByID_NotFound(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
			return nil, daos.ErrWorkoutFeedbackNotFound
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.GetByID(nil, 7, 1)

	require.ErrorIs(t, err, daos.ErrWorkoutFeedbackNotFound)
}

func TestWorkoutFeedbackService_Search_SelfScope(t *testing.T) {
	var captured daos.WorkoutFeedbackSearchFilters
	mock := &mockWorkoutFeedbackDao{
		searchFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
			captured = filters
			return nil, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{})

	require.NoError(t, err)
	require.NotNil(t, captured.SelfUserID)
	assert.Equal(t, int64(7), *captured.SelfUserID)
}

func TestWorkoutFeedbackService_Search_ScopeSelfWithExtraFilters(t *testing.T) {
	var captured daos.WorkoutFeedbackSearchFilters
	sessionID := int64(10)
	exerciseID := int64(20)
	ownerID := int64(30)
	mock := &mockWorkoutFeedbackDao{
		searchFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
			captured = filters
			return nil, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{
		AssignedSessionID:   &sessionID,
		AssignedExerciseID:  &exerciseID,
		FeedbackOwnerUserID: &ownerID,
	})

	require.NoError(t, err)
	assert.Equal(t, int64(7), *captured.SelfUserID)
	assert.Equal(t, int64(10), *captured.AssignedSessionID)
	assert.Equal(t, int64(20), *captured.AssignedExerciseID)
	assert.Equal(t, int64(30), *captured.FeedbackOwnerUserID)
}

func TestWorkoutFeedbackService_Search_TeamScope(t *testing.T) {
	var captured daos.WorkoutFeedbackSearchFilters
	teamID := int64(3)
	mock := &mockWorkoutFeedbackDao{
		teamExistsFn: func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
		isTeamOwnerFn: func(ctx *gin.Context, id, user int64) (bool, error) { return true, nil },
		searchFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
			captured = filters
			return nil, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{TeamID: &teamID})

	require.NoError(t, err)
	require.NotNil(t, captured.TeamID)
	assert.Equal(t, int64(3), *captured.TeamID)
	assert.Nil(t, captured.SelfUserID)
}

func TestWorkoutFeedbackService_Search_TeamNotFound(t *testing.T) {
	teamID := int64(3)
	mock := &mockWorkoutFeedbackDao{
		teamExistsFn: func(ctx *gin.Context, id int64) (bool, error) { return false, nil },
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{TeamID: &teamID})

	require.ErrorIs(t, err, ErrTeamNotFound)
}

func TestWorkoutFeedbackService_Search_TeamNotOwner(t *testing.T) {
	teamID := int64(3)
	mock := &mockWorkoutFeedbackDao{
		teamExistsFn:  func(ctx *gin.Context, id int64) (bool, error) { return true, nil },
		isTeamOwnerFn: func(ctx *gin.Context, id, user int64) (bool, error) { return false, nil },
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{TeamID: &teamID})

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_Search_AthleteScope(t *testing.T) {
	var captured daos.WorkoutFeedbackSearchFilters
	athleteID := int64(5)
	mock := &mockWorkoutFeedbackDao{
		userInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) { return true, nil },
		searchFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
			captured = filters
			return nil, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{AthleteUserID: &athleteID})

	require.NoError(t, err)
	require.NotNil(t, captured.AthleteUserID)
	assert.Equal(t, int64(5), *captured.AthleteUserID)
}

func TestWorkoutFeedbackService_Search_AthleteNotOwned(t *testing.T) {
	athleteID := int64(5)
	mock := &mockWorkoutFeedbackDao{
		userInTeamOwnedByFn: func(ctx *gin.Context, targetUserID, ownerUserID int64) (bool, error) { return false, nil },
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{AthleteUserID: &athleteID})

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_Search_AthleteIsSelf(t *testing.T) {
	var captured daos.WorkoutFeedbackSearchFilters
	athleteID := int64(7)
	mock := &mockWorkoutFeedbackDao{
		searchFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
			captured = filters
			return nil, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{AthleteUserID: &athleteID})

	require.NoError(t, err)
	require.NotNil(t, captured.AthleteUserID)
	assert.Equal(t, int64(7), *captured.AthleteUserID)
}

func TestWorkoutFeedbackService_Search_InvalidDates(t *testing.T) {
	svc := NewWorkoutFeedbackService(&mockWorkoutFeedbackDao{})
	badFrom := "not-a-date"

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{SessionDateFrom: &badFrom})

	require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
}

func TestWorkoutFeedbackService_Search_DateRangePassed(t *testing.T) {
	var captured daos.WorkoutFeedbackSearchFilters
	from := "2026-01-01"
	to := "2026-01-31"
	mock := &mockWorkoutFeedbackDao{
		searchFn: func(ctx *gin.Context, filters daos.WorkoutFeedbackSearchFilters) ([]dbs.WorkoutFeedback, error) {
			captured = filters
			return nil, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Search(nil, 7, workoutfeedback.SearchFilters{SessionDateFrom: &from, SessionDateTo: &to})

	require.NoError(t, err)
	require.NotNil(t, captured.SessionDateFrom)
	require.NotNil(t, captured.SessionDateTo)
	assert.Equal(t, "2026-01-01", captured.SessionDateFrom.Format("2006-01-02"))
	assert.Equal(t, "2026-01-31", captured.SessionDateTo.Format("2006-01-02"))
}

func TestWorkoutFeedbackService_Update_Partial(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 7, FeedbackOwnerUserID: 7}
	var applied map[string]interface{}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		updateFn: func(ctx *gin.Context, id int64, updates map[string]interface{}) (*dbs.WorkoutFeedback, error) {
			applied = updates
			updated := *fb
			return &updated, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	rpe := int16(8)
	req := workoutfeedback.UpdateFeedbackRequest{RPE: &rpe}
	_, err := svc.Update(nil, 7, 1, req)

	require.NoError(t, err)
	assert.Equal(t, int16(8), applied["rpe"])
	_, hasSession := applied["assigned_session_id"]
	assert.False(t, hasSession, "no debe editarse assigned_session_id si no viene")
}

func TestWorkoutFeedbackService_Update_NotFound(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
			return nil, daos.ErrWorkoutFeedbackNotFound
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Update(nil, 7, 1, workoutfeedback.UpdateFeedbackRequest{})

	require.ErrorIs(t, err, daos.ErrWorkoutFeedbackNotFound)
}

func TestWorkoutFeedbackService_Update_Forbidden(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 9}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Update(nil, 7, 1, workoutfeedback.UpdateFeedbackRequest{})

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_Update_Duplicate(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 7, FeedbackOwnerUserID: 7}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		updateFn: func(ctx *gin.Context, id int64, updates map[string]interface{}) (*dbs.WorkoutFeedback, error) {
			return nil, daos.ErrWorkoutFeedbackDuplicate
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.Update(nil, 7, 1, workoutfeedback.UpdateFeedbackRequest{SetNumber: fbIntPtr(5)})

	require.ErrorIs(t, err, daos.ErrWorkoutFeedbackDuplicate)
}

func TestWorkoutFeedbackService_SoftDelete_Reporter(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 7}
	var deleted bool
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			deleted = true
			return nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	err := svc.SoftDelete(nil, 7, 1)

	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestWorkoutFeedbackService_SoftDelete_Forbidden(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 9}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
	}
	svc := NewWorkoutFeedbackService(mock)

	err := svc.SoftDelete(nil, 7, 1)

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_SoftDelete_NotFound(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
			return nil, daos.ErrWorkoutFeedbackNotFound
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	err := svc.SoftDelete(nil, 7, 1)

	require.ErrorIs(t, err, daos.ErrWorkoutFeedbackNotFound)
}

func validPointsRequest() workoutfeedback.CreatePointsRequest {
	return workoutfeedback.CreatePointsRequest{
		Points: []workoutfeedback.PointInput{
			{Order: 0, SessionInstanceID: 3, ExerciseInstanceID: 4, Latitude: -34.6, Longitude: -58.4, RecordedAt: time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)},
			{Order: 1, SessionInstanceID: 3, ExerciseInstanceID: 4, Latitude: -34.61, Longitude: -58.41, RecordedAt: time.Date(2026, 9, 24, 14, 0, 1, 0, time.UTC)},
		},
	}
}

func TestWorkoutFeedbackService_CreatePoints_Success(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 7, FeedbackOwnerUserID: 7}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		bulkCreatePointsFn: func(ctx *gin.Context, feedbackID int64, points []dbs.WorkoutFeedbackPoint) (int64, error) {
			assert.Equal(t, int64(1), feedbackID)
			assert.Len(t, points, 2)
			assert.Equal(t, 0, points[0].Order)
			assert.Equal(t, -34.6, points[0].Latitude)
			assert.Equal(t, time.Date(2026, 9, 24, 14, 0, 1, 0, time.UTC), points[1].RecordedAt)
			return 2, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	result, err := svc.CreatePoints(nil, 7, 1, validPointsRequest())

	require.NoError(t, err)
	assert.Equal(t, 2, result.Created)
	assert.Equal(t, 0, result.Skipped)
}

func TestWorkoutFeedbackService_CreatePoints_RetryComputesSkipped(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 7, FeedbackOwnerUserID: 7}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		bulkCreatePointsFn: func(ctx *gin.Context, feedbackID int64, points []dbs.WorkoutFeedbackPoint) (int64, error) {
			// Reintento: la DB ya tenía los 2 posiciones → no inserta ninguna.
			return 0, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	result, err := svc.CreatePoints(nil, 7, 1, validPointsRequest())

	require.NoError(t, err)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 2, result.Skipped)
}

func TestWorkoutFeedbackService_CreatePoints_NotFound(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
			return nil, daos.ErrWorkoutFeedbackNotFound
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.CreatePoints(nil, 7, 1, validPointsRequest())

	require.ErrorIs(t, err, daos.ErrWorkoutFeedbackNotFound)
}

func TestWorkoutFeedbackService_CreatePoints_Forbidden(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 9}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.CreatePoints(nil, 7, 1, validPointsRequest())

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_CreatePoints_Validation(t *testing.T) {
	base := validPointsRequest()

	cases := []struct {
		name    string
		mutate  func(*workoutfeedback.CreatePointsRequest)
	}{
		{
			name: "empty points",
			mutate: func(r *workoutfeedback.CreatePointsRequest) {
				r.Points = []workoutfeedback.PointInput{}
			},
		},
		{
			name: "too many points",
			mutate: func(r *workoutfeedback.CreatePointsRequest) {
				points := make([]workoutfeedback.PointInput, maxPointsPerBulk+1)
				for i := range points {
					points[i] = base.Points[0]
					points[i].Order = i
				}
				r.Points = points
			},
		},
		{
			name: "negative order",
			mutate: func(r *workoutfeedback.CreatePointsRequest) {
				r.Points[1].Order = -1
			},
		},
		{
			name: "zero session instance",
			mutate: func(r *workoutfeedback.CreatePointsRequest) {
				r.Points[1].SessionInstanceID = 0
			},
		},
		{
			name: "zero exercise instance",
			mutate: func(r *workoutfeedback.CreatePointsRequest) {
				r.Points[1].ExerciseInstanceID = 0
			},
		},
		{
			name: "latitude out of range",
			mutate: func(r *workoutfeedback.CreatePointsRequest) {
				r.Points[1].Latitude = 91
			},
		},
		{
			name: "longitude out of range",
			mutate: func(r *workoutfeedback.CreatePointsRequest) {
				r.Points[1].Longitude = -181
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 7, FeedbackOwnerUserID: 7}
			mock := &mockWorkoutFeedbackDao{
				getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
			}
			svc := NewWorkoutFeedbackService(mock)

			req := validPointsRequest()
			tc.mutate(&req)
			_, err := svc.CreatePoints(nil, 7, 1, req)

			require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
		})
	}
}

func TestWorkoutFeedbackService_GetPoints_Success(t *testing.T) {
	teamID := int64(10)
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 9, TeamID: &teamID}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		isTeamOwnerFn: func(ctx *gin.Context, teamID, userID int64) (bool, error) {
			assert.Equal(t, int64(10), teamID)
			assert.Equal(t, int64(7), userID)
			return true, nil
		},
		getPointsByFeedbackFn: func(ctx *gin.Context, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error) {
			assert.Equal(t, int64(1), feedbackID)
			return []dbs.WorkoutFeedbackPoint{
				{ID: 11, FeedbackID: 1, Order: 0, Latitude: -34.6, Longitude: -58.4},
			}, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	points, err := svc.GetPoints(nil, 7, 1)

	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.Equal(t, -34.6, points[0].Latitude)
}

func TestWorkoutFeedbackService_GetPoints_EmptyIsArrayNotNil(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 7, FeedbackOwnerUserID: 7}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
		getPointsByFeedbackFn: func(ctx *gin.Context, feedbackID int64) ([]dbs.WorkoutFeedbackPoint, error) {
			return nil, nil
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	points, err := svc.GetPoints(nil, 7, 1)

	require.NoError(t, err)
	assert.NotNil(t, points)
	assert.Empty(t, points)
}

func TestWorkoutFeedbackService_GetPoints_Forbidden(t *testing.T) {
	fb := &dbs.WorkoutFeedback{ID: 1, AthleteUserID: 8, FeedbackOwnerUserID: 9}
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) { return fb, nil },
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.GetPoints(nil, 7, 1)

	require.ErrorIs(t, err, ErrWorkoutFeedbackForbidden)
}

func TestWorkoutFeedbackService_GetPoints_NotFound(t *testing.T) {
	mock := &mockWorkoutFeedbackDao{
		getByIDFn: func(ctx *gin.Context, id int64) (*dbs.WorkoutFeedback, error) {
			return nil, daos.ErrWorkoutFeedbackNotFound
		},
	}
	svc := NewWorkoutFeedbackService(mock)

	_, err := svc.GetPoints(nil, 7, 1)

	require.ErrorIs(t, err, daos.ErrWorkoutFeedbackNotFound)
}

// Helpers de punteros.
func fbInt16Ptr(v int16) *int16    { return &v }
func fbIntPtr(v int) *int          { return &v }
func fbInt64Ptr(v int64) *int64    { return &v }
func fbFloat64Ptr(v float64) *float64 { return &v }

var (
	past    = time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	earlier = time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
)

func fbTimePtr(v time.Time) *time.Time { return &v }