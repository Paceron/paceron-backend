package dashboard

import (
	"math"

	"simple-arq-golang/cmd/api/domains/constants"
)

// DefaultTolerance es la tolerancia de cumplimiento por defecto (design.md D5):
// sin ella "cumplió el objetivo" es una línea binaria donde el 5,001 % falla.
// Pasarse cuenta como cumplimiento.
const DefaultTolerance = 0.05

// isCompliant evalúa una serie contra su objetivo.
//
// El kind declara qué criterios exige y con qué relación: `ritmo_continuo`
// exige tiempo Y distancia (ir más rápido pero quedarse corto de distancia no
// es cumplir), `liviano` cumple con cualquiera de los dos que tenga cargados.
// Superar el objetivo también cumple.
//
// Sin objetivo cargado devuelve false: el que necesita saber si hubo objetivo
// pregunta por hasTarget, y el que tiene que exponer `met_target: null` usa
// metTarget.
func isCompliant(kind string, targets SeriesTargets, achieved SeriesAchieved, tolerance float64) bool {
	if !hasTarget(kind, targets) {
		return false
	}

	timeMet := timeCriterionMet(targets, achieved, tolerance)
	distanceMet := distanceCriterionMet(targets, achieved, tolerance)

	switch constants.ExerciseKind(kind) {
	case constants.ExerciseKindRitmoContinuo:
		return timeMet && distanceMet
	case constants.ExerciseKindLiviano:
		return timeMet || distanceMet
	case constants.ExerciseKindElongacion, constants.ExerciseKindVelocidadTiempo:
		return timeMet
	case constants.ExerciseKindVelocidadDistancia:
		return distanceMet
	default:
		return false
	}
}

// metTarget es el `met_target` de la respuesta: null cuando el ejercicio no
// tiene objetivo cargado, que no es lo mismo que false.
func metTarget(kind string, targets SeriesTargets, achieved SeriesAchieved, tolerance float64) *bool {
	if !hasTarget(kind, targets) {
		return nil
	}
	met := isCompliant(kind, targets, achieved, tolerance)
	return &met
}

// deltaPct es "cuánto mejor o peor respecto del objetivo", en porcentaje
// positivo hacia la mejora. En el eje temporal el signo se invierte: 240 s de
// objetivo con 230 s logrados es +4,2 %, no −4,2 %.
//
// En el eje doble el delta se mide sobre la velocidad —la única magnitud única
// que representa a los dos ejes—, por el mismo motivo: medirlo sobre el
// tiempo haría que un corredor que mejora su tiempo vea un número con color de
// retroceso.
func deltaPct(kind string, targets SeriesTargets, achieved SeriesAchieved) *float64 {
	axis := objectiveAxisForKind(kind, targets)
	if !hasTarget(kind, targets) {
		return nil
	}

	var target, actual float64
	switch axis {
	case AxisTime:
		minutes, ok := positiveInt(targets.Minutes)
		if !ok || achieved.TimeMs <= 0 {
			return nil
		}
		target = minutes * msPerMinute
		actual = float64(achieved.TimeMs)
	case AxisDistance:
		distanceM, ok := positiveInt(targets.DistanceM)
		achievedM, hasAchieved := positiveFloat(achieved.DistanceM)
		if !ok || !hasAchieved {
			return nil
		}
		target = distanceM
		actual = achievedM
	case AxisDistanceAndTime:
		targetKph := targetSpeedKph(targets)
		achievedKph := achievedSpeedKph(achieved)
		if targetKph == nil || achievedKph == nil || !(*targetKph > 0) {
			return nil
		}
		target = *targetKph
		actual = *achievedKph
	default:
		return nil
	}

	var pct float64
	if axis == AxisTime {
		pct = (target - actual) / target * 100
	} else {
		pct = (actual - target) / target * 100
	}
	if isNotFinite(pct) {
		return nil
	}
	return &pct
}

func timeCriterionMet(targets SeriesTargets, achieved SeriesAchieved, tolerance float64) bool {
	minutes, ok := positiveInt(targets.Minutes)
	if !ok || achieved.TimeMs <= 0 {
		return false
	}
	return float64(achieved.TimeMs) <= minutes*msPerMinute*(1+tolerance)
}

func distanceCriterionMet(targets SeriesTargets, achieved SeriesAchieved, tolerance float64) bool {
	targetM, ok := positiveInt(targets.DistanceM)
	if !ok {
		return false
	}
	achievedM, hasAchieved := positiveFloat(achieved.DistanceM)
	if !hasAchieved {
		return false
	}
	return achievedM >= targetM*(1-tolerance)
}

func isNotFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}
