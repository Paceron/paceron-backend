package dashboard

import (
	"errors"
	"math"
	"time"
)

// ErrInvalidRunnerPeriod es el error de un `period` fuera de
// month|week|day. El service lo traduce al code estable INVALID_PERIOD.
var ErrInvalidRunnerPeriod = errors.New("dashboard: período del Home inválido")

// RunnerPeriod es el rango calendario de un request del Home: `from` y `to` son
// inclusivos y no llevan hora. `reference_date` es la fecha ya normalizada (la
// del pedido o la del servidor cuando no vino).
type RunnerPeriod struct {
	From           time.Time
	To             time.Time
	DaysConsidered int
	ReferenceDate  time.Time
}

// ResolveRunnerPeriod calcula el período calendario a partir de la fecha de
// referencia. Las reglas son las del contrato: el mes es el mes completo (28,
// 29, 30 o 31 días), la semana va de lunes a domingo (ISO) y el día es la propia
// fecha de referencia.
func ResolveRunnerPeriod(period string, reference time.Time) (RunnerPeriod, error) {
	date := runnerDateOnly(reference)
	switch period {
	case "month":
		from := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(date.Year(), date.Month()+1, 0, 0, 0, 0, 0, time.UTC)
		return RunnerPeriod{
			From:           from,
			To:             to,
			DaysConsidered: to.Day(),
			ReferenceDate:  date,
		}, nil
	case "week":
		// weekday de Go: domingo = 0. (weekday+6)%7 da 0 para lunes y 6 para
		// domingo, que es el desplazamiento hacia atrás hasta el lunes ISO.
		offset := (int(date.Weekday()) + 6) % 7
		from := date.AddDate(0, 0, -offset)
		to := from.AddDate(0, 0, 6)
		return RunnerPeriod{
			From:           from,
			To:             to,
			DaysConsidered: 7,
			ReferenceDate:  date,
		}, nil
	case "day":
		return RunnerPeriod{
			From:           date,
			To:             date,
			DaysConsidered: 1,
			ReferenceDate:  date,
		}, nil
	default:
		return RunnerPeriod{}, ErrInvalidRunnerPeriod
	}
}

// runnerDateOnly normaliza una fecha a medianoche UTC conservando el día
// calendario de la zona de la referencia. El endpoint compara fechas, no
// instantes: la hora del servidor no puede mover el período de día.
func runnerDateOnly(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
}

// RunnerKpiTab es la pestaña del Home. Cada request calcula una sola.
type RunnerKpiTab string

const (
	RunnerKpiTabGeneral            RunnerKpiTab = "general"
	RunnerKpiTabRitmoContinuo      RunnerKpiTab = "ritmo_continuo"
	RunnerKpiTabVelocidadTiempo    RunnerKpiTab = "velocidad_tiempo"
	RunnerKpiTabVelocidadDistancia RunnerKpiTab = "velocidad_distancia"
)

// IsValidRunnerKpiTab dice si el valor es una de las cuatro pestañas.
func IsValidRunnerKpiTab(tab RunnerKpiTab) bool {
	switch tab {
	case RunnerKpiTabGeneral, RunnerKpiTabRitmoContinuo,
		RunnerKpiTabVelocidadTiempo, RunnerKpiTabVelocidadDistancia:
		return true
	default:
		return false
	}
}

// RunnerKpisMetrics son las métricas del Home. Todas las claves viajan siempre;
// las que no aplican a la pestaña o no son calculables quedan en nil para
// serializarse como null.
type RunnerKpisMetrics struct {
	TotalDistanceM   *float64 `json:"total_distance_m"`
	BestDayDistanceM *float64 `json:"best_day_distance_m"`
	BestDayDate      *string  `json:"best_day_date"`
	AverageSpeedKph  *float64 `json:"average_speed_kph"`
	BestSpeedKph     *float64 `json:"best_speed_kph"`
	BestTimeMs       *int64   `json:"best_time_ms"`
	BestDistanceM    *float64 `json:"best_distance_m"`
}

// FilterRunnerKpiRows recorta las filas que entran a la pestaña. General incluye
// todos los tipos (warmup, main y cooldown); las otras tres filtran por kind.
func FilterRunnerKpiRows(rows []ScopedSeries, tab RunnerKpiTab) []ScopedSeries {
	if tab == RunnerKpiTabGeneral {
		return rows
	}
	out := make([]ScopedSeries, 0, len(rows))
	for _, row := range rows {
		if row.Kind == string(tab) {
			out = append(out, row)
		}
	}
	return out
}

// BuildRunnerKpis calcula el bloque de métricas de una pestaña sobre las filas
// ya scoped por el DAO. Reutiliza las fórmulas del dashboard (tiempo logrado,
// velocidad derivada y validez) en vez de duplicarlas.
func BuildRunnerKpis(rows []ScopedSeries, tab RunnerKpiTab) RunnerKpisMetrics {
	metrics := RunnerKpisMetrics{}
	tabRows := FilterRunnerKpiRows(rows, tab)
	if len(tabRows) == 0 {
		return metrics
	}

	metrics.TotalDistanceM = runnerTotalDistance(tabRows)

	switch tab {
	case RunnerKpiTabGeneral:
		metrics.BestDayDistanceM, metrics.BestDayDate = runnerBestDay(tabRows)
	case RunnerKpiTabRitmoContinuo:
		metrics.BestDistanceM = runnerBestDistance(tabRows)
	case RunnerKpiTabVelocidadTiempo:
		metrics.BestTimeMs = runnerBestTime(tabRows)
	}

	// best_speed aplica a general, ritmo_continuo y velocidad_distancia. En
	// general considera todos los tipos: el máximo no se acota por kind.
	if tab != RunnerKpiTabVelocidadTiempo {
		metrics.BestSpeedKph = runnerBestSpeed(tabRows)
	}

	// En general la fila se rotula "Velocidad promedio en ritmo continuo", así
	// que el promedio se pondera solo con esas series. En las demás pestañas se
	// pondera con las series incluidas.
	averageRows := tabRows
	if tab == RunnerKpiTabGeneral {
		averageRows = rowsOfKind(tabRows, RunnerKpiTabRitmoContinuo)
	}
	metrics.AverageSpeedKph = runnerAverageSpeed(averageRows)

	return metrics
}

func rowsOfKind(rows []ScopedSeries, tab RunnerKpiTab) []ScopedSeries {
	out := make([]ScopedSeries, 0, len(rows))
	for _, row := range rows {
		if row.Kind == string(tab) {
			out = append(out, row)
		}
	}
	return out
}

// runnerTotalDistance suma las distancias positivas de las series incluidas. Un
// cero aporta cero, pero no convierte la métrica en null: null es "ninguna serie
// tiene distancia", no "la distancia sumó cero".
func runnerTotalDistance(rows []ScopedSeries) *float64 {
	var total float64
	hasDistance := false
	for _, row := range rows {
		distance, ok := finiteDistance(row.DistanceMeters)
		if !ok {
			continue
		}
		hasDistance = true
		if distance > 0 {
			total += distance
		}
	}
	if !hasDistance {
		return nil
	}
	return &total
}

// runnerBestDay agrupa por fecha calendario y devuelve el máximo de la suma
// diaria junto a su fecha ISO. Los empates se resuelven por la fecha más
// antigua.
func runnerBestDay(rows []ScopedSeries) (*float64, *string) {
	type dayTotal struct {
		total float64
		date  time.Time
	}
	days := map[string]*dayTotal{}
	for _, row := range rows {
		if _, ok := finiteDistance(row.DistanceMeters); !ok {
			continue
		}
		date := runnerDateOnly(row.SessionDate)
		key := FormatDate(date)
		entry, ok := days[key]
		if !ok {
			entry = &dayTotal{date: date}
			days[key] = entry
		}
		if *row.DistanceMeters > 0 {
			entry.total += *row.DistanceMeters
		}
	}
	if len(days) == 0 {
		return nil, nil
	}

	var best *dayTotal
	for _, entry := range days {
		switch {
		case best == nil,
			entry.total > best.total,
			entry.total == best.total && entry.date.Before(best.date):
			best = entry
		}
	}
	date := FormatDate(best.date)
	return &best.total, &date
}

// runnerBestSpeed es el máximo de las velocidades promedio calculadas serie por
// serie. La velocidad siempre se deriva de distancia y tiempo logrado; nunca se
// lee de la instancia.
func runnerBestSpeed(rows []ScopedSeries) *float64 {
	var best *float64
	for _, row := range rows {
		speed := achievedFromSeries(row.Series).SpeedKph
		if speed == nil {
			continue
		}
		if best == nil || *speed > *best {
			best = speed
		}
	}
	return best
}

// runnerAverageSpeed es la media ponderada por distancia: una serie de 21 km a
// 5 km/h pesa más que una de 4 km a 15 km/h. Devuelve nil si ninguna serie tiene
// distancia y tiempo calculables.
func runnerAverageSpeed(rows []ScopedSeries) *float64 {
	var weight, weighted float64
	for _, row := range rows {
		distance, ok := positiveFloat(row.DistanceMeters)
		if !ok {
			continue
		}
		speed := achievedFromSeries(row.Series).SpeedKph
		if speed == nil {
			continue
		}
		weight += distance
		weighted += distance * *speed
	}
	if weight <= 0 {
		return nil
	}
	average := weighted / weight
	if math.IsNaN(average) || math.IsInf(average, 0) {
		return nil
	}
	return &average
}

// runnerBestTime es el mínimo tiempo logrado entre las series de la pestaña.
func runnerBestTime(rows []ScopedSeries) *int64 {
	var best *int64
	for _, row := range rows {
		timeMs := achievedFromSeries(row.Series).TimeMs
		if timeMs <= 0 {
			continue
		}
		if best == nil || timeMs < *best {
			value := timeMs
			best = &value
		}
	}
	return best
}

// runnerBestDistance es la máxima distancia lograda en una sola serie.
func runnerBestDistance(rows []ScopedSeries) *float64 {
	var best *float64
	for _, row := range rows {
		distance, ok := positiveFloat(row.DistanceMeters)
		if !ok {
			continue
		}
		if best == nil || distance > *best {
			value := distance
			best = &value
		}
	}
	return best
}

// finiteDistance acepta cualquier distancia numérica, incluido el cero, para
// distinguir "hay una distancia registrada" de "no hay dato".
func finiteDistance(distance *float64) (float64, bool) {
	if distance == nil {
		return 0, false
	}
	if math.IsNaN(*distance) || math.IsInf(*distance, 0) {
		return 0, false
	}
	return *distance, true
}
