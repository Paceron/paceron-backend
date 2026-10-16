package dashboard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scopedRow arma una serie del scope con las magnitudes justas para que cada test
// fije a mano lo que las fórmulas van a mirar.
func scopedRow(kind string, minutes, distanceM *int, timeMs int64, achievedM *float64, date time.Time) ScopedSeries {
	return ScopedSeries{
		Series: Series{
			Kind:             kind,
			DurationMs:       &timeMs,
			ActiveDurationMs: &timeMs,
			DistanceMeters:   achievedM,
			HasRunnerSession: true,
		},
		SessionInstanceID: 1,
		SessionDate:       date,
		BucketStart:       date,
		AthleteUserID:     100,
		SetNumber:         1,
		TargetMinutes:     minutes,
		TargetDistanceM:   distanceM,
	}
}

func day(n int) time.Time {
	return time.Date(2026, 9, n, 0, 0, 0, 0, time.UTC)
}

// Ritmo continuo de 1000 m en 4 min = 15 km/h objetivo. La serie del test corre
// 1000 m en 3:55 = 15,27 km/h, o sea +1,8 % contra el objetivo.
func ritmoRows(t *testing.T, count int, targetMinutes int, achievedMs int64) []ScopedSeries {
	t.Helper()
	rows := make([]ScopedSeries, 0, count)
	for i := 0; i < count; i++ {
		rows = append(rows, scopedRow(kindRitmoContinuo, ptrInt(targetMinutes), ptrInt(1000),
			achievedMs, ptrFloat(1000), day(1+i)))
	}
	return rows
}

func TestBuildSummary_VerificacionAritmeticaDelEjemplo(t *testing.T) {
	// El ejemplo de la spec: 68 series válidas, 42 cumplidas = 61,8 %, con el delta
	// medido sobre la velocidad —corriendo más lento de lo previsto, negativo—.
	// Objetivo de 1000 m en 8 min (480 s): el umbral `met` con 5 % de tolerancia
	// son 504 s, así que las 42 primeras cumplen y las 26 últimas no.
	rows := make([]ScopedSeries, 0, 68)
	for i := 0; i < 68; i++ {
		// Umbral `met` con objetivo de 8 min (480 s) y tolerancia 0,05: 504 s.
		timeMs := int64(560000)
		if i < 42 {
			timeMs = 500000
		}
		rows = append(rows, scopedRow(kindRitmoContinuo, ptrInt(8), ptrInt(1000), timeMs, ptrFloat(1000), day(1)))
	}

	summary := BuildSummary(rows, DefaultTolerance, 1)

	assert.Equal(t, string(AxisDistanceAndTime), summary.Axis)
	assert.True(t, summary.HasTarget)
	assert.Equal(t, 68, summary.Totals.Series)
	assert.Equal(t, 68, summary.Totals.ValidSeries)
	assert.Equal(t, 68, summary.SeriesCount)
	require.NotNil(t, summary.Totals.ComplianceRate)
	assert.InDelta(t, 61.76, *summary.Totals.ComplianceRate, 0.1, "42/68 = 61,8 %")
	require.NotNil(t, summary.Totals.DistanceM)
	assert.InDelta(t, 68000, *summary.Totals.DistanceM, 1)
	require.NotNil(t, summary.Totals.TimeMs)
	assert.Equal(t, int64(35560000), *summary.Totals.TimeMs)
	// 68 000 m en 9 h 52 min dan 6,9 km/h, y el objetivo de 1000 m en 8 min son
	// 7,5 km/h: se corrió más lento de lo previsto, así que el delta es negativo.
	require.NotNil(t, summary.Totals.AchievedSpeedKph)
	assert.InDelta(t, 6.9, *summary.Totals.AchievedSpeedKph, 0.1)
	require.NotNil(t, summary.Totals.DeltaPct)
	assert.Less(t, *summary.Totals.DeltaPct, 0.0)
	assert.Equal(t, 1, summary.DataQuality.OpenSessions)
	assert.InDelta(t, 0.05, summary.ToleranceApplied, 0.0001)
	assert.Equal(t, UnitsBlock, summary.Units)
}

// El resumen no inventa valores donde no hay dato: sin series válidas, todo lo
// no calculable viene null y no 0.
func TestBuildSummary_SinSeriesDevuelveNull(t *testing.T) {
	summary := BuildSummary(nil, DefaultTolerance, 0)

	assert.Nil(t, summary.Totals.DistanceM)
	assert.Nil(t, summary.Totals.TimeMs)
	assert.Nil(t, summary.Totals.AchievedSpeedKph)
	assert.Nil(t, summary.Totals.TargetSpeedKph)
	assert.Nil(t, summary.Totals.DeltaPct)
	assert.Nil(t, summary.Totals.ComplianceRate)
	assert.Nil(t, summary.Totals.BestTimeMs)
	assert.Nil(t, summary.Consistency.CV)
	assert.Nil(t, summary.Trend.TrendPct)
	assert.Equal(t, TrendStable, summary.Trend.Verdict)
	assert.True(t, summary.DataQuality.InsufficientData)
	// El bloque está presente aunque todo sea cero: el cliente distingue "no hay
	// datos" de "los datos están limpios".
	assert.Equal(t, DataQuality{InsufficientData: true}, summary.DataQuality)
}

// Una elongación sin series viene con los tres null que exige la spec, no en 0.
func TestBuildSummary_ElongacionSinSeries(t *testing.T) {
	summary := BuildSummary(nil, DefaultTolerance, 0)

	assert.Nil(t, summary.Totals.AchievedSpeedKph)
	assert.Nil(t, summary.Totals.DeltaPct)
	assert.Nil(t, summary.Totals.ComplianceRate)
}

// axis y has_target vienen del dato, no del kind: un velocidad_distancia sin
// distancia sigue siendo eje distance con has_target false.
func TestBuildSummary_EjeVieneDelDato(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindVelocidadDistancia, nil, nil, 15000, ptrFloat(100), day(1)),
	}

	summary := BuildSummary(rows, DefaultTolerance, 0)

	assert.Equal(t, string(AxisDistance), summary.Axis)
	assert.False(t, summary.HasTarget)
	assert.Nil(t, summary.Totals.TargetSpeedKph)
	assert.Nil(t, summary.Totals.DeltaPct)
	assert.Nil(t, summary.Totals.ComplianceRate)
	// El mejor tiempo y la velocidad sí se calculan: el objetivo no es lo único que
	// se deriva de las series.
	require.NotNil(t, summary.Totals.BestTimeMs)
	assert.Equal(t, int64(15000), *summary.Totals.BestTimeMs)
	require.NotNil(t, summary.Totals.AchievedSpeedKph)
	assert.InDelta(t, 24.0, *summary.Totals.AchievedSpeedKph, 0.01)
}

func TestBuildSummary_LivianoSoloConDistanciaEsEjeDistancia(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindLiviano, nil, ptrInt(3000), 900000, ptrFloat(3000), day(1)),
	}

	summary := BuildSummary(rows, DefaultTolerance, 0)

	assert.Equal(t, string(AxisDistance), summary.Axis)
	assert.True(t, summary.HasTarget)
	require.NotNil(t, summary.Totals.DeltaPct)
	assert.InDelta(t, 0.0, *summary.Totals.DeltaPct, 0.01, "cumplió exacto = 0 %")
}

// El signo del delta se invierte en el eje temporal: 240 s de objetivo con 230 s
// logrados es +4,2 %, no −4,2 %.
func TestBuildSummary_DeltaDelEjeTemporalInvierteElSigno(t *testing.T) {
	faster := BuildSummary([]ScopedSeries{
		scopedRow(kindVelocidadTiempo, ptrInt(4), nil, 230000, nil, day(1)),
	}, DefaultTolerance, 0)
	require.NotNil(t, faster.Totals.DeltaPct)
	assert.InDelta(t, 4.17, *faster.Totals.DeltaPct, 0.1, "230 s contra 240 s es +4,2 %")

	slower := BuildSummary([]ScopedSeries{
		scopedRow(kindVelocidadTiempo, ptrInt(4), nil, 250000, nil, day(1)),
	}, DefaultTolerance, 0)
	require.NotNil(t, slower.Totals.DeltaPct)
	assert.InDelta(t, -4.17, *slower.Totals.DeltaPct, 0.1, "250 s contra 240 s es −4,2 %")
}

// La tolerancia de la consulta se usa para clasificar, no la default: 240 s de
// objetivo con 250 s logrados cumple con 5 % y no con 1 %.
func TestBuildSummary_UsaLaToleranciaDeLaConsulta(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindVelocidadTiempo, ptrInt(4), nil, 250000, nil, day(1)),
	}

	// 250 s contra el objetivo de 240 s (4 min) pasa el umbral de 5 % (252 s) y
	// no el de 1 % (242,4 s): la clasificación tiene que usar la tolerancia que
	// vino en la request, no la default.
	assert.InDelta(t, 100, *BuildSummary(rows, DefaultTolerance, 0).Totals.ComplianceRate, 0.01)
	assert.InDelta(t, 0, *BuildSummary(rows, 0.01, 0).Totals.ComplianceRate, 0.01)
}

// Las series no calculables quedan fuera de los promedios pero se cuentan: 9 de 10
// válidas, invalid_series = 1.
func TestBuildSummary_UnaSerieInvalidaNoInvalidaElResto(t *testing.T) {
	rows := ritmoRows(t, 10, 5, 230000)
	rows = append(rows, scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 0, nil, day(20)))

	summary := BuildSummary(rows, DefaultTolerance, 0)

	assert.Equal(t, 11, summary.Totals.Series)
	assert.Equal(t, 10, summary.Totals.ValidSeries)
	assert.Equal(t, 1, summary.DataQuality.InvalidSeries)
}

func TestBuildSummary_DataQualityCuentaDegradadasYDesconocidas(t *testing.T) {
	rows := ritmoRows(t, 3, 5, 230000)
	for i := range rows {
		rows[i].Series.ActiveDurationMs = nil
		rows[i].CompletionStatus = ptrString("saltada")
	}

	summary := BuildSummary(rows, DefaultTolerance, 3)

	assert.Equal(t, 3, summary.DataQuality.DegradedTime)
	assert.Equal(t, 3, summary.DataQuality.UnknownStatus, "completion_status desconocido no descarta la serie")
	assert.Equal(t, 3, summary.Totals.ValidSeries)
}

// Un kind legacy no entra a ningún KPI: no produce estadísticas y no rompe.
func TestBuildSummary_KindLegacyNoEntra(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindLegacyRunning, ptrInt(5), ptrInt(1000), 230000, ptrFloat(1000), day(1)),
	}

	summary := BuildSummary(rows, DefaultTolerance, 0)

	assert.Equal(t, string(AxisNone), summary.Axis)
	assert.False(t, summary.HasTarget)
	assert.Equal(t, 0, summary.Totals.ValidSeries)
	assert.Equal(t, 1, summary.Totals.Series)
	assert.Equal(t, 1, summary.DataQuality.InvalidSeries)
	assert.Nil(t, summary.Totals.DeltaPct)
	assert.Nil(t, summary.Totals.ComplianceRate)
}

// kind agrega ponderado por volumen: el 21 km a 5 km/h pesa mucho más que el
// 4 km a 15 km/h. Con media simple darían 10 km/h.
func TestBuildSummary_KindsAgregaPonderadoPorDistancia(t *testing.T) {
	rows := []ScopedSeries{
		// 21 000 m en 4:12 (15120 s) ≈ 5 km/h.
		scopedRow(kindRitmoContinuo, ptrInt(252), ptrInt(21000), 15120000, ptrFloat(21000), day(1)),
		// 4 000 m en 0:16 (960 s) = 15 km/h.
		scopedRow(kindRitmoContinuo, ptrInt(16), ptrInt(4000), 960000, ptrFloat(4000), day(2)),
	}

	summary := BuildSummary(rows, DefaultTolerance, 0)

	require.NotNil(t, summary.Totals.AchievedSpeedKph)
	// Ponderado: (21000·5.0 + 4000·15.0) / 25000 = 6,6 km/h.
	assert.InDelta(t, 6.6, *summary.Totals.AchievedSpeedKph, 0.05)
	// Con media simple daría 10 km/h: el resultado del volumen, no el promedio de
	// las dos velocidades, es el que importa.
	assert.NotEqual(t, 10.0, *summary.Totals.AchievedSpeedKph)
}

// ---------- timeseries ----------

func TestBucketStart_AnclajesDeCalendario(t *testing.T) {
	// 2026-06-10 es miércoles: la semana arranca el lunes 2026-06-08.
	wednesday := time.Date(2026, 6, 10, 23, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-06-08", BucketStart(wednesday, "week").Format(DateLayoutISO))
	// El lunes mismo no se corre para el lunes anterior.
	monday := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-06-08", BucketStart(monday, "week").Format(DateLayoutISO))
	sunday := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "2026-06-08", BucketStart(sunday, "week").Format(DateLayoutISO),
		"el domingo pertenece a la semana que arranca en lunes")

	assert.Equal(t, "2026-06-10", BucketStart(wednesday, "session").Format(DateLayoutISO))
	assert.Equal(t, "2026-06-01", BucketStart(wednesday, "month").Format(DateLayoutISO))
	assert.Equal(t, "2026-04-01", BucketStart(wednesday, "quarter").Format(DateLayoutISO))

	// Trimestres: enero, abril, julio y octubre.
	for month, expected := range map[int]string{1: "2026-01-01", 3: "2026-01-01", 4: "2026-04-01", 6: "2026-04-01",
		7: "2026-07-01", 9: "2026-07-01", 10: "2026-10-01", 12: "2026-10-01"} {
		date := time.Date(2026, time.Month(month), 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, expected, BucketStart(date, "quarter").Format(DateLayoutISO))
	}
}

func TestBuildTimeSeries_BucketsOrdenadosYConBandCounts(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 235000, ptrFloat(1000), day(1)),
		scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 260000, ptrFloat(950), day(2)),
		scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 235000, ptrFloat(1000), day(20)),
	}

	series := BuildTimeSeries(rows, "month", DefaultTolerance, 0)

	assert.Equal(t, "month", series.Granularity)
	require.Len(t, series.Buckets, 1)
	bucket := series.Buckets[0]
	assert.Equal(t, "2026-09-01", bucket.PeriodStart)
	assert.Equal(t, 3, bucket.Series)
	assert.Equal(t, 3, bucket.ValidSeries)
	// En el eje doble la banda se mide sobre la velocidad derivada: el objetivo es
	// 1000 m en 240 s (15 km/h) y con tolerancia 0,05 `met` es ≥ 14,25 km/h y
	// `near` ≥ 13,5 km/h. Las dos series de 235 s cumplen; la de 950 m en 260 s
	// (13,15 km/h) queda en `missed`.
	assert.Equal(t, 2, bucket.BandCounts.Met)
	assert.Equal(t, 0, bucket.BandCounts.Near)
	assert.Equal(t, 1, bucket.BandCounts.Missed)
	require.NotNil(t, bucket.TargetSpeedKph)
	require.NotNil(t, bucket.DeltaPct)
	assert.Equal(t, UnitsBlock, series.Units)
}

func TestBuildTimeSeries_BucketSinObjetivoVieneNull(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindVelocidadDistancia, nil, nil, 15000, ptrFloat(100), day(1)),
	}

	series := BuildTimeSeries(rows, "session", DefaultTolerance, 0)

	require.Len(t, series.Buckets, 1)
	bucket := series.Buckets[0]
	assert.Nil(t, bucket.TargetSpeedKph)
	assert.Nil(t, bucket.DeltaPct)
	assert.Equal(t, BandCounts{}, bucket.BandCounts, "sin objetivo no hay banda que clasificar")
}

func TestBuildTimeSeries_GranularidadSemanalAgruparPorLunes(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 235000, ptrFloat(1000), time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)),
		scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 235000, ptrFloat(1000), time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)),
		scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 235000, ptrFloat(1000), time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)),
	}

	series := BuildTimeSeries(rows, "week", DefaultTolerance, 0)

	require.Len(t, series.Buckets, 2)
	assert.Equal(t, "2026-06-01", series.Buckets[0].PeriodStart)
	assert.Equal(t, 2, series.Buckets[0].Series)
	assert.Equal(t, "2026-06-08", series.Buckets[1].PeriodStart)
	assert.Equal(t, 1, series.Buckets[1].Series)
}

// ---------- detalle de series ----------

func TestBuildSeriesDetail_SerieConObjetivo(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 235000, ptrFloat(1000), day(1)),
	}

	response := BuildSeriesDetail(rows, DefaultTolerance, 1, 20, DataQuality{})

	assert.Equal(t, string(AxisDistanceAndTime), response.Axis)
	assert.True(t, response.HasTarget)
	require.Len(t, response.Items, 1)
	item := response.Items[0]
	assert.Equal(t, "2026-09-01", item.SessionDate)
	assert.Equal(t, string(TimeQualityHigh), item.TimeQuality)
	require.NotNil(t, item.MetTarget)
	assert.True(t, *item.MetTarget)
	require.NotNil(t, item.Band)
	assert.Equal(t, string(BandMet), *item.Band)
	require.NotNil(t, item.SpeedKph)
	assert.InDelta(t, 15.3, *item.SpeedKph, 0.05)
	assert.InDelta(t, 0.05, item.ToleranceApplied, 0.0001)
}

func TestBuildSeriesDetail_SerieSinObjetivoNoTieneBanda(t *testing.T) {
	rows := []ScopedSeries{
		scopedRow(kindVelocidadDistancia, nil, nil, 15000, ptrFloat(100), day(1)),
	}

	response := BuildSeriesDetail(rows, DefaultTolerance, 1, 20, DataQuality{})

	item := response.Items[0]
	assert.Nil(t, item.MetTarget, "sin objetivo met_target es null, no false")
	assert.Nil(t, item.DeltaPct)
	assert.Nil(t, item.Band)
}

func TestBuildSeriesDetail_SerieDegradada(t *testing.T) {
	row := scopedRow(kindRitmoContinuo, ptrInt(4), ptrInt(1000), 250000, ptrFloat(1000), day(1))
	row.Series.ActiveDurationMs = nil

	response := BuildSeriesDetail([]ScopedSeries{row}, DefaultTolerance, 1, 20, DataQuality{})

	assert.Equal(t, string(TimeQualityDegraded), response.Items[0].TimeQuality)
	assert.Equal(t, int64(250000), *response.Items[0].DurationMs)
}

// ---------- leaderboard ----------

func leaderboardEntries(t *testing.T) []LeaderboardEntry {
	t.Helper()
	build := func(id int64, name string, rows []ScopedSeries) LeaderboardEntry {
		return NewLeaderboardEntry(id, name, name, rows, DefaultTolerance)
	}
	return []LeaderboardEntry{
		build(1, "Ana", ritmoRows(t, 3, 5, 240000)),
		build(2, "Beto", ritmoRows(t, 3, 5, 220000)),
		build(3, "Caro", ritmoRows(t, 3, 5, 260000)),
	}
}

func TestBuildLeaderboard_DefaultPonePrimeroAlMasRapido(t *testing.T) {
	response := BuildLeaderboard(leaderboardEntries(t), SortBestTimeMs, OrderAsc, 1, 20, 2, DataQuality{})

	require.Len(t, response.Items, 3)
	assert.Equal(t, int64(2), response.Items[0].AthleteUserID, "menor tiempo primero")
	assert.Equal(t, SortBestTimeMs, response.Sort)
	assert.Equal(t, OrderAsc, response.Order)
	assert.Equal(t, 1, response.Items[0].Rank)
	assert.Equal(t, 1, response.Items[0].Position)
	assert.True(t, response.Items[0].IsSelf)
	assert.Equal(t, 3, response.Total)
}

// position es la posición GLOBAL aunque la fila no esté en la página actual, y usa
// exactamente el mismo criterio que el ORDER BY.
func TestBuildLeaderboard_PositionGlobalFueraDeLaPagina(t *testing.T) {
	entries := make([]LeaderboardEntry, 0, 30)
	for i := int64(1); i <= 30; i++ {
		row := scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 300000-int64(i)*1000, ptrFloat(1000), day(1))
		entries = append(entries, NewLeaderboardEntry(i, "N", "A", []ScopedSeries{row}, DefaultTolerance))
	}

	response := BuildLeaderboard(entries, SortBestTimeMs, OrderAsc, 1, 10, 99, DataQuality{})

	require.Len(t, response.Items, 10)
	assert.Equal(t, 30, response.Total)
	// El corredor 99 no está en la página: no hay items suyos, y el máximo position
	// que ve la página es coherente con el corte.
	assert.Equal(t, 1, response.Items[0].Position)
	assert.Equal(t, 10, response.Items[9].Position)

	lastPage := BuildLeaderboard(entries, SortBestTimeMs, OrderAsc, 3, 10, 99, DataQuality{})
	require.Len(t, lastPage.Items, 10)
	// La posición global no se reinicia por página: el primer item de la última
	// página sigue siendo el 21 global, no el 1.
	assert.Equal(t, 21, lastPage.Items[0].Position)
	assert.Equal(t, 30, lastPage.Items[9].Position)
	// Una página más allá del total no rompe: viene vacía.
	assert.Empty(t, BuildLeaderboard(entries, SortBestTimeMs, OrderAsc, 4, 10, 99, DataQuality{}).Items)
}

// RANK(): los empates comparten posición y la siguiente salta.
func TestBuildLeaderboard_EmpatesCompartenPosicion(t *testing.T) {
	same := func(id int64, name string, timeMs int64, distance float64) LeaderboardEntry {
		row := scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), timeMs, ptrFloat(distance), day(1))
		return NewLeaderboardEntry(id, name, name, []ScopedSeries{row}, DefaultTolerance)
	}

	response := BuildLeaderboard([]LeaderboardEntry{
		same(1, "Ana", 230000, 1000),
		same(2, "Beto", 230000, 1000),
		same(3, "Caro", 230000, 1000),
	}, SortBestTimeMs, OrderAsc, 1, 20, 0, DataQuality{})

	require.Len(t, response.Items, 3)
	for _, item := range response.Items {
		assert.Equal(t, 1, item.Position, "los tres empatan en el mejor tiempo")
	}
}

// El desempate es determinista: menor tiempo, luego mayor distancia, luego
// alfabético. Dos requests tienen que devolver el mismo orden.
func TestBuildLeaderboard_DesempateDeterminista(t *testing.T) {
	entry := func(id int64, name string, timeMs int64, distance float64) LeaderboardEntry {
		row := scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), timeMs, ptrFloat(distance), day(1))
		return NewLeaderboardEntry(id, name, name, []ScopedSeries{row}, DefaultTolerance)
	}
	// Ana y Beto empatan en tiempo; Ana corrió más lejos, así que va primero.
	entries := []LeaderboardEntry{
		entry(2, "Beto", 230000, 900),
		entry(1, "Ana", 230000, 1000),
		entry(3, "Caro", 230000, 1000),
	}

	first := BuildLeaderboard(entries, SortBestTimeMs, OrderAsc, 1, 20, 0, DataQuality{})
	assert.Equal(t, int64(1), first.Items[0].AthleteUserID, "mayor distancia")
	assert.Equal(t, int64(3), first.Items[1].AthleteUserID, "desempate alfabético")

	reversed := BuildLeaderboard([]LeaderboardEntry{entries[2], entries[1], entries[0]},
		SortBestTimeMs, OrderAsc, 1, 20, 0, DataQuality{})
	assert.Equal(t, first.Items[0].AthleteUserID, reversed.Items[0].AthleteUserID,
		"el orden de entrada no cambia el resultado")
}

func TestBuildLeaderboard_OrdenPorVelocidadYDesc(t *testing.T) {
	response := BuildLeaderboard(leaderboardEntries(t), SortMeanSpeedKph, OrderDesc, 1, 20, 0, DataQuality{})

	require.Len(t, response.Items, 3)
	assert.Equal(t, SortMeanSpeedKph, response.Sort)
	assert.Equal(t, OrderDesc, response.Order)
	assert.Equal(t, 1, response.Items[0].Position, "descendente por velocidad: el más rápido primero")
	// Con mean_speed_kph ascendente pondría primero al más lento: por eso el
	// default del endpoint es best_time_ms asc.
	ascending := BuildLeaderboard(leaderboardEntries(t), SortMeanSpeedKph, OrderAsc, 1, 20, 0, DataQuality{})
	assert.Equal(t, int64(3), ascending.Items[0].AthleteUserID)
}

// ---------- comparaciones ----------

func TestBuildComparisons_UnaFilaPorCorredor(t *testing.T) {
	rows := ritmoRows(t, 3, 5, 240000)
	ana := append([]ScopedSeries(nil), rows...)
	for i := range ana {
		ana[i].AthleteUserID = 1
	}
	beto := append([]ScopedSeries(nil), rows...)
	for i := range beto {
		beto[i].AthleteUserID = 2
	}

	response := BuildComparisons(append(ana, beto...), DefaultTolerance, DataQuality{})

	require.Len(t, response.Series, 2)
	assert.Equal(t, int64(1), response.Series[0].AthleteUserID)
	assert.Equal(t, int64(2), response.Series[1].AthleteUserID)
	assert.Equal(t, 3, response.Series[0].ValidSeries)
	require.NotNil(t, response.Series[0].BestTimeMs)
	assert.Equal(t, UnitsBlock, response.Units)
}

func TestBuildComparisons_SinVolumenVieneNull(t *testing.T) {
	response := BuildComparisons(nil, DefaultTolerance, DataQuality{})

	assert.Empty(t, response.Series)
}

// ---------- períodos ----------

func TestDerivePreviousPeriod(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	previousFrom, previousTo := DerivePreviousPeriod(from, to)

	assert.Equal(t, "2026-08-02", FormatDate(previousFrom))
	assert.Equal(t, "2026-08-31", FormatDate(previousTo))

	weekFrom := time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)
	weekTo := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	previousFrom, previousTo = DerivePreviousPeriod(weekFrom, weekTo)
	assert.Equal(t, "2026-06-01", FormatDate(previousFrom))
	assert.Equal(t, "2026-06-07", FormatDate(previousTo))
}

func TestBuildPeriods_DeltaConSignosDeclarados(t *testing.T) {
	currentFrom := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	currentTo := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	previousFrom, previousTo := DerivePreviousPeriod(currentFrom, currentTo)

	// Objetivo 1000 m en 5 min (300 s): con 5 % de tolerancia el umbral `met` son
	// 315 s. Actual: 3 de 5 cumplen (60 %) y el mejor tiempo es 229 s.
	current := []ScopedSeries{
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 229000, ptrFloat(1000), day(1)),
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 290000, ptrFloat(1000), day(2)),
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 300000, ptrFloat(1000), day(3)),
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 320000, ptrFloat(1000), day(4)),
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 400000, ptrFloat(1000), day(5)),
	}
	// Anterior: 3 de 3 cumplen (100 %) y el mejor tiempo es 233 s.
	previous := []ScopedSeries{
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 233000, ptrFloat(1000), time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)),
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 300000, ptrFloat(1000), time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC)),
		scopedRow(kindRitmoContinuo, ptrInt(5), ptrInt(1000), 310000, ptrFloat(1000), time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)),
	}

	response := BuildPeriods(currentFrom, currentTo, previousFrom, previousTo,
		current, previous, DefaultTolerance, DataQuality{})

	require.NotNil(t, response.Previous)
	require.NotNil(t, response.Delta)
	// 233 s → 229 s: (233000 − 229000) / 233000 = +1,7 %, positivo porque mejoró.
	require.NotNil(t, response.Delta.BestTimePct)
	assert.InDelta(t, 1.72, *response.Delta.BestTimePct, 0.1)
	require.NotNil(t, response.Delta.CompliancePP)
	// compliance_pp son puntos porcentuales: una diferencia de puntos, no un
	// porcentaje relativo de la tasa.
	assert.InDelta(t, -40, *response.Delta.CompliancePP, 0.1)
	assert.Equal(t, TrendRegressing, response.Verdict)
	assert.Equal(t, 2, response.Delta.Series)
	assert.Equal(t, "2026-09-01", response.Current.From)
	assert.Equal(t, "2026-08-02", response.Previous.From)
	assert.Equal(t, UnitsBlock, response.Units)
}

func TestBuildPeriods_PeriodoAnteriorSinDatosVieneNull(t *testing.T) {
	currentFrom := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	currentTo := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	previousFrom, previousTo := DerivePreviousPeriod(currentFrom, currentTo)

	response := BuildPeriods(currentFrom, currentTo, previousFrom, previousTo,
		ritmoRows(t, 3, 5, 230000), nil, DefaultTolerance, DataQuality{})

	assert.NotNil(t, response.Current)
	assert.Nil(t, response.Previous, "sin datos no hay con qué comparar")
	assert.Nil(t, response.Delta)
	assert.Equal(t, TrendStable, response.Verdict)
}

func TestBuildPeriods_Verdict(t *testing.T) {
	// Objetivo de 4 min: 3:20 (200 s) cumple, 5:00 (300 s) no cumple con 5 % de
	// tolerancia. El período actual se arma con distinta mezcla que el anterior
	// para mover el delta de cumplimiento de un lado del umbral de ±2 pp.
	build := func(currentMet, currentTotal, previousMet, previousTotal int) string {
		buildRows := func(met, total int, date time.Time) []ScopedSeries {
			rows := make([]ScopedSeries, 0, total)
			for i := 0; i < total; i++ {
				elapsed := int64(300000)
				if i < met {
					elapsed = 200000
				}
				rows = append(rows, scopedRow(kindVelocidadTiempo, ptrInt(4), nil, elapsed, nil, date))
			}
			return rows
		}
		response := BuildPeriods(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
			buildRows(currentMet, currentTotal, day(1)),
			buildRows(previousMet, previousTotal, time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)),
			DefaultTolerance, DataQuality{})
		require.NotNil(t, response.Delta)
		require.NotNil(t, response.Delta.CompliancePP)
		return response.Verdict
	}

	assert.Equal(t, TrendImproving, build(3, 3, 1, 3))
	assert.Equal(t, TrendRegressing, build(0, 3, 3, 3))
	// 50 % contra 66,7 % son −16,7 pp: bien abajo del umbral.
	assert.Equal(t, TrendRegressing, build(1, 2, 2, 3))
	// Dentro de ±2 pp es estable: 50 de 60 contra 50 de 60 son los mismos 83,3 %.
	assert.Equal(t, TrendStable, build(50, 60, 50, 60))
}
