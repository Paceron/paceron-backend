package main

import (
	"fmt"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/services"
)

// verify corre el MISMO service que los endpoints del dashboard y compara los
// seis contadores de data_quality contra lo que el propio plan del seed esperaba.
//
// Es el criterio de aceptación de la tarea 9.3, y por eso son dos números
// comparados y no uno impreso: si el seed quedara "todo limpio", los seis
// contadores darían cero y la pantalla se vería igual de linda sin ejercitar
// ninguna de las ramas que el change especifico. La comparación falla el seed si
// el dataset y las fórmulas dejan de concordar.
//
// La expectativa NO recalcula la calidad a mano: se arman ScopedSeries con las
// filas que el plansays que se escribieron y se pasa por dashboard.BuildSummary,
// o sea que usa la misma lógica del dominio que usa el endpoint.
func (s *seeder) verify(days []seededDay, planned []plannedSeries) error {
	dashboardService := services.NewDashboardService(
		daos.NewDashboardDao(s.db),
		services.NewDashboardScopeService(daos.NewGroupDao(s.db), daos.NewGroupUserDao(s.db), daos.NewTeamDao(s.db)),
		daos.NewUserDao(s.db),
		daos.NewGroupUserDao(s.db),
	)

	groupID := s.groupID
	query := services.MetricsQuery{
		TeamID:      s.teamID,
		GroupID:     &groupID,
		From:        s.window.from().Format(dateLayout),
		To:          s.window.to().Format(dateLayout),
		Granularity: "session",
		Tolerance:   "0.05",
	}

	summary, err := dashboardService.Summary(nil, s.ownerID, query)
	if err != nil {
		return fmt.Errorf("el dashboard no pudo leer el dataset sembrado: %w", err)
	}
	expected := expectedDataQuality(planned)
	actual := summary.DataQuality

	fmt.Println("\ndata_quality (summary del scope completo):")
	fmt.Printf("  %-20s esperado %-6d obtenido %-6d\n", "invalid_series", expected.InvalidSeries, actual.InvalidSeries)
	fmt.Printf("  %-20s esperado %-6d obtenido %-6d\n", "degraded_time", expected.DegradedTime, actual.DegradedTime)
	fmt.Printf("  %-20s esperado %-6d obtenido %-6d\n", "unknown_status", expected.UnknownStatus, actual.UnknownStatus)
	fmt.Printf("  %-20s esperado %-6d obtenido %-6d\n", "open_sessions", expected.OpenSessions, actual.OpenSessions)
	fmt.Printf("  %-20s esperado %-6d obtenido %-6d\n", "legacy_sessions", expected.LegacySessions, actual.LegacySessions)
	fmt.Printf("  %-20s esperado %-6t obtenido %-6t\n", "insufficient_data", expected.InsufficientData, actual.InsufficientData)
	fmt.Printf("  (%d series entraron al KPI)\n", summary.SeriesCount)

	if err := compareQuality(expected, actual); err != nil {
		return err
	}

	s.reportDefects(planned)
	return s.verifyInsufficientData(dashboardService, query)
}

// verifyInsufficientData comprueba el segundo criterio: el caso de "pocos datos"
// no se ve en el scope completo (donde hay cientos de series), sino filtrando por
// el corredor que hizo una sola sesión.
func (s *seeder) verifyInsufficientData(dashboardService services.DashboardServiceInterface, base services.MetricsQuery) error {
	var athlete seedAthlete
	found := false
	for _, candidate := range s.roster {
		if candidate.Archetype.Key == archetypeEnProgreso {
			athlete, found = candidate, true
			break
		}
	}
	if !found {
		s.warnings = append(s.warnings,
			"el roster no tiene corredor 'en progreso': no se pudo verificar insufficient_data por corredor")
		return nil
	}
	query := base
	query.AthleteUserIDs = []int64{athlete.UserID}
	summary, err := dashboardService.Summary(nil, s.ownerID, query)
	if err != nil {
		return fmt.Errorf("el dashboard no pudo leer el filtro por corredor: %w", err)
	}
	fmt.Printf("\ninsufficient_data filtrando por el corredor #%d (%s): %d series, %t\n",
		athlete.Index, athlete.Archetype.Label, summary.SeriesCount, summary.DataQuality.InsufficientData)
	if !summary.DataQuality.InsufficientData {
		return fmt.Errorf("el corredor con una sola sesión dio %d series y insufficient_data=%t; se esperaba true",
			summary.SeriesCount, summary.DataQuality.InsufficientData)
	}
	return nil
}

// reportDefects lista las series rotas a propósito: es el mapa entre "el contador
// dio 7" y la serie concreta que la produce, para no tener que ir a la base a buscarla.
func (s *seeder) reportDefects(planned []plannedSeries) {
	broken := make([]plannedSeries, 0, len(defectPlan))
	for _, item := range planned {
		if item.spec.Defect != defectNone {
			broken = append(broken, item)
		}
	}
	if len(broken) == 0 {
		return
	}
	fmt.Println("\nseries rotas a propósito:")
	for _, item := range broken {
		fmt.Printf("  %s  corredor #%d  %s  serie %d  → %s\n",
			item.day.Date.Format(dateLayout), item.athlete.Index,
			item.spec.Input.Exercise.Name, item.spec.Input.Set, item.spec.Defect)
	}
}

func compareQuality(expected, actual dashboard.DataQuality) error {
	var mismatches []string
	if expected.InvalidSeries != actual.InvalidSeries {
		mismatches = append(mismatches, fmt.Sprintf("invalid_series: esperado %d, obtenido %d", expected.InvalidSeries, actual.InvalidSeries))
	}
	if expected.DegradedTime != actual.DegradedTime {
		mismatches = append(mismatches, fmt.Sprintf("degraded_time: esperado %d, obtenido %d", expected.DegradedTime, actual.DegradedTime))
	}
	if expected.UnknownStatus != actual.UnknownStatus {
		mismatches = append(mismatches, fmt.Sprintf("unknown_status: esperado %d, obtenido %d", expected.UnknownStatus, actual.UnknownStatus))
	}
	if expected.OpenSessions != actual.OpenSessions {
		mismatches = append(mismatches, fmt.Sprintf("open_sessions: esperado %d, obtenido %d", expected.OpenSessions, actual.OpenSessions))
	}
	if expected.LegacySessions != actual.LegacySessions {
		mismatches = append(mismatches, fmt.Sprintf("legacy_sessions: esperado %d, obtenido %d", expected.LegacySessions, actual.LegacySessions))
	}
	if expected.InsufficientData != actual.InsufficientData {
		mismatches = append(mismatches, fmt.Sprintf("insufficient_data: esperado %t, obtenido %t", expected.InsufficientData, actual.InsufficientData))
	}
	if len(mismatches) == 0 {
		return nil
	}
	return fmt.Errorf("los contadores de data_quality no coinciden con el plan del seed:\n  - %v",
		mismatches)
}

// expectedDataQuality calcula la calidad esperada armando las ScopedSeries que el
// plan del seed dice que se escribieron y pasándolas por el agregador del dominio.
//
// Replica el WHERE de la query del dashboard (role main, fuera de las sesiones
// wip) con lo que el seed sabe: qué serie se planeó y en qué sesión.
// No recalcula los contadores a mano justamente para no duplicar las reglas del
// dominio; si isValidSeries cambia, la expectativa cambia con ella y la
// comparación sigue siendo una prueba de que el dataset es el que se pidió.
func expectedDataQuality(planned []plannedSeries) dashboard.DataQuality {
	rows := make([]dashboard.ScopedSeries, 0, len(planned))
	openSessions := 0
	seenOpen := map[[2]int]struct{}{}
	for _, item := range planned {
		if !isMainRole(item.block.Role) {
			continue
		}
		if isWipSession(item.day.DayIndex, item.athlete.Index) {
			key := [2]int{item.day.DayIndex, item.athlete.Index}
			if _, seen := seenOpen[key]; !seen {
				seenOpen[key] = struct{}{}
				openSessions++
			}
			// El WHERE de las series excluye las wip: no llegan a contarse.
			continue
		}
		rows = append(rows, dashboard.ScopedSeries{
			Series: dashboard.Series{
				Kind:             item.spec.Kind,
				DurationMs:       item.spec.DurationMs,
				ActiveDurationMs: item.spec.ActiveDurationMs,
				DistanceMeters:   item.spec.DistanceMeters,
				CompletionStatus: item.spec.CompletionStatus,
				HasRunnerSession: !isLegacyAthlete(item.athlete.Index),
			},
			SetNumber:       item.spec.Input.Set,
			TargetMinutes:   item.spec.TargetMinutes,
			TargetDistanceM: item.spec.TargetDistanceM,
		})
	}
	return dashboard.BuildSummary(rows, seedSummaryTolerance, openSessions).DataQuality
}

func isWipSession(day, athlete int) bool {
	for _, open := range openSessionPlan {
		if open.Day == day && open.Athlete == athlete {
			return true
		}
	}
	return false
}
