package dashboard

import "simple-arq-golang/cmd/api/domains/constants"

// Series es una fila de workout_feedback con el contexto que las fórmulas del
// KPI necesitan y que no están en la fila: el kind del ejercicio (vive en
// exercise_instances) y si su sesión tuvo fila en runner_session.
//
// No es el modelo de dbs: el DAO mapea las columnas a esta struct, que es la
// entrada única de todo el paquete.
type Series struct {
	Kind             string
	DurationMs       *int64
	ActiveDurationMs *int64
	DistanceMeters   *float64
	CompletionStatus *string
	HasRunnerSession bool
}

// TimeQuality marca si el tiempo logrado viene del tiempo activo o del
// fallback: el número entra igual en los KPIs, pero la degradación queda
// visible (design.md D4).
type TimeQuality string

const (
	TimeQualityHigh     TimeQuality = "alta"
	TimeQualityDegraded TimeQuality = "degraded"
)

// Completion es el vocabulario reconocido de completion_status. El campo es
// texto libre sin validación (design.md D10), así que lo no reconocido cuenta
// como unknown y NO descarta la serie.
type Completion string

const (
	CompletionCompleta Completion = "completa"
	CompletionPartial  Completion = "partial"
	CompletionSkipped  Completion = "skipped"
	CompletionUnknown  Completion = "unknown"
)

// timeQuality es "alta" si el tiempo activo es usable. Un active_duration_ms en
// 0 o negativo no es un tiempo: cae al fallback, y la serie queda inválida si
// el duration tampoco sirve.
func timeQuality(s Series) TimeQuality {
	if activeDurationMs(s) > 0 {
		return TimeQualityHigh
	}
	return TimeQualityDegraded
}

// achievedTimeMs es COALESCE(active_duration_ms, duration_ms). La serie se
// evalúa sobre el tiempo logrado, no sobre duration_ms en exclusiva: una serie
// con active_duration_ms válido y duration_ms nulo es perfectamente medible.
func achievedTimeMs(s Series) int64 {
	if active := activeDurationMs(s); active > 0 {
		return active
	}
	if s.DurationMs != nil && *s.DurationMs > 0 {
		return *s.DurationMs
	}
	return 0
}

func activeDurationMs(s Series) int64 {
	if s.ActiveDurationMs == nil {
		return 0
	}
	return *s.ActiveDurationMs
}

func completionTaxonomy(s Series) Completion {
	if s.CompletionStatus == nil {
		return CompletionUnknown
	}
	switch Completion(*s.CompletionStatus) {
	case CompletionCompleta:
		return CompletionCompleta
	case CompletionPartial:
		return CompletionPartial
	case CompletionSkipped:
		return CompletionSkipped
	default:
		return CompletionUnknown
	}
}

// isValidSeries decide si la serie entra a los promedios, al cumplimiento y a
// los rankings. Exige las dos cosas de la spec: tiempo logrado mayor a cero y
// el campo que su kind exige para el eje, también mayor a cero.
//
// Un kind fuera de los cinco vigentes no es calculable: devuelve false y la
// serie queda fuera de los cálculos en vez de romperlos.
func isValidSeries(kind string, s Series) bool {
	hasTime := achievedTimeMs(s) > 0
	_, hasDistance := positiveFloat(s.DistanceMeters)

	switch constants.ExerciseKind(kind) {
	case constants.ExerciseKindLiviano:
		return hasTime || hasDistance
	case constants.ExerciseKindElongacion, constants.ExerciseKindVelocidadTiempo:
		return hasTime
	case constants.ExerciseKindRitmoContinuo:
		return hasTime && hasDistance
	case constants.ExerciseKindVelocidadDistancia:
		return hasDistance
	default:
		return false
	}
}

// validSpeeds son las velocidades logradas de las series válidas, en el orden
// en que llegan (cronológico): es el input de trendPct, de
// coefficientOfVariation y del contador insufficient_data. Las series sin
// distancia quedan fuera —no tienen velocidad definible—, y las inválidas
// también.
func validSpeeds(series []Series) []float64 {
	speeds := make([]float64, 0, len(series))
	for _, s := range series {
		if !isValidSeries(s.Kind, s) {
			continue
		}
		if speed := achievedSpeedKph(achievedFromSeries(s)); speed != nil {
			speeds = append(speeds, *speed)
		}
	}
	return speeds
}
