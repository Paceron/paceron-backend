package dashboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBand(t *testing.T) {
	const tolerance = 0.05

	cases := []struct {
		name              string
		kind              string
		minutes           *int
		targetDistanceM   *int
		achievedTimeMs    int64
		achievedDistanceM *float64
		want              Band
	}{
		// Eje tiempo: 240 s de objetivo, 252 s = 1,05x, 264 s = 1,10x.
		{"tiempo dentro de la tolerancia", kindElongacion, ptrInt(4), nil, 240000, nil, BandMet},
		{"tiempo justo en el borde de met", kindElongacion, ptrInt(4), nil, 252000, nil, BandMet},
		{"tiempo un ms pasado el borde de met", kindElongacion, ptrInt(4), nil, 252001, nil, BandNear},
		{"tiempo justo en el borde de near", kindElongacion, ptrInt(4), nil, 264000, nil, BandNear},
		{"tiempo pasado el doble de tolerancia", kindElongacion, ptrInt(4), nil, 264001, nil, BandMissed},
		{"mejorar el tiempo es met", kindElongacion, ptrInt(4), nil, 230000, nil, BandMet},

		// Eje distancia: 1000 m de objetivo, 950 m = 0,95x, 900 m = 0,90x.
		{"distancia dentro de la tolerancia", kindVelocidadDistancia, nil, ptrInt(1000), 240000, ptrFloat(950), BandMet},
		{"distancia justo en el borde de met", kindVelocidadDistancia, nil, ptrInt(1000), 240000, ptrFloat(950), BandMet},
		{"distancia un metro pasado el borde", kindVelocidadDistancia, nil, ptrInt(1000), 240000, ptrFloat(949.99), BandNear},
		{"distancia justo en el borde de near", kindVelocidadDistancia, nil, ptrInt(1000), 240000, ptrFloat(900), BandNear},
		{"distancia pasada el doble de tolerancia", kindVelocidadDistancia, nil, ptrInt(1000), 240000, ptrFloat(899), BandMissed},
		{"ir mas lejos es met", kindVelocidadDistancia, nil, ptrInt(1000), 240000, ptrFloat(1400), BandMet},

		// Eje doble: se compara la velocidad, que va del lado de la distancia.
		{"ritmo_continuo dentro de la tolerancia", kindRitmoContinuo, ptrInt(4), ptrInt(1000), 240000, ptrFloat(1000), BandMet},
		{"ritmo_continuo corriendo mas rapido es met", kindRitmoContinuo, ptrInt(4), ptrInt(1000), 235000, ptrFloat(1000), BandMet},
		{"ritmo_continuo corriendo mas lento es near", kindRitmoContinuo, ptrInt(4), ptrInt(1000), 260000, ptrFloat(1000), BandNear},
		{"ritmo_continuo claramente lento es missed", kindRitmoContinuo, ptrInt(4), ptrInt(1000), 280000, ptrFloat(1000), BandMissed},

		// Objetivo cargado pero la serie no trae el campo de su eje.
		{"sin tiempo logrado", kindElongacion, ptrInt(4), nil, 0, nil, BandUnknown},
		{"sin distancia lograda", kindVelocidadDistancia, nil, ptrInt(1000), 240000, nil, BandUnknown},
		{"velocidad no calculable sin distancia lograda", kindRitmoContinuo, ptrInt(4), ptrInt(1000), 240000, nil, BandUnknown},

		// Sin objetivo cargado la banda es null, que no es lo mismo que unknown.
		{"sin objetivo cargado", kindRitmoContinuo, nil, nil, 240000, ptrFloat(1000), ""},
		{"sin objetivo cargado en velocidad maxima", kindVelocidadDistancia, nil, nil, 12000, ptrFloat(100), ""},
		{"kind legacy", kindLegacyRunning, ptrInt(10), ptrInt(1600), 600000, ptrFloat(1600), ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			targets := targetFromExerciseInstance(tc.minutes, tc.targetDistanceM)
			achieved := achievedFor(tc.achievedTimeMs, tc.achievedDistanceM)

			assert.Equal(t, tc.want, band(tc.kind, targets, achieved, tolerance))
		})
	}
}

func TestBandToleranceAjustable(t *testing.T) {
	// 265 s sobre un objetivo de 240 s es +10,4 %.
	targets := targetFromExerciseInstance(ptrInt(4), nil)
	achieved := achievedFor(265000, nil)

	assert.Equal(t, BandMissed, band(kindElongacion, targets, achieved, DefaultTolerance))
	assert.Equal(t, BandNear, band(kindElongacion, targets, achieved, 0.08))
	assert.Equal(t, BandMet, band(kindElongacion, targets, achieved, 0.12))
}

func TestBandCounts(t *testing.T) {
	series := []Evaluable{
		evaluable(kindElongacion, ptrInt(4), nil, 240000, nil),              // met
		evaluable(kindElongacion, ptrInt(4), nil, 255000, nil),              // near
		evaluable(kindElongacion, ptrInt(4), nil, 300000, nil),              // missed
		evaluable(kindElongacion, ptrInt(4), nil, 0, nil),                   // unknown
		evaluable(kindVelocidadDistancia, nil, nil, 240000, ptrFloat(1000)), // sin objetivo: no cuenta banda
		evaluable(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 240000, nil),  // unknown
	}

	counts := bandCounts(series, DefaultTolerance)

	assert.Equal(t, BandCounts{Met: 1, Near: 1, Missed: 1, Unknown: 2}, counts)
}

func TestBandCountsVacio(t *testing.T) {
	assert.Equal(t, BandCounts{}, bandCounts(nil, DefaultTolerance))
}
