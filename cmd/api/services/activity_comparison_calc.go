package services

import (
	"math"
	"sort"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// Cálculo puro de la comparación planificado vs realizado (design.md D4-D6).
// No toca la DB: recibe lo que trajo el DAO y arma la respuesta.

const msPerMinute = 60000

// pctOf convierte una proporción (0..1) en porcentaje con 1 decimal, igual que
// buildAttendanceSummary.
func pctOf(ratio float64) float64 {
	return math.Round(ratio*1000) / 10
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// plannedSetsOf devuelve las series planificadas de un ejercicio: repeat_count,
// con 1 como mínimo (mismo criterio que el frontend al armar la sesión).
func plannedSetsOf(ex daos.ComparisonPlannedExercise) int {
	if ex.RepeatCount < 1 {
		return 1
	}
	return ex.RepeatCount
}

func hasMinutes(ex daos.ComparisonPlannedExercise) bool {
	return ex.Minutes != nil && *ex.Minutes > 0
}

func hasDistance(ex daos.ComparisonPlannedExercise) bool {
	return ex.DistanceM != nil && *ex.DistanceM > 0
}

// effectiveDurationMs es la duración que se compara: la activa si existe, si no
// la total.
func effectiveDurationMs(fb *dbs.WorkoutFeedback) *int64 {
	if fb.ActiveDurationMs != nil && *fb.ActiveDurationMs > 0 {
		return fb.ActiveDurationMs
	}
	if fb.DurationMs != nil && *fb.DurationMs > 0 {
		return fb.DurationMs
	}
	return nil
}

// feedbackStatus traduce completion_status. Una fila sin estado pero con
// tiempo o distancia se toma como completada (filas cargadas a mano antes de
// que el estado fuera obligatorio).
func feedbackStatus(fb *dbs.WorkoutFeedback) string {
	if fb.CompletionStatus != nil {
		switch *fb.CompletionStatus {
		case comparison.SetStatusCompleted:
			return comparison.SetStatusCompleted
		case comparison.SetStatusSkipped:
			return comparison.SetStatusSkipped
		}
		return comparison.SetStatusUnregistered
	}
	if effectiveDurationMs(fb) != nil || (fb.DistanceMeters != nil && *fb.DistanceMeters > 0) {
		return comparison.SetStatusCompleted
	}
	return comparison.SetStatusUnregistered
}

// normalizeFeedback agrupa el feedback por exercise_instance_id (D6): descarta
// los borrados, pasa a base 1 los ejercicios con alguna serie 0 y deja una sola
// fila por serie, priorizando la del propio atleta y después la más reciente.
func normalizeFeedback(rows []dbs.WorkoutFeedback, athleteID int64) map[int64][]dbs.WorkoutFeedback {
	byExercise := make(map[int64][]dbs.WorkoutFeedback)
	for _, row := range rows {
		if row.DeletedAt != nil {
			continue
		}
		byExercise[row.AssignedExerciseID] = append(byExercise[row.AssignedExerciseID], row)
	}

	for exerciseID, list := range byExercise {
		zeroBased := false
		for _, row := range list {
			if row.SetNumber == 0 {
				zeroBased = true
				break
			}
		}

		bySet := make(map[int]dbs.WorkoutFeedback)
		for _, row := range list {
			if zeroBased {
				row.SetNumber++
			}
			current, exists := bySet[row.SetNumber]
			if !exists || preferFeedback(row, current, athleteID) {
				bySet[row.SetNumber] = row
			}
		}

		deduped := make([]dbs.WorkoutFeedback, 0, len(bySet))
		for _, row := range bySet {
			deduped = append(deduped, row)
		}
		sort.Slice(deduped, func(i, j int) bool { return deduped[i].SetNumber < deduped[j].SetNumber })
		byExercise[exerciseID] = deduped
	}
	return byExercise
}

// preferFeedback dice si candidate le gana a current para la misma serie.
func preferFeedback(candidate, current dbs.WorkoutFeedback, athleteID int64) bool {
	candidateOwn := candidate.FeedbackOwnerUserID == athleteID
	currentOwn := current.FeedbackOwnerUserID == athleteID
	if candidateOwn != currentOwn {
		return candidateOwn
	}
	if !candidate.UpdatedAt.Equal(current.UpdatedAt) {
		return candidate.UpdatedAt.After(current.UpdatedAt)
	}
	return candidate.ID > current.ID
}

// plannedPace calcula el ritmo planificado en segundos por km a partir de la
// velocidad.
func plannedPace(ex daos.ComparisonPlannedExercise) *float64 {
	if ex.SpeedKph == nil || *ex.SpeedKph <= 0 {
		return nil
	}
	pace := round1(3600 / *ex.SpeedKph)
	return &pace
}

// actualPace usa el ritmo guardado si existe; si no, lo deriva de la duración y
// la distancia (ms / m = segundos por km).
func actualPace(fb *dbs.WorkoutFeedback, durationMs *int64) *float64 {
	if fb.PaceSecPerKM != nil && *fb.PaceSecPerKM > 0 {
		pace := round1(*fb.PaceSecPerKM)
		return &pace
	}
	if durationMs == nil || fb.DistanceMeters == nil || *fb.DistanceMeters <= 0 {
		return nil
	}
	pace := round1(float64(*durationMs) / *fb.DistanceMeters)
	return &pace
}

// setRatio es el cumplimiento de una serie completada como proporción (D4).
func setRatio(ex daos.ComparisonPlannedExercise, durationMs *int64, distance *float64) float64 {
	switch {
	case hasMinutes(ex):
		if durationMs == nil {
			return 0
		}
		return math.Min(float64(*durationMs)/float64(*ex.Minutes*msPerMinute), 1)
	case hasDistance(ex):
		if distance == nil {
			return 0
		}
		return math.Min(*distance/float64(*ex.DistanceM), 1)
	default:
		return 1
	}
}

// compareSet arma la comparación de una serie. fb es nil si no hay registro.
// Devuelve también la proporción de cumplimiento (0 para las series extra, que
// no cuentan).
func compareSet(ex daos.ComparisonPlannedExercise, setNumber int, fb *dbs.WorkoutFeedback, extra bool) (comparison.SetComparison, float64) {
	set := comparison.SetComparison{SetNumber: setNumber, Status: comparison.SetStatusUnregistered, Extra: extra}
	ratio := 0.0
	if fb != nil {
		id := fb.ID
		set.FeedbackID = &id
		set.Status = feedbackStatus(fb)
		set.RPE = fb.RPE
	}

	if set.Status == comparison.SetStatusCompleted {
		duration := effectiveDurationMs(fb)
		set.DurationMs = duration
		set.DistanceMeters = fb.DistanceMeters
		set.PaceSecPerKm = actualPace(fb, duration)
		ratio = setRatio(ex, duration, fb.DistanceMeters)

		if hasMinutes(ex) && duration != nil {
			deviation := *duration - int64(*ex.Minutes*msPerMinute)
			set.DeviationDurationMs = &deviation
		}
		if hasDistance(ex) && fb.DistanceMeters != nil {
			deviation := round1(*fb.DistanceMeters - float64(*ex.DistanceM))
			set.DeviationDistanceM = &deviation
		}
	}

	if extra {
		return set, 0
	}
	pct := pctOf(ratio)
	set.CompliancePct = &pct
	return set, ratio
}

// compareExercise arma la comparación de un ejercicio con sus series
// planificadas y las registradas de más. Devuelve la suma de proporciones de
// sus series planificadas.
func compareExercise(ex daos.ComparisonPlannedExercise, feedback []dbs.WorkoutFeedback) (comparison.ExerciseComparison, float64) {
	planned := plannedSetsOf(ex)
	result := comparison.ExerciseComparison{
		ExerciseInstanceID: ex.ExerciseInstanceID,
		Name:               ex.Name,
		Role:               ex.Role,
		Kind:               ex.Kind,
		Intensity:          ex.Intensity,
		HasTarget:          hasMinutes(ex) || hasDistance(ex),
		Planned: comparison.PlannedTarget{
			Sets:         planned,
			Minutes:      ex.Minutes,
			DistanceM:    ex.DistanceM,
			SpeedKph:     ex.SpeedKph,
			PaceSecPerKm: plannedPace(ex),
		},
		Sets: make([]comparison.SetComparison, 0, planned),
	}

	bySet := make(map[int]*dbs.WorkoutFeedback, len(feedback))
	for i := range feedback {
		bySet[feedback[i].SetNumber] = &feedback[i]
	}

	ratioSum := 0.0
	for n := 1; n <= planned; n++ {
		set, ratio := compareSet(ex, n, bySet[n], false)
		ratioSum += ratio
		result.Sets = append(result.Sets, set)
	}
	for i := range feedback {
		if feedback[i].SetNumber > planned {
			set, _ := compareSet(ex, feedback[i].SetNumber, &feedback[i], true)
			result.Sets = append(result.Sets, set)
		}
	}

	for _, set := range result.Sets {
		switch set.Status {
		case comparison.SetStatusCompleted:
			result.CompletedSets++
		case comparison.SetStatusSkipped:
			result.SkippedSets++
		}
	}
	result.CompliancePct = pctOf(ratioSum / float64(planned))
	return result, ratioSum
}

// sessionComparison es el resultado del cálculo para una sesión.
type sessionComparison struct {
	exercises     []comparison.ExerciseComparison
	totals        comparison.SessionTotals
	compliancePct float64
	kinds         []string
}

// compareSession arma la comparación completa de una sesión a partir de sus
// ejercicios planificados (en orden) y el feedback del atleta.
func compareSession(exercises []daos.ComparisonPlannedExercise, feedback []dbs.WorkoutFeedback, athleteID int64) sessionComparison {
	byExercise := normalizeFeedback(feedback, athleteID)
	result := sessionComparison{
		exercises: make([]comparison.ExerciseComparison, 0, len(exercises)),
		kinds:     make([]string, 0, len(exercises)),
	}

	ratioSum := 0.0
	var plannedDuration int64
	var plannedDistance float64
	hasPlannedDuration, hasPlannedDistance := false, false
	seenKinds := make(map[string]bool)

	for _, ex := range exercises {
		exResult, exRatio := compareExercise(ex, byExercise[ex.ExerciseInstanceID])
		ratioSum += exRatio
		result.exercises = append(result.exercises, exResult)

		planned := plannedSetsOf(ex)
		result.totals.PlannedSets += planned
		if hasMinutes(ex) {
			hasPlannedDuration = true
			plannedDuration += int64(planned * *ex.Minutes * msPerMinute)
		}
		if hasDistance(ex) {
			hasPlannedDistance = true
			plannedDistance += float64(planned * *ex.DistanceM)
		}
		if ex.Kind != "" && !seenKinds[ex.Kind] {
			seenKinds[ex.Kind] = true
			result.kinds = append(result.kinds, ex.Kind)
		}

		for _, set := range exResult.Sets {
			switch set.Status {
			case comparison.SetStatusCompleted:
				result.totals.CompletedSets++
				if set.DurationMs != nil {
					result.totals.ActualDurationMs += *set.DurationMs
				}
				if set.DistanceMeters != nil {
					result.totals.ActualDistanceM += *set.DistanceMeters
				}
			case comparison.SetStatusSkipped:
				result.totals.SkippedSets++
			default:
				if !set.Extra {
					result.totals.UnregisteredSets++
				}
			}
		}
	}

	if hasPlannedDuration {
		result.totals.PlannedDurationMs = &plannedDuration
	}
	if hasPlannedDistance {
		result.totals.PlannedDistanceM = &plannedDistance
	}
	result.totals.ActualDistanceM = round1(result.totals.ActualDistanceM)
	if result.totals.PlannedSets > 0 {
		result.compliancePct = pctOf(ratioSum / float64(result.totals.PlannedSets))
	}
	return result
}
