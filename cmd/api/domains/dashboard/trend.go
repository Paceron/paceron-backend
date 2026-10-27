package dashboard

import "math"

const (
	// trendMinSeries es el volumen mínimo para que un porcentaje signifique algo:
	// sobre dos puntos es ruido con formato de dato (design.md D11).
	trendMinSeries = 3
	// trendThresholdPct separa mejora de retroceso. Dentro de ±3 % la señal no
	// distingue del ruido de medición.
	trendThresholdPct = 3.0
)

const (
	TrendImproving  = "improving"
	TrendStable     = "stable"
	TrendRegressing = "regressing"
)

// trendPct compara el tercil más reciente contra el más antiguo del período, en
// porcentaje sobre la velocidad (más alto siempre es mejor, así que el signo se
// lee sin ambigüedad).
//
// Las velocidades llegan ordenadas de la más antigua a la más reciente y tienen
// que ser las de las series válidas: ver validSpeeds.
func trendPct(speeds []float64) *float64 {
	if len(speeds) < trendMinSeries {
		return nil
	}

	third := len(speeds) / trendMinSeries
	oldest := mean(speeds[:third])
	newest := mean(speeds[len(speeds)-third:])
	if !(oldest > 0) {
		return nil
	}

	pct := (newest - oldest) / oldest * 100
	if math.IsNaN(pct) || math.IsInf(pct, 0) {
		return nil
	}
	return &pct
}

// trendVerdict clasifica la señal. Sin tendencia la respuesta es "stable": no
// hay nada que clasificar, y llamar "regressing" a la ausencia de datos sería
// mentir.
func trendVerdict(pct *float64) string {
	if pct == nil {
		return TrendStable
	}
	switch {
	case *pct > trendThresholdPct:
		return TrendImproving
	case *pct < -trendThresholdPct:
		return TrendRegressing
	default:
		return TrendStable
	}
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
