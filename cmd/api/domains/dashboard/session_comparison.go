package dashboard

import (
	"sort"

	"simple-arq-golang/cmd/api/domains/constants"
)

const sessionComparisonDirectionBMinusA = "b_minus_a"

// SessionComparisonContext aporta etiquetas del lado que el service ya resolvió
// dentro del scope. No contiene reglas de autorización ni consulta datos.
type SessionComparisonContext struct {
	TeamID            int64
	GroupID           int64
	GroupName         string
	AthleteUserID     int64
	AthleteName       string
	AthleteSurname    string
	SessionInstanceID int64
	ExerciseID        int64
	ExerciseName      string
}

// BuildSessionComparisonSide agrega una ejecución concreta. Las filas no
// calculables permanecen en series/series_count, pero no entran a KPIs ni a los
// extremos. El RPE se agrega por separado porque no condiciona la validez física.
func BuildSessionComparisonSide(context SessionComparisonContext, rows []ScopedSeries) SessionComparisonSide {
	rows = append([]ScopedSeries(nil), rows...)
	SortSessionComparisonSeries(rows)
	metrics := newSetMetrics(rows)
	side := SessionComparisonSide{
		TeamID:            context.TeamID,
		GroupID:           context.GroupID,
		GroupName:         context.GroupName,
		AthleteUserID:     context.AthleteUserID,
		AthleteName:       context.AthleteName,
		AthleteSurname:    context.AthleteSurname,
		SessionInstanceID: context.SessionInstanceID,
		ExerciseID:        context.ExerciseID,
		ExerciseName:      context.ExerciseName,
		SeriesCount:       metrics.totalSeries,
		ValidSeries:       metrics.validSeries,
		MeanSpeedKph:      metrics.AchievedSpeedKph(),
		Series:            make([]SessionComparisonPoint, 0, len(rows)),
	}
	if len(rows) == 0 {
		return side
	}

	side.Kind = rows[0].Kind
	side.SessionDate = FormatDate(rows[0].SessionDate)

	var distanceTotal float64
	hasDistance := false
	var rpeTotal int64
	for _, row := range rows {
		side.Series = append(side.Series, comparisonPoint(row))
		if row.Valid() {
			if distance, ok := positiveFloat(row.DistanceMeters); ok {
				distanceTotal += distance
				hasDistance = true
			}
		}
		if row.RPE != nil && *row.RPE >= 1 && *row.RPE <= 10 {
			rpeTotal += int64(*row.RPE)
			side.RPECount++
		}
	}
	if hasDistance {
		side.TotalDistanceM = &distanceTotal
	}
	if side.RPECount > 0 {
		mean := float64(rpeTotal) / float64(side.RPECount)
		side.MeanRPE = &mean
	}
	side.Best, side.Worst = comparisonExtremes(rows, side.Kind)
	return side
}

// BuildSessionComparison arma la respuesta final con diferencias B−A y unidades
// estables. Los punteros nulos distinguen ausencia de dato de un valor cero.
func BuildSessionComparison(a, b SessionComparisonSide) SessionComparisonResponse {
	delta := SessionComparisonDelta{Direction: sessionComparisonDirectionBMinusA}
	delta.SpeedDeltaKph = subtractNullable(b.MeanSpeedKph, a.MeanSpeedKph)
	if delta.SpeedDeltaKph != nil && a.MeanSpeedKph != nil && *a.MeanSpeedKph > 0 {
		pct := *delta.SpeedDeltaKph / *a.MeanSpeedKph * 100
		delta.SpeedDeltaPct = &pct
	}
	delta.DistanceDeltaM = subtractNullable(b.TotalDistanceM, a.TotalDistanceM)

	return SessionComparisonResponse{
		A:     a,
		B:     b,
		Delta: delta,
		Units: SessionComparisonUnits{
			Duration: "ms",
			Distance: "m",
			Speed:    "km/h",
			RPE:      "1-10",
		},
	}
}

func comparisonPoint(row ScopedSeries) SessionComparisonPoint {
	return SessionComparisonPoint{
		FeedbackID:       row.FeedbackID,
		SetNumber:        row.SetNumber,
		DurationMs:       row.DurationMs,
		ActiveDurationMs: row.ActiveDurationMs,
		DistanceMeters:   row.DistanceMeters,
		SpeedKph:         row.Achieved().SpeedKph,
		TargetMinutes:    row.TargetMinutes,
		TargetDistanceM:  row.TargetDistanceM,
		RPE:              row.RPE,
	}
}

func comparisonExtremes(rows []ScopedSeries, kind string) (best, worst *SessionComparisonPoint) {
	var bestRow, worstRow *ScopedSeries
	for i := range rows {
		row := &rows[i]
		if !row.Valid() {
			continue
		}
		value, ok := comparisonAxisValue(*row, kind)
		if !ok {
			continue
		}
		if bestRow == nil {
			bestRow, worstRow = row, row
			continue
		}
		bestValue, _ := comparisonAxisValue(*bestRow, kind)
		worstValue, _ := comparisonAxisValue(*worstRow, kind)
		if comparisonBetter(value, bestValue, kind) || (value == bestValue && comparisonRowBefore(*row, *bestRow)) {
			bestRow = row
		}
		if comparisonBetter(worstValue, value, kind) || (value == worstValue && comparisonRowBefore(*row, *worstRow)) {
			worstRow = row
		}
	}
	if bestRow != nil {
		point := comparisonPoint(*bestRow)
		best = &point
	}
	if worstRow != nil {
		point := comparisonPoint(*worstRow)
		worst = &point
	}
	return best, worst
}

func comparisonBetter(value, incumbent float64, kind string) bool {
	if constants.ExerciseKind(kind) == constants.ExerciseKindRitmoContinuo {
		return value < incumbent
	}
	return value > incumbent
}

func comparisonAxisValue(row ScopedSeries, kind string) (float64, bool) {
	switch constants.ExerciseKind(kind) {
	case constants.ExerciseKindRitmoContinuo:
		if value := row.AchievedTimeMs(); value > 0 {
			return float64(value), true
		}
	case constants.ExerciseKindVelocidadDistancia:
		if speed := row.Achieved().SpeedKph; speed != nil {
			return *speed, true
		}
	case constants.ExerciseKindVelocidadTiempo:
		if distance, ok := positiveFloat(row.DistanceMeters); ok {
			return distance, true
		}
	}
	return 0, false
}

func comparisonRowBefore(a, b ScopedSeries) bool {
	if !a.SessionDate.Equal(b.SessionDate) {
		return a.SessionDate.Before(b.SessionDate)
	}
	if a.SetNumber != b.SetNumber {
		return a.SetNumber < b.SetNumber
	}
	return a.FeedbackID < b.FeedbackID
}

// SortSessionComparisonSeries estabiliza el orden del gráfico incluso si la
// fuente de datos no preserva el orden SQL.
func SortSessionComparisonSeries(rows []ScopedSeries) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].SetNumber != rows[j].SetNumber {
			return rows[i].SetNumber < rows[j].SetNumber
		}
		return rows[i].FeedbackID < rows[j].FeedbackID
	})
}

func subtractNullable(b, a *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	delta := *b - *a
	return &delta
}
