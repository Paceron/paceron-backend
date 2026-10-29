package dashboard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runnerDay(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// runnerRow arma una serie con las magnitudes que miran las fórmulas del Home.
// No existe ningún campo de velocidad en Series: la velocidad siempre se deriva
// de distancia y tiempo logrado.
func runnerRow(kind string, date time.Time, distance *float64, timeMs int64) ScopedSeries {
	return ScopedSeries{
		Series: Series{
			Kind:             kind,
			DurationMs:       ptrInt64(timeMs),
			ActiveDurationMs: ptrInt64(timeMs),
			DistanceMeters:   distance,
		},
		SessionInstanceID: 1,
		SessionDate:       date,
		BucketStart:       date,
		AthleteUserID:     100,
		SetNumber:         1,
	}
}

func TestResolveRunnerPeriod(t *testing.T) {
	cases := []struct {
		name           string
		period         string
		reference      time.Time
		from           string
		to             string
		daysConsidered int
	}{
		{
			name:      "mes de octubre",
			period:    "month",
			reference: runnerDay(2026, time.October, 28),
			from:      "2026-10-01", to: "2026-10-31", daysConsidered: 31,
		},
		{
			name:      "febrero bisiesto",
			period:    "month",
			reference: runnerDay(2028, time.February, 10),
			from:      "2028-02-01", to: "2028-02-29", daysConsidered: 29,
		},
		{
			name:      "febrero común",
			period:    "month",
			reference: runnerDay(2026, time.February, 10),
			from:      "2026-02-01", to: "2026-02-28", daysConsidered: 28,
		},
		{
			name:      "semana ISO que cruza de mes",
			period:    "week",
			reference: runnerDay(2026, time.October, 28),
			from:      "2026-10-26", to: "2026-11-01", daysConsidered: 7,
		},
		{
			name:      "semana ISO que cruza de año",
			period:    "week",
			reference: runnerDay(2026, time.January, 1),
			from:      "2025-12-29", to: "2026-01-04", daysConsidered: 7,
		},
		{
			name:      "domingo es el último día de la semana",
			period:    "week",
			reference: runnerDay(2026, time.November, 1),
			from:      "2026-10-26", to: "2026-11-01", daysConsidered: 7,
		},
		{
			name:      "día",
			period:    "day",
			reference: runnerDay(2026, time.October, 28),
			from:      "2026-10-28", to: "2026-10-28", daysConsidered: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			period, err := ResolveRunnerPeriod(tc.period, tc.reference)

			require.NoError(t, err)
			assert.Equal(t, tc.from, FormatDate(period.From))
			assert.Equal(t, tc.to, FormatDate(period.To))
			assert.Equal(t, tc.daysConsidered, period.DaysConsidered)
			assert.Equal(t, FormatDate(tc.reference), FormatDate(period.ReferenceDate))
			assert.False(t, period.From.After(period.To), "from <= to siempre")
		})
	}
}

func TestResolveRunnerPeriod_RechazaPeriodoDesconocido(t *testing.T) {
	_, err := ResolveRunnerPeriod("year", runnerDay(2026, time.October, 28))

	assert.ErrorIs(t, err, ErrInvalidRunnerPeriod)
}

// Una fecha imposible no se normaliza en silencio: el parseo estricto del
// contrato la rechaza antes de resolver el período.
func TestRunnerPeriod_FechaImposibleNoSeNormaliza(t *testing.T) {
	_, err := time.Parse(DateLayoutISO, "2026-02-30")

	assert.Error(t, err)
}

func TestIsValidRunnerKpiTab(t *testing.T) {
	valid := []RunnerKpiTab{
		RunnerKpiTabGeneral, RunnerKpiTabRitmoContinuo,
		RunnerKpiTabVelocidadTiempo, RunnerKpiTabVelocidadDistancia,
	}
	for _, tab := range valid {
		assert.True(t, IsValidRunnerKpiTab(tab), "%s es válida", tab)
	}
	for _, tab := range []RunnerKpiTab{"", "comparacion", "GENERAL"} {
		assert.False(t, IsValidRunnerKpiTab(tab), "%q no es válida", tab)
	}
}

func TestBuildRunnerKpis_SinDatosDevuelveTodoNull(t *testing.T) {
	for _, tab := range []RunnerKpiTab{
		RunnerKpiTabGeneral, RunnerKpiTabRitmoContinuo,
		RunnerKpiTabVelocidadTiempo, RunnerKpiTabVelocidadDistancia,
	} {
		t.Run(string(tab), func(t *testing.T) {
			metrics := BuildRunnerKpis(nil, tab)

			assert.Nil(t, metrics.TotalDistanceM)
			assert.Nil(t, metrics.BestDayDistanceM)
			assert.Nil(t, metrics.BestDayDate)
			assert.Nil(t, metrics.AverageSpeedKph)
			assert.Nil(t, metrics.BestSpeedKph)
			assert.Nil(t, metrics.BestTimeMs)
			assert.Nil(t, metrics.BestDistanceM)
		})
	}
}

// El ejemplo de la spec: 5 km de ritmo continuo, 2 km de velocidad en tiempo y
// 1 km de liviano suman 8000 m en general, sin importar grupos ni roles.
func TestBuildRunnerKpis_GeneralSumaTodosLosTipos(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(5000), 1800000),
		runnerRow(kindVelocidadTiempo, runnerDay(2026, time.October, 3), ptrFloat(2000), 900000),
		runnerRow(kindLiviano, runnerDay(2026, time.October, 3), ptrFloat(1000), 600000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabGeneral)

	require.NotNil(t, metrics.TotalDistanceM)
	assert.Equal(t, 8000.0, *metrics.TotalDistanceM)
	require.NotNil(t, metrics.BestDayDistanceM)
	assert.Equal(t, 8000.0, *metrics.BestDayDistanceM)
	require.NotNil(t, metrics.BestDayDate)
	assert.Equal(t, "2026-10-03", *metrics.BestDayDate)
	// El promedio general pondera solo ritmo_continuo: 5 km en 30 min = 10 km/h.
	require.NotNil(t, metrics.AverageSpeedKph)
	assert.InDelta(t, 10.0, *metrics.AverageSpeedKph, 0.0001)
	// La mejor velocidad general sí mira todos los tipos: liviano corrió 10 km/h,
	// velocidad_tiempo 8 km/h y ritmo_continuo 10 km/h.
	require.NotNil(t, metrics.BestSpeedKph)
	assert.InDelta(t, 10.0, *metrics.BestSpeedKph, 0.0001)
	assert.Nil(t, metrics.BestTimeMs)
	assert.Nil(t, metrics.BestDistanceM)
}

func TestBuildRunnerKpis_GeneralPromedioSoloRitmoContinuo(t *testing.T) {
	rows := []ScopedSeries{
		// 10 km en 60 min = 10 km/h.
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(10000), 3600000),
		// 5 km en 15 min = 20 km/h.
		runnerRow(kindVelocidadDistancia, runnerDay(2026, time.October, 4), ptrFloat(5000), 900000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabGeneral)

	require.NotNil(t, metrics.TotalDistanceM)
	assert.Equal(t, 15000.0, *metrics.TotalDistanceM)
	require.NotNil(t, metrics.AverageSpeedKph)
	assert.InDelta(t, 10.0, *metrics.AverageSpeedKph, 0.0001,
		"la fila se rotula velocidad promedio en ritmo continuo")
	require.NotNil(t, metrics.BestSpeedKph)
	assert.InDelta(t, 20.0, *metrics.BestSpeedKph, 0.0001,
		"mejor velocidad en general no excluye tipos")
}

func TestBuildRunnerKpis_GeneralSinRitmoContinuoPromedioNull(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindVelocidadTiempo, runnerDay(2026, time.October, 3), ptrFloat(5000), 1200000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabGeneral)

	require.NotNil(t, metrics.TotalDistanceM)
	assert.Equal(t, 5000.0, *metrics.TotalDistanceM)
	assert.Nil(t, metrics.AverageSpeedKph)
	require.NotNil(t, metrics.BestSpeedKph)
	assert.InDelta(t, 15.0, *metrics.BestSpeedKph, 0.0001)
}

func TestBuildRunnerKpis_MejorDia(t *testing.T) {
	cases := []struct {
		name   string
		rows   []ScopedSeries
		amount float64
		date   string
	}{
		{
			name: "gana el día de mayor distancia",
			rows: []ScopedSeries{
				runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(8000), 2880000),
				runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 11), ptrFloat(12000), 4320000),
			},
			amount: 12000,
			date:   "2026-10-11",
		},
		{
			name: "empate resuelve por la fecha más antigua",
			rows: []ScopedSeries{
				runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 11), ptrFloat(8000), 2880000),
				runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(8000), 2880000),
			},
			amount: 8000,
			date:   "2026-10-03",
		},
		{
			name: "suma varias series del mismo día",
			rows: []ScopedSeries{
				runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(4000), 1440000),
				runnerRow(kindVelocidadTiempo, runnerDay(2026, time.October, 3), ptrFloat(4000), 1800000),
				runnerRow(kindLiviano, runnerDay(2026, time.October, 4), ptrFloat(7000), 4200000),
			},
			amount: 8000,
			date:   "2026-10-03",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := BuildRunnerKpis(tc.rows, RunnerKpiTabGeneral)

			require.NotNil(t, metrics.BestDayDistanceM)
			assert.Equal(t, tc.amount, *metrics.BestDayDistanceM)
			require.NotNil(t, metrics.BestDayDate)
			assert.Equal(t, tc.date, *metrics.BestDayDate)
		})
	}
}

// best_speed es el máximo de las velocidades promedio por serie, no el promedio
// simple (que daría 13,25) ni un porcentaje contra el objetivo.
func TestBuildRunnerKpis_MejorVelocidadPorSerie(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindVelocidadDistancia, runnerDay(2026, time.October, 3), ptrFloat(5000), 1440000), // 12,5 km/h
		runnerRow(kindVelocidadDistancia, runnerDay(2026, time.October, 4), ptrFloat(7000), 1800000), // 14 km/h
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabVelocidadDistancia)

	require.NotNil(t, metrics.BestSpeedKph)
	assert.InDelta(t, 14.0, *metrics.BestSpeedKph, 0.0001)
}

func TestBuildRunnerKpis_PromedioPonderadoPorDistancia(t *testing.T) {
	rows := []ScopedSeries{
		// 21 km a 5 km/h = 4,2 h.
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(21000), 15120000),
		// 4 km a 15 km/h = 16 min.
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 4), ptrFloat(4000), 960000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabRitmoContinuo)

	require.NotNil(t, metrics.AverageSpeedKph)
	assert.InDelta(t, 6.6, *metrics.AverageSpeedKph, 0.0001,
		"la media pondera por distancia, no es el promedio simple de 10 km/h")
}

func TestBuildRunnerKpis_PestanasFiltranPorKind(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(5000), 1800000),     // 10 km/h
		runnerRow(kindVelocidadTiempo, runnerDay(2026, time.October, 4), ptrFloat(3000), 720000),    // 15 km/h
		runnerRow(kindVelocidadDistancia, runnerDay(2026, time.October, 5), ptrFloat(2000), 600000), // 12 km/h
	}

	t.Run("ritmo_continuo", func(t *testing.T) {
		metrics := BuildRunnerKpis(rows, RunnerKpiTabRitmoContinuo)

		require.NotNil(t, metrics.TotalDistanceM)
		assert.Equal(t, 5000.0, *metrics.TotalDistanceM)
		require.NotNil(t, metrics.BestDistanceM)
		assert.Equal(t, 5000.0, *metrics.BestDistanceM)
		assert.InDelta(t, 10.0, *metrics.BestSpeedKph, 0.0001)
		assert.Nil(t, metrics.BestDayDistanceM)
		assert.Nil(t, metrics.BestTimeMs)
	})

	t.Run("velocidad_tiempo", func(t *testing.T) {
		metrics := BuildRunnerKpis(rows, RunnerKpiTabVelocidadTiempo)

		require.NotNil(t, metrics.TotalDistanceM)
		assert.Equal(t, 3000.0, *metrics.TotalDistanceM)
		require.NotNil(t, metrics.BestTimeMs)
		assert.Equal(t, int64(720000), *metrics.BestTimeMs)
		require.NotNil(t, metrics.AverageSpeedKph)
		assert.InDelta(t, 15.0, *metrics.AverageSpeedKph, 0.0001)
		assert.Nil(t, metrics.BestSpeedKph, "best_speed no aplica a velocidad_tiempo")
		assert.Nil(t, metrics.BestDistanceM)
		assert.Nil(t, metrics.BestDayDistanceM)
	})

	t.Run("velocidad_distancia", func(t *testing.T) {
		metrics := BuildRunnerKpis(rows, RunnerKpiTabVelocidadDistancia)

		require.NotNil(t, metrics.TotalDistanceM)
		assert.Equal(t, 2000.0, *metrics.TotalDistanceM)
		require.NotNil(t, metrics.BestSpeedKph)
		assert.InDelta(t, 12.0, *metrics.BestSpeedKph, 0.0001)
		require.NotNil(t, metrics.AverageSpeedKph)
		assert.InDelta(t, 12.0, *metrics.AverageSpeedKph, 0.0001)
		assert.Nil(t, metrics.BestTimeMs)
		assert.Nil(t, metrics.BestDistanceM)
		assert.Nil(t, metrics.BestDayDistanceM)
	})
}

func TestBuildRunnerKpis_BestTimeEsElMinimo(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindVelocidadTiempo, runnerDay(2026, time.October, 3), nil, 540000),
		runnerRow(kindVelocidadTiempo, runnerDay(2026, time.October, 4), nil, 420000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabVelocidadTiempo)

	require.NotNil(t, metrics.BestTimeMs)
	assert.Equal(t, int64(420000), *metrics.BestTimeMs)
	assert.Nil(t, metrics.TotalDistanceM, "sin distancia no hay total, no es 0")
}

// Una serie sin tiempo no aporta velocidad, pero su distancia sí suma.
func TestBuildRunnerKpis_SerieSinTiempoNoAportaVelocidad(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(3000), 0),
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 4), ptrFloat(2000), 600000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabGeneral)

	require.NotNil(t, metrics.TotalDistanceM)
	assert.Equal(t, 5000.0, *metrics.TotalDistanceM)
	require.NotNil(t, metrics.AverageSpeedKph)
	assert.InDelta(t, 12.0, *metrics.AverageSpeedKph, 0.0001)
	require.NotNil(t, metrics.BestSpeedKph)
	assert.InDelta(t, 12.0, *metrics.BestSpeedKph, 0.0001)
}

// Una serie sin distancia no hace 0 al total: no hay dato.
func TestBuildRunnerKpis_SinDistanciaEsNullNoCero(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindVelocidadTiempo, runnerDay(2026, time.October, 3), nil, 300000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabGeneral)

	assert.Nil(t, metrics.TotalDistanceM)
	assert.Nil(t, metrics.BestDayDistanceM)
	assert.Nil(t, metrics.BestDayDate)
	assert.Nil(t, metrics.AverageSpeedKph)
	assert.Nil(t, metrics.BestSpeedKph)
	assert.Nil(t, metrics.BestDistanceM)
}

// El cero es un resultado real cuando hay una distancia registrada en cero; no
// convierte la métrica en null ni descarta la serie.
func TestBuildRunnerKpis_CeroReal(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(0), 300000),
		runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 4), ptrFloat(3000), 900000),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabGeneral)

	require.NotNil(t, metrics.TotalDistanceM)
	assert.Equal(t, 3000.0, *metrics.TotalDistanceM)

	onlyZero := BuildRunnerKpis(rows[:1], RunnerKpiTabGeneral)
	require.NotNil(t, onlyZero.TotalDistanceM)
	assert.Equal(t, 0.0, *onlyZero.TotalDistanceM)
}

func TestBuildRunnerKpis_VelocidadDistanciaSinTiempoNoTieneVelocidad(t *testing.T) {
	rows := []ScopedSeries{
		runnerRow(kindVelocidadDistancia, runnerDay(2026, time.October, 3), ptrFloat(5000), 0),
	}

	metrics := BuildRunnerKpis(rows, RunnerKpiTabVelocidadDistancia)

	require.NotNil(t, metrics.TotalDistanceM)
	assert.Equal(t, 5000.0, *metrics.TotalDistanceM)
	assert.Nil(t, metrics.AverageSpeedKph)
	assert.Nil(t, metrics.BestSpeedKph)
}

func TestBuildRunnerKpis_UmbralDeVelocidadNoSeLeeDeLaInstancia(t *testing.T) {
	// La serie declara un objetivo de 1 km en 1 min (60 km/h) pero corrió 1 km en
	// 10 min (6 km/h). La métrica tiene que usar lo logrado, nunca el objetivo ni
	// una velocidad de instancia.
	row := runnerRow(kindRitmoContinuo, runnerDay(2026, time.October, 3), ptrFloat(1000), 600000)
	row.TargetMinutes = ptrInt(1)
	row.TargetDistanceM = ptrInt(1000)

	metrics := BuildRunnerKpis([]ScopedSeries{row}, RunnerKpiTabRitmoContinuo)

	require.NotNil(t, metrics.AverageSpeedKph)
	assert.InDelta(t, 6.0, *metrics.AverageSpeedKph, 0.0001)
	require.NotNil(t, metrics.BestSpeedKph)
	assert.InDelta(t, 6.0, *metrics.BestSpeedKph, 0.0001)
}
