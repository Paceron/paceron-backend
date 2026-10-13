package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrendPct(t *testing.T) {
	cases := []struct {
		name   string
		speeds []float64
		want   *float64
	}{
		{"sin series", nil, nil},
		{"una serie", []float64{12}, nil},
		{"dos series no generan tendencia", []float64{12, 13}, nil},
		{"tres series", []float64{10, 10, 12}, ptrFloat(20)},
		{"cinco series", []float64{10, 10, 10, 11, 11}, ptrFloat(10)},
		{"retroceso", []float64{12, 12, 12, 10, 10}, ptrFloat(-16.6666667)},
		{"estable", []float64{10, 10, 10, 10.2, 10.2}, ptrFloat(2)},
		{"tercio mas antiguo en cero", []float64{0, 10, 10, 10, 10}, nil},
		{"velocidades negativas", []float64{-10, -10, -5}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pct := trendPct(tc.speeds)

			if tc.want == nil {
				assert.Nil(t, pct)
				return
			}
			require.NotNil(t, pct)
			assert.InDelta(t, *tc.want, *pct, 1e-6)
		})
	}
}

func TestTrendVerdict(t *testing.T) {
	cases := []struct {
		name string
		pct  *float64
		want string
	}{
		{"sin tendencia es estable", nil, TrendStable},
		{"justo en el umbral positivo", ptrFloat(3), TrendStable},
		{"pasado el umbral positivo", ptrFloat(3.0001), TrendImproving},
		{"claramente improving", ptrFloat(12.5), TrendImproving},
		{"dentro de la banda", ptrFloat(0), TrendStable},
		{"negativo dentro de la banda", ptrFloat(-2.9), TrendStable},
		{"justo en el umbral negativo", ptrFloat(-3), TrendStable},
		{"pasado el umbral negativo", ptrFloat(-3.0001), TrendRegressing},
		{"claramente regressing", ptrFloat(-18), TrendRegressing},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, trendVerdict(tc.pct))
		})
	}
}

func TestTrendSobreSeriesValidas(t *testing.T) {
	// Cinco series válidas de ritmo continuo: el tercil más reciente es más
	// rápido que el más antiguo, así que la tendencia mejora.
	series := []Series{
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(264000), DistanceMeters: ptrFloat(1000)},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(258000), DistanceMeters: ptrFloat(1000)},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(252000), DistanceMeters: ptrFloat(1000)},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(246000), DistanceMeters: ptrFloat(1000)},
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), DistanceMeters: ptrFloat(1000)},
	}

	pct := trendPct(validSpeeds(series))

	require.NotNil(t, pct)
	assert.InDelta(t, 10, *pct, 1e-6)
	assert.Equal(t, TrendImproving, trendVerdict(pct))
}
