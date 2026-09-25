package daos

import (
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewWorkoutFeedbackDao(t *testing.T) {
	dao := NewWorkoutFeedbackDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestWorkoutFeedbackDao_ImplementsInterface(t *testing.T) {
	dao := NewWorkoutFeedbackDao(&gorm.DB{})
	var iface WorkoutFeedbackDAOInterface = dao
	_ = iface
}

// testFeedback crea y guarda un feedback válido directo por GORM (bypass del DAO).
// MediaURLs siempre seteada (nil → Null) porque un pgtype.TextArray undefined
// fallaría al codificar.
func testFeedback(t *testing.T, db *gorm.DB, athleteID, ownerID, sessionID, exerciseID int64, teamID *int64, setNumber int) *dbs.WorkoutFeedback {
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/a.jpg"}))
	feedback := &dbs.WorkoutFeedback{
		TeamID:              teamID,
		AssignedSessionID:   sessionID,
		AssignedExerciseID:  exerciseID,
		AthleteUserID:       athleteID,
		FeedbackOwnerUserID: ownerID,
		ReportSource:        "corredor",
		SessionDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		SetNumber:           setNumber,
		MediaURLs:           media,
	}
	require.NoError(t, db.Create(feedback).Error)
	return feedback
}

func TestWorkoutFeedbackDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	feedback := &dbs.WorkoutFeedback{
		AssignedSessionID:   1,
		AssignedExerciseID:  1,
		AthleteUserID:       1,
		FeedbackOwnerUserID: 1,
		ReportSource:        "corredor",
		SessionDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/a.jpg"}))
	feedback.MediaURLs = media

	err := dao.Create(nil, feedback)

	require.NoError(t, err)
	assert.NotZero(t, feedback.ID)
}

func TestWorkoutFeedbackDao_Create_DuplicateActiveSet(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))

	base := &dbs.WorkoutFeedback{
		AssignedSessionID:   1,
		AssignedExerciseID:  1,
		AthleteUserID:       1,
		FeedbackOwnerUserID: 1,
		ReportSource:        "corredor",
		SessionDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		MediaURLs:           media,
	}
	require.NoError(t, dao.Create(nil, base))

	// ID se reinicia a 0: el duplicado debe chocar contra el índice único parcial
	// (mismo set activo), no contra la primary key.
	dup := *base
	dup.ID = 0
	err := dao.Create(nil, &dup)

	require.ErrorIs(t, err, ErrWorkoutFeedbackDuplicate)
}

func TestWorkoutFeedbackDao_Create_AfterSoftDeleteAllowed(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))

	base := &dbs.WorkoutFeedback{
		AssignedSessionID:   1,
		AssignedExerciseID:  1,
		AthleteUserID:       1,
		FeedbackOwnerUserID: 1,
		ReportSource:        "corredor",
		SessionDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		MediaURLs:           media,
	}
	require.NoError(t, dao.Create(nil, base))
	require.NoError(t, dao.SoftDelete(nil, base.ID))

	// El set original queda soft-deleteado (deleted_at NOT NULL) → el índice único
	// parcial (WHERE deleted_at IS NULL) ya no lo ocupa y recrear el set es válido.
	// ID a 0: el insert debe generar una primary key nueva, no tocar la original.
	again := *base
	again.ID = 0
	err := dao.Create(nil, &again)

	require.NoError(t, err)
	assert.NotZero(t, again.ID)
}

func TestWorkoutFeedbackDao_MediaURLs_RoundTrip(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{"https://media.foo/a.jpg", "https://media.foo/b.jpg"}))
	feedback := &dbs.WorkoutFeedback{
		AssignedSessionID:   1,
		AssignedExerciseID:  1,
		AthleteUserID:       1,
		FeedbackOwnerUserID: 1,
		ReportSource:        "corredor",
		SessionDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		MediaURLs:           media,
	}
	require.NoError(t, dao.Create(nil, feedback))

	got, err := dao.GetByID(nil, feedback.ID)

	require.NoError(t, err)
	var urls []string
	for _, e := range got.MediaURLs.Elements {
		if e.Status == pgtype.Present {
			urls = append(urls, e.String)
		}
	}
	assert.Equal(t, []string{"https://media.foo/a.jpg", "https://media.foo/b.jpg"}, urls)
}

func TestWorkoutFeedbackDao_GetByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	got, err := dao.GetByID(nil, feedback.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, feedback.ID, got.ID)
	assert.Equal(t, int64(1), got.AthleteUserID)
}

func TestWorkoutFeedbackDao_GetByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	_, err := dao.GetByID(nil, 999999)

	require.ErrorIs(t, err, ErrWorkoutFeedbackNotFound)
}

func TestWorkoutFeedbackDao_GetByID_SoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	require.NoError(t, dao.SoftDelete(nil, feedback.ID))

	_, err := dao.GetByID(nil, feedback.ID)

	require.ErrorIs(t, err, ErrWorkoutFeedbackNotFound)
}

func TestWorkoutFeedbackDao_Update_Partial(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	weight := 80.5
	reps := 12
	updated, err := dao.Update(nil, feedback.ID, map[string]interface{}{"weight_kg": weight, "reps": reps})

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 80.5, *updated.WeightKg)
	assert.Equal(t, 12, *updated.Reps)
	// Campos no actualizados se conservan
	assert.Equal(t, feedback.ReportSource, updated.ReportSource)
}

func TestWorkoutFeedbackDao_Update_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	_, err := dao.Update(nil, 999999, map[string]interface{}{"weight_kg": 80.5})

	require.ErrorIs(t, err, ErrWorkoutFeedbackNotFound)
}

func TestWorkoutFeedbackDao_SoftDelete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	err := dao.SoftDelete(nil, feedback.ID)

	require.NoError(t, err)
}

func TestWorkoutFeedbackDao_SoftDelete_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	err := dao.SoftDelete(nil, 999999)

	require.ErrorIs(t, err, ErrWorkoutFeedbackNotFound)
}

func TestWorkoutFeedbackDao_Search_SelfScope(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	testFeedback(t, db, 1, 1, 1, 1, nil, 0) // atleta y owner = 1
	testFeedback(t, db, 2, 2, 1, 1, nil, 0) // atleta y owner = 2
	testFeedback(t, db, 3, 1, 1, 1, nil, 0) // atleta 3, reportado por 1

	self := int64(1)
	feedbacks, err := dao.Search(nil, WorkoutFeedbackSearchFilters{SelfUserID: &self})

	require.NoError(t, err)
	require.Len(t, feedbacks, 2) // id de atleta 1 + reportado por owner 1
}

func TestWorkoutFeedbackDao_Search_FilterByTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	owner := persistUser(db, "fb-team-owner@test.com", "31000001")
	team := testTeam(db, "fb_team_search", owner.ID)
	teamID := team.ID
	other := int64(999)

	testFeedback(t, db, 1, 1, 1, 1, &teamID, 0)
	testFeedback(t, db, 2, 2, 1, 1, &other, 0)

	feedbacks, err := dao.Search(nil, WorkoutFeedbackSearchFilters{TeamID: &teamID})

	require.NoError(t, err)
	require.Len(t, feedbacks, 1)
	assert.Equal(t, teamID, *feedbacks[0].TeamID)
}

func TestWorkoutFeedbackDao_Search_FilterByAthlete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	testFeedback(t, db, 2, 2, 1, 1, nil, 0)

	athlete := int64(2)
	feedbacks, err := dao.Search(nil, WorkoutFeedbackSearchFilters{AthleteUserID: &athlete})

	require.NoError(t, err)
	require.Len(t, feedbacks, 1)
	assert.Equal(t, int64(2), feedbacks[0].AthleteUserID)
}

func TestWorkoutFeedbackDao_Search_CombinedFilters(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	testFeedback(t, db, 1, 1, 1, 1, nil, 0) // session 1 exercise 1, set 0
	testFeedback(t, db, 1, 1, 1, 1, nil, 1) // session 1 exercise 1, set 1
	testFeedback(t, db, 1, 1, 1, 2, nil, 0) // session 1 exercise 2
	testFeedback(t, db, 1, 1, 2, 1, nil, 0) // session 2 exercise 1

	session := int64(1)
	exercise := int64(1)
	set := 0
	feedbacks, err := dao.Search(nil, WorkoutFeedbackSearchFilters{
		AthleteUserID:      &[]int64{1}[0],
		AssignedSessionID:  &session,
		AssignedExerciseID: &exercise,
	})

	require.NoError(t, err)
	require.Len(t, feedbacks, 2)
	for _, fb := range feedbacks {
		assert.Equal(t, int64(1), fb.AssignedSessionID)
		assert.Equal(t, int64(1), fb.AssignedExerciseID)
	}
	_ = set
}

func TestWorkoutFeedbackDao_Search_DateRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	jan := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)

	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))
	base := dbs.WorkoutFeedback{
		AssignedSessionID:   1,
		AssignedExerciseID:  1,
		AthleteUserID:       1,
		FeedbackOwnerUserID: 1,
		ReportSource:        "corredor",
		MediaURLs:           media,
	}
	for date, sessionID := range map[time.Time]int64{jan: 1, feb: 2} {
		fb := base
		fb.SessionDate = date
		fb.AssignedSessionID = sessionID
		require.NoError(t, db.Create(&fb).Error)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	feedbacks, err := dao.Search(nil, WorkoutFeedbackSearchFilters{SessionDateFrom: &from, SessionDateTo: &to})

	require.NoError(t, err)
	require.Len(t, feedbacks, 1)
	assert.Equal(t, int64(1), feedbacks[0].AssignedSessionID)
}

func TestWorkoutFeedbackDao_Search_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	require.NoError(t, dao.SoftDelete(nil, feedback.ID))

	self := int64(1)
	feedbacks, err := dao.Search(nil, WorkoutFeedbackSearchFilters{SelfUserID: &self})

	require.NoError(t, err)
	assert.Empty(t, feedbacks)
}

func TestWorkoutFeedbackDao_RPE_CheckDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)

	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))
	rpe := int16(15)
	feedback := &dbs.WorkoutFeedback{
		AssignedSessionID:   1,
		AssignedExerciseID:  1,
		AthleteUserID:       1,
		FeedbackOwnerUserID: 1,
		ReportSource:        "corredor",
		SessionDate:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		MediaURLs:           media,
		RPE:                 &rpe,
	}

	err := dao.Create(nil, feedback)

	require.Error(t, err) // el CHECK de la DB rechaza rpe fuera de 1..10
}

func TestWorkoutFeedbackDao_CreatePoints_BulkAndList(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.61, Longitude: -58.41, RecordedAt: time.Date(2026, 9, 24, 14, 0, 1, 0, time.UTC)},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 2, Latitude: -34.62, Longitude: -58.42, RecordedAt: time.Date(2026, 9, 24, 14, 0, 2, 0, time.UTC)},
	}

	created, err := dao.BulkCreatePoints(nil, feedback.ID, points)

	require.NoError(t, err)
	assert.Equal(t, int64(3), created)

	got, err := dao.GetPointsByFeedback(nil, feedback.ID)

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, 0, got[0].Order)
	assert.Equal(t, 1, got[1].Order)
	assert.Equal(t, 2, got[2].Order)
	assert.Equal(t, feedback.ID, got[0].FeedbackID)
}

func TestWorkoutFeedbackDao_CreatePoints_IdempotentRetry(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.61, Longitude: -58.41, RecordedAt: time.Date(2026, 9, 24, 14, 0, 1, 0, time.UTC)},
	}

	created1, err := dao.BulkCreatePoints(nil, feedback.ID, points)
	require.NoError(t, err)
	assert.Equal(t, int64(2), created1)

	// Reintento del MISMO bulk: el índice único uq_feedback_point_order +
	// ON CONFLICT DO NOTHING deja todo como skipped, sin error ni duplicados.
	created2, err := dao.BulkCreatePoints(nil, feedback.ID, points)

	require.NoError(t, err)
	assert.Equal(t, int64(0), created2)

	// Mezcla: un punto nuevo (order 2) y un reintento (order 0) → solo 1 crea.
	points = append(points, dbs.WorkoutFeedbackPoint{
		FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 2, Latitude: -34.62, Longitude: -58.42, RecordedAt: time.Date(2026, 9, 24, 14, 0, 2, 0, time.UTC),
	})
	created3, err := dao.BulkCreatePoints(nil, feedback.ID, points)

	require.NoError(t, err)
	assert.Equal(t, int64(1), created3)

	got, err := dao.GetPointsByFeedback(nil, feedback.ID)
	require.NoError(t, err)
	require.Len(t, got, 3)
}

func TestWorkoutFeedbackDao_GetPointsByFeedback_NoPoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	got, err := dao.GetPointsByFeedback(nil, feedback.ID)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestWorkoutFeedbackDao_GetPointsByFeedback_OtherSetsIsolated(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedbackA := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	feedbackB := testFeedback(t, db, 1, 1, 1, 1, nil, 1)

	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedbackA.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)},
	}
	_, err := dao.BulkCreatePoints(nil, feedbackA.ID, points)
	require.NoError(t, err)

	gotB, err := dao.GetPointsByFeedback(nil, feedbackB.ID)

	require.NoError(t, err)
	assert.Empty(t, gotB)
}