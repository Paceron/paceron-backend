package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
)

func TestDefaultWindow_TerminaLaSemanaPasada(t *testing.T) {
	// Martes 13/10/2026.
	window := defaultWindow(time.Date(2026, 10, 13, 11, 0, 0, 0, time.UTC))

	assert.Equal(t, "2026-09-28", window.Start.Format(dateLayout))
	assert.Equal(t, "2026-09-28", window.from().Format(dateLayout))
	assert.Equal(t, "2026-10-11", window.to().Format(dateLayout))
	assert.Equal(t, seedSessionsCount, len(window.stampTargetDates()))
}

// El seed siembra histórico: si la ventana incluyera el día de hoy, el stamp
// chocaría con el guard de día cerrado en las presenciales.
func TestDefaultWindow_NuncaIncluyeHoy(t *testing.T) {
	for _, today := range []time.Time{
		time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 19, 23, 59, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC),
	} {
		window := defaultWindow(today)
		assert.True(t, window.to().Before(startOfDay(today)),
			"la ventana %s tiene que terminar antes de hoy (%s)", window.to().Format(dateLayout), today.Format(dateLayout))
	}
}

func TestStartOfWeek_LunesISO(t *testing.T) {
	// Un miércoles tiene que devolver el lunes anterior, no el siguiente.
	monday := startOfWeek(time.Date(2026, 10, 14, 15, 0, 0, 0, time.UTC))
	assert.Equal(t, "2026-10-12", monday.Format(dateLayout))

	// Un lunes devuelve el mismo día.
	same := startOfWeek(time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC))
	assert.Equal(t, "2026-10-12", same.Format(dateLayout))

	// Un domingo es el último día de la semana que arranca el lunes anterior.
	sunday := startOfWeek(time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	assert.Equal(t, "2026-10-12", sunday.Format(dateLayout))
}

func TestWindow_EstampaMartesYJueves(t *testing.T) {
	window := seedWindow{Start: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)}

	dates := window.stampTargetDates()
	require.Len(t, dates, seedSessionsCount)
	assert.Equal(t, []string{"2026-09-29", "2026-10-01", "2026-10-06", "2026-10-08"},
		formatDates(dates))
	for _, date := range dates {
		weekday := date.Weekday()
		assert.True(t, weekday == time.Tuesday || weekday == time.Thursday,
			"%s cae en %s y el seed solo estampa martes y jueves", date.Format(dateLayout), weekday)
	}
}

func TestWindow_ExcludeDatesSonLosDiasDeDescanso(t *testing.T) {
	window := seedWindow{Start: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)}

	excluded := window.excludeDatesForWeek(0)
	require.Len(t, excluded, planDaysPerWeek-2)
	assert.Equal(t, []string{"2026-09-28", "2026-09-30", "2026-10-02", "2026-10-03", "2026-10-04"}, excluded)

	second := window.excludeDatesForWeek(1)
	require.Len(t, second, planDaysPerWeek-2)
	assert.Equal(t, "2026-10-05", second[0])
	// Ninguna fecha excluida puede ser un martes o jueves de esa semana.
	for _, date := range second {
		assert.NotEqual(t, "2026-10-06", date)
		assert.NotEqual(t, "2026-10-08", date)
	}
}

func TestResolveWindow_StartTieneQueSerLunes(t *testing.T) {
	_, err := resolveWindow("2026-10-13")
	require.Error(t, err, "un martes no puede ser el arranque de dos semanas de lunes a lunes")
	assert.Contains(t, err.Error(), "lunes")

	_, err = resolveWindow("2026-10-12")
	require.NoError(t, err)
}

func TestWeekIndexOfDay(t *testing.T) {
	window := seedWindow{Start: time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)}
	assert.Equal(t, 0, window.weekIndexOfDay(0))
	assert.Equal(t, 0, window.weekIndexOfDay(6))
	assert.Equal(t, 1, window.weekIndexOfDay(7))
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func formatDates(dates []time.Time) []string {
	out := make([]string, 0, len(dates))
	for _, date := range dates {
		out = append(out, date.Format(dateLayout))
	}
	return out
}

// El stamp mapea sequence_no a fecha, así que el bloque principal tiene que caer
// en la fecha que el runner_session va a referenciar.
func TestPlanTemplates_TrainingSoloMartesYJueves(t *testing.T) {
	for _, template := range plans {
		days := template.planDayRequest(map[string]int64{"velocidad": 1, "volumen": 2, "progresiva": 3, "larga": 4})
		require.Len(t, days, planDaysPerWeek)
		training := 0
		for _, day := range days {
			if day.Kind != string(constants.PlanDayKindTraining) {
				assert.Nil(t, day.SessionID, "un día de descanso no lleva session_id")
				continue
			}
			training++
			require.NotNil(t, day.SessionID)
			require.NotNil(t, day.DefaultPresencial, "las dos sesiones por semana son presenciales")
			require.NotNil(t, day.DefaultTimeFrom)
			require.NotNil(t, day.DefaultTimeTo)
			require.NotNil(t, day.DefaultLocation)
		}
		assert.Equal(t, 2, training, "el plan %q tiene que tener 2 días de entrenamiento", template.Name)
	}
}

// Tarea 7.4: desde 8 km el bloque principal es único. Antes puede traer más de
// uno, y es lo que hace que una sesión de volumen tenga dos tipos de series.
func TestSessionTemplates_BloquePrincipalUnicoDesde8km(t *testing.T) {
	for _, template := range sessions {
		mains := 0
		longest := 0
		for _, block := range template.Blocks {
			if block.Role != string(constants.SessionExerciseRoleMain) {
				continue
			}
			mains++
			item, ok := catalogByKey(block.ExerciseKey)
			require.True(t, ok)
			if item.DistanceM != nil && *item.DistanceM > longest {
				longest = *item.DistanceM
			}
		}
		require.NotZero(t, mains)
		if longest >= 8000 {
			assert.Equal(t, 1, mains, "la sesión %q tiene un bloque de %d m: tiene que ser el único principal", template.Name, longest)
		}
	}
}

// Toda sesión tiene la estructura de la tarea 7.4, y el service exige al menos un
// bloque de cada rol, así que sin esto el stamp ni siquiera crearía la sesión.
func TestSessionTemplates_EstructuraCompleta(t *testing.T) {
	for _, template := range sessions {
		roles := map[string]int{}
		for _, block := range template.Blocks {
			roles[block.Role]++
		}
		assert.NotZero(t, roles[string(constants.SessionExerciseRoleWarmup)], template.Name)
		assert.NotZero(t, roles[string(constants.SessionExerciseRoleMain)], template.Name)
		assert.NotZero(t, roles[string(constants.SessionExerciseRoleCooldown)], template.Name)
	}
}
