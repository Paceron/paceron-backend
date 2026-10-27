package dashboard

import "math"

// coefficientOfVariation es σ(v) / μ(v) sobre la velocidad de las series
// válidas, en km/h.
//
// Es la velocidad y no el ritmo (min/km) a propósito: son inversas, así que
// darían valores distintos con el signo de la mejora invertido. Con velocidad,
// menos dispersión y más alto significan las dos cosas "mejor".
//
// σ es poblacional (divide por n), no muestral: las series son el conjunto
// completo del período, no una muestra de una población mayor.
//
// Devuelve nil con menos de 2 valores o con μ = 0 —dividir por cero daría
// Infinity, que el front no tiene forma de pintar— y descarta los valores no
// positivos o infinitos, que no son velocidades.
func coefficientOfVariation(speeds []float64) *float64 {
	values := make([]float64, 0, len(speeds))
	for _, v := range speeds {
		if v > 0 && !math.IsInf(v, 0) {
			values = append(values, v)
		}
	}
	if len(values) < 2 {
		return nil
	}

	mu := mean(values)
	if !(mu > 0) {
		return nil
	}

	var sumSquares float64
	for _, v := range values {
		delta := v - mu
		sumSquares += delta * delta
	}
	sigma := math.Sqrt(sumSquares / float64(len(values)))

	cv := sigma / mu
	if math.IsNaN(cv) || math.IsInf(cv, 0) {
		return nil
	}
	return &cv
}
