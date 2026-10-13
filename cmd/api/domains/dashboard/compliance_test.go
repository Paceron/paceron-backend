package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsCompliant(t *testing.T) {
	const tolerance = DefaultTolerance

	cases := []struct {
		name              string
		kind              string
		minutes           *int
		targetDistanceM   *int
		achievedTimeMs    int64
		achievedDistanceM *float64
		want              bool
	}{
		{
			name:           "elongacion dentro de la tolerancia",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 250000,
			want:           true,
		},
		{
			name:           "elongacion justo en el borde de la tolerancia",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 252000,
			want:           true,
		},
		{
			name:           "elongacion un ms pasado el borde",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 252001,
			want:           false,
		},
		{
			name:           "superar el objetivo cuenta como cumplimiento",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 230000,
			want:           true,
		},
		{
			name:           "sin tiempo logrado no cumple",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 0,
			want:           false,
		},
		{
			name:              "velocidad_tiempo mide tiempo y no distancia",
			kind:              kindVelocidadTiempo,
			minutes:           ptrInt(2),
			achievedTimeMs:    118000,
			achievedDistanceM: ptrFloat(500),
			want:              true,
		},
		{
			name:              "ritmo_continuo cumple con tiempo y distancia",
			kind:              kindRitmoContinuo,
			minutes:           ptrInt(4),
			targetDistanceM:   ptrInt(1000),
			achievedTimeMs:    235000,
			achievedDistanceM: ptrFloat(1000),
			want:              true,
		},
		{
			name:              "ritmo_continuo mas rapido pero corto de distancia no cumple",
			kind:              kindRitmoContinuo,
			minutes:           ptrInt(4),
			targetDistanceM:   ptrInt(1000),
			achievedTimeMs:    220000,
			achievedDistanceM: ptrFloat(940),
			want:              false,
		},
		{
			name:              "ritmo_continuo justo en el borde de distancia",
			kind:              kindRitmoContinuo,
			minutes:           ptrInt(4),
			targetDistanceM:   ptrInt(1000),
			achievedTimeMs:    240000,
			achievedDistanceM: ptrFloat(950),
			want:              true,
		},
		{
			name:            "ritmo_continuo sin distancia lograda no cumple",
			kind:            kindRitmoContinuo,
			minutes:         ptrInt(4),
			targetDistanceM: ptrInt(1000),
			achievedTimeMs:  235000,
			want:            false,
		},
		{
			name:              "liviano cumple con cualquiera de los dos ejes",
			kind:              kindLiviano,
			targetDistanceM:   ptrInt(3000),
			achievedTimeMs:    900000,
			achievedDistanceM: ptrFloat(3000),
			want:              true,
		},
		{
			name:              "liviano cargado solo con distancia",
			kind:              kindLiviano,
			targetDistanceM:   ptrInt(3000),
			achievedTimeMs:    0,
			achievedDistanceM: ptrFloat(3000),
			want:              true,
		},
		{
			name:           "liviano cargado solo con tiempo",
			kind:           kindLiviano,
			minutes:        ptrInt(10),
			achievedTimeMs: 600000,
			want:           true,
		},
		{
			name:              "velocidad_distancia mide distancia y no tiempo",
			kind:              kindVelocidadDistancia,
			targetDistanceM:   ptrInt(100),
			achievedTimeMs:    60000,
			achievedDistanceM: ptrFloat(99),
			want:              true,
		},
		{
			name:              "velocidad maxima sin tiempo de objetivo",
			kind:              kindVelocidadDistancia,
			targetDistanceM:   ptrInt(100),
			achievedTimeMs:    12000,
			achievedDistanceM: ptrFloat(90),
			want:              false,
		},
		{
			name:           "sin objetivo cargado nunca cumple",
			kind:           kindRitmoContinuo,
			minutes:        ptrInt(4),
			achievedTimeMs: 235000,
			want:           false,
		},
		{
			name:              "kind legacy no cumple ni rompe",
			kind:              kindLegacyWalking,
			minutes:           ptrInt(30),
			targetDistanceM:   ptrInt(3000),
			achievedTimeMs:    1800000,
			achievedDistanceM: ptrFloat(3000),
			want:              false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets := targetFromExerciseInstance(tc.minutes, tc.targetDistanceM)
			achieved := achievedFor(tc.achievedTimeMs, tc.achievedDistanceM)

			assert.Equal(t, tc.want, isCompliant(tc.kind, targets, achieved, tolerance))
		})
	}
}

func TestIsCompliantToleranciaAjustable(t *testing.T) {
	targets := targetFromExerciseInstance(ptrInt(4), nil)

	// 260 s es +8,3 % sobre el objetivo de 240 s.
	achieved := achievedFor(260000, nil)

	assert.False(t, isCompliant(kindElongacion, targets, achieved, DefaultTolerance))
	assert.False(t, isCompliant(kindElongacion, targets, achieved, 0.08))
	assert.True(t, isCompliant(kindElongacion, targets, achieved, 0.09))
	assert.True(t, isCompliant(kindElongacion, targets, achieved, 0.20))
}

func TestMetTarget(t *testing.T) {
	targets := targetFromExerciseInstance(ptrInt(4), ptrInt(1000))

	t.Run("sin objetivo cargado es null, no false", func(t *testing.T) {
		sinObjetivo := targetFromExerciseInstance(nil, nil)
		assert.Nil(t, metTarget(kindRitmoContinuo, sinObjetivo, achievedFor(240000, ptrFloat(1000)), DefaultTolerance))
	})

	t.Run("cumplida", func(t *testing.T) {
		met := metTarget(kindRitmoContinuo, targets, achievedFor(235000, ptrFloat(1000)), DefaultTolerance)
		require.NotNil(t, met)
		assert.True(t, *met)
	})

	t.Run("no cumplida", func(t *testing.T) {
		met := metTarget(kindRitmoContinuo, targets, achievedFor(300000, ptrFloat(1000)), DefaultTolerance)
		require.NotNil(t, met)
		assert.False(t, *met)
	})
}

func TestDeltaPct(t *testing.T) {
	cases := []struct {
		name              string
		kind              string
		minutes           *int
		targetDistanceM   *int
		achievedTimeMs    int64
		achievedDistanceM *float64
		want              *float64
	}{
		{
			name:           "mejorar el tiempo es positivo",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 230000,
			want:           ptrFloat(4.1666667),
		},
		{
			name:           "empeorar el tiempo es negativo",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 250000,
			want:           ptrFloat(-4.1666667),
		},
		{
			name:              "superar la distancia es positivo",
			kind:              kindVelocidadDistancia,
			targetDistanceM:   ptrInt(4000),
			achievedTimeMs:    900000,
			achievedDistanceM: ptrFloat(4200),
			want:              ptrFloat(5),
		},
		{
			name:              "quedarse corto de distancia es negativo",
			kind:              kindVelocidadDistancia,
			targetDistanceM:   ptrInt(4000),
			achievedTimeMs:    900000,
			achievedDistanceM: ptrFloat(3800),
			want:              ptrFloat(-5),
		},
		{
			name:              "eje doble: se mide sobre la velocidad",
			kind:              kindRitmoContinuo,
			minutes:           ptrInt(4),
			targetDistanceM:   ptrInt(1000),
			achievedTimeMs:    235000,
			achievedDistanceM: ptrFloat(1000),
			want:              ptrFloat(2.1276596),
		},
		{
			name:              "liviano con ambos ejes usa la velocidad",
			kind:              kindLiviano,
			minutes:           ptrInt(10),
			targetDistanceM:   ptrInt(1200),
			achievedTimeMs:    570000,
			achievedDistanceM: ptrFloat(1200),
			want:              ptrFloat(5.2631579),
		},
		{
			name:           "sin objetivo cargado es null",
			kind:           kindRitmoContinuo,
			minutes:        ptrInt(4),
			achievedTimeMs: 235000,
			want:           nil,
		},
		{
			name:           "sin tiempo logrado es null y no infinito",
			kind:           kindElongacion,
			minutes:        ptrInt(4),
			achievedTimeMs: 0,
			want:           nil,
		},
		{
			name:            "distancia lograda nula en el eje distancia",
			kind:            kindVelocidadDistancia,
			targetDistanceM: ptrInt(1000),
			achievedTimeMs:  240000,
			want:            nil,
		},
		{
			name:              "distancia negativa no calcula",
			kind:              kindVelocidadDistancia,
			targetDistanceM:   ptrInt(1000),
			achievedTimeMs:    240000,
			achievedDistanceM: ptrFloat(-10),
			want:              nil,
		},
		{
			name:              "kind legacy no produce delta",
			kind:              kindLegacyJogging,
			minutes:           ptrInt(30),
			targetDistanceM:   ptrInt(3000),
			achievedTimeMs:    1750000,
			achievedDistanceM: ptrFloat(3000),
			want:              nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets := targetFromExerciseInstance(tc.minutes, tc.targetDistanceM)
			achieved := achievedFor(tc.achievedTimeMs, tc.achievedDistanceM)

			pct := deltaPct(tc.kind, targets, achieved)

			if tc.want == nil {
				assert.Nil(t, pct)
				return
			}
			require.NotNil(t, pct)
			assert.InDelta(t, *tc.want, *pct, 1e-6)
		})
	}
}
