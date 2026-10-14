package main

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
)

func catalogItemForTest(t *testing.T, key string) catalogItem {
	t.Helper()
	item, ok := catalogByKey(key)
	require.True(t, ok, "no existe el ejercicio %q en el catálogo del seed", key)
	return item
}

func inputForTest(t *testing.T, key string, arch archetype, day, week, set, repeat int) seriesInput {
	t.Helper()
	return seriesInput{
		DayIndex:   day,
		WeekIndex:  week,
		Date:       time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC),
		Exercise:   catalogItemForTest(t, key),
		Role:       string(constants.SessionExerciseRoleMain),
		Set:        set,
		Repeat:     repeat,
		Archetype:  arch,
		AthleteIdx: 99,
		StartAt:    time.Date(2026, 10, 6, 8, 0, 0, 0, time.UTC),
	}
}

// La semilla fija es lo que hace que §9.3 sirva: mismo seed, mismo dataset.
func TestGenerateSeries_EsDeterminista(t *testing.T) {
	in := inputForTest(t, keyRitmo10k, archetypes[0], 2, 1, 1, 1)

	first := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in)
	second := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in)

	assert.Equal(t, first.DurationMs, second.DurationMs)
	assert.Equal(t, first.ActiveDurationMs, second.ActiveDurationMs)
	assert.Equal(t, first.DistanceMeters, second.DistanceMeters)
	assert.Equal(t, first.Annotations, second.Annotations)
}

func TestGenerateSeries_TiempoPositivoYDistanciaCercaDelObjetivo(t *testing.T) {
	in := inputForTest(t, keyRitmo10k, archetypes[2], 2, 1, 1, 1)
	spec := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in)

	require.NotNil(t, spec.ActiveDurationMs)
	assert.Greater(t, *spec.ActiveDurationMs, int64(0))
	require.NotNil(t, spec.DistanceMeters)
	assert.InDelta(t, 10000, *spec.DistanceMeters, 1200, "el ruido no puedeotopicar el objetivo de distancia")
	assert.Greater(t, *spec.DurationMs, *spec.ActiveDurationMs,
		"el tiempo de pared tiene que ser mayor que el activo: si no, nunca hay pausas y nada es degraded")
}

// La fatiga intra-ejercicio (tarea 7.8) tiene que notarse: la última serie de un
// bloque de 5 nunca es la más rápida.
func TestGenerateSeries_FatigaIntraEjercicio(t *testing.T) {
	arch := archetypes[2]
	first := generateSeries(rand.New(rand.NewSource(1)), inputForTest(t, keyPasadas1000, arch, 0, 0, 1, 5))
	fifth := generateSeries(rand.New(rand.NewSource(1)), inputForTest(t, keyPasadas1000, arch, 0, 0, 5, 5))

	require.NotNil(t, first.ActiveDurationMs)
	require.NotNil(t, fifth.ActiveDurationMs)
	assert.Greater(t, *fifth.ActiveDurationMs, *first.ActiveDurationMs)
}

// El velocista es más rápido que el fondista en el mismo ejercicio: sin esto los
// arquetipos serían decoración y el leaderboard no tendría sentido.
func TestGenerateSeries_ArquetiposSeDifieren(t *testing.T) {
	var velocista, fondista int64
	for seed := int64(0); seed < 40; seed++ {
		velocista += *generateSeries(rand.New(rand.NewSource(seed)), inputForTest(t, keyRitmo4k, archetypes[0], 0, 0, 1, 1)).ActiveDurationMs
		fondista += *generateSeries(rand.New(rand.NewSource(seed)), inputForTest(t, keyRitmo4k, archetypes[1], 0, 0, 1, 1)).ActiveDurationMs
	}
	assert.Less(t, velocista, fondista, "el velocista no puede tardar más que el fondista en la misma carrera")
}

// La progresión semanal: en la semana 1 el mismo corredor va mejor que en la semana 0.
func TestGenerateSeries_ProgresionSemanal(t *testing.T) {
	var week0, week1 int64
	for seed := int64(0); seed < 40; seed++ {
		week0 += *generateSeries(rand.New(rand.NewSource(seed)), inputForTest(t, keyRitmo8k, archetypes[2], 0, 0, 1, 1)).ActiveDurationMs
		week1 += *generateSeries(rand.New(rand.NewSource(seed)), inputForTest(t, keyRitmo8k, archetypes[2], 7, 1, 1, 1)).ActiveDurationMs
	}
	assert.Less(t, week1, week0, "en la segunda semana el corredor tiene que haber mejorado")
}

// ~10% de las series fuera de objetivo (tarea 7.8): ni todas se cumplen (el
// dataset no sería creíble) ni ninguna.
func TestGenerateSeries_UnaParteSeVaDelObjetivo(t *testing.T) {
	arch := archetypes[2]
	outs := 0
	const samples = 400
	for seed := int64(0); seed < samples; seed++ {
		spec := generateSeries(rand.New(rand.NewSource(seed)), inputForTest(t, keyRitmo4k, arch, 1, 0, 1, 1))
		if *spec.CompletionStatus == "partial" {
			outs++
		}
	}
	ratio := float64(outs) / samples
	assert.Greater(t, ratio, 0.02, "no hay series falladas: el dataset parece de laboratorio")
	assert.Less(t, ratio, 0.40, "demasiadas series falladas: no queda nada para comparar")
}

// Velocidad maxima 100 m viene sin objetivo de tiempo (0,2 minutos no caben en un
// INT), pero el tiempo logrado existe y tiene que ser creíble.
func TestGenerateSeries_VelocidadMaximaSinObjetivoDeTiempo(t *testing.T) {
	item := catalogItemForTest(t, keyMax100)
	require.Nil(t, item.Minutes)
	require.NotNil(t, item.DistanceM)

	spec := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), inputForTest(t, keyMax100, archetypes[0], 0, 0, 1, 3))

	require.NotNil(t, spec.ActiveDurationMs)
	assert.GreaterOrEqual(t, *spec.ActiveDurationMs, int64(8000), "100 m de velocista no bajan de 8 s")
	assert.LessOrEqual(t, *spec.ActiveDurationMs, int64(40000), "100 m notakes40 s")
	require.NotNil(t, spec.DistanceMeters)
	assert.InDelta(t, 100, *spec.DistanceMeters, 10)
}

// El plan de defectos tiene que producir exactamente las series que dice, y en
// series que el dashboard puede ver (role main, sesión no wip).
func TestDefectPlan_CubreLosCincoContadoresDeSerie(t *testing.T) {
	counts := map[defectKind]int{}
	for _, defect := range defectPlan {
		counts[defect.Defect]++
		require.NotEqual(t, defectNone, defect.Defect)
		_, ok := catalogByKey(defect.Exercise)
		require.True(t, ok, "el defecto apunta al ejercicio %q, que no está en el catálogo", defect.Exercise)
		assert.GreaterOrEqual(t, defect.Set, 1)
		assert.Less(t, defect.Day, seedSessionsCount)
		require.Less(t, defect.Athlete, desiredAthletes)
		// Ninguna sesión wip puede tener una ruptura: el WHERE de las series ya
		// las excluye y el contador no la vería.
		assert.False(t, isWipSession(defect.Day, defect.Athlete),
			"el defecto de la fila %d caería en una sesión wip", defect.Ordinal)
	}
	assert.Equal(t, 5, counts[defectNoActiveTime])
	assert.Equal(t, 2, counts[defectNoDistance])
	assert.Equal(t, 2, counts[defectNoTime])
	assert.Equal(t, 3, counts[defectUnknownStatus])
}

// El generator tiene que romper lo que el plan dice, no otra cosa.
func TestGenerateSeries_AplicaElDefectPlan(t *testing.T) {
	defect := defectPlan[0]
	require.Equal(t, defectNoActiveTime, defect.Defect)

	in := inputForTest(t, defect.Exercise, archetypes[0], defect.Day, 0, defect.Set, 1)
	in.AthleteIdx = defect.Athlete
	spec := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in)

	assert.Equal(t, defectNoActiveTime, spec.Defect)
	assert.Nil(t, spec.ActiveDurationMs, "degraded_time es active_duration_ms en null, con duration_ms de respaldo")
	require.NotNil(t, spec.DurationMs)
	assert.Greater(t, *spec.DurationMs, int64(0))
}

func TestGenerateSeries_InvalidPorFaltaDeDistancia(t *testing.T) {
	var planned plannedDefect
	for _, defect := range defectPlan {
		if defect.Defect == defectNoDistance {
			planned = defect
			break
		}
	}
	in := inputForTest(t, planned.Exercise, archetypes[0], planned.Day, 0, planned.Set, 1)
	in.AthleteIdx = planned.Athlete
	spec := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in)

	assert.Nil(t, spec.DistanceMeters)
	require.NotNil(t, spec.ActiveDurationMs, "el tiempo sigue: lo que falta es la distancia")
}

func TestGenerateSeries_InvalidPorFaltaDeTiempo(t *testing.T) {
	var planned plannedDefect
	for _, defect := range defectPlan {
		if defect.Defect == defectNoTime {
			planned = defect
			break
		}
	}
	in := inputForTest(t, planned.Exercise, archetypes[0], planned.Day, 0, planned.Set, 1)
	in.AthleteIdx = planned.Athlete
	spec := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in)

	assert.Nil(t, spec.DurationMs)
	assert.Nil(t, spec.ActiveDurationMs)
	require.NotNil(t, spec.DistanceMeters)
}

func TestGenerateSeries_UnknownStatusFueraDelVocabulario(t *testing.T) {
	var planned plannedDefect
	for _, defect := range defectPlan {
		if defect.Defect == defectUnknownStatus {
			planned = defect
			break
		}
	}
	in := inputForTest(t, planned.Exercise, archetypes[0], planned.Day, 0, planned.Set, 1)
	in.AthleteIdx = planned.Athlete
	spec := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in)

	if status := spec.CompletionStatus; status != nil {
		assert.NotContains(t, []string{"completa", "partial", "skipped"}, *status,
			"el vocabulario reconocido no puede aparecer en una serie planeada como desconocida")
	}
}

// Las anotaciones son contextuales (tarea 7.9): dos series del mismo ejercicio en
// la misma sesión no pueden decir lo mismo.
func TestGenerateSeries_AnotacionesContextuales(t *testing.T) {
	seen := map[string]int{}
	for seed := int64(0); seed < 30; seed++ {
		in := inputForTest(t, keyRitmo21k, archetypes[int(seed)%archetypeCount], 3, 1, 1, 1)
		spec := generateSeries(rand.New(rand.NewSource(seed)), in)
		require.NotNil(t, spec.Annotations)
		text := *spec.Annotations
		assert.NotEmpty(t, text)
		assert.Contains(t, text, "FC ", "la anotación tiene que traer el dato de la serie")
		assert.Contains(t, text, "RPE ")
		seen[text]++
	}
	assert.Greater(t, len(seen), 5, "las anotaciones se repiten: el dataset se lee como un texto genérico")
	for text, count := range seen {
		assert.Less(t, count, 30, "la anotación %q aparece en todas las series", text)
	}
}

func TestGenerateSeries_TodoEjercicioDelCatalogoSeGenera(t *testing.T) {
	for _, item := range catalog {
		for _, arch := range archetypes {
			spec := generateSeries(rand.New(rand.NewSource(seedRandomSeed)), inputForTest(t, item.Key, arch, 1, 0, 1, 1))
			assert.Positive(t, *spec.ActiveDurationMs, "%s no generó tiempo", item.Name)
			if item.DistanceM == nil {
				assert.Nil(t, spec.DistanceMeters, "%s no tiene distancia objetivo y no puede tener lograda", item.Name)
			} else {
				require.NotNil(t, spec.DistanceMeters)
				assert.Greater(t, *spec.DistanceMeters, float64(0))
				assert.False(t, math.IsNaN(*spec.DistanceMeters))
			}
			require.NotNil(t, spec.CompletionStatus)
		}
	}
}

// El "en progreso" viene de cara: una sola sesión y menos de 3 series válidas,
// que es lo que produce insufficient_data.
func TestArchetypeEnProgreso_UnaSolaSesion(t *testing.T) {
	enProgreso := archetypes[4]
	require.Equal(t, archetypeEnProgreso, enProgreso.Key)
	attended := 0
	for day := 0; day < seedSessionsCount; day++ {
		if enProgreso.attendsIn(day) {
			attended++
		}
	}
	assert.Equal(t, 1, attended)
}

func TestArquetipos_CubrenTodosLosCorredores(t *testing.T) {
	assigned := map[string]int{}
	for i := 0; i < desiredAthletes; i++ {
		assigned[archetypes[i%archetypeCount].Key]++
	}
	for _, arch := range archetypes {
		assert.NotZero(t, assigned[arch.Key], "el arquetipo %q no quedó asignado a nadie", arch.Key)
	}
	// "2-3 destacados por especialidad" (tarea 7.8).
	assert.GreaterOrEqual(t, assigned[archetypeVelocista], 2)
	assert.GreaterOrEqual(t, assigned[archetypeFondista], 2)
}

func TestAnnotationPool_NoEsGenerico(t *testing.T) {
	for _, kind := range []string{
		string(constants.ExerciseKindLiviano),
		string(constants.ExerciseKindElongacion),
		string(constants.ExerciseKindRitmoContinuo),
		string(constants.ExerciseKindVelocidadDistancia),
	} {
		hit := annotationPool(kind, false)
		missed := annotationPool(kind, true)
		assert.GreaterOrEqual(t, len(hit), 3, "%s sin textos de serie cumplida", kind)
		assert.GreaterOrEqual(t, len(missed), 3, "%s sin textos de serie fallada", kind)
		for _, text := range append(append([]string{}, hit...), missed...) {
			assert.Greater(t, len([]rune(text)), 25, "anotación demasiado corta para ser contextual: %q", text)
		}
	}
	// Un tipo sin textos propios tiene que caer al fallback, no a un índice vacío.
	assert.NotEmpty(t, annotationPool("walking", false))
}

func TestPaceLabel(t *testing.T) {
	// 4 km en 30 min es 7:30/km.
	assert.Equal(t, "7.50 min/km", paceLabel(catalogItemForTest(t, keyRitmo4k)))
	assert.Contains(t, paceLabel(catalogItemForTest(t, keyMax100)), "min/km")
	assert.Equal(t, "no aplica", paceLabel(catalogItemForTest(t, keyElongacion)))
}

func TestBlockDuration(t *testing.T) {
	assert.Equal(t, 30*time.Minute, blockDuration(instanceBlock{Minutes: intPtr(30)}, 1))
	assert.Equal(t, 20*time.Minute, blockDuration(instanceBlock{Minutes: intPtr(4)}, 5))
	assert.Positive(t, blockDuration(instanceBlock{DistanceM: intPtr(100)}, 3).Minutes(),
		"un bloque sin objetivo de tiempo también ocupa lugar en el reloj")
}

func TestDayStart_UsaElHorarioPresencialDelDia(t *testing.T) {
	from := time.Date(1, 1, 1, 18, 30, 0, 0, time.UTC)
	day := seededDay{Date: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), PresencialFrom: &from}
	assert.Equal(t, "2026-10-06T18:30:00Z", dayStart(day).UTC().Format(time.RFC3339))

	// Sin horario presencial: un default fijo y no un panic.
	plain := seededDay{Date: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)}
	assert.False(t, dayStart(plain).IsZero())
}

func TestPlannedStatus(t *testing.T) {
	assert.Equal(t, "wip", plannedStatus(openSessionPlan[0].Day, openSessionPlan[0].Athlete))
	assert.Equal(t, "interrupted", plannedStatus(interruptedSession.Day, interruptedSession.Athlete))
	assert.Equal(t, "finished", plannedStatus(0, 5))
	for _, open := range openSessionPlan {
		assert.NotEqual(t, interruptedSession, open, "una sesión no puede ser wip e interrupted a la vez")
	}
}

func TestExpectedDataQuality_CubrirLosSeisContadores(t *testing.T) {
	planned := fakePlannedSeries(t)
	quality := expectedDataQuality(planned)

	assert.Positive(t, quality.InvalidSeries)
	assert.Positive(t, quality.DegradedTime)
	assert.Positive(t, quality.UnknownStatus)
	assert.Positive(t, quality.OpenSessions)
	assert.Positive(t, quality.LegacySessions)
}

func TestIsLegacyAthlete(t *testing.T) {
	assert.True(t, isLegacyAthlete(legacySessionPlan[0]))
	assert.False(t, isLegacyAthlete(999))
	assert.Len(t, legacySessionPlan, 2, "dos de siete corredores sin registro de sesión es la proporción que se quiere ver")
}

// fakePlannedSeries arma un plan mínimo con los valores reales que produce el
// generador, para ejercitar expectedDataQuality sin base: una serie por cada
// ruptura del plan, una serie limpia de un corredor legacy y una de una sesión wip
// (que el WHERE de las series excluye y por eso no puede contar en ningún lado).
func fakePlannedSeries(t *testing.T) []plannedSeries {
	t.Helper()
	date := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	planned := make([]plannedSeries, 0, len(defectPlan)+2)
	for _, defect := range defectPlan {
		in := inputForTest(t, defect.Exercise, archetypes[defect.Athlete%archetypeCount], defect.Day, 0, defect.Set, 1)
		in.AthleteIdx = defect.Athlete
		in.Role = string(constants.SessionExerciseRoleMain)
		planned = append(planned, plannedSeries{
			spec:    generateSeries(rand.New(rand.NewSource(seedRandomSeed+int64(defect.Ordinal))), in),
			day:     seededDay{DayIndex: defect.Day, Date: date},
			block:   instanceBlock{Role: string(constants.SessionExerciseRoleMain)},
			athlete: seedAthlete{Index: defect.Athlete},
		})
	}
	legacy := legacySessionPlan[0]
	in := inputForTest(t, keyRitmo4k, archetypes[legacy%archetypeCount], 1, 0, 1, 1)
	in.AthleteIdx = legacy
	planned = append(planned, plannedSeries{
		spec:    generateSeries(rand.New(rand.NewSource(seedRandomSeed)), in),
		day:     seededDay{DayIndex: 1, Date: date},
		block:   instanceBlock{Role: string(constants.SessionExerciseRoleMain)},
		athlete: seedAthlete{Index: legacy},
	})
	wip := openSessionPlan[0]
	in = inputForTest(t, keyRitmo4k, archetypes[wip.Athlete%archetypeCount], wip.Day, 1, 1, 1)
	in.AthleteIdx = wip.Athlete
	planned = append(planned, plannedSeries{
		spec:    generateSeries(rand.New(rand.NewSource(seedRandomSeed+99)), in),
		day:     seededDay{DayIndex: wip.Day, Date: date},
		block:   instanceBlock{Role: string(constants.SessionExerciseRoleMain)},
		athlete: seedAthlete{Index: wip.Athlete},
	})
	return planned
}

// La serie wip no puede entrar a ningún contador: es la razón de ser del WHERE.
func TestExpectedDataQuality_LaSesionWipNoCuenta(t *testing.T) {
	withWip := expectedDataQuality(fakePlannedSeries(t))
	all := fakePlannedSeries(t)
	trimmed := all[:len(all)-1]
	quality := expectedDataQuality(trimmed)

	assert.Equal(t, quality.InvalidSeries, withWip.InvalidSeries, "la serie wip no era inválida: el fixture cambió")
	assert.Equal(t, quality.OpenSessions+1, withWip.OpenSessions,
		"la sesión wip solo puede aparecer en open_sessions")
}

func TestSeedMarker_NoEsGenerico(t *testing.T) {
	assert.Equal(t, fmt.Sprintf("[%s]", "seed_dashboard"), seedMarker)
}
