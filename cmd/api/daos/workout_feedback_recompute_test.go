package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Recompute de pace/peak al subir points (gamificacion-core D6), contra
// Postgres real. Sin backfill: los feedbacks viejos quedan NULL.

func TestWorkoutFeedbackDao_Recompute_ConDistanceYGPS(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	duration := int64(600000) // 10 min
	distance := 2000.0        // 2 km
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	require.NoError(t, db.Model(feedback).Updates(map[string]interface{}{
		"duration_ms":     duration,
		"distance_meters": distance,
	}).Error)

	base := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	// Segmento rápido: 0.01° lat ≈ 1111.9 m en 60 s → ~66.7 km/h.
	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: base},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.61, Longitude: -58.4, RecordedAt: base.Add(time.Minute)},
	}

	created, err := dao.BulkCreatePointsRecompute(nil, feedback.ID, points)

	require.NoError(t, err)
	assert.Equal(t, int64(2), created)

	updated, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.PaceSecPerKM)
	// 600000 ms / 2000 m = 300 s/km.
	assert.InDelta(t, 300.0, *updated.PaceSecPerKM, 0.0001)
	require.NotNil(t, updated.PeakSpeedKPH)
	assert.InDelta(t, 1111.9/60*3.6, *updated.PeakSpeedKPH, 1.0)
}

func TestWorkoutFeedbackDao_Recompute_IdempotenteYParcial(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	duration := int64(600000)
	distance := 2000.0
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	require.NoError(t, db.Model(feedback).Updates(map[string]interface{}{
		"duration_ms":     duration,
		"distance_meters": distance,
	}).Error)

	base := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: base},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.61, Longitude: -58.4, RecordedAt: base.Add(time.Minute)},
	}
	resetIDs := func() {
		for i := range points {
			points[i].ID = 0
		}
	}

	_, err := dao.BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.NoError(t, err)
	first, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	require.NotNil(t, first.PeakSpeedKPH)
	firstPeak := *first.PeakSpeedKPH

	// Reintento del MISMO batch (created = 0): no recompute, valores intactos.
	resetIDs()
	_, err = dao.BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.NoError(t, err)
	same, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	require.NotNil(t, same.PeakSpeedKPH)
	assert.Equal(t, firstPeak, *same.PeakSpeedKPH)

	// Subida parcial con un segmento más rápido (0.02° en 30 s): el recompute
	// es sobre TODO el recorrido, no solo el batch nuevo.
	points = append(points, dbs.WorkoutFeedbackPoint{
		FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 2, Latitude: -34.62, Longitude: -58.41, RecordedAt: base.Add(90 * time.Second),
	})
	resetIDs()
	created, err := dao.BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.NoError(t, err)
	assert.Equal(t, int64(1), created)

	grown, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	require.NotNil(t, grown.PeakSpeedKPH)
	assert.Greater(t, *grown.PeakSpeedKPH, firstPeak)
	// El pace no cambia: duration/distance del feedback son las mismas.
	require.NotNil(t, grown.PaceSecPerKM)
	assert.InDelta(t, 300.0, *grown.PaceSecPerKM, 0.0001)
}

func TestWorkoutFeedbackDao_Recompute_SinDistancePaceNULL(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	base := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: base},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.61, Longitude: -58.41, RecordedAt: base.Add(time.Minute)},
	}

	_, err := dao.BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.NoError(t, err)

	updated, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.PaceSecPerKM, "sin distance → pace NULL")
	require.NotNil(t, updated.PeakSpeedKPH, "con GPS → peak presente")
}

func TestWorkoutFeedbackDao_Recompute_SinGPSPeakNULL(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	duration := int64(600000)
	distance := 2000.0
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	require.NoError(t, db.Model(feedback).Updates(map[string]interface{}{
		"duration_ms":     duration,
		"distance_meters": distance,
	}).Error)

	// Un solo punto: peak no computable → NULL.
	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)},
	}

	_, err := dao.BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.NoError(t, err)

	updated, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.PaceSecPerKM)
	assert.InDelta(t, 300.0, *updated.PaceSecPerKM, 0.0001)
	assert.Nil(t, updated.PeakSpeedKPH, "GPS insuficiente → peak NULL")
}

func TestWorkoutFeedbackDao_Recompute_SobrescribeNULLsPrevios(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	duration := int64(600000)
	distance := 2000.0
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	// Estado previo con columnas seteadas: el recompute completo pisa.
	require.NoError(t, db.Model(feedback).Updates(map[string]interface{}{
		"duration_ms":     duration,
		"distance_meters": distance,
		"pace_sec_per_km": 999.0,
		"peak_speed_kph":  999.0,
	}).Error)

	base := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: base},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.6, Longitude: -58.4, RecordedAt: base.Add(time.Minute)},
	}

	_, err := dao.BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.NoError(t, err)

	updated, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.PaceSecPerKM)
	assert.InDelta(t, 300.0, *updated.PaceSecPerKM, 0.0001)
	require.NotNil(t, updated.PeakSpeedKPH)
	// Segmento sin desplazamiento → 0 km/h, no el 999 previo.
	assert.InDelta(t, 0.0, *updated.PeakSpeedKPH, 0.0001)
}

// Atomicidad D6: si el recompute falla, el insert de points también se
// revierte (misma transacción).
func TestWorkoutFeedbackDao_Recompute_FalloAtomaTransaccion(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewWorkoutFeedbackDao(db)
	duration := int64(600000)
	distance := 2000.0
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)
	require.NoError(t, db.Model(feedback).Updates(map[string]interface{}{
		"duration_ms":     duration,
		"distance_meters": distance,
	}).Error)

	base := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: base},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.61, Longitude: -58.4, RecordedAt: base.Add(time.Minute)},
	}

	// Falla el UPDATE del recompute (1er update) → rollback del insert.
	failing := testutils.FailingDB(t, db, nthFail("update", 1))
	_, err := NewWorkoutFeedbackDao(failing).BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.Error(t, err)

	pointsInDB, err := dao.GetPointsByFeedback(nil, feedback.ID)
	require.NoError(t, err)
	assert.Empty(t, pointsInDB, "el insert se revierte con el recompute")

	clean, err := dao.GetByID(nil, feedback.ID)
	require.NoError(t, err)
	assert.Nil(t, clean.PaceSecPerKM)
	assert.Nil(t, clean.PeakSpeedKPH)
}

// Falla del SELECT del recompute (2do select dentro de la tx) → error.
func TestWorkoutFeedbackDao_Recompute_FalloSelectSegundo(t *testing.T) {
	db := testutils.SetupTestDB(t)
	feedback := testFeedback(t, db, 1, 1, 1, 1, nil, 0)

	base := time.Date(2026, 9, 24, 14, 0, 0, 0, time.UTC)
	points := []dbs.WorkoutFeedbackPoint{
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 0, Latitude: -34.6, Longitude: -58.4, RecordedAt: base},
		{FeedbackID: feedback.ID, SessionInstanceID: 3, ExerciseInstanceID: 4, Order: 1, Latitude: -34.61, Longitude: -58.4, RecordedAt: base.Add(time.Minute)},
	}

	// 1er select = carga del feedback; 2do = carga de todos los points.
	failing := testutils.FailingDB(t, db, nthFail("select", 2))
	_, err := NewWorkoutFeedbackDao(failing).BulkCreatePointsRecompute(nil, feedback.ID, points)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error loading points for recompute")
}
