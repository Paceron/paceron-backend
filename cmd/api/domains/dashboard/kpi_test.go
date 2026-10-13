package dashboard

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectiveAxisForKind(t *testing.T) {
	cases := []struct {
		name      string
		kind      string
		minutes   *int
		distanceM *int
		want      Axis
	}{
		{"liviano solo tiempo", kindLiviano, ptrInt(10), nil, AxisTime},
		{"liviano solo distancia", kindLiviano, nil, ptrInt(1200), AxisDistance},
		{"liviano con ambos", kindLiviano, ptrInt(10), ptrInt(1200), AxisDistanceAndTime},
		{"liviano sin objetivo sigue declarando eje", kindLiviano, nil, nil, AxisTime},
		{"elongacion tiempo", kindElongacion, ptrInt(10), nil, AxisTime},
		{"elongacion ignora la distancia", kindElongacion, ptrInt(10), ptrInt(1200), AxisTime},
		{"ritmo_continuo es el eje doble", kindRitmoContinuo, ptrInt(4), ptrInt(1000), AxisDistanceAndTime},
		{"ritmo_continuo sin distancia sigue siendo el eje doble", kindRitmoContinuo, ptrInt(4), nil, AxisDistanceAndTime},
		{"velocidad_tiempo sin distancia", kindVelocidadTiempo, ptrInt(2), nil, AxisTime},
		{"velocidad_tiempo con distancia no cambia el eje", kindVelocidadTiempo, ptrInt(2), ptrInt(1000), AxisTime},
		{"velocidad maxima con objetivo solo de distancia", kindVelocidadDistancia, nil, ptrInt(100), AxisDistance},

		// Los legacy siguen siendo válidos para la API, pero el dashboard no
		// sabe medirlos: sin eje, sin KPI y sin error.
		{"legacy walking", kindLegacyWalking, ptrInt(30), ptrInt(3000), AxisNone},
		{"legacy jogging", kindLegacyJogging, ptrInt(30), ptrInt(3000), AxisNone},
		{"legacy elongation", kindLegacyElongation, ptrInt(10), nil, AxisNone},
		{"legacy cruising", kindLegacyCruising, ptrInt(30), ptrInt(3000), AxisNone},
		{"legacy running", kindLegacyRunning, ptrInt(10), ptrInt(1600), AxisNone},
		{"kind vacío", "", ptrInt(10), nil, AxisNone},
		{"kind desconocido", "intervalo", ptrInt(10), ptrInt(100), AxisNone},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets := targetFromExerciseInstance(tc.minutes, tc.distanceM)
			assert.Equal(t, tc.want, objectiveAxisForKind(tc.kind, targets))
		})
	}
}

func TestHasTarget(t *testing.T) {
	cases := []struct {
		name      string
		kind      string
		minutes   *int
		distanceM *int
		want      bool
	}{
		{"ritmo_continuo con ambos", kindRitmoContinuo, ptrInt(4), ptrInt(1000), true},
		{"ritmo_continuo sin distancia", kindRitmoContinuo, ptrInt(4), nil, false},
		{"ritmo_continuo sin tiempo", kindRitmoContinuo, nil, ptrInt(1000), false},
		{"elongacion con tiempo", kindElongacion, ptrInt(10), nil, true},
		{"elongacion con objetivo cero", kindElongacion, ptrInt(0), nil, false},
		{"elongacion con objetivo negativo", kindElongacion, ptrInt(-10), nil, false},
		{"velocidad_distancia con distancia", kindVelocidadDistancia, nil, ptrInt(100), true},
		{"velocidad_distancia sin distancia", kindVelocidadDistancia, ptrInt(1), nil, false},
		{"liviano solo distancia", kindLiviano, nil, ptrInt(3000), true},
		{"liviano solo tiempo", kindLiviano, ptrInt(10), nil, true},
		{"liviano sin nada", kindLiviano, nil, nil, false},
		{"legacy", kindLegacyWalking, ptrInt(30), ptrInt(3000), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets := targetFromExerciseInstance(tc.minutes, tc.distanceM)
			assert.Equal(t, tc.want, hasTarget(tc.kind, targets))
		})
	}
}

func TestTargetFromExerciseInstance(t *testing.T) {
	cases := []struct {
		name      string
		minutes   *int
		distanceM *int
		wantSpeed *float64
	}{
		{"ambos objetivos", ptrInt(4), ptrInt(1000), ptrFloat(15)},
		{"sin distancia no hay velocidad esperada", ptrInt(4), nil, nil},
		{"sin tiempo no hay velocidad esperada", nil, ptrInt(1000), nil},
		{"ambos nulos", nil, nil, nil},
		{"distancia cero", ptrInt(4), ptrInt(0), nil},
		{"cinco km en media hora", ptrInt(30), ptrInt(5000), ptrFloat(10)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets := targetFromExerciseInstance(tc.minutes, tc.distanceM)

			assert.Equal(t, tc.minutes, targets.Minutes)
			assert.Equal(t, tc.distanceM, targets.DistanceM)
			if tc.wantSpeed == nil {
				assert.Nil(t, targets.SpeedKph)
				return
			}
			require.NotNil(t, targets.SpeedKph)
			assert.InDelta(t, *tc.wantSpeed, *targets.SpeedKph, 1e-9)
		})
	}
}

func TestSpeedFrom(t *testing.T) {
	cases := []struct {
		name       string
		distanceM  float64
		durationMs int64
		want       *float64
	}{
		{"un km en cuatro minutos", 1000, 240000, ptrFloat(15)},
		{"cinco km en media hora", 5000, 1800000, ptrFloat(10)},
		{"100 m en doce segundos", 100, 12000, ptrFloat(30)},
		{"con decimales", 1234.5, 100000, ptrFloat(44.442)},
		{"distancia cero", 0, 240000, nil},
		{"distancia negativa", -1000, 240000, nil},
		{"tiempo cero", 1000, 0, nil},
		{"tiempo negativo", 1000, -1, nil},
		{"distancia NaN", math.NaN(), 240000, nil},
		{"distancia infinita", math.Inf(1), 240000, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			speed := speedFrom(tc.distanceM, tc.durationMs)

			if tc.want == nil {
				assert.Nil(t, speed)
				return
			}
			require.NotNil(t, speed)
			assert.InDelta(t, *tc.want, *speed, 1e-9)
		})
	}
}

func TestAchievedFromSeries(t *testing.T) {
	t.Run("prioriza el tiempo activo", func(t *testing.T) {
		s := Series{
			Kind:             kindRitmoContinuo,
			DurationMs:       ptrInt64(250000),
			ActiveDurationMs: ptrInt64(230000),
			DistanceMeters:   ptrFloat(1000),
		}

		achieved := achievedFromSeries(s)

		assert.Equal(t, int64(230000), achieved.TimeMs)
		require.NotNil(t, achieved.SpeedKph)
		assert.InDelta(t, 15.652173913, *achieved.SpeedKph, 1e-6)
	})

	t.Run("cae al duration_ms cuando no hay tiempo activo", func(t *testing.T) {
		s := Series{
			Kind:           kindRitmoContinuo,
			DurationMs:     ptrInt64(250000),
			DistanceMeters: ptrFloat(1000),
		}

		achieved := achievedFromSeries(s)

		assert.Equal(t, int64(250000), achieved.TimeMs)
		require.NotNil(t, achieved.SpeedKph)
		assert.InDelta(t, 14.4, *achieved.SpeedKph, 1e-9)
	})

	t.Run("sin distancia no hay velocidad", func(t *testing.T) {
		achieved := achievedFromSeries(Series{Kind: kindElongacion, ActiveDurationMs: ptrInt64(230000)})

		assert.Equal(t, int64(230000), achieved.TimeMs)
		assert.Nil(t, achieved.SpeedKph)
		assert.Nil(t, achieved.DistanceM)
	})
}
