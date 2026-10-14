package main

import (
	"math"
	"math/rand"
	"time"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dashboard"
)

// Semilla fija del generador: el mismo seed produce exactamente el mismo
// dataset en cualquier máquina, que es lo que hace que la tarea 9.3 ("guardar
// el generador con su semilla fija") sirva de dataset de desarrollo para el
// front.
const seedRandomSeed int64 = 20261013

// archetype es un perfil de corredor. Los factores multiplican el objetivo:
// TimeFactor < 1 significa que el corredor logra el objetivo antes que el
// tiempo previsto, DistanceFactor > 1 que llega más lejos.
type archetype struct {
	Key            string
	Label          string
	TimeFactor     float64
	DistanceFactor float64
	Noise          float64
	WeeklyGain     float64
}

const (
	archetypeVelocista  = "velocista"
	archetypeFondista   = "fondista"
	archetypeResistente = "resistente"
	archetypeRegular    = "regular"
	archetypeEnProgreso = "en progreso"
	archetypeCount      = 5
	// onlyDayForEnProgreso es el único día al que asiste el corredor "en
	// progreso": viene de cara al grupo. Como esa sesión tiene un solo bloque
	// principal, su filtro queda con menos de 3 series válidas, que es el caso
	// de insufficient_data.
	onlyDayForEnProgreso = 2
)

// archetypes se asignan al roster en orden: el primer miembro del grupo es
// velocista, el segundo fondista, y así. Con 7 miembros quedan 2 velocistas,
// 2 fondistas, 1 resistente, 1 regular y 1 en progreso — los dos primeros
// arquetipos se repiten, que es como se cumple el "2-3 destacados por
// especialidad" de la tarea 7.8.
var archetypes = [archetypeCount]archetype{
	{Key: archetypeVelocista, Label: "velocista", TimeFactor: 0.93, DistanceFactor: 0.99, Noise: 0.03, WeeklyGain: 0.004},
	{Key: archetypeFondista, Label: "fondista", TimeFactor: 1.07, DistanceFactor: 1.02, Noise: 0.04, WeeklyGain: 0.005},
	{Key: archetypeResistente, Label: "resistente", TimeFactor: 1.00, DistanceFactor: 1.00, Noise: 0.05, WeeklyGain: 0.006},
	{Key: archetypeRegular, Label: "regular", TimeFactor: 1.03, DistanceFactor: 1.00, Noise: 0.06, WeeklyGain: 0.003},
	{Key: archetypeEnProgreso, Label: "en progreso", TimeFactor: 1.08, DistanceFactor: 0.96, Noise: 0.07, WeeklyGain: 0.002},
}

// minAthletes es el mínimo de corredores que el seed necesita para tener los
// cinco arquetipos representados: con menos, el de "en progreso" no existe y se
// pierde el caso de insufficient_data.
const minAthletes = archetypeCount

// attendsIn dice si el corredor asiste a la sesión del día.
func (a archetype) attendsIn(dayIndex int) bool {
	if a.Key == archetypeEnProgreso {
		return dayIndex == onlyDayForEnProgreso
	}
	return true
}

// defectKind es una ruptura deliberada del dataset. El seed NO produce un
// dataset "todo limpio": el change se esforzó en especificar las ramas que se
// disparan con datos sucios, y un dataset donde los seis contadores de
// data_quality dan cero no las ejercita (tarea 9.3).
type defectKind int

const (
	defectNone defectKind = iota
	// defectNoActiveTime saca active_duration_ms: el tiempo sigue siendo usable
	// por el fallback a duration_ms, así que la serie SIGUE siendo válida y suma
	// en degraded_time (design.md D4).
	defectNoActiveTime
	// defectNoDistance saca distance_meters de un ritmo_continuo: sin distancia
	// el tipo no es calculable, así que la serie sale de todos los promedios y
	// suma en invalid_series (design.md D6).
	defectNoDistance
	// defectNoTime saca duration_ms y active_duration_ms a la vez: la serie no
	// tiene tiempo, y un ritmo_continuo sin tiempo tampoco es calculable.
	defectNoTime
	// defectUnknownStatus escribe un completion_status fuera del vocabulario
	// reconocido (o null), que D10 cuenta pero no descarta.
	defectUnknownStatus
)

// String es la etiqueta del contador que la ruptura alimenta: sirve para el log
// por serie del seed y para leer el plan de una vistazo.
func (d defectKind) String() string {
	switch d {
	case defectNoActiveTime:
		return "degraded_time"
	case defectNoDistance:
		return "invalid_series(sin distancia)"
	case defectNoTime:
		return "invalid_series(sin tiempo)"
	case defectUnknownStatus:
		return "unknown_status"
	}
	return "limpio"
}

// plannedDefect es una serie rota a propósito. Van escritas una por una y no
// como "el 5% al azar" porque el criterio de aceptación de la tarea 9.3 es
// verificar los CONTADORES: con una regla, no se puede afirmar qué número tiene
// que dar.
type plannedDefect struct {
	Day      int    // índice del día del seed (0..3)
	Athlete  int    // índice en el roster ordenado por user_id
	Exercise string // catalogKey
	Set      int    // 1-based
	Defect   defectKind
	// Ordinal es la posición en defectPlan: sirve para que dos rupturas del
	// mismo tipo no escriban siempre el mismo valor.
	Ordinal int
}

// defectPlan cubre los cinco contadores que salen del result set de series.
// Ninguna serie del plan cae en una sesión wip (open_sessions), porque el WHERE
// de las series ya excluye las wip y el contador no las vería.
var defectPlan = []plannedDefect{
	// degraded_time: 5 series con active_duration_ms en null.
	{Day: 1, Athlete: 0, Exercise: keyRitmo4k, Set: 1, Defect: defectNoActiveTime},
	{Day: 0, Athlete: 1, Exercise: keyPasadas1000, Set: 1, Defect: defectNoActiveTime},
	{Day: 0, Athlete: 1, Exercise: keyPasadas1000, Set: 3, Defect: defectNoActiveTime},
	{Day: 2, Athlete: 2, Exercise: keyRitmo8k, Set: 1, Defect: defectNoActiveTime},
	{Day: 3, Athlete: 3, Exercise: keyRitmo21k, Set: 1, Defect: defectNoActiveTime},

	// invalid_series: 4 series no calculables (2 sin distancia, 2 sin tiempo).
	{Day: 1, Athlete: 5, Exercise: keyRitmo4k, Set: 1, Defect: defectNoDistance},
	{Day: 2, Athlete: 5, Exercise: keyRitmo8k, Set: 1, Defect: defectNoDistance},
	{Day: 2, Athlete: 3, Exercise: keyRitmo8k, Set: 1, Defect: defectNoTime},
	{Day: 3, Athlete: 5, Exercise: keyRitmo21k, Set: 1, Defect: defectNoTime},

	// unknown_status: 3 series con completion_status fuera del vocabulario.
	{Day: 1, Athlete: 2, Exercise: keyRitmo6k, Set: 1, Defect: defectUnknownStatus},
	{Day: 0, Athlete: 3, Exercise: keyPasadas1000, Set: 2, Defect: defectUnknownStatus},
	{Day: 2, Athlete: 0, Exercise: keyRitmo8k, Set: 1, Defect: defectUnknownStatus},
}

func init() {
	for i := range defectPlan {
		defectPlan[i].Ordinal = i
	}
}

// unknownCompletionStatuses son los valores con los que se rompe el vocabulario
// de completion_status: dos strings que la app nunca escribe y un null. El
// primero es el que menos se parece al vocabulario real, a propósito.
var unknownCompletionStatuses = []*string{
	strPtr("terminada"),
	nil,
	strPtr("wip manual"),
}

type defectKey struct {
	day      int
	athlete  int
	exercise string
	set      int
}

var defectIndex = func() map[defectKey]plannedDefect {
	index := make(map[defectKey]plannedDefect, len(defectPlan))
	for _, d := range defectPlan {
		index[defectKey{day: d.Day, athlete: d.Athlete, exercise: d.Exercise, set: d.Set}] = d
	}
	return index
}()

// lookupDefect devuelve la ruptura planificada para esa serie, si hay.
func lookupDefect(day, athlete int, exerciseKey string, set int) (plannedDefect, bool) {
	d, ok := defectIndex[defectKey{day: day, athlete: athlete, exercise: exerciseKey, set: set}]
	return d, ok
}

// openSession es una sesión (día + corredor) que queda a medias: feedback
// cargado y runner_session en wip. Las cuenta data_quality.open_sessions con una
// segunda query (tarea 3.4d), y su feedback NO entra al KPI (D7).
type openSession struct {
	Day     int
	Athlete int
}

// openSessionPlan deja a medias la última sesión de dos corredores.
var openSessionPlan = []openSession{
	{Day: 3, Athlete: 0},
	{Day: 3, Athlete: 1},
}

// interruptedSession queda en runner_session.status = interrupted: ningún
// contador de calidad lo mira (es una sesión terminada a propósito), pero el KPI
// lo cuenta igual y el front lo muestra distinto de una finalizada.
var interruptedSession = openSession{Day: 2, Athlete: 5}

// legacyInstanceDays son los días cuyas ExerciseInstance quedan con
// source_exercise_id en NULL al final del seed, para que el fallback por nombre
// normalizado (tarea 4.1b) tenga algo que resolver.
//
// El stamp SIEMPRE escribe la referencia —es lo que significa instanciar—, así
// que simular "instancia anterior a la migración" obliga a borrarla a posteriori.
// Ojo con el lifecycle: el backfill del ítem 11 de postgres.go corre en cada
// arranque del backend y rellena estas filas por nombre, que es justamente para
// lo que existe. Para volver a ejercitar el fallback: se corre el seed de nuevo.
var legacyInstanceDays = []int{0}

// legacySessionPlan son los índices de los corredores SIN fila en runner_session:
// su feedback se creó antes del feature de registro de sesión. Es el caso
// informativo de legacy_sessions, no un defecto.
var legacySessionPlan = []int{4, 6}

// seriesInput es lo que entra al generador para una serie concreta.
type seriesInput struct {
	DayIndex   int
	WeekIndex  int
	Date       time.Time
	Exercise   catalogItem
	Role       string
	Set        int
	Repeat     int
	Archetype  archetype
	AthleteIdx int
	StartAt    time.Time
}

// seriesSpec es la serie ya resuelta: lo que el seed va a persistir.
type seriesSpec struct {
	Input  seriesInput
	Kind   string
	Defect defectKind
	// objetivo congelado de la instancia (design.md D3)
	TargetMinutes   *int
	TargetDistanceM *int

	ActiveDurationMs    *int64
	DurationMs          *int64
	DistanceMeters      *float64
	CompletionStatus    *string
	RPE                 *int16
	AvgHeartRate        *int16
	MaxHeartRate        *int16
	ElevationGainMeters *float64
	Cadence             *int16
	Annotations         *string
	StartedAt           *time.Time
	EndedAt             *time.Time
}

// Parámetros del generador que producen el ~10% de series fuera de objetivo
// (tarea 7.8) y la fatiga dentro del ejercicio.
const (
	missRate          = 0.10
	missTimePenalty   = 1.12
	missDistPenalty   = 0.92
	fatiguePerSet     = 0.025
	fatigueDistPerSet = 0.005
	msPerMinute       = 60_000
	// paceSecPer100m es el ritmo de referencia para los ejercicios sin objetivo
	// de tiempo (Velocidad Máxima 100/200 m: 0,2 minutos no son representables).
	paceSecPer100m = 13.5
)

// generateSeries resuelve una serie. Todo el ruido sale de rng, así que la
// secuencia completa es reproducible con la semilla fija.
func generateSeries(rng *rand.Rand, in seriesInput) seriesSpec {
	planned, isPlanned := lookupDefect(in.DayIndex, in.AthleteIdx, in.Exercise.Key, in.Set)

	spec := seriesSpec{
		Input:           in,
		Kind:            in.Exercise.Kind,
		TargetMinutes:   in.Exercise.Minutes,
		TargetDistanceM: in.Exercise.DistanceM,
	}
	if isPlanned {
		spec.Defect = planned.Defect
	}

	// 1. Factor de esfuerzo del corredor: perfil, progresión semanal y ruido.
	factor := in.Archetype.TimeFactor
	factor *= 1 - in.Archetype.WeeklyGain*float64(in.WeekIndex)
	factor *= 1 + jitter(rng, in.Archetype.Noise)
	// 2. Fatiga intra-ejercicio: la última serie siempre es la más lenta.
	factor *= 1 + fatiguePerSet*float64(in.Set-1)
	// 3. ~10% de las series se van del objetivo (tarea 7.8).
	missed := rng.Float64() < missRate
	if missed {
		factor *= missTimePenalty
	}

	distanceFactor := in.Archetype.DistanceFactor * (1 + jitter(rng, in.Archetype.Noise/2))
	distanceFactor *= 1 - fatigueDistPerSet*float64(in.Set-1)
	if missed {
		distanceFactor *= missDistPenalty
	}

	var achievedMs int64
	switch {
	case in.Exercise.Minutes != nil && *in.Exercise.Minutes > 0:
		achievedMs = int64(math.Round(float64(*in.Exercise.Minutes*msPerMinute) * factor))
	case in.Exercise.DistanceM != nil && *in.Exercise.DistanceM > 0:
		achievedMs = int64(math.Round(paceSecPer100m * 1000 * float64(*in.Exercise.DistanceM) / 100 * factor))
	}
	achievedMs = maxInt64(achievedMs, 1000)

	var achievedDistance *float64
	if in.Exercise.DistanceM != nil && *in.Exercise.DistanceM > 0 {
		achievedDistance = floatPtr(math.Round(float64(*in.Exercise.DistanceM) * distanceFactor))
	}

	// duration_ms es el tiempo de pared (con pausas y transiciones) y
	// active_duration_ms el tiempo corriendo. La diferencia entre los dos es lo
	// que hace que una sesión normal NO sea degraded.
	wallClock := achievedMs + int64(math.Round(float64(achievedMs)*wallClockRatio(in)))

	spec.DurationMs = &wallClock
	spec.ActiveDurationMs = &achievedMs
	spec.DistanceMeters = achievedDistance
	spec.CompletionStatus = completionStatus(missed, rng)
	spec.RPE = rpe(factor)
	spec.AvgHeartRate, spec.MaxHeartRate = heartRates(in, factor, rng)
	spec.Cadence = cadence(in.Exercise.Kind, factor, rng)
	spec.ElevationGainMeters = elevation(in, achievedDistance, rng)

	startedAt := in.StartAt.Add(time.Duration(rng.Intn(240)) * time.Second)
	endedAt := startedAt.Add(time.Duration(wallClock) * time.Millisecond)
	spec.StartedAt, spec.EndedAt = &startedAt, &endedAt

	if isPlanned {
		switch planned.Defect {
		case defectNoActiveTime:
			spec.ActiveDurationMs = nil
		case defectNoDistance:
			spec.DistanceMeters = nil
		case defectNoTime:
			spec.DurationMs, spec.ActiveDurationMs = nil, nil
		case defectUnknownStatus:
			spec.CompletionStatus = unknownCompletionStatuses[planned.Ordinal%len(unknownCompletionStatuses)]
		}
	}
	spec.Annotations = strPtr(annotationFor(rng, in, missed, *spec.AvgHeartRate, *spec.RPE))
	return spec
}

func jitter(rng *rand.Rand, amplitude float64) float64 {
	if amplitude <= 0 {
		return 0
	}
	return (rng.Float64()*2 - 1) * amplitude
}

// wallClockRatio es la proporción de pausas sobre el tiempo activo. En running
// es alta (semáforos, beber agua), y por eso D4 compara contra el tiempo
// activo: hacerlo contra el reloj corrido penalizaría a cualquiera que entrene
// en la calle.
func wallClockRatio(in seriesInput) float64 {
	switch constants.ExerciseKind(in.Exercise.Kind) {
	case constants.ExerciseKindElongacion:
		return 0.02
	case constants.ExerciseKindVelocidadDistancia, constants.ExerciseKindVelocidadTiempo:
		return 0.25
	case constants.ExerciseKindLiviano:
		return 0.14
	default:
		return 0.12
	}
}

func completionStatus(missed bool, rng *rand.Rand) *string {
	switch {
	case missed:
		return strPtr(string(dashboard.CompletionPartial))
	case rng.Float64() < 0.12:
		return strPtr(string(dashboard.CompletionPartial))
	default:
		return strPtr(string(dashboard.CompletionCompleta))
	}
}

func rpe(factor float64) *int16 {
	value := int16(math.Round(3 + (factor-0.85)*9))
	if value < 1 {
		value = 1
	}
	if value > 10 {
		value = 10
	}
	return &value
}

func heartRates(in seriesInput, factor float64, rng *rand.Rand) (*int16, *int16) {
	var avg int16
	switch constants.ExerciseKind(in.Exercise.Kind) {
	case constants.ExerciseKindElongacion:
		avg = 96
	case constants.ExerciseKindLiviano:
		avg = 132
	case constants.ExerciseKindVelocidadDistancia, constants.ExerciseKindVelocidadTiempo:
		avg = 168
	default:
		avg = 152
	}
	// El esfuerzo relativo sube el pulso: un fondista que se pasa del objetivo
	// tiene más pulsación que un velocista que lo cumple.
	shift := int16(math.Round((factor - 1) * 24))
	avg = clampInt16(avg + shift + int16(rng.Intn(7)) - 3)
	max := clampInt16(avg + int16(14+rng.Intn(12)))
	return &avg, &max
}

func cadence(kind string, factor float64, rng *rand.Rand) *int16 {
	if constants.ExerciseKind(kind) == constants.ExerciseKindElongacion {
		return nil
	}
	var base int16
	switch constants.ExerciseKind(kind) {
	case constants.ExerciseKindVelocidadDistancia, constants.ExerciseKindVelocidadTiempo:
		base = 196
	case constants.ExerciseKindLiviano:
		base = 168
	default:
		base = 178
	}
	value := clampInt16(base - int16(math.Round((factor-1)*30)) + int16(rng.Intn(5)) - 2)
	return &value
}

// elevation deriva el desnivel de la distancia recorrida. Las series de
// velocidad casi no lo tienen, y en una elongación no aplica.
func elevation(in seriesInput, distanceM *float64, rng *rand.Rand) *float64 {
	if distanceM == nil || constants.ExerciseKind(in.Exercise.Kind) == constants.ExerciseKindVelocidadDistancia {
		return nil
	}
	grade := 0.008 + float64(rng.Intn(8))/1000.0
	return floatPtr(math.Round(*distanceM * grade))
}

func clampInt16(v int16) int16 {
	if v < 40 {
		return 40
	}
	if v > 200 {
		return 200
	}
	return v
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
