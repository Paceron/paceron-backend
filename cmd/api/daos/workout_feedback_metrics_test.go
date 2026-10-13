package daos

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
)

// Métricas denormalizadas D6: pace = duration_ms / distance_meters y peak =
// max velocidad entre segmentos consecutivos. Puntos sintéticos, sin DB.

func secondsPtr(s int) *int64      { v := int64(s * 1000); return &v }
func metersPtr(m float64) *float64 { return &m }

func TestComputePaceSecPerKM(t *testing.T) {
	// 3.600.000 ms (1h) / 10.000 m (10km) = 360 s/km.
	pace := computePaceSecPerKM(secondsPtr(3600), metersPtr(10000))
	require.NotNil(t, pace)
	assert.InDelta(t, 360.0, *pace, 0.0001)

	assert.Nil(t, computePaceSecPerKM(nil, metersPtr(1000)), "sin duration")
	assert.Nil(t, computePaceSecPerKM(secondsPtr(600), nil), "sin distance")
	assert.Nil(t, computePaceSecPerKM(secondsPtr(600), metersPtr(0)), "distance 0")
	assert.Nil(t, computePaceSecPerKM(secondsPtr(0), metersPtr(1000)), "duration 0")
}

func TestComputePeakSpeedKPH_SegmentoSintetico(t *testing.T) {
	base := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	// Segmento de ~1112 metros en 60 segundos (0.01° de lat ≈ 111.2 km).
	points := []dbs.WorkoutFeedbackPoint{
		{Order: 0, Latitude: 0, Longitude: 0, RecordedAt: base},
		{Order: 1, Latitude: 0.01, Longitude: 0, RecordedAt: base.Add(time.Minute)},
	}
	peak := computePeakSpeedKPH(points)
	require.NotNil(t, peak)
	assert.InDelta(t, 1111.9/60*3.6, *peak, 1.0)
}

func TestComputePeakSpeedKPH_MaxEntreSegmentos(t *testing.T) {
	base := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	// Segmento lento (0.001° en 60s ≈ 6.7 km/h) y rápido (0.01° en 30s ≈ 133 km/h).
	points := []dbs.WorkoutFeedbackPoint{
		{Order: 0, Latitude: 0, Longitude: 0, RecordedAt: base},
		{Order: 1, Latitude: 0.001, Longitude: 0, RecordedAt: base.Add(time.Minute)},
		{Order: 2, Latitude: 0.011, Longitude: 0, RecordedAt: base.Add(90 * time.Second)},
	}
	peak := computePeakSpeedKPH(points)
	require.NotNil(t, peak)
	assert.InDelta(t, 1111.9/30*3.6, *peak, 1.0)
}

func TestComputePeakSpeedKPH_OrdenaPorOrder(t *testing.T) {
	base := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	// Desordenados por Order: el segmento correcto va 0 → 1.
	points := []dbs.WorkoutFeedbackPoint{
		{Order: 1, Latitude: 0.01, Longitude: 0, RecordedAt: base.Add(time.Minute)},
		{Order: 0, Latitude: 0, Longitude: 0, RecordedAt: base},
	}
	peak := computePeakSpeedKPH(points)
	require.NotNil(t, peak)
	assert.InDelta(t, 1111.9/60*3.6, *peak, 1.0)
}

func TestComputePeakSpeedKPH_CasosNulos(t *testing.T) {
	base := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	assert.Nil(t, computePeakSpeedKPH(nil))
	assert.Nil(t, computePeakSpeedKPH([]dbs.WorkoutFeedbackPoint{
		{Order: 0, Latitude: 0, Longitude: 0, RecordedAt: base},
	}), "un solo punto")

	// Δt = 0 en todos los segmentos → ningún segmento válido.
	assert.Nil(t, computePeakSpeedKPH([]dbs.WorkoutFeedbackPoint{
		{Order: 0, Latitude: 0, Longitude: 0, RecordedAt: base},
		{Order: 1, Latitude: 0.01, Longitude: 0, RecordedAt: base},
	}))
}

func TestHaversineMeters(t *testing.T) {
	assert.InDelta(t, 0, haversineMeters(0, 0, 0, 0), 0.0001)
	// 1 grado de latitud ≈ 111.19 km.
	assert.InDelta(t, 111190, haversineMeters(0, 0, 1, 0), 100)
	assert.InDelta(t, haversineMeters(0, 0, 1, 0), haversineMeters(1, 0, 0, 0), 0.0001)
	assert.False(t, math.IsNaN(haversineMeters(-34.6, -58.4, 40.4, -3.7)))
}

// Regresión D6: el DTO de feedback NO expone pace_sec_per_km ni
// peak_speed_kph — quedan internos para el leaderboard.
func TestWorkoutFeedbackDTO_NoExponeColumnasLeaderboard(t *testing.T) {
	response := workoutfeedback.WorkoutFeedbackResponse{}
	body, err := json.Marshal(response)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "pace_sec_per_km")
	assert.NotContains(t, string(body), "peak_speed_kph")

	pointBody, err := json.Marshal(workoutfeedback.WorkoutFeedbackPointResponse{})
	require.NoError(t, err)
	assert.NotContains(t, string(pointBody), "pace_sec_per_km")
	assert.NotContains(t, string(pointBody), "peak_speed_kph")
}
