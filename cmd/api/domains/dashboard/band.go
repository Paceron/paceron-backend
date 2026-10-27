package dashboard

// Band es la banda de cumplimiento de una serie, para el gráfico de barras
// apiladas. El valor vacío es el `band: null` de la respuesta: la serie no tiene
// objetivo cargado, que es distinto de la banda unknown.
type Band string

const (
	BandMet     Band = "met"
	BandNear    Band = "near"
	BandMissed  Band = "missed"
	BandUnknown Band = "unknown"
)

// BandCounts son los cuatro contadores de `band_counts` por bucket. Van
// presentes aunque estén todos en cero, para que el gráfico pueda pintar la
// estructura sin tratarla como dato ausente.
type BandCounts struct {
	Met     int `json:"met"`
	Near    int `json:"near"`
	Missed  int `json:"missed"`
	Unknown int `json:"unknown"`
}

// band clasifica una serie alrededor del objetivo, con la misma tolerancia que
// el cumplimiento.
//
// La velocidad va del lado de la distancia, no del tiempo: ir más rápido es
// mejorar igual que ir más lejos, y con la velocidad en la columna de "menos es
// mejor" un corredor que mejora caería en `missed`.
func band(kind string, targets SeriesTargets, achieved SeriesAchieved, tolerance float64) Band {
	if !hasTarget(kind, targets) {
		return ""
	}

	axis := objectiveAxisForKind(kind, targets)
	target, actual, ok := magnitudes(axis, targets, achieved)
	if !ok {
		return BandUnknown
	}

	if axis == AxisTime {
		switch {
		case actual <= target*(1+tolerance):
			return BandMet
		case actual <= target*(1+2*tolerance):
			return BandNear
		default:
			return BandMissed
		}
	}

	switch {
	case actual >= target*(1-tolerance):
		return BandMet
	case actual >= target*(1-2*tolerance):
		return BandNear
	default:
		return BandMissed
	}
}

// bandCounts agrega las bandas de un conjunto de series. Las series sin objetivo
// cargado no cuentan banda (su band es null), las que lo tienen cargado pero no
// se pueden comparar cuentan como unknown.
//
// La tolerancia es la de la consulta, no DefaultTolerance: un bucket agregado
// con tolerancia 0,10 tiene que clasificar con 0,10.
func bandCounts(series []Evaluable, tolerance float64) BandCounts {
	var counts BandCounts
	for _, s := range series {
		switch band(s.Kind, s.Targets, s.Achieved, tolerance) {
		case BandMet:
			counts.Met++
		case BandNear:
			counts.Near++
		case BandMissed:
			counts.Missed++
		case BandUnknown:
			counts.Unknown++
		}
	}
	return counts
}

// magnitudes traduce el eje a la magnitud que se compara: tiempo en ms,
// distancia en m o velocidad en km/h. ok=false cuando falta cualquiera de los
// dos lados, que es la serie unknown.
func magnitudes(axis Axis, targets SeriesTargets, achieved SeriesAchieved) (target, actual float64, ok bool) {
	switch axis {
	case AxisTime:
		minutes, hasTarget := positiveInt(targets.Minutes)
		if !hasTarget || achieved.TimeMs <= 0 {
			return 0, 0, false
		}
		return minutes * msPerMinute, float64(achieved.TimeMs), true
	case AxisDistance:
		targetM, hasTarget := positiveInt(targets.DistanceM)
		achievedM, hasAchieved := positiveFloat(achieved.DistanceM)
		if !hasTarget || !hasAchieved {
			return 0, 0, false
		}
		return targetM, achievedM, true
	case AxisDistanceAndTime:
		targetKph := targetSpeedKph(targets)
		achievedKph := achievedSpeedKph(achieved)
		if targetKph == nil || achievedKph == nil || !(*targetKph > 0) {
			return 0, 0, false
		}
		return *targetKph, *achievedKph, true
	default:
		return 0, 0, false
	}
}
