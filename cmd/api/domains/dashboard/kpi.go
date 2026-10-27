package dashboard

import (
	"math"

	"simple-arq-golang/cmd/api/domains/constants"
)

const (
	msPerMinute = 60_000
	msPerHour   = 3_600_000
)

// Axis es el eje del objetivo que declara el kind del ejercicio. El kind es lo
// único que decide cuál es, y el signo del delta se invierte en el temporal
// (menos es mejor): sin esta separación el mismo número significa cosas
// opuestas según el tipo (design.md D1).
type Axis string

const (
	AxisNone            Axis = "none"
	AxisTime            Axis = "time"
	AxisDistance        Axis = "distance"
	AxisDistanceAndTime Axis = "distance_and_time"
)

// SeriesTargets es el objetivo congelado en la ExerciseInstance de la sesión
// (design.md D3): `minutes` y `distance_m` de esa fila, no los del catálogo
// vigente. SpeedKph es la velocidad esperada, siempre derivada de los dos
// anteriores y nunca leída de exercise_instances.speed_kph (design.md D2).
type SeriesTargets struct {
	Minutes   *int
	DistanceM *int
	SpeedKph  *float64
}

// SeriesAchieved es lo que el corredor efectivamente hizo: TimeMs ya viene
// resuelto (COALESCE de active_duration_ms y duration_ms) y SpeedKph es la
// velocidad lograda, derivada.
type SeriesAchieved struct {
	TimeMs    int64
	DistanceM *float64
	SpeedKph  *float64
}

// Evaluable es una serie con todo lo que las fórmulas necesitan: su kind, el
// objetivo congelado de su instancia y lo logrado.
type Evaluable struct {
	Kind     string
	Targets  SeriesTargets
	Achieved SeriesAchieved
}

// objectiveAxisForKind resuelve el eje del objetivo a partir del kind y de qué
// objetivo quedó cargado en la instancia.
//
// El parámetro de targets no es opcional por `liviano`: su eje es el que esté
// cargado, así que el kind solo no alcanza. Un kind fuera de los cinco
// vigentes —incluidos los legacy— no declara eje y devuelve AxisNone; es el
// único caso que produce AxisNone, porque un tipo conocido sin objetivo sigue
// teniendo eje (lo que no tiene es objetivo).
func objectiveAxisForKind(kind string, targets SeriesTargets) Axis {
	switch constants.ExerciseKind(kind) {
	case constants.ExerciseKindElongacion, constants.ExerciseKindVelocidadTiempo:
		return AxisTime
	case constants.ExerciseKindVelocidadDistancia:
		return AxisDistance
	case constants.ExerciseKindRitmoContinuo:
		return AxisDistanceAndTime
	case constants.ExerciseKindLiviano:
		return livianoAxis(targets)
	default:
		return AxisNone
	}
}

// Con los dos objetivos cargados, `liviano` declara el eje doble: es el único
// caso en que ese eje no implica "tiempo Y distancia" —el cumplimiento de
// `liviano` es OR— pero sí el de la velocidad derivada.
func livianoAxis(targets SeriesTargets) Axis {
	_, hasDistance := positiveInt(targets.DistanceM)
	_, hasMinutes := positiveInt(targets.Minutes)
	switch {
	case hasDistance && hasMinutes:
		return AxisDistanceAndTime
	case hasDistance:
		return AxisDistance
	default:
		return AxisTime
	}
}

// hasTarget dice si hay objetivo cargado en el eje que el kind declara. Es lo
// que separa `met_target: null` (no hay objetivo con el cual comparar) de
// `met_target: false` (hay objetivo y no se cumplió).
func hasTarget(kind string, targets SeriesTargets) bool {
	switch objectiveAxisForKind(kind, targets) {
	case AxisTime:
		_, ok := positiveInt(targets.Minutes)
		return ok
	case AxisDistance:
		_, ok := positiveInt(targets.DistanceM)
		return ok
	case AxisDistanceAndTime:
		_, hasDistance := positiveInt(targets.DistanceM)
		_, hasMinutes := positiveInt(targets.Minutes)
		return hasDistance && hasMinutes
	default:
		return false
	}
}

// targetFromExerciseInstance arma el objetivo desde las dos columnas de la
// ExerciseInstance. `minutes` es INT y representa minutos enteros; la
// velocidad esperada solo existe si además hay distancia.
func targetFromExerciseInstance(minutes *int, distanceM *int) SeriesTargets {
	targets := SeriesTargets{Minutes: minutes, DistanceM: distanceM}
	targets.SpeedKph = targetSpeedKph(targets)
	return targets
}

// achievedFromSeries arma lo logrado desde una fila de feedback: el tiempo es
// el activo con fallback al total (design.md D4) y la velocidad sale de ese
// tiempo, nunca de exercise_instances.speed_kph.
func achievedFromSeries(s Series) SeriesAchieved {
	achieved := SeriesAchieved{TimeMs: achievedTimeMs(s), DistanceM: s.DistanceMeters}
	if distanceM, ok := positiveFloat(s.DistanceMeters); ok {
		achieved.SpeedKph = speedFrom(distanceM, achieved.TimeMs)
	}
	return achieved
}

// speedFrom deriva km/h desde distancia en metros y tiempo en ms. Devuelve nil
// —no 0— cuando no es calculable, para que el que falta se distinga del que
// es cero.
func speedFrom(distanceM float64, durationMs int64) *float64 {
	// !(x > 0) en vez de x <= 0: es la forma de descartar NaN sin tratarlo
	// como un valor válido.
	if !(distanceM > 0) || !(durationMs > 0) {
		return nil
	}
	speed := distanceM / 1000 / (float64(durationMs) / msPerHour)
	if math.IsNaN(speed) || math.IsInf(speed, 0) {
		return nil
	}
	return &speed
}

// targetSpeedKph y achievedSpeedKph derivan la velocidad de los dos lados. Las
// fórmulas los usan en vez de leer SeriesTargets.SpeedKph / SeriesAchieved
// .SpeedKph para que un struct armado a mano no pueda mentir.
func targetSpeedKph(targets SeriesTargets) *float64 {
	distanceM, hasDistance := positiveInt(targets.DistanceM)
	minutes, hasMinutes := positiveInt(targets.Minutes)
	if !hasDistance || !hasMinutes {
		return nil
	}
	return speedFrom(distanceM, int64(minutes*msPerMinute))
}

func achievedSpeedKph(achieved SeriesAchieved) *float64 {
	distanceM, ok := positiveFloat(achieved.DistanceM)
	if !ok {
		return nil
	}
	return speedFrom(distanceM, achieved.TimeMs)
}

func positiveInt(v *int) (float64, bool) {
	if v == nil || !(*v > 0) {
		return 0, false
	}
	return float64(*v), true
}

func positiveFloat(v *float64) (float64, bool) {
	if v == nil || !(*v > 0) || math.IsInf(*v, 0) {
		return 0, false
	}
	return *v, true
}
