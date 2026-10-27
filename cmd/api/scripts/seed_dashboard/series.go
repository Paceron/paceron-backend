package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/services"
)

// plannedSeries es una serie con el ids resueltos: a qué día, a qué instancia y
// a qué corredor corresponde. El generador no sabe de ids —solo de objetivos—
// para que se pueda testear sin base.
type plannedSeries struct {
	spec    seriesSpec
	day     seededDay
	block   instanceBlock
	athlete seedAthlete
}

// buildSeriesPlan recorre días, corredores y bloques en un orden fijo y
// devuelve las series generadas. El orden es lo que hace determinista el ruido:
// mismo seed, misma lista.
func (s *seeder) buildSeriesPlan(days []seededDay) []plannedSeries {
	rng := rand.New(rand.NewSource(seedRandomSeed))
	planned := make([]plannedSeries, 0, 256)
	for _, day := range days {
		weekIndex := s.window.weekIndexOfDay(day.DayIndex)
		for _, athlete := range s.roster {
			if !athlete.Archetype.attendsIn(day.DayIndex) {
				continue
			}
			// Cada corredor entra a la sesión con su propio margen: en una
			// presencial no arrancan todos en el mismo minuto.
			athleteStart := dayStart(day).Add(time.Duration(athlete.Index*3) * time.Minute)
			cursor := athleteStart
			for _, block := range day.Blocks {
				exercise := catalogItem{
					Key:       block.ExerciseKey,
					Name:      block.Name,
					Kind:      block.Kind,
					Minutes:   block.Minutes,
					DistanceM: block.DistanceM,
				}
				repeat := block.Repeat
				if repeat < 1 {
					repeat = 1
				}
				for set := 1; set <= repeat; set++ {
					spec := generateSeries(rng, seriesInput{
						DayIndex:   day.DayIndex,
						WeekIndex:  weekIndex,
						Date:       day.Date,
						Exercise:   exercise,
						Role:       block.Role,
						Set:        set,
						Repeat:     repeat,
						Archetype:  athlete.Archetype,
						AthleteIdx: athlete.Index,
						StartAt:    cursor,
					})
					planned = append(planned, plannedSeries{
						spec:    spec,
						day:     day,
						block:   block,
						athlete: athlete,
					})
				}
				cursor = cursor.Add(blockDuration(block, repeat))
			}
		}
	}
	s.count("series_planificadas", int64(len(planned)))
	return planned
}

// blockDuration estima cuánto ocupa un bloque en el reloj de pared, para ubicar el
// inicio del siguiente. Un bloque sin objetivo de tiempo (Velocidad Máxima 100/200
// m) se estima con el ritmo de referencia del generador.
func blockDuration(block instanceBlock, repeat int) time.Duration {
	minutes := 0
	switch {
	case block.Minutes != nil && *block.Minutes > 0:
		minutes = *block.Minutes
	case block.DistanceM != nil && *block.DistanceM > 0:
		seconds := paceSecPer100m * float64(*block.DistanceM) / 100
		minutes = int(seconds/60) + 1
	}
	return time.Duration(minutes*repeat) * time.Minute
}

// dayStart es la hora de arranque de la sesión presencial del día. Sale del propio
// calendario (presencial_time_from), que es timestamptz con fecha 0000-01-01 y se
// escribe y lee en UTC por convención del repo: por eso el .UTC() antes de leer la
// hora.
func dayStart(day seededDay) time.Time {
	date := time.Date(day.Date.Year(), day.Date.Month(), day.Date.Day(), 0, 0, 0, 0, time.UTC)
	if day.PresencialFrom == nil {
		return date.Add(8 * time.Hour)
	}
	from := day.PresencialFrom.UTC()
	return date.Add(time.Duration(from.Hour())*time.Hour + time.Duration(from.Minute())*time.Minute)
}

// writeFeedback persiste el feedback de cada serie.
//
// Va por el service y no por el DAO porque el service es el que valida el body
// (tiempos no negativos, ended_at >= started_at, formato de session_date): el
// seed no debería poder escribir una fila que la app no podría crear. El
// corredor se reporta a sí mismo (auth = atleta), que es el camino que no exige
// que el entrenador sea dueño del equipo del atleta.
func (s *seeder) writeFeedback(days []seededDay, planned []plannedSeries) error {
	feedbackService := services.NewWorkoutFeedbackService(daos.NewWorkoutFeedbackDao(s.db))
	created, duplicates := 0, 0
	for _, item := range planned {
		req := s.feedbackRequest(item)
		_, err := feedbackService.Create(nil, item.athlete.UserID, req)
		switch {
		case err == nil:
			created++
		case errors.Is(err, daos.ErrWorkoutFeedbackDuplicate):
			// La serie ya estaba de una corrida anterior: la idempotencia no
			// necesita comparar nada más, el índice único parcial
			// (session, exercise, athlete, owner, set) ya resolvió el choque.
			duplicates++
		default:
			return fmt.Errorf("error guardando el feedback de %s en %s (corredor %d): %w",
				item.spec.Input.Exercise.Name, item.day.Date.Format(dateLayout), item.athlete.UserID, err)
		}
	}
	s.count("feedback_creados", int64(created))
	s.count("feedback_ya_existentes", int64(duplicates))
	return nil
}

func (s *seeder) feedbackRequest(item plannedSeries) workoutfeedback.CreateFeedbackRequest {
	spec := item.spec
	return workoutfeedback.CreateFeedbackRequest{
		TeamID:              int64Ptr(s.teamID),
		AssignedSessionID:   item.day.SessionInstance,
		AssignedExerciseID:  item.block.ExerciseInstance,
		ReportSource:        "corredor",
		SessionDate:         spec.Input.Date.Format(dateLayout),
		SetNumber:           spec.Input.Set,
		StartedAt:           spec.StartedAt,
		EndedAt:             spec.EndedAt,
		DurationMs:          spec.DurationMs,
		ActiveDurationMs:    spec.ActiveDurationMs,
		DistanceMeters:      spec.DistanceMeters,
		RPE:                 spec.RPE,
		AvgHeartRate:        spec.AvgHeartRate,
		MaxHeartRate:        spec.MaxHeartRate,
		CompletionStatus:    spec.CompletionStatus,
		ElevationGainMeters: spec.ElevationGainMeters,
		Cadence:             spec.Cadence,
		Annotations:         spec.Annotations,
	}
}

// writeRunnerSessions deja el estado de sesión de cada corredor.
//
// Tres estados y ningún otro: finished (el caso normal), wip (open_sessions) e
// interrupted (una sesión cortada a propósito). Los corredores del plan legacy no
// reciben fila: es el caso que data_quality.legacy_sessions cuenta.
func (s *seeder) writeRunnerSessions(days []seededDay) error {
	dao := daos.NewRunnerSessionDao(s.db)
	created, updated := 0, 0
	for _, day := range days {
		for _, athlete := range s.roster {
			if isLegacyAthlete(athlete.Index) || !athlete.Archetype.attendsIn(day.DayIndex) {
				continue
			}
			status := plannedStatus(day.DayIndex, athlete.Index)
			wasCreated, err := dao.Create(nil, &dbs.RunnerSession{
				SessionInstanceID: day.SessionInstance,
				AthleteUserID:     athlete.UserID,
				Status:            status,
				StartDate:         dayStart(day).Add(time.Duration(athlete.Index*3) * time.Minute),
			})
			if err != nil {
				return fmt.Errorf("error creando el runner_session del corredor %d en %s: %w",
					athlete.UserID, day.Date.Format(dateLayout), err)
			}
			if wasCreated {
				created++
				continue
			}
			// Ya existía (corrida anterior): el status puede haber cambiado solo
			// si alguien lo editó a mano.
			current, err := dao.GetBySessionAndAthlete(nil, day.SessionInstance, athlete.UserID)
			if err != nil {
				return fmt.Errorf("error leyendo el runner_session del corredor %d: %w", athlete.UserID, err)
			}
			if current.Status == status {
				continue
			}
			endDate := day.Date.Add(10 * time.Hour)
			if err := dao.UpdateStatus(nil, current.ID, status, []string{current.Status}, endDate); err != nil {
				return fmt.Errorf("error actualizando el runner_session del corredor %d a %s: %w",
					athlete.UserID, status, err)
			}
			updated++
		}
	}
	s.count("runner_session_creados", int64(created))
	s.count("runner_session_actualizados", int64(updated))
	return nil
}

// plannedStatus es el estado que le toca a un corredor en un día según el plan de
// datos: finished por defecto, wip para los pares de openSessionPlan e
// interrupted para el caso declarado.
func plannedStatus(day, athlete int) string {
	for _, open := range openSessionPlan {
		if open.Day == day && open.Athlete == athlete {
			return runnersession.RunnerSessionStatusWip
		}
	}
	if interruptedSession.Day == day && interruptedSession.Athlete == athlete {
		return runnersession.RunnerSessionStatusInterrupted
	}
	return runnersession.RunnerSessionStatusFinished
}

// markLegacyInstances deja en NULL el source_exercise_id de las instancias de los
// días indicados, para que el fallback por nombre normalizado del dashboard
// (tarea 4.1b) tenga algo que resolver.
//
// El stamp SIEMPRE escribe la referencia —es lo que significa instanciar—, así
// que simular "instancia anterior a la migración" obliga a borrarla después. El
// lifecycle es parte del runbook: el backfill del ítem 11 de postgres.go corre en
// cada arranque y las vuelve a llenar por nombre, que es justamente para lo que
// existe.
// isLegacyInstanceDay dice si el día tiene instancias a las que el seed les
// borró el source_exercise_id. El espejo de lectura de markLegacyInstances: sin
// esta, la verificación del seed no puede modelar el caso que él mismo crea.
func isLegacyInstanceDay(dayIndex int) bool {
	for _, d := range legacyInstanceDays {
		if d == dayIndex {
			return true
		}
	}
	return false
}

func (s *seeder) markLegacyInstances(days []seededDay) error {
	targets := make(map[int]struct{}, len(legacyInstanceDays))
	for _, dayIndex := range legacyInstanceDays {
		targets[dayIndex] = struct{}{}
	}
	cleared := 0
	for _, day := range days {
		if _, ok := targets[day.DayIndex]; !ok {
			continue
		}
		for _, block := range day.Blocks {
			res := s.db.Model(&dbs.ExerciseInstance{}).
				Where("id = ? AND source_exercise_id IS NOT NULL", block.ExerciseInstance).
				Update("source_exercise_id", nil)
			if res.Error != nil {
				return fmt.Errorf("error limpiando source_exercise_id de la instancia %d: %w", block.ExerciseInstance, res.Error)
			}
			cleared += int(res.RowsAffected)
		}
	}
	s.count("instancias_sin_source_exercise_id", int64(cleared))
	return nil
}

func isLegacyAthlete(index int) bool {
	for _, legacy := range legacySessionPlan {
		if legacy == index {
			return true
		}
	}
	return false
}

func isMainRole(role string) bool {
	return role == string(constants.SessionExerciseRoleMain)
}
