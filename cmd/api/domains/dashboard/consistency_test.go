package dashboard

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCoefficientOfVariation(t *testing.T) {
	cases := []struct {
		name   string
		speeds []float64
		want   *float64
	}{
		{"sin datos", nil, nil},
		{"una sola serie no tiene dispersión", []float64{12}, nil},
		{"sin dispersión", []float64{12, 12, 12}, ptrFloat(0)},
		{"dos valores", []float64{10, 20}, ptrFloat(1.0 / 3.0)},
		{"ritmo continuo con dispersión real", []float64{15, 14.6341463, 15, 14.0243902, 15.6521739}, ptrFloat(0.0358063986)},
		{"velocidades cero se descartan", []float64{0, 10, 20}, ptrFloat(1.0 / 3.0)},
		{"todas cero", []float64{0, 0}, nil},
		{"negativas se descartan", []float64{-10, -20}, nil},
		{"infinito se descarta", []float64{math.Inf(1), 10, 20}, ptrFloat(1.0 / 3.0)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cv := coefficientOfVariation(tc.speeds)

			if tc.want == nil {
				assert.Nil(t, cv)
				return
			}
			require.NotNil(t, cv)
			assert.InDelta(t, *tc.want, *cv, 1e-6)
		})
	}
}

// Coeficiente de variación poblacional: las series del período son el conjunto
// completo, no una muestra de una población mayor.
func TestCoefficientOfVariationEsPoblacional(t *testing.T) {
	cv := coefficientOfVariation([]float64{10, 20})

	require.NotNil(t, cv)
	mu := 15.0
	sigma := math.Sqrt(((10-mu)*(10-mu) + (20-mu)*(20-mu)) / 2)
	muestral := math.Sqrt(((10-mu)*(10-mu) + (20-mu)*(20-mu)) / 1)

	assert.InDelta(t, sigma/mu, *cv, 1e-9)
	assert.Greater(t, abs(muestral/mu-*cv), 0.1, "la sigma muestral daría otro número")
}

func TestCoefficientOfVariationSobreSeriesValidas(t *testing.T) {
	// Las elongaciones son válidas pero no tienen distancia, así que no tienen
	// velocidad definible y quedan fuera del cálculo.
	elongaciones := []Series{
		{Kind: kindElongacion, ActiveDurationMs: ptrInt64(600000), DistanceMeters: ptrFloat(0)},
		{Kind: kindElongacion, ActiveDurationMs: ptrInt64(620000)},
	}

	assert.Nil(t, coefficientOfVariation(validSpeeds(elongaciones)))

	ritmos := []Series{
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), DistanceMeters: ptrFloat(1000)},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(480000), DistanceMeters: ptrFloat(2000)},
	}

	cv := coefficientOfVariation(validSpeeds(append(elongaciones, ritmos...)))

	require.NotNil(t, cv)
	assert.InDelta(t, 0, *cv, 1e-9)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
