package dashboard

import (
	"math"
	"sort"
	"time"

	"simple-arq-golang/cmd/api/domains/constants"
)

// DateLayoutISO es el único formato de fecha que sale del dashboard. El repo
// parsea "YYYY-MM-DD" al otro lado (parseSessionDate,
// workout_feedback_service.go:463), así que emitirlo es lo consistente.
//
// OJO con el otro lado: el front emite DD/MM/YYYY en DateField, así que este
// valor no se puede sembrar tal cual en un DateField sin convertirlo antes.
const DateLayoutISO = "2006-01-02"

// FormatDate formatea una fecha lógica del dashboard como ISO.
func FormatDate(date time.Time) string {
	return date.Format(DateLayoutISO)
}

// ---------- setMetrics: los acumulados que comparten todos los endpoints ----------

// setMetrics son los acumulados de un conjunto de series. Existe para que el
// resumen, el timeseries, el leaderboard y las comparaciones compartan las MISMAS
// reglas sin reimplementar ninguna: cada endpoint pasa su recorte (todo el
// período, un bucket, un corredor) por acá.
type setMetrics struct {
	rows          []ScopedSeries
	totalSeries   int
	validSeries   int
	sessions      int
	targetedCount int

	totalDistanceM float64
	totalTimeMs    float64

	// Sumas de los objetivos, solo sobre filas con objetivo cargado: el delta del
	// agregado compara lo logrado contra lo que el corredor se había propuesto en
	// esas mismas filas, no contra un promedio de objetivos que nadie se propuso.
	sumTargetTimeMs    float64
	sumTargetDistanceM float64

	// Ponderación por DISTANCIA de las velocidades (spec: "kinds[] agrega los
	// ejercicios de esos tipos ya resueltos en el backend... ponderación por
	// volumen"). Ponderar por distancia y no hacer media simple no es cosmético:
	// un 21 km a 5 km/h y un 4 km a 15 km/h promedian 10 km/h con media simple,
	// cuando el volumen real está casi entero en el 21 km.
	weightedSpeedNum  float64
	weightedSpeedDen  float64
	weightedTargetNum float64
	weightedTargetDen float64

	bestTimeMs *int64
	speeds     []float64
	validRows  []ScopedSeries
	// kinds son los tipos VIGENTES presentes: los legacy no cuentan para decidir el
	// eje, porque objectiveAxisForKind los devuelve como none y ensuciarían el
	// agregado de un set que sí tiene tipos válidos.
	kinds map[string]bool
	// firstTargets es el objetivo de la primera fila de cada kind, que es el que
	// define el eje del conjunto (para `liviano` el eje depende de qué esté
	// cargado, así que no alcanza con el kind solo).
	firstTargets map[string]SeriesTargets
}

// newSetMetrics recorre el conjunto una sola vez y deja todos los acumulados
// listos. Las filas llegan ordenadas por el DAO (session_date, athlete, set), que
// es el orden cronológico que trendPct necesita.
func newSetMetrics(rows []ScopedSeries) *setMetrics {
	m := &setMetrics{
		rows:         rows,
		totalSeries:  len(rows),
		kinds:        map[string]bool{},
		firstTargets: map[string]SeriesTargets{},
	}
	sessions := map[int64]struct{}{}
	for _, row := range rows {
		sessions[row.SessionInstanceID] = struct{}{}
		if constants.IsCurrentExerciseKind(row.Kind) {
			m.kinds[row.Kind] = true
			if _, seen := m.firstTargets[row.Kind]; !seen {
				m.firstTargets[row.Kind] = row.Targets()
			}
		}
		if !row.Valid() {
			// Una serie no calculable no puede entrar a un promedio, pero no se
			// descarta en silencio: dataQualityFromSeries la cuenta (design.md D6).
			continue
		}
		achieved := row.Achieved()
		m.validSeries++
		m.validRows = append(m.validRows, row)
		m.totalDistanceM += positiveValue(achieved.DistanceM)
		m.totalTimeMs += float64(row.AchievedTimeMs())
		if achieved.SpeedKph != nil {
			distance := positiveValue(achieved.DistanceM)
			m.weightedSpeedNum += distance * *achieved.SpeedKph
			m.weightedSpeedDen += distance
			m.speeds = append(m.speeds, *achieved.SpeedKph)
		}
		if row.AchievedTimeMs() > 0 && (m.bestTimeMs == nil || row.AchievedTimeMs() < *m.bestTimeMs) {
			best := row.AchievedTimeMs()
			m.bestTimeMs = &best
		}
	}
	m.sessions = len(sessions)
	for _, row := range m.validRows {
		if !hasTarget(row.Kind, row.Targets()) {
			continue
		}
		m.targetedCount++
		targets := row.Targets()
		m.sumTargetTimeMs += positiveIntValue(targets.Minutes) * msPerMinute
		m.sumTargetDistanceM += positiveIntValue(targets.DistanceM)
		if targetSpeed := targetSpeedKph(targets); targetSpeed != nil {
			distance := positiveIntValue(targets.DistanceM)
			m.weightedTargetNum += distance * *targetSpeed
			m.weightedTargetDen += distance
		}
	}
	return m
}

// complianceRate cuenta con la tolerancia de la consulta, no con la default: un
// resumen pedido con tolerancia 0,10 tiene que clasificar con 0,10.
//
// Es met/targeted y no met/valid: una serie sin objetivo cargado no es "no
// cumplida", es "no comparable", así que no puede estar en el denominador.
func (m *setMetrics) complianceRate(tolerance float64) *float64 {
	if m.targetedCount == 0 {
		return nil
	}
	met := 0
	for _, row := range m.validRows {
		if !hasTarget(row.Kind, row.Targets()) {
			continue
		}
		if isCompliant(row.Kind, row.Targets(), row.Achieved(), tolerance) {
			met++
		}
	}
	rate := float64(met) / float64(m.targetedCount) * 100
	return &rate
}

// AchievedSpeedKph es la media de las velocidades logradas ponderada por distancia.
func (m *setMetrics) AchievedSpeedKph() *float64 {
	return weightedMean(m.weightedSpeedNum, m.weightedSpeedDen)
}

// TargetSpeedKph es la del objetivo, con la misma ponderación. Null sin objetivo:
// presentarlo como 0 haría que el cliente leyera "corrió mucho más rápido de lo
// previsto".
func (m *setMetrics) TargetSpeedKph() *float64 {
	return weightedMean(m.weightedTargetNum, m.weightedTargetDen)
}

// axisAndHasTarget decide el eje del conjunto y si hay objetivo cargado.
//
// El eje lo declara el kind, y existe aunque no haya objetivo: un
// `velocidad_distancia` sin distance_m sigue siendo eje "distance" con
// has_target false (spec "La conmutación depende del dato, no del kind"). Solo
// `none` aparece cuando no hay ningún tipo vigente en el conjunto.
//
// Con varios kinds el agregado es una comparación de velocidad ponderada por
// distancia —que es lo que la spec declara para `kinds[]`—, así que el eje del
// agregado es el doble.
func (m *setMetrics) axisAndHasTarget() (Axis, bool) {
	switch len(m.kinds) {
	case 0:
		return AxisNone, false
	case 1:
		for kind := range m.kinds {
			return objectiveAxisForKind(kind, m.firstTargets[kind]), m.targetedCount > 0
		}
	}
	return AxisDistanceAndTime, m.targetedCount > 0
}

// deltaPct es "cuánto mejor o peor respecto del objetivo", positivo hacia la
// mejora. El signo se invierte en el eje temporal, donde menos es mejor: 240 s de
// objetivo con 230 s logrados es +4,2 %, no −4,2 %.
func (m *setMetrics) deltaPct(axis Axis) *float64 {
	var target, actual float64
	switch axis {
	case AxisTime:
		target = m.sumTargetTimeMs
		actual = m.sumAchievedTargetedTimeMs()
	case AxisDistance:
		target = m.sumTargetDistanceM
		actual = m.sumAchievedTargetedDistanceM()
	case AxisDistanceAndTime:
		targetSpeed, achievedSpeed := m.TargetSpeedKph(), m.AchievedSpeedKph()
		if targetSpeed == nil || achievedSpeed == nil {
			return nil
		}
		target = *targetSpeed
		actual = *achievedSpeed
	default:
		return nil
	}
	if !(target > 0) {
		return nil
	}
	pct := (actual - target) / target * 100
	if axis == AxisTime {
		pct = -pct
	}
	if isNotFinite(pct) {
		return nil
	}
	return &pct
}

// sumAchievedTargetedTimeMs y sumAchievedTargetedDistanceM son las sumas de lo
// logrado SOLO sobre las filas con objetivo cargado, para que el delta compare
// contra los objetivos de esas mismas filas.
func (m *setMetrics) sumAchievedTargetedTimeMs() float64 {
	var sum float64
	for _, row := range m.targetedSeriesRows() {
		sum += float64(row.AchievedTimeMs())
	}
	return sum
}

func (m *setMetrics) sumAchievedTargetedDistanceM() float64 {
	var sum float64
	for _, row := range m.targetedSeriesRows() {
		sum += positiveValue(row.Achieved().DistanceM)
	}
	return sum
}

func (m *setMetrics) targetedSeriesRows() []ScopedSeries {
	rows := make([]ScopedSeries, 0, m.targetedCount)
	for _, row := range m.validRows {
		if hasTarget(row.Kind, row.Targets()) {
			rows = append(rows, row)
		}
	}
	return rows
}

// dataQuality arma el bloque de calidad del conjunto. Los cinco contadores de
// serie salen del mismo result set (dataQualityFromSeries) y open_sessions lo
// agrega el service desde una SEGUNDA query: el WHERE de las series ya sacó las
// sesiones wip del set, así que contarlas ahí daría siempre cero.
func (m *setMetrics) dataQuality(openSessions int) DataQuality {
	quality := dataQualityFromSeries(seriesOf(m.rows))
	quality.OpenSessions = openSessions
	return quality
}

func seriesOf(rows []ScopedSeries) []Series {
	out := make([]Series, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Series)
	}
	return out
}

// ---------- resumen ----------

// BuildSummary arma el resumen del período.
func BuildSummary(rows []ScopedSeries, tolerance float64, openSessions int) SummaryResponse {
	m := newSetMetrics(rows)
	axis, hasTarget := m.axisAndHasTarget()

	totals := SummaryTotals{
		Sessions:         m.sessions,
		Series:           m.totalSeries,
		ValidSeries:      m.validSeries,
		BestTimeMs:       m.bestTimeMs,
		AchievedSpeedKph: m.AchievedSpeedKph(),
		ComplianceRate:   m.complianceRate(tolerance),
	}
	if m.validSeries > 0 {
		distance := m.totalDistanceM
		timeMs := int64(m.totalTimeMs)
		totals.DistanceM = &distance
		totals.TimeMs = &timeMs
	}
	if hasTarget {
		totals.TargetSpeedKph = m.TargetSpeedKph()
		totals.DeltaPct = m.deltaPct(axis)
	}

	trend := trendPct(m.speeds)
	return SummaryResponse{
		Axis:      string(axis),
		HasTarget: hasTarget,
		Units:     UnitsBlock,
		Totals:    totals,
		Consistency: Consistency{
			CV: coefficientOfVariation(m.speeds),
		},
		Trend:            Trend{TrendPct: trend, Verdict: trendVerdict(trend)},
		ToleranceApplied: tolerance,
		SeriesCount:      m.validSeries,
		DataQuality:      m.dataQuality(openSessions),
	}
}

// ---------- timeseries ----------

// BucketStart es el anclaje de calendario del bucket para una granularidad y una
// fecha lógica. Los cuatro anclajes son de calendario, no relativos al from, así
// que un bucket puede empezar antes del from: el recorte nunca parte una semana
// ni un trimestre por la mitad para acomodar los bordes.
func BucketStart(date time.Time, granularity string) time.Time {
	day := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	switch granularity {
	case "week":
		// lunes ISO 8601: time.Weekday() da domingo=0.
		offset := (int(day.Weekday()) + 6) % 7
		return day.AddDate(0, 0, -offset)
	case "month":
		return time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "quarter":
		firstMonthOfQuarter := ((int(day.Month())-1)/3)*3 + 1
		return time.Date(day.Year(), time.Month(firstMonthOfQuarter), 1, 0, 0, 0, 0, time.UTC)
	default:
		return day
	}
}

// BuildTimeSeries agrupa las series en buckets de calendario y corre los MISMOS
// acumulados que el resumen en cada bucket. Un bucket sin series no aparece: la
// serie temporal no inventa períodos que no pasó nada.
func BuildTimeSeries(rows []ScopedSeries, granularity string, tolerance float64, openSessions int) TimeseriesResponse {
	grouped := map[string][]ScopedSeries{}
	for _, row := range rows {
		key := BucketStart(row.SessionDate, granularity).Format(DateLayoutISO)
		grouped[key] = append(grouped[key], row)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	buckets := make([]TimeseriesBucket, 0, len(keys))
	for _, key := range keys {
		bucketRows := grouped[key]
		m := newSetMetrics(bucketRows)
		axis, hasTarget := m.axisAndHasTarget()
		bucket := TimeseriesBucket{
			PeriodStart:      key,
			Sessions:         m.sessions,
			Series:           m.totalSeries,
			ValidSeries:      m.validSeries,
			AchievedSpeedKph: m.AchievedSpeedKph(),
			ComplianceRate:   m.complianceRate(tolerance),
			BestTimeMs:       m.bestTimeMs,
			BandCounts:       bandCountsOf(bucketRows, tolerance),
		}
		if hasTarget {
			bucket.TargetSpeedKph = m.TargetSpeedKph()
			bucket.DeltaPct = m.deltaPct(axis)
		}
		buckets = append(buckets, bucket)
	}

	return TimeseriesResponse{
		Granularity: granularity,
		Units:       UnitsBlock,
		Buckets:     buckets,
		DataQuality: newSetMetrics(rows).dataQuality(openSessions),
	}
}

// bandCountsOf clasifica cada serie con SU eje. Con has_target false las cuatro
// bandas quedan en cero: sin objetivo cargado no hay cumplimiento que clasificar.
func bandCountsOf(rows []ScopedSeries, tolerance float64) BandCounts {
	evaluable := make([]Evaluable, 0, len(rows))
	for _, row := range rows {
		evaluable = append(evaluable, row.Evaluable())
	}
	return bandCounts(evaluable, tolerance)
}

// ---------- detalle de series ----------

// BuildSeriesDetail arma el detalle por serie de /exercises/{id}/series con todo
// lo que el cliente necesita para pintar cumplimiento y dispersión sin recalcular
// nada. Los objetivos son los CONGELADOS de cada ExerciseInstance, que pueden
// venir de un source_exercise_id distinto o nulo.
func BuildSeriesDetail(rows []ScopedSeries, tolerance float64, page, pageSize int, quality DataQuality) SeriesResponse {
	m := newSetMetrics(rows)
	axis, hasTarget := m.axisAndHasTarget()

	total := len(rows)
	items := make([]SeriesDetailItem, 0, pageSize)
	start := (page - 1) * pageSize
	for i := start; i < total && i < start+pageSize; i++ {
		row := rows[i]
		achieved := row.Achieved()
		item := SeriesDetailItem{
			SessionInstanceID:   row.SessionInstanceID,
			SessionDate:         FormatDate(row.SessionDate),
			AthleteUserID:       row.AthleteUserID,
			FeedbackOwnerUserID: row.FeedbackOwnerUserID,
			SetNumber:           row.SetNumber,
			DurationMs:          row.DurationMs,
			ActiveDurationMs:    row.ActiveDurationMs,
			TimeQuality:         string(row.TimeQuality()),
			DistanceMeters:      row.DistanceMeters,
			SpeedKph:            achieved.SpeedKph,
			TargetMinutes:       row.TargetMinutes,
			TargetDistanceM:     row.TargetDistanceM,
			MetTarget:           metTarget(row.Kind, row.Targets(), achieved, tolerance),
			DeltaPct:            deltaPct(row.Kind, row.Targets(), achieved),
			CompletionStatus:    row.CompletionStatus,
			ToleranceApplied:    tolerance,
		}
		if classified := band(row.Kind, row.Targets(), achieved, tolerance); classified != "" {
			value := string(classified)
			item.Band = &value
		}
		items = append(items, item)
	}

	return SeriesResponse{
		Axis:        string(axis),
		HasTarget:   hasTarget,
		Items:       items,
		Page:        page,
		PageSize:    pageSize,
		Total:       total,
		SeriesCount: m.validSeries,
		Units:       UnitsBlock,
		DataQuality: quality,
	}
}

// ---------- leaderboard ----------

// Columnas por las que el leaderboard puede ordenar. Cualquier otra se rechaza
// antes de tocar la DB (la whitelist vive en el service, el precedente es
// historySortWhitelist en workout_feedback_service.go).
const (
	SortBestTimeMs     = "best_time_ms"
	SortMeanSpeedKph   = "mean_speed_kph"
	SortTotalDistanceM = "total_distance_m"
	SortComplianceRate = "compliance_rate"

	OrderAsc  = "asc"
	OrderDesc = "desc"
)

// DashboardSortWhitelist son las columnas de sort aceptadas por el leaderboard.
var DashboardSortWhitelist = map[string]bool{
	SortBestTimeMs:     true,
	SortMeanSpeedKph:   true,
	SortTotalDistanceM: true,
	SortComplianceRate: true,
}

// AthleteName es lo mínimo de un corredor que necesitan las respuestas del
// dashboard. Vive acá para que el dominio no dependa de dbs.
type AthleteName struct {
	Name    string
	Surname string
}

// LeaderboardEntry es un corredor ya agregado.
type LeaderboardEntry struct {
	AthleteUserID  int64
	Name           string
	Surname        string
	Sessions       int
	BestTimeMs     *int64
	MeanSpeedKph   *float64
	TotalDistanceM *float64
	ComplianceRate *float64
}

// NewLeaderboardEntry agrega las series de un corredor. Los valores sin objetivo o
// sin volumen quedan null, con la misma regla del resumen.
func NewLeaderboardEntry(athleteUserID int64, name, surname string, rows []ScopedSeries, tolerance float64) LeaderboardEntry {
	m := newSetMetrics(rows)
	entry := LeaderboardEntry{
		AthleteUserID:  athleteUserID,
		Name:           name,
		Surname:        surname,
		Sessions:       m.sessions,
		BestTimeMs:     m.bestTimeMs,
		MeanSpeedKph:   m.AchievedSpeedKph(),
		ComplianceRate: m.complianceRate(tolerance),
	}
	if m.validSeries > 0 {
		distance := m.totalDistanceM
		entry.TotalDistanceM = &distance
	}
	return entry
}

// BuildLeaderboard ordena a los corredores del scope, calcula la posición global
// de cada uno y devuelve la página pedida.
//
// La posición se calcula con la SEMÁNTICA de RANK() sobre TODAS las entradas del
// scope —los empates comparten posición y la siguiente salta— y con exactamente
// el mismo comparador que el ORDER BY final, porque de lo contrario el número que
// ve el corredor no es su posición real.
//
// El ordenamiento y el rank se resuelven acá y no en SQL a propósito: el valor de
// sort depende del cumplimiento, y el cumplimiento lo decide el kind de cada
// serie (design.md D1). Calcularlo en SQL obligaría a reimplementar compliance.go
// dentro de una consulta, que es justo lo que no se puede hacer con una fórmula
// que ya tiene su suite de tests.
func BuildLeaderboard(entries []LeaderboardEntry, sortColumn, order string, page, pageSize int, callerID int64, quality DataQuality) LeaderboardResponse {
	sorted := append([]LeaderboardEntry(nil), entries...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return lessLeaderboardEntry(sorted[i], sorted[j], sortColumn, order)
	})

	positions := make(map[int64]int, len(sorted))
	position := 0
	previousValue, previousMissing := 0.0, false
	for i, entry := range sorted {
		value, missing := leaderboardComparableValue(entry, sortColumn)
		// RANK(): si la fila empata con la anterior comparte posición; si no, su
		// posición es la cantidad de filas que la anteceden más una.
		if i == 0 || previousMissing != missing || previousValue != value {
			position = i + 1
		}
		positions[entry.AthleteUserID] = position
		previousValue, previousMissing = value, missing
	}

	total := len(sorted)
	items := make([]LeaderboardItem, 0, pageSize)
	start := (page - 1) * pageSize
	for i := start; i < total && i < start+pageSize; i++ {
		entry := sorted[i]
		items = append(items, LeaderboardItem{
			Rank:           i - start + 1,
			Position:       positions[entry.AthleteUserID],
			AthleteUserID:  entry.AthleteUserID,
			Name:           entry.Name,
			Surname:        entry.Surname,
			Sessions:       entry.Sessions,
			BestTimeMs:     entry.BestTimeMs,
			MeanSpeedKph:   entry.MeanSpeedKph,
			TotalDistanceM: entry.TotalDistanceM,
			ComplianceRate: entry.ComplianceRate,
			IsSelf:         entry.AthleteUserID == callerID,
		})
	}

	return LeaderboardResponse{
		Items:       items,
		Page:        page,
		PageSize:    pageSize,
		Total:       total,
		Sort:        sortColumn,
		Order:       order,
		Units:       UnitsBlock,
		DataQuality: quality,
	}
}

// lessLeaderboardEntry es el comparador único del leaderboard: lo usan el ORDER BY
// de la página y el cálculo de position, así que no pueden divergir.
func lessLeaderboardEntry(a, b LeaderboardEntry, sort, order string) bool {
	// Desempate determinista, en el orden que declara la spec: menor tiempo, luego
	// mayor distancia, luego alfabético. Sin esto, dos filas empatadas pueden
	// paginarse en distinto orden entre dos requests.
	tie := func() bool {
		if a.BestTimeMs != nil && b.BestTimeMs != nil && *a.BestTimeMs != *b.BestTimeMs {
			return *a.BestTimeMs < *b.BestTimeMs
		}
		if a.TotalDistanceM != nil && b.TotalDistanceM != nil && *a.TotalDistanceM != *b.TotalDistanceM {
			return *a.TotalDistanceM > *b.TotalDistanceM
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Surname != b.Surname {
			return a.Surname < b.Surname
		}
		return a.AthleteUserID < b.AthleteUserID
	}

	descending := order == OrderDesc
	av, aok := leaderboardComparableValue(a, sort)
	bv, bok := leaderboardComparableValue(b, sort)
	switch {
	case aok && bok:
		if av == bv {
			return tie()
		}
		if descending {
			return av > bv
		}
		return av < bv
	case aok:
		// Sin dato al final en cualquier orden: un corredor sin mejor tiempo no es
		// el más rápido, y "sin dato" no puede ganar的位置 ni por desc.
		return true
	case bok:
		return false
	default:
		return tie()
	}
}

func leaderboardComparableValue(entry LeaderboardEntry, sort string) (float64, bool) {
	switch sort {
	case SortBestTimeMs:
		if entry.BestTimeMs == nil {
			return 0, false
		}
		return float64(*entry.BestTimeMs), true
	case SortMeanSpeedKph:
		if entry.MeanSpeedKph == nil {
			return 0, false
		}
		return *entry.MeanSpeedKph, true
	case SortTotalDistanceM:
		if entry.TotalDistanceM == nil {
			return 0, false
		}
		return *entry.TotalDistanceM, true
	case SortComplianceRate:
		if entry.ComplianceRate == nil {
			return 0, false
		}
		return *entry.ComplianceRate, true
	default:
		return 0, false
	}
}

// ---------- comparaciones ----------

// BuildComparisons devuelve una fila por corredor comparable. Los valores sin
// objetivo o sin volumen suficiente vienen null, con la misma regla del resumen.
func BuildComparisons(rows []ScopedSeries, tolerance float64, quality DataQuality) ComparisonsResponse {
	byAthlete := map[int64][]ScopedSeries{}
	order := []int64{}
	for _, row := range rows {
		if _, seen := byAthlete[row.AthleteUserID]; !seen {
			order = append(order, row.AthleteUserID)
		}
		byAthlete[row.AthleteUserID] = append(byAthlete[row.AthleteUserID], row)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	series := make([]ComparisonRow, 0, len(order))
	for _, athleteID := range order {
		athleteRows := byAthlete[athleteID]
		m := newSetMetrics(athleteRows)
		trend := trendPct(m.speeds)
		row := ComparisonRow{
			AthleteUserID:  athleteID,
			Sessions:       m.sessions,
			ValidSeries:    m.validSeries,
			BestTimeMs:     m.bestTimeMs,
			MeanSpeedKph:   m.AchievedSpeedKph(),
			ComplianceRate: m.complianceRate(tolerance),
			Trend:          Trend{TrendPct: trend, Verdict: trendVerdict(trend)},
		}
		if m.validSeries > 0 {
			distance := m.totalDistanceM
			row.TotalDistanceM = &distance
		}
		series = append(series, row)
	}

	return ComparisonsResponse{Series: series, Units: UnitsBlock, DataQuality: quality}
}

// ApplyAthleteNames completa los nombres de las filas de comparación con el batch
// lookup de usuarios: el DAO devuelve series, no personas.
func ApplyAthleteNames(series []ComparisonRow, names map[int64]AthleteName) {
	for i := range series {
		name, ok := names[series[i].AthleteUserID]
		if !ok {
			continue
		}
		series[i].Name = name.Name
		series[i].Surname = name.Surname
	}
}

// ---------- comparación de períodos ----------

// DerivePreviousPeriod desplaza el período actual hacia atrás exactamente su
// cantidad de días: del 1 al 30 de septiembre son 30 días, así que el anterior es
// del 2 al 31 de agosto.
func DerivePreviousPeriod(currentFrom, currentTo time.Time) (time.Time, time.Time) {
	length := int(currentTo.Sub(currentFrom).Hours()/24) + 1
	return currentFrom.AddDate(0, 0, -length), currentTo.AddDate(0, 0, -length)
}

// BuildPeriods compara el período actual contra el anterior.
//
// Si cualquiera de los dos períodos no tiene series válidas, previous y delta
// vienen null y el verdict es stable: presentar una comparación sin datos como
// "estable" sería mentir.
func BuildPeriods(currentFrom, currentTo, previousFrom, previousTo time.Time, current, previous []ScopedSeries, tolerance float64, quality DataQuality) PeriodsResponse {
	currentMetrics := newSetMetrics(current)
	previousMetrics := newSetMetrics(previous)

	response := PeriodsResponse{
		Current:     periodMetrics(currentFrom, currentTo, currentMetrics, tolerance),
		Units:       UnitsBlock,
		DataQuality: quality,
		Verdict:     TrendStable,
	}
	if currentMetrics.validSeries == 0 || previousMetrics.validSeries == 0 {
		return response
	}

	response.Previous = periodMetrics(previousFrom, previousTo, previousMetrics, tolerance)
	response.Delta = periodsDelta(currentMetrics, previousMetrics, tolerance)
	response.Verdict = periodsVerdict(response.Delta.CompliancePP)
	return response
}

func periodMetrics(from, to time.Time, m *setMetrics, tolerance float64) *PeriodMetrics {
	metrics := &PeriodMetrics{
		From:           FormatDate(from),
		To:             FormatDate(to),
		Series:         m.totalSeries,
		ValidSeries:    m.validSeries,
		MeanSpeedKph:   m.AchievedSpeedKph(),
		ComplianceRate: m.complianceRate(tolerance),
		BestTimeMs:     m.bestTimeMs,
	}
	if m.validSeries > 0 {
		distance := m.totalDistanceM
		metrics.TotalDistanceM = &distance
	}
	return metrics
}

func periodsDelta(current, previous *setMetrics, tolerance float64) *PeriodsDelta {
	return &PeriodsDelta{
		Series:       current.validSeries - previous.validSeries,
		DistancePct:  relativePct(pointer(current.totalDistanceM), pointer(previous.totalDistanceM)),
		SpeedPct:     relativePct(current.AchievedSpeedKph(), previous.AchievedSpeedKph()),
		CompliancePP: subtractPercentages(current.complianceRate(tolerance), previous.complianceRate(tolerance)),
		// best_time_pct invierte el signo: bajar el tiempo es mejorar, así que un
		// tiempo menor da un delta positivo.
		BestTimePct: relativePctMs(previous.bestTimeMs, current.bestTimeMs),
	}
}

// periodsVerdict clasifica por cumplimiento, con ±2 pp de umbral.
func periodsVerdict(compliancePP *float64) string {
	if compliancePP == nil {
		return TrendStable
	}
	switch {
	case *compliancePP > 2:
		return TrendImproving
	case *compliancePP < -2:
		return TrendRegressing
	default:
		return TrendStable
	}
}

// relativePct es (curr − prev) / prev × 100. Null en vez de 0 cuando falta
// cualquiera de los dos lados: sin denominador no hay porcentaje.
func relativePct(curr, prev *float64) *float64 {
	if curr == nil || prev == nil || !(*prev > 0) {
		return nil
	}
	pct := (*curr - *prev) / *prev * 100
	if isNotFinite(pct) {
		return nil
	}
	return &pct
}

// relativePctMs es relativePct para tiempos en ms. El signo ya viene invertido en
// el llamada: previous contra current, porque bajar el tiempo es mejorar.
func relativePctMs(prev, curr *int64) *float64 {
	if prev == nil || curr == nil || !(*prev > 0) {
		return nil
	}
	previousMs := float64(*prev)
	currentMs := float64(*curr)
	pct := (previousMs - currentMs) / previousMs * 100
	if isNotFinite(pct) {
		return nil
	}
	return &pct
}

// subtractPercentages son puntos porcentuales: 58 % contra 64 % es −6 pp, no −9,4 %.
func subtractPercentages(curr, prev *float64) *float64 {
	if curr == nil || prev == nil {
		return nil
	}
	diff := *curr - *prev
	if isNotFinite(diff) {
		return nil
	}
	return &diff
}

func pointer(v float64) *float64 { return &v }

func weightedMean(num, den float64) *float64 {
	if !(den > 0) {
		return nil
	}
	mean := num / den
	if math.IsNaN(mean) || math.IsInf(mean, 0) {
		return nil
	}
	return &mean
}

func positiveValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func positiveIntValue(v *int) float64 {
	if v == nil {
		return 0
	}
	return float64(*v)
}
