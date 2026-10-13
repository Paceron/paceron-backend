package routematch

// Tests de geometría pura del route matching (D10): parseo, sampleo a pasos
// fijos y match % (100, 0 sin GPS, parcial, borde del radio).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// metrosPorGradoLatReal es el valor derivado del radio usado por el paquete
// (2*pi*6371000/360): construye distancias conocidas en los tests.
const metrosPorGradoLatReal = 111194.927

func TestParseLineString_OK(t *testing.T) {
	points, err := ParseLineString(`{"type":"LineString","coordinates":[[-64.18,-31.42],[-64.17,-31.41],[-64.16,-31.40]]}`)
	require.NoError(t, err)
	require.Len(t, points, 3)
	assert.InDelta(t, -31.42, points[0].Lat, 1e-9)
	assert.InDelta(t, -64.18, points[0].Lng, 1e-9)
}

func TestParseLineString_Errores(t *testing.T) {
	for name, raw := range map[string]string{
		"vacío":        "",
		"no es json":   "{}",
		"type ajeno":   `{"type":"Point","coordinates":[[0,0]]}`,
		"sin coords":   `{"type":"LineString","coordinates":[]}`, // LineString formalmente exige >= 2
		"coord corta":  `{"type":"LineString","coordinates":[[0]]}`,
		"coords nulls": `{"type":"LineString","coordinates":null}`,
	} {
		_, err := ParseLineString(raw)
		// null coords no falla el unmarshal pero sí el shape.
		assert.Error(t, err, name)
	}
	// LineString de un solo vértice: válido (ruta degenerada).
	points, err := ParseLineString(`{"type":"LineString","coordinates":[[0,0]]}`)
	require.NoError(t, err)
	require.Len(t, points, 1)
}

func TestSampleRoute_PasosFijos(t *testing.T) {
	// Segmento meridional de ~111 m: solo el vértice final no alcanza, hay
	// samples intermedios cada 40 m.
	route := []Point{{Lat: 0, Lng: 0}, {Lat: 111.0 / metrosPorGradoLatReal, Lng: 0}}
	samples := SampleRoute(route)
	d := route[1].Lat * metrosPorGradoLatReal
	require.Len(t, samples, 4) // inicio + samples al 40/80 m + vértice final
	assert.InDelta(t, 0, samples[0].Lat, 1e-12)
	assert.InDelta(t, 40.0/metrosPorGradoLatReal, samples[1].Lat, 1e-9)
	assert.InDelta(t, 80.0/metrosPorGradoLatReal, samples[2].Lat, 1e-9)
	assert.InDelta(t, d, samples[3].Lat*metrosPorGradoLatReal, 1e-6)

	// Segmento más corto que el paso: solo vértices.
	corto := []Point{{Lat: 0, Lng: 0}, {Lat: 0, Lng: 10.0 / metrosPorGradoLatReal}}
	assert.Len(t, SampleRoute(corto), 2)

	// Multi-segmento no pierde vértices (segmentos de ~10m y ~11m).
	multi := SampleRoute([]Point{
		{Lat: 0, Lng: 0}, {Lat: 0, Lng: 10.0 / metrosPorGradoLatReal}, {Lat: 11.0 / metrosPorGradoLatReal, Lng: 0},
	})
	assert.Equal(t, multi[len(multi)-1], Point{Lat: 11.0 / metrosPorGradoLatReal, Lng: 0})
	assert.Len(t, SampleRoute(nil), 0)
}

func TestMatchRatio_Casos(t *testing.T) {
	// Aprox 100 m de ruta norte, samples cada 40 m.
	route := []Point{{Lat: 0, Lng: 0}, {Lat: 100.0 / metrosPorGradoLatReal, Lng: 0}}
	samples := SampleRoute(route)

	// GPS cubre todos los samples (los samples mismos + tolerancia.
	gpsTotal := make([]Point, len(samples))
	for i, s := range samples {
		gpsTotal[i] = s
	}
	assert.InDelta(t, 100.0, MatchPercent(samples, 50, gpsTotal), 1e-9)

	// Sin GPS → 0.
	assert.InDelta(t, 0, MatchPercent(samples, 50, nil), 1e-9)

	// GPS parcial: solo alrededor del primer sample (radio 10 m: los samples
	// están a 40 m entre sí, vecinos fuera del radio).
	assert.InDelta(t, 25.0, MatchPercent(samples, 10, []Point{{Lat: 0, Lng: 0}}), 0.75)

	// Borde del radio: dentro a 49.9 m, fuera a 50.1 m.
	dentro := 49.9 / metrosPorGradoLatReal
	fuera := 50.1 / metrosPorGradoLatReal
	assert.Equal(t, float64(1), MatchRatio([]Point{{Lat: 0, Lng: 0}}, 50, []Point{{Lat: dentro, Lng: 0}}))
	assert.Equal(t, float64(0), MatchRatio([]Point{{Lat: 0, Lng: 0}}, 50, []Point{{Lat: fuera, Lng: 0}}))

	// Radio 0/negativo → 0.
	assert.Equal(t, float64(0), MatchRatio(samples, 0, gpsTotal))
}

func TestHaversineMeters_DistanciasConocidas(t *testing.T) {
	// 1 grado de latitud ≈ 111.19 km (radio medio de la Tierra usado acá).
	assert.InEpsilon(t, metrosPorGradoLatReal, HaversineMeters(0, 0, 1, 0), 1e-6)
	// Mismo punto → 0.
	assert.InDelta(t, 0, HaversineMeters(10, 20, 10, 20), 1e-9)
}
