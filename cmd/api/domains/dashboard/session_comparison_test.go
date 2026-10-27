package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
)

func TestBuildSessionComparisonSide_RitmoContinuoOrdenaPorTiempoYPromediaRPE(t *testing.T) {
	rows := []ScopedSeries{
		sessionComparisonRow(constants.ExerciseKindRitmoContinuo, 2, 1000, 240_000, 230_000, 7),
		sessionComparisonRow(constants.ExerciseKindRitmoContinuo, 1, 1000, 220_000, 210_000, 5),
		sessionComparisonRow(constants.ExerciseKindRitmoContinuo, 3, 0, 250_000, 0, 0),
	}
	rows[0].FeedbackID = 22
	rows[1].FeedbackID = 11
	rows[2].FeedbackID = 33

	side := BuildSessionComparisonSide(SessionComparisonContext{
		TeamID: 4, GroupID: 5, GroupName: "Sub-18", AthleteUserID: 6,
		SessionInstanceID: 7, ExerciseID: 8, ExerciseName: "Fondo",
	}, rows)

	assert.Equal(t, "ritmo_continuo", side.Kind)
	assert.Equal(t, "2026-09-01", side.SessionDate)
	assert.Equal(t, 3, side.SeriesCount)
	assert.Equal(t, 2, side.ValidSeries)
	require.NotNil(t, side.TotalDistanceM)
	assert.Equal(t, 2000.0, *side.TotalDistanceM)
	require.NotNil(t, side.MeanRPE)
	assert.Equal(t, 6.0, *side.MeanRPE)
	assert.Equal(t, 2, side.RPECount)
	require.NotNil(t, side.Best)
	require.NotNil(t, side.Worst)
	assert.Equal(t, int64(11), side.Best.FeedbackID)
	assert.Equal(t, int64(22), side.Worst.FeedbackID)
	assert.Equal(t, 1000.0, *side.Best.DistanceMeters, "el extremo incluye distancia real")
	assert.Equal(t, 1, side.Series[0].SetNumber, "los puntos se ordenan por número de serie")
}

func TestBuildSessionComparisonSide_VelocidadDistanciaOrdenaPorVelocidad(t *testing.T) {
	rows := []ScopedSeries{
		sessionComparisonRow(constants.ExerciseKindVelocidadDistancia, 1, 200, 60_000, 60_000, 4),
		sessionComparisonRow(constants.ExerciseKindVelocidadDistancia, 2, 200, 50_000, 50_000, 6),
	}
	rows[0].FeedbackID = 1
	rows[1].FeedbackID = 2

	side := BuildSessionComparisonSide(SessionComparisonContext{}, rows)

	require.NotNil(t, side.Best)
	require.NotNil(t, side.Worst)
	assert.Equal(t, int64(2), side.Best.FeedbackID)
	assert.Equal(t, int64(1), side.Worst.FeedbackID)
}

func TestBuildSessionComparisonSide_VelocidadTiempoOrdenaPorDistancia(t *testing.T) {
	rows := []ScopedSeries{
		sessionComparisonRow(constants.ExerciseKindVelocidadTiempo, 1, 500, 60_000, 60_000, 5),
		sessionComparisonRow(constants.ExerciseKindVelocidadTiempo, 2, 700, 60_000, 60_000, 7),
	}
	rows[0].FeedbackID = 1
	rows[1].FeedbackID = 2

	side := BuildSessionComparisonSide(SessionComparisonContext{}, rows)

	require.NotNil(t, side.Best)
	require.NotNil(t, side.Worst)
	assert.Equal(t, int64(2), side.Best.FeedbackID)
	assert.Equal(t, int64(1), side.Worst.FeedbackID)
}

func TestBuildSessionComparisonSide_PonderaVelocidadPorDistancia(t *testing.T) {
	rows := []ScopedSeries{
		sessionComparisonRow(constants.ExerciseKindRitmoContinuo, 1, 1000, 60_000, 60_000, 0),
		sessionComparisonRow(constants.ExerciseKindRitmoContinuo, 2, 3000, 600_000, 600_000, 0),
	}
	side := BuildSessionComparisonSide(SessionComparisonContext{}, rows)

	require.NotNil(t, side.MeanSpeedKph)
	assert.InDelta(t, 28.5, *side.MeanSpeedKph, 0.001)
}

func TestBuildSessionComparisonSide_EmpateEsDeterministaYValoresAusentesSonNull(t *testing.T) {
	rows := []ScopedSeries{
		sessionComparisonRow(constants.ExerciseKindRitmoContinuo, 2, 1000, 200_000, 190_000, 0),
		sessionComparisonRow(constants.ExerciseKindRitmoContinuo, 1, 1000, 200_000, 190_000, 0),
	}
	rows[0].FeedbackID = 20
	rows[1].FeedbackID = 10
	rows[0].SessionDate = day(2)
	rows[1].SessionDate = day(1)

	side := BuildSessionComparisonSide(SessionComparisonContext{}, rows)

	require.NotNil(t, side.Best)
	require.NotNil(t, side.Worst)
	assert.Equal(t, int64(10), side.Best.FeedbackID)
	assert.Equal(t, int64(10), side.Worst.FeedbackID)
	assert.Nil(t, side.MeanRPE)
	assert.Equal(t, 0, side.RPECount)

	emptyDistance := sessionComparisonRow(constants.ExerciseKindVelocidadTiempo, 1, 0, 60_000, 0, 0)
	withoutDistance := BuildSessionComparisonSide(SessionComparisonContext{}, []ScopedSeries{emptyDistance})
	assert.Nil(t, withoutDistance.TotalDistanceM)
	assert.Nil(t, withoutDistance.Best)
	assert.Nil(t, withoutDistance.Worst)
}

func TestBuildSessionComparison_DeltasSonBMenosA(t *testing.T) {
	a := SessionComparisonSide{
		MeanSpeedKph:   ptrFloat(10),
		TotalDistanceM: ptrFloat(1200),
	}
	b := SessionComparisonSide{
		MeanSpeedKph:   ptrFloat(12),
		TotalDistanceM: ptrFloat(1500),
	}

	response := BuildSessionComparison(a, b)

	assert.Equal(t, sessionComparisonDirectionBMinusA, response.Delta.Direction)
	assert.Equal(t, 2.0, *response.Delta.SpeedDeltaKph)
	assert.Equal(t, 20.0, *response.Delta.SpeedDeltaPct)
	assert.Equal(t, 300.0, *response.Delta.DistanceDeltaM)
	assert.Equal(t, "km/h", response.Units.Speed)
	assert.Equal(t, "1-10", response.Units.RPE)

	a = SessionComparisonSide{MeanSpeedKph: ptrFloat(10), TotalDistanceM: ptrFloat(1000)}
	b = SessionComparisonSide{MeanSpeedKph: ptrFloat(9), TotalDistanceM: ptrFloat(500)}
	response = BuildSessionComparison(a, b)
	assert.Equal(t, -1.0, *response.Delta.SpeedDeltaKph)
	assert.Equal(t, -10.0, *response.Delta.SpeedDeltaPct)
	assert.Equal(t, -500.0, *response.Delta.DistanceDeltaM)

	response = BuildSessionComparison(SessionComparisonSide{}, b)
	assert.Nil(t, response.Delta.SpeedDeltaKph)
	assert.Nil(t, response.Delta.SpeedDeltaPct)
	assert.Nil(t, response.Delta.DistanceDeltaM)

	response = BuildSessionComparison(
		SessionComparisonSide{MeanSpeedKph: ptrFloat(0)},
		SessionComparisonSide{MeanSpeedKph: ptrFloat(2)},
	)
	assert.Equal(t, 2.0, *response.Delta.SpeedDeltaKph)
	assert.Nil(t, response.Delta.SpeedDeltaPct, "no existe porcentaje relativo si A es cero")
}

func sessionComparisonRow(kind constants.ExerciseKind, set int, distance int, duration, active int64, rpe int16) ScopedSeries {
	var distanceM *float64
	if distance > 0 {
		value := float64(distance)
		distanceM = &value
	}
	var rpeValue *int16
	if rpe > 0 {
		rpeValue = &rpe
	}
	return ScopedSeries{
		Series: Series{
			Kind:             string(kind),
			DurationMs:       &duration,
			ActiveDurationMs: &active,
			DistanceMeters:   distanceM,
			RPE:              rpeValue,
		},
		FeedbackID:        int64(set),
		SessionInstanceID: 50,
		SessionDate:       day(1),
		SetNumber:         set,
		TargetMinutes:     ptrInt(1),
		TargetDistanceM:   ptrInt(distance),
	}
}
