package dashboard

import "time"

// ScopedSeries es una fila de feedback con el contexto que las fórmulas del KPI
// necesitan y que no están en workout_feedback: el kind y el objetivo congelado
// de la ExerciseInstance (design.md D1/D3), y a quién y cuándo corresponde.
//
// Es la entrada única de las agregaciones del paquete: el DAO mapea sus columnas
// a esta struct y todos los endpoints arman sobre las mismas funciones puras, en
// vez de recalcular el KPI en cada uno.
type ScopedSeries struct {
	Series
	FeedbackID          int64
	SessionInstanceID   int64
	ExerciseInstanceID  int64
	SessionDate         time.Time
	BucketStart         time.Time
	AthleteUserID       int64
	FeedbackOwnerUserID int64
	SetNumber           int
	TargetMinutes       *int
	TargetDistanceM     *int
}

// Targets arma el objetivo congelado de la instancia. Se usa en vez del objetivo
// vigente del catálogo: cada día se juzga contra lo que el corredor intentaba ese
// día, no contra un objetivo redefinido después.
func (s ScopedSeries) Targets() SeriesTargets {
	return targetFromExerciseInstance(s.TargetMinutes, s.TargetDistanceM)
}

// Achieved arma lo logrado: tiempo con fallback y velocidad derivada.
func (s ScopedSeries) Achieved() SeriesAchieved {
	return achievedFromSeries(s.Series)
}

// Evaluable junta las tres cosas que necesitan las fórmulas.
func (s ScopedSeries) Evaluable() Evaluable {
	return Evaluable{Kind: s.Kind, Targets: s.Targets(), Achieved: s.Achieved()}
}

// Valid dice si la serie entra a promedios, cumplimiento y rankings.
func (s ScopedSeries) Valid() bool { return isValidSeries(s.Kind, s.Series) }

// TimeQuality es "alta" si el tiempo viene del tiempo activo, "degraded" si cayó
// al fallback (design.md D4).
func (s ScopedSeries) TimeQuality() TimeQuality { return timeQuality(s.Series) }

// Completion es el vocabulario reconocido de completion_status; lo no reconocido
// cuenta como unknown y NO descarta la serie (design.md D10).
func (s ScopedSeries) Completion() Completion { return completionTaxonomy(s.Series) }

// AchievedTimeMs es COALESCE(active_duration_ms, duration_ms).
func (s ScopedSeries) AchievedTimeMs() int64 { return achievedTimeMs(s.Series) }

// ---------- filtros ----------

// FiltersResponse es el contrato de GET /dashboard/filters. teams/groups/
// athletes/exercises vienen YA recortados por el scope del solicitante: no hay
// forma de pedir el catálogo de otro equipo.
type FiltersResponse struct {
	Teams         []FiltersTeam     `json:"teams"`
	Groups        []FiltersGroup    `json:"groups"`
	Athletes      []FiltersAthlete  `json:"athletes"`
	ExerciseTypes []string          `json:"exercise_types"`
	Exercises     []FiltersExercise `json:"exercises"`
	DateRange     *DateRange        `json:"date_range"`
}

type FiltersTeam struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// FiltersGroup lleva role para que el front distinga "grupo mío como entrenador"
// de "grupo donde estoy como corredor" sin pedir nada extra.
type FiltersGroup struct {
	ID     int64  `json:"id"`
	TeamID int64  `json:"team_id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

type FiltersAthlete struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

// FiltersExercise expone el objetivo VIGENTE del catálogo, explícitamente
// rotulado: no es el objetivo con el que se evalúa una sesión histórica, ese va
// congelado en la ExerciseInstance y solo aparece en /exercises/{id}/series.
type FiltersExercise struct {
	ID                     int64  `json:"id"`
	Name                   string `json:"name"`
	Kind                   string `json:"kind"`
	CurrentTargetMinutes   *int   `json:"current_target_minutes"`
	CurrentTargetDistanceM *int   `json:"current_target_distance_m"`
}

// DateRange es el MIN/MAX real de session_date dentro del scope. Viene null —no
// {min: null, max: null}— cuando el scope no tiene ninguna sesión registrada,
// porque el front usa ese null para distinguir "nunca hubo datos" de "hay datos
// acotados".
type DateRange struct {
	Min string `json:"min"`
	Max string `json:"max"`
}

// ---------- métricas ----------

// Trend es la variación contra el tercil más reciente y su clasificación.
type Trend struct {
	TrendPct *float64 `json:"trend_pct"`
	Verdict  string   `json:"verdict"`
}

// Consistency expone el coeficiente de variación de la velocidad de las series
// válidas.
type Consistency struct {
	CV *float64 `json:"cv"`
}

// SummaryTotals son los acumulados del período. Los campos que no son calculables
// vienen null y no 0: 0 es un resultado legítimamente distinto de "no hay dato"
// (spec "El resumen no inventa valores donde no hay dato").
type SummaryTotals struct {
	Sessions         int      `json:"sessions"`
	Series           int      `json:"series"`
	ValidSeries      int      `json:"valid_series"`
	DistanceM        *float64 `json:"distance_m"`
	TimeMs           *int64   `json:"time_ms"`
	AchievedSpeedKph *float64 `json:"achieved_speed_kph"`
	TargetSpeedKph   *float64 `json:"target_speed_kph"`
	DeltaPct         *float64 `json:"delta_pct"`
	// ComplianceRate es un PORCENTAJE 0-100, no una fracción, para que no compita
	// con delta_pct.
	ComplianceRate *float64 `json:"compliance_rate"`
	BestTimeMs     *int64   `json:"best_time_ms"`
}

type SummaryResponse struct {
	Axis        string        `json:"axis"`
	HasTarget   bool          `json:"has_target"`
	Units       Units         `json:"units"`
	Totals      SummaryTotals `json:"totals"`
	Consistency Consistency   `json:"consistency"`
	Trend       Trend         `json:"trend"`
	// ToleranceApplied y SeriesCount viajan porque el tooltip de KPI necesita
	// mostrar la tolerancia usada y cuántas series entraron, y no hay otra fuente.
	ToleranceApplied float64     `json:"tolerance_applied"`
	SeriesCount      int         `json:"series_count"`
	DataQuality      DataQuality `json:"data_quality"`
}

// TimeseriesBucket es un bucket de calendario. band_counts viene presente aunque
// los cuatro contadores estén en cero, para que el gráfico de barras apiladas
// pueda pintar la estructura sin tratarla como dato ausente.
type TimeseriesBucket struct {
	PeriodStart      string     `json:"period_start"`
	Sessions         int        `json:"sessions"`
	Series           int        `json:"series"`
	ValidSeries      int        `json:"valid_series"`
	AchievedSpeedKph *float64   `json:"achieved_speed_kph"`
	TargetSpeedKph   *float64   `json:"target_speed_kph"`
	DeltaPct         *float64   `json:"delta_pct"`
	ComplianceRate   *float64   `json:"compliance_rate"`
	BestTimeMs       *int64     `json:"best_time_ms"`
	BandCounts       BandCounts `json:"band_counts"`
}

type TimeseriesResponse struct {
	Granularity string             `json:"granularity"`
	Units       Units              `json:"units"`
	Buckets     []TimeseriesBucket `json:"buckets"`
	DataQuality DataQuality        `json:"data_quality"`
}

// LeaderboardItem lleva rank (posición dentro de la página) y position (posición
// global, calculada sobre TODAS las filas del scope con el mismo criterio de
// orden). Sin position el cliente no puede mostrar "vas 14°" sin traer todas las
// páginas.
type LeaderboardItem struct {
	Rank           int      `json:"rank"`
	Position       int      `json:"position"`
	AthleteUserID  int64    `json:"athlete_user_id"`
	Name           string   `json:"name"`
	Surname        string   `json:"surname"`
	Sessions       int      `json:"sessions"`
	BestTimeMs     *int64   `json:"best_time_ms"`
	MeanSpeedKph   *float64 `json:"mean_speed_kph"`
	TotalDistanceM *float64 `json:"total_distance_m"`
	ComplianceRate *float64 `json:"compliance_rate"`
	IsSelf         bool     `json:"is_self"`
}

type LeaderboardResponse struct {
	Items       []LeaderboardItem `json:"items"`
	Page        int               `json:"page"`
	PageSize    int               `json:"page_size"`
	Total       int               `json:"total"`
	Sort        string            `json:"sort"`
	Order       string            `json:"order"`
	Units       Units             `json:"units"`
	DataQuality DataQuality       `json:"data_quality"`
}

// SeriesDetailItem es el detalle por serie de /exercises/{id}/series: todo lo que
// el cliente necesita para pintar cumplimiento y dispersión SIN recalcular nada.
type SeriesDetailItem struct {
	SessionInstanceID   int64    `json:"session_instance_id"`
	SessionDate         string   `json:"session_date"`
	AthleteUserID       int64    `json:"athlete_user_id"`
	FeedbackOwnerUserID int64    `json:"feedback_owner_user_id"`
	SetNumber           int      `json:"set_number"`
	DurationMs          *int64   `json:"duration_ms"`
	ActiveDurationMs    *int64   `json:"active_duration_ms"`
	TimeQuality         string   `json:"time_quality"`
	DistanceMeters      *float64 `json:"distance_meters"`
	// SpeedKph es DERIVADA de distancia y tiempo, nunca leída de
	// exercise_instances.speed_kph (design.md D2).
	SpeedKph         *float64 `json:"speed_kph"`
	TargetMinutes    *int     `json:"target_minutes"`
	TargetDistanceM  *int     `json:"target_distance_m"`
	MetTarget        *bool    `json:"met_target"`
	DeltaPct         *float64 `json:"delta_pct"`
	CompletionStatus *string  `json:"completion_status"`
	// Band es null cuando has_target es false: sin objetivo no hay cumplimiento
	// que clasificar. No es lo mismo que la banda "unknown".
	Band             *string `json:"band"`
	ToleranceApplied float64 `json:"tolerance_applied"`
}

type SeriesResponse struct {
	Axis      string             `json:"axis"`
	HasTarget bool               `json:"has_target"`
	Items     []SeriesDetailItem `json:"items"`
	Page      int                `json:"page"`
	PageSize  int                `json:"page_size"`
	Total     int                `json:"total"`
	// SeriesCount son las series válidas del ejercicio en el scope; Total son las
	// filas de la página.
	SeriesCount int         `json:"series_count"`
	Units       Units       `json:"units"`
	DataQuality DataQuality `json:"data_quality"`
}

// ComparisonRow es una fila por corredor comparable.
type ComparisonRow struct {
	AthleteUserID  int64    `json:"athlete_user_id"`
	Name           string   `json:"name"`
	Surname        string   `json:"surname"`
	Sessions       int      `json:"sessions"`
	ValidSeries    int      `json:"valid_series"`
	BestTimeMs     *int64   `json:"best_time_ms"`
	MeanSpeedKph   *float64 `json:"mean_speed_kph"`
	TotalDistanceM *float64 `json:"total_distance_m"`
	ComplianceRate *float64 `json:"compliance_rate"`
	Trend          Trend    `json:"trend"`
}

type ComparisonsResponse struct {
	Series      []ComparisonRow `json:"series"`
	Units       Units           `json:"units"`
	DataQuality DataQuality     `json:"data_quality"`
}

// PeriodMetrics es un período completo, inclusivo de ambos extremos.
type PeriodMetrics struct {
	From           string   `json:"from"`
	To             string   `json:"to"`
	Series         int      `json:"series"`
	ValidSeries    int      `json:"valid_series"`
	TotalDistanceM *float64 `json:"total_distance_m"`
	MeanSpeedKph   *float64 `json:"mean_speed_kph"`
	ComplianceRate *float64 `json:"compliance_rate"`
	BestTimeMs     *int64   `json:"best_time_ms"`
}

// PeriodsDelta mezcla dos escalas a propósito y las declara: distance_pct y
// speed_pct usan (curr − prev) / prev, donde más alto es mejor, y best_time_pct
// usa (prev − curr) / prev, donde bajar el tiempo es mejorar. Invertir el signo de
// un campo sin declararlo es la forma más fácil de que un cliente muestre
// "retroceso" donde hubo mejora.
type PeriodsDelta struct {
	Series      int      `json:"series"`
	DistancePct *float64 `json:"distance_pct"`
	SpeedPct    *float64 `json:"speed_pct"`
	// CompliancePP son puntos porcentuales, no porcentaje relativo: 58 % contra
	// 64 % es −6 pp, no −9,4 %.
	CompliancePP *float64 `json:"compliance_pp"`
	BestTimePct  *float64 `json:"best_time_pct"`
}

type PeriodsResponse struct {
	Current     *PeriodMetrics `json:"current"`
	Previous    *PeriodMetrics `json:"previous"`
	Delta       *PeriodsDelta  `json:"delta"`
	Verdict     string         `json:"verdict"`
	Units       Units          `json:"units"`
	DataQuality DataQuality    `json:"data_quality"`
}

// ---------- requests ----------

// PeriodRange es un período inclusivo de ambos extremos, en "YYYY-MM-DD".
type PeriodRange struct {
	From string `json:"from" binding:"required"`
	To   string `json:"to" binding:"required"`
}

// ComparisonsRequest son los cuerpos de POST /dashboard/comparisons y
// /comparisons/periods.
//
// team_id es obligatorio en todos los endpoints del dashboard (spec "El scope
// entra por el equipo del que se llega"); group_id y athlete_user_ids son
// filtros de scope opcionales que se resuelven del scope del solicitante cuando
// faltan. group_id explícito NO amplía el scope.
type ComparisonsRequest struct {
	TeamID      int64   `json:"team_id" binding:"required"`
	GroupID     *int64  `json:"group_id"`
	ExerciseID  *int64  `json:"exercise_id"`
	ExerciseIDs []int64 `json:"exercise_ids"`
	// Kind es un valor único, no una lista: el filtro de tipo del front es de
	// selección única y su opción "todos" viaja como ausencia del parámetro. Solo
	// admiten los tres tipos con métrica.
	Kind           *string  `json:"kind"`
	From           string   `json:"from"`
	To             string   `json:"to"`
	Tolerance      *float64 `json:"tolerance"`
	AthleteUserIDs []int64  `json:"athlete_user_ids"`
	// Rango de datos: metros y minutos enteros, igual que en los GET.
	DistanceFromM *int64 `json:"distance_from_m"`
	DistanceToM   *int64 `json:"distance_to_m"`
	TimeFromMin   *int64 `json:"time_from_min"`
	TimeToMin     *int64 `json:"time_to_min"`
}

type PeriodsRequest struct {
	TeamID      int64       `json:"team_id" binding:"required"`
	GroupID     *int64      `json:"group_id"`
	ExerciseID  *int64      `json:"exercise_id"`
	ExerciseIDs []int64     `json:"exercise_ids"`
	Kind        *string     `json:"kind"`
	Current     PeriodRange `json:"current" binding:"required"`
	// Previous es opcional: si no viene, el backend lo deriva desplazando current
	// hacia atrás exactamente su misma cantidad de días.
	Previous  *PeriodRange `json:"previous"`
	Tolerance *float64     `json:"tolerance"`
	// El rango de datos acota las series de los DOS períodos por igual: si el
	// current y el previous midieran ventanas distintas, el delta compararía dos
	// cosas que el usuario no pidió comparar.
	DistanceFromM *int64 `json:"distance_from_m"`
	DistanceToM   *int64 `json:"distance_to_m"`
	TimeFromMin   *int64 `json:"time_from_min"`
	TimeToMin     *int64 `json:"time_to_min"`
}
