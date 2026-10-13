package dashboard

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidSeries(t *testing.T) {
	cases := []struct {
		name string
		kind string
		s    Series
		want bool
	}{
		{
			name: "tiempo activo con duration_ms nulo",
			kind: kindRitmoContinuo,
			s:    Series{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(230000), DistanceMeters: ptrFloat(1000)},
			want: true,
		},
		{
			name: "sin ningun tiempo",
			kind: kindElongacion,
			s:    Series{Kind: kindElongacion, DistanceMeters: ptrFloat(1000)},
			want: false,
		},
		{
			name: "tiempo activo cero cae al tiempo total",
			kind: kindElongacion,
			s:    Series{Kind: kindElongacion, ActiveDurationMs: ptrInt64(0), DurationMs: ptrInt64(250000)},
			want: true,
		},
		{
			name: "duración negativa no es tiempo",
			kind: kindElongacion,
			s:    Series{Kind: kindElongacion, DurationMs: ptrInt64(-1)},
			want: false,
		},
		{
			name: "velocidad_distancia sin distancia",
			kind: kindVelocidadDistancia,
			s:    Series{Kind: kindVelocidadDistancia, ActiveDurationMs: ptrInt64(230000)},
			want: false,
		},
		{
			name: "velocidad_distancia solo con distancia",
			kind: kindVelocidadDistancia,
			s:    Series{Kind: kindVelocidadDistancia, DistanceMeters: ptrFloat(100)},
			want: true,
		},
		{
			name: "distancia cero no es distancia",
			kind: kindVelocidadDistancia,
			s:    Series{Kind: kindVelocidadDistancia, DistanceMeters: ptrFloat(0), ActiveDurationMs: ptrInt64(12000)},
			want: false,
		},
		{
			name: "ritmo_continuo sin distancia",
			kind: kindRitmoContinuo,
			s:    Series{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(230000)},
			want: false,
		},
		{
			name: "elongacion con distancia y sin tiempo",
			kind: kindElongacion,
			s:    Series{Kind: kindElongacion, DistanceMeters: ptrFloat(100)},
			want: false,
		},
		{
			name: "liviano solo con tiempo",
			kind: kindLiviano,
			s:    Series{Kind: kindLiviano, ActiveDurationMs: ptrInt64(230000)},
			want: true,
		},
		{
			name: "liviano solo con distancia",
			kind: kindLiviano,
			s:    Series{Kind: kindLiviano, DistanceMeters: ptrFloat(2300)},
			want: true,
		},
		{
			name: "liviano sin nada",
			kind: kindLiviano,
			s:    Series{Kind: kindLiviano},
			want: false,
		},
		{
			name: "velocidad_tiempo sin distancia sigue siendo valida",
			kind: kindVelocidadTiempo,
			s:    Series{Kind: kindVelocidadTiempo, ActiveDurationMs: ptrInt64(118000)},
			want: true,
		},
		{
			name: "distancia NaN no es distancia",
			kind: kindVelocidadDistancia,
			s:    Series{Kind: kindVelocidadDistancia, ActiveDurationMs: ptrInt64(12000), DistanceMeters: ptrFloat(math.NaN())},
			want: false,
		},

		// Los legacy no entran a ningún KPI, pero tampoco rompen nada.
		{name: "legacy walking", kind: kindLegacyWalking, s: Series{Kind: kindLegacyWalking, ActiveDurationMs: ptrInt64(1800000), DistanceMeters: ptrFloat(3000)}, want: false},
		{name: "legacy jogging", kind: kindLegacyJogging, s: Series{Kind: kindLegacyJogging, ActiveDurationMs: ptrInt64(1800000), DistanceMeters: ptrFloat(5000)}, want: false},
		{name: "legacy elongation", kind: kindLegacyElongation, s: Series{Kind: kindLegacyElongation, ActiveDurationMs: ptrInt64(600000)}, want: false},
		{name: "legacy cruising", kind: kindLegacyCruising, s: Series{Kind: kindLegacyCruising, ActiveDurationMs: ptrInt64(1800000), DistanceMeters: ptrFloat(8000)}, want: false},
		{name: "legacy running", kind: kindLegacyRunning, s: Series{Kind: kindLegacyRunning, ActiveDurationMs: ptrInt64(600000), DistanceMeters: ptrFloat(1600)}, want: false},
		{name: "kind vacío", kind: "", s: Series{ActiveDurationMs: ptrInt64(600000), DistanceMeters: ptrFloat(1600)}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isValidSeries(tc.kind, tc.s))
		})
	}
}

func TestTimeQuality(t *testing.T) {
	cases := []struct {
		name string
		s    Series
		want TimeQuality
	}{
		{"con tiempo activo", Series{ActiveDurationMs: ptrInt64(230000), DurationMs: ptrInt64(250000)}, TimeQualityHigh},
		{"sin tiempo activo", Series{DurationMs: ptrInt64(250000)}, TimeQualityDegraded},
		{"tiempo activo cero cae al fallback", Series{ActiveDurationMs: ptrInt64(0), DurationMs: ptrInt64(250000)}, TimeQualityDegraded},
		{"tiempo activo negativo cae al fallback", Series{ActiveDurationMs: ptrInt64(-1), DurationMs: ptrInt64(250000)}, TimeQualityDegraded},
		{"sin ningún tiempo", Series{}, TimeQualityDegraded},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, timeQuality(tc.s))
		})
	}
}

func TestAchievedTimeMs(t *testing.T) {
	assert.Equal(t, int64(230000), achievedTimeMs(Series{ActiveDurationMs: ptrInt64(230000), DurationMs: ptrInt64(250000)}))
	assert.Equal(t, int64(250000), achievedTimeMs(Series{DurationMs: ptrInt64(250000)}))
	assert.Equal(t, int64(250000), achievedTimeMs(Series{ActiveDurationMs: ptrInt64(0), DurationMs: ptrInt64(250000)}), "un active en cero no es un tiempo: cae al total")
	assert.Equal(t, int64(0), achievedTimeMs(Series{}))
}

func TestCompletionTaxonomy(t *testing.T) {
	cases := []struct {
		name   string
		status *string
		want   Completion
	}{
		{"completa", ptrString("completa"), CompletionCompleta},
		{"partial", ptrString("partial"), CompletionPartial},
		{"skipped", ptrString("skipped"), CompletionSkipped},
		{"valor no reconocido", ptrString("saltada"), CompletionUnknown},
		{"vacío", ptrString(""), CompletionUnknown},
		{"diferente de mayúsculas", ptrString("COMPLETA"), CompletionUnknown},
		{"nulo", nil, CompletionUnknown},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, completionTaxonomy(Series{CompletionStatus: tc.status}))
		})
	}
}

func TestValidSpeeds(t *testing.T) {
	series := []Series{
		// 1000 m en 4 min -> 15 km/h
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), DistanceMeters: ptrFloat(1000), HasRunnerSession: true},
		// 1000 m en 4:10 -> 14,4 km/h
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(250000), DistanceMeters: ptrFloat(1000), HasRunnerSession: true},
		// 2000 m en 8 min -> 15 km/h
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(480000), DistanceMeters: ptrFloat(2000), HasRunnerSession: true},
		// legacy: fuera
		{Kind: kindLegacyJogging, ActiveDurationMs: ptrInt64(1800000), DistanceMeters: ptrFloat(5000), HasRunnerSession: true},
		// inválida por distancia ausente: fuera
		{Kind: kindRitmoContinuo, ActiveDurationMs: ptrInt64(240000), HasRunnerSession: true},
		// válida pero sin distancia: no tiene velocidad, así que fuera
		{Kind: kindElongacion, ActiveDurationMs: ptrInt64(600000), HasRunnerSession: true},
	}

	speeds := validSpeeds(series)

	assert.Len(t, speeds, 3)
	assert.InDelta(t, 15, speeds[0], 1e-9)
	assert.InDelta(t, 14.4, speeds[1], 1e-9)
	assert.InDelta(t, 15, speeds[2], 1e-9)
}
