package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/domains/dbs"
)

func cmpInt(v int) *int              { return &v }
func cmpI64(v int64) *int64          { return &v }
func cmpF64(v float64) *float64      { return &v }
func cmpStr(v string) *string        { return &v }
func cmpI16(v int16) *int16          { return &v }
func cmpTime(v time.Time) *time.Time { return &v }

const cmpAthlete = int64(19)

// cmpExercise arma un ejercicio planificado de 1 serie de "liviano".
func cmpExercise(id int64, minutes, distance *int) daos.ComparisonPlannedExercise {
	return daos.ComparisonPlannedExercise{
		LinkID: id + 1000, SessionInstanceID: 455, ExerciseInstanceID: id,
		Role: "main", RepeatCount: 1, Name: "caminar", Kind: "liviano",
		Minutes: minutes, DistanceM: distance,
	}
}

// cmpFeedback arma una fila de feedback del atleta para un ejercicio.
func cmpFeedback(id, exerciseID int64, setNumber int, status string, durationMs *int64, distance *float64) dbs.WorkoutFeedback {
	return dbs.WorkoutFeedback{
		ID: id, AssignedSessionID: 455, AssignedExerciseID: exerciseID,
		AthleteUserID: cmpAthlete, FeedbackOwnerUserID: cmpAthlete,
		SetNumber: setNumber, CompletionStatus: &status,
		DurationMs: durationMs, DistanceMeters: distance,
		SessionDate: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 10, 12, 13, 0, 0, 0, time.UTC),
	}
}

func TestCompareSession_SeededRunGives50Percent(t *testing.T) {
	// El caso de la HU: caminata completa, la segunda al 50%, la tercera salteada.
	exercises := []daos.ComparisonPlannedExercise{
		cmpExercise(1, cmpInt(10), nil),
		cmpExercise(2, cmpInt(10), nil),
		cmpExercise(3, cmpInt(10), nil),
	}
	feedback := []dbs.WorkoutFeedback{
		cmpFeedback(181, 1, 1, "completed", cmpI64(602340), cmpF64(835.42)),
		cmpFeedback(182, 2, 1, "completed", cmpI64(300000), cmpF64(418.07)),
		cmpFeedback(183, 3, 1, "skipped", nil, nil),
	}

	result := compareSession(exercises, feedback, cmpAthlete)

	require.Len(t, result.exercises, 3)
	assert.Equal(t, 100.0, result.exercises[0].CompliancePct)
	assert.Equal(t, 50.0, result.exercises[1].CompliancePct)
	assert.Equal(t, 0.0, result.exercises[2].CompliancePct)
	assert.Equal(t, 50.0, result.compliancePct)
	assert.Equal(t, int64(-300000), *result.exercises[1].Sets[0].DeviationDurationMs)
	assert.Equal(t, int64(2340), *result.exercises[0].Sets[0].DeviationDurationMs)
	assert.Equal(t, comparison.SetStatusSkipped, result.exercises[2].Sets[0].Status)
	assert.Nil(t, result.exercises[2].Sets[0].DeviationDurationMs)

	assert.Equal(t, 3, result.totals.PlannedSets)
	assert.Equal(t, 2, result.totals.CompletedSets)
	assert.Equal(t, 1, result.totals.SkippedSets)
	assert.Equal(t, 0, result.totals.UnregisteredSets)
	assert.Equal(t, int64(1800000), *result.totals.PlannedDurationMs)
	assert.Equal(t, int64(902340), result.totals.ActualDurationMs)
	assert.Nil(t, result.totals.PlannedDistanceM)
	assert.Equal(t, 1253.5, result.totals.ActualDistanceM)
	assert.Equal(t, []string{"liviano"}, result.kinds)
}

func TestCompareSet_Targets(t *testing.T) {
	tests := []struct {
		name          string
		exercise      daos.ComparisonPlannedExercise
		feedback      *dbs.WorkoutFeedback
		wantStatus    string
		wantPct       float64
		wantDevMs     *int64
		wantDevMeters *float64
	}{
		{
			name:       "tiempo parcial",
			exercise:   cmpExercise(1, cmpInt(10), nil),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "completed", cmpI64(420000), nil)),
			wantStatus: comparison.SetStatusCompleted, wantPct: 70, wantDevMs: cmpI64(-180000),
		},
		{
			name:       "tiempo de más se topea en 100",
			exercise:   cmpExercise(1, cmpInt(10), nil),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "completed", cmpI64(720000), nil)),
			wantStatus: comparison.SetStatusCompleted, wantPct: 100, wantDevMs: cmpI64(120000),
		},
		{
			name:       "distancia sin minutos",
			exercise:   cmpExercise(1, nil, cmpInt(1000)),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "completed", cmpI64(300000), cmpF64(750))),
			wantStatus: comparison.SetStatusCompleted, wantPct: 75, wantDevMeters: cmpF64(-250),
		},
		{
			name:       "tiempo y distancia: manda el tiempo",
			exercise:   cmpExercise(1, cmpInt(10), cmpInt(1000)),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "completed", cmpI64(300000), cmpF64(1000))),
			wantStatus: comparison.SetStatusCompleted, wantPct: 50, wantDevMs: cmpI64(-300000), wantDevMeters: cmpF64(0),
		},
		{
			name:       "sin objetivo completada vale 100",
			exercise:   cmpExercise(1, nil, nil),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "completed", cmpI64(5000), nil)),
			wantStatus: comparison.SetStatusCompleted, wantPct: 100,
		},
		{
			name:       "completada sin duración con objetivo de tiempo",
			exercise:   cmpExercise(1, cmpInt(10), nil),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "completed", nil, nil)),
			wantStatus: comparison.SetStatusCompleted, wantPct: 0,
		},
		{
			name:       "completada sin distancia con objetivo de distancia",
			exercise:   cmpExercise(1, nil, cmpInt(1000)),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "completed", cmpI64(60000), nil)),
			wantStatus: comparison.SetStatusCompleted, wantPct: 0,
		},
		{
			name:       "salteada vale 0",
			exercise:   cmpExercise(1, cmpInt(10), nil),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "skipped", nil, nil)),
			wantStatus: comparison.SetStatusSkipped, wantPct: 0,
		},
		{
			name:       "sin registro vale 0",
			exercise:   cmpExercise(1, cmpInt(10), nil),
			feedback:   nil,
			wantStatus: comparison.SetStatusUnregistered, wantPct: 0,
		},
		{
			name:       "estado desconocido cuenta como sin registro",
			exercise:   cmpExercise(1, cmpInt(10), nil),
			feedback:   ptrFeedback(cmpFeedback(1, 1, 1, "in_progress", cmpI64(600000), nil)),
			wantStatus: comparison.SetStatusUnregistered, wantPct: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set, _ := compareSet(tt.exercise, 1, tt.feedback, false)
			assert.Equal(t, tt.wantStatus, set.Status)
			require.NotNil(t, set.CompliancePct)
			assert.Equal(t, tt.wantPct, *set.CompliancePct)
			assert.Equal(t, tt.wantDevMs, set.DeviationDurationMs)
			assert.Equal(t, tt.wantDevMeters, set.DeviationDistanceM)
		})
	}
}

func ptrFeedback(fb dbs.WorkoutFeedback) *dbs.WorkoutFeedback { return &fb }

func TestCompareSet_PrefersActiveDurationAndKeepsRPE(t *testing.T) {
	fb := cmpFeedback(1, 1, 1, "completed", cmpI64(900000), nil)
	fb.ActiveDurationMs = cmpI64(480000)
	fb.RPE = cmpI16(7)

	set, ratio := compareSet(cmpExercise(1, cmpInt(10), nil), 1, &fb, false)

	assert.Equal(t, int64(480000), *set.DurationMs)
	assert.Equal(t, 80.0, *set.CompliancePct)
	assert.InDelta(t, 0.8, ratio, 0.0001)
	assert.Equal(t, int16(7), *set.RPE)
	assert.Equal(t, int64(1), *set.FeedbackID)
}

func TestCompareSet_ExtraDoesNotCount(t *testing.T) {
	fb := cmpFeedback(1, 1, 3, "completed", cmpI64(600000), nil)

	set, ratio := compareSet(cmpExercise(1, cmpInt(10), nil), 3, &fb, true)

	assert.True(t, set.Extra)
	assert.Nil(t, set.CompliancePct)
	assert.Equal(t, 0.0, ratio)
	assert.Equal(t, int64(0), *set.DeviationDurationMs)
}

func TestFeedbackStatus_WithoutStatusUsesData(t *testing.T) {
	withTime := dbs.WorkoutFeedback{DurationMs: cmpI64(1000)}
	withDistance := dbs.WorkoutFeedback{DistanceMeters: cmpF64(10)}
	empty := dbs.WorkoutFeedback{}

	assert.Equal(t, comparison.SetStatusCompleted, feedbackStatus(&withTime))
	assert.Equal(t, comparison.SetStatusCompleted, feedbackStatus(&withDistance))
	assert.Equal(t, comparison.SetStatusUnregistered, feedbackStatus(&empty))
}

func TestNormalizeFeedback_ZeroBasedDeletedAndDuplicates(t *testing.T) {
	deleted := cmpFeedback(10, 1, 1, "completed", cmpI64(1), nil)
	deleted.DeletedAt = cmpTime(time.Now())

	legacy0 := cmpFeedback(11, 1, 0, "completed", cmpI64(600000), nil)
	legacy1 := cmpFeedback(12, 1, 1, "skipped", nil, nil)

	trainerCopy := cmpFeedback(20, 2, 1, "completed", cmpI64(100), nil)
	trainerCopy.FeedbackOwnerUserID = 99
	trainerCopy.UpdatedAt = trainerCopy.UpdatedAt.Add(time.Hour)
	own := cmpFeedback(21, 2, 1, "completed", cmpI64(600000), nil)

	olderOther := cmpFeedback(30, 3, 1, "completed", cmpI64(100), nil)
	olderOther.FeedbackOwnerUserID = 98
	newerOther := cmpFeedback(31, 3, 1, "completed", cmpI64(200), nil)
	newerOther.FeedbackOwnerUserID = 97
	newerOther.UpdatedAt = newerOther.UpdatedAt.Add(time.Minute)

	sameTimeLow := cmpFeedback(40, 4, 1, "completed", cmpI64(100), nil)
	sameTimeHigh := cmpFeedback(41, 4, 1, "completed", cmpI64(200), nil)

	got := normalizeFeedback([]dbs.WorkoutFeedback{deleted, legacy1, legacy0, trainerCopy, own, newerOther, olderOther, sameTimeHigh, sameTimeLow}, cmpAthlete)

	require.Len(t, got[1], 2)
	assert.Equal(t, 1, got[1][0].SetNumber)
	assert.Equal(t, int64(11), got[1][0].ID)
	assert.Equal(t, 2, got[1][1].SetNumber)
	assert.Equal(t, int64(12), got[1][1].ID)

	require.Len(t, got[2], 1)
	assert.Equal(t, int64(21), got[2][0].ID, "gana la fila del propio atleta")

	require.Len(t, got[3], 1)
	assert.Equal(t, int64(31), got[3][0].ID, "entre filas de otros gana la más reciente")

	require.Len(t, got[4], 1)
	assert.Equal(t, int64(41), got[4][0].ID, "a igual fecha gana el id mayor")
}

func TestCompareExercise_UnregisteredAndExtraSets(t *testing.T) {
	ex := cmpExercise(1, cmpInt(10), nil)
	ex.RepeatCount = 2
	feedback := []dbs.WorkoutFeedback{
		cmpFeedback(1, 1, 1, "completed", cmpI64(600000), nil),
		cmpFeedback(2, 1, 3, "completed", cmpI64(300000), nil),
	}

	result, ratioSum := compareExercise(ex, feedback)

	require.Len(t, result.Sets, 3)
	assert.Equal(t, comparison.SetStatusCompleted, result.Sets[0].Status)
	assert.Equal(t, comparison.SetStatusUnregistered, result.Sets[1].Status)
	assert.True(t, result.Sets[2].Extra)
	assert.Equal(t, 50.0, result.CompliancePct)
	assert.InDelta(t, 1.0, ratioSum, 0.0001)
	assert.Equal(t, 2, result.CompletedSets)
	assert.True(t, result.HasTarget)
}

func TestCompareSession_TotalsAndEdgeCases(t *testing.T) {
	distanceEx := cmpExercise(1, nil, cmpInt(500))
	distanceEx.Kind = "velocidad_distancia"
	distanceEx.RepeatCount = 0 // se toma como 1
	noTarget := cmpExercise(2, nil, nil)
	noTarget.Kind = ""

	feedback := []dbs.WorkoutFeedback{cmpFeedback(1, 1, 1, "completed", cmpI64(120000), cmpF64(400))}
	result := compareSession([]daos.ComparisonPlannedExercise{distanceEx, noTarget}, feedback, cmpAthlete)

	assert.Equal(t, 2, result.totals.PlannedSets)
	assert.Nil(t, result.totals.PlannedDurationMs)
	assert.Equal(t, 500.0, *result.totals.PlannedDistanceM)
	assert.Equal(t, 1, result.totals.UnregisteredSets)
	assert.Equal(t, 40.0, result.compliancePct) // (0,8 + 0) / 2
	assert.Equal(t, []string{"velocidad_distancia"}, result.kinds)
	assert.False(t, result.exercises[1].HasTarget)

	empty := compareSession(nil, nil, cmpAthlete)
	assert.Equal(t, 0.0, empty.compliancePct)
	assert.Equal(t, 0, empty.totals.PlannedSets)
	assert.Empty(t, empty.exercises)
}

func TestPaces(t *testing.T) {
	ex := cmpExercise(1, nil, nil)
	assert.Nil(t, plannedPace(ex))
	ex.SpeedKph = cmpF64(12)
	assert.Equal(t, 300.0, *plannedPace(ex))
	ex.SpeedKph = cmpF64(0)
	assert.Nil(t, plannedPace(ex))

	stored := dbs.WorkoutFeedback{PaceSecPerKM: cmpF64(345.67)}
	assert.Equal(t, 345.7, *actualPace(&stored, cmpI64(1)))

	derived := dbs.WorkoutFeedback{DistanceMeters: cmpF64(1000)}
	assert.Equal(t, 600.0, *actualPace(&derived, cmpI64(600000)))

	noDistance := dbs.WorkoutFeedback{DistanceMeters: cmpF64(0)}
	assert.Nil(t, actualPace(&noDistance, cmpI64(600000)))
	assert.Nil(t, actualPace(&derived, nil))
}
