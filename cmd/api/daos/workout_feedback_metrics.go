package daos

import (
	"sort"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/utils/routematch"
)

// Métricas denormalizadas del leaderboard (gamificacion-core D6): funciones
// puras de recompute sobre feedback + points. Deterministas por diseño para
// que re-subir points no cambie el valor.

// computePaceSecPerKM devuelve duration_ms / distance_meters (s/km, menor
// mejor). NULL si falta alguno de los dos o alguno es <= 0 (evita división
// por cero y ritmos sin sentido).
func computePaceSecPerKM(durationMs *int64, distanceMeters *float64) *float64 {
	if durationMs == nil || distanceMeters == nil || *durationMs <= 0 || *distanceMeters <= 0 {
		return nil
	}
	pace := float64(*durationMs) / *distanceMeters
	return &pace
}

// computePeakSpeedKPH devuelve la velocidad instantánea máxima en km/h entre
// segmentos de points consecutivos (ordenados por Order). NULL con menos de
// dos puntos o si ningún segmento tiene Δt > 0. El haversine vive en
// utils/routematch (única fuente, compartido con el matching de rutas Task 6).
func computePeakSpeedKPH(points []dbs.WorkoutFeedbackPoint) *float64 {
	if len(points) < 2 {
		return nil
	}
	sorted := make([]dbs.WorkoutFeedbackPoint, len(points))
	copy(sorted, points)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Order < sorted[j].Order })

	peak := -1.0
	for i := 1; i < len(sorted); i++ {
		dt := sorted[i].RecordedAt.Sub(sorted[i-1].RecordedAt).Seconds()
		if dt <= 0 {
			continue
		}
		speedKPH := routematch.HaversineMeters(
			sorted[i-1].Latitude, sorted[i-1].Longitude,
			sorted[i].Latitude, sorted[i].Longitude,
		) / dt * 3.6
		if speedKPH > peak {
			peak = speedKPH
		}
	}
	if peak < 0 {
		return nil
	}
	return &peak
}
