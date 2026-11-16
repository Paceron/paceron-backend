package main

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// appliedCounts son las filas realmente afectadas por --apply (RowsAffected),
// que pueden diferir del plan si algo cambió entre el dry-run y el apply.
type appliedCounts struct {
	Days           int64
	Instances      int64
	Exercises      int64
	Links          int64
	Feedback       int64
	Points         int64
	Attendances    int64
	RunnerSessions int64
	Achievements   int64
	Stamps         int64
	Unseen         int64
	StreakStates   int64
}

// applyPlan borra todo en una transacción: si cualquier DELETE falla, no queda
// nada a medias.
func applyPlan(db *gorm.DB, p *plan) (*appliedCounts, error) {
	res := &appliedCounts{}
	err := db.Transaction(func(tx *gorm.DB) error {
		// Hijo → padre. Los puntos GPS van antes que el feedback porque
		// feedback_id es FK opaca (sin cascade en DB).
		for _, chunk := range chunkIDs(p.feedbackIDs) {
			points := tx.Where("feedback_id IN ?", chunk).Delete(&dbs.WorkoutFeedbackPoint{})
			if points.Error != nil {
				return fmt.Errorf("borrar puntos GPS: %w", points.Error)
			}
			res.Points += points.RowsAffected

			feedback := tx.Where("id IN ?", chunk).Delete(&dbs.WorkoutFeedback{})
			if feedback.Error != nil {
				return fmt.Errorf("borrar feedback: %w", feedback.Error)
			}
			res.Feedback += feedback.RowsAffected
		}

		if err := deleteRows(tx, &dbs.Attendance{}, "id", attendanceIDs(p.attendances), &res.Attendances); err != nil {
			return fmt.Errorf("borrar asistencias: %w", err)
		}
		if err := deleteRows(tx, &dbs.RunnerSession{}, "id", runnerSessionIDs(p.runnerSessions), &res.RunnerSessions); err != nil {
			return fmt.Errorf("borrar runner_session: %w", err)
		}
		if err := deleteRows(tx, &dbs.GroupCalendarDay{}, "id", p.dayIDs, &res.Days); err != nil {
			return fmt.Errorf("borrar días de calendario: %w", err)
		}

		// Ya no hay días, feedback, asistencias ni runner_session del rango, así
		// que la clasificación solo protege instancias referenciadas por datos de
		// OTRO rango.
		cleanup, err := classifyInstances(tx, p.instanceIDs, pendingDeletes{})
		if err != nil {
			return err
		}
		if err := deleteRows(tx, &dbs.SessionExerciseInstance{}, "session_instance_id", cleanup.Deletable, &res.Links); err != nil {
			return fmt.Errorf("borrar vínculos de ejercicios: %w", err)
		}
		if err := deleteRows(tx, &dbs.ExerciseInstance{}, "id", uniqueIDs(cleanup.ExerciseIDs), &res.Exercises); err != nil {
			return fmt.Errorf("borrar exercise_instances: %w", err)
		}
		if err := deleteRows(tx, &dbs.SessionInstance{}, "id", cleanup.Deletable, &res.Instances); err != nil {
			return fmt.Errorf("borrar session_instances: %w", err)
		}

		if err := deleteGamification(tx, p, res); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// deleteRows borra por una columna IN (chunked) y acumula RowsAffected.
func deleteRows(tx *gorm.DB, model any, column string, ids []int64, total *int64) error {
	for _, chunk := range chunkIDs(ids) {
		res := tx.Where(column+" IN ?", chunk).Delete(model)
		if res.Error != nil {
			return res.Error
		}
		*total += res.RowsAffected
	}
	return nil
}

// deleteGamification limpia lo que quedó desprendido de los entrenamientos
// borrados. Los logros son permanentes (ON CONFLICT DO NOTHING, nunca se
// re-evalúan) y el streak_state es un cache que tampoco se recalcula solo al
// borrar filas, así que ambos hay que sacarlos a mano.
func deleteGamification(tx *gorm.DB, p *plan, res *appliedCounts) error {
	if len(p.athletes) == 0 {
		return nil
	}
	until := untilExclusive(p.scope.to)
	var err error
	if res.Achievements, err = deleteByAthletes(tx, &dbs.UserAchievementProgress{}, p.athletes, "unlocked_at", p.scope.from, until); err != nil {
		return fmt.Errorf("borrar logros: %w", err)
	}
	if res.Stamps, err = deleteByAthletes(tx, &dbs.GamificationStamp{}, p.athletes, "granted_at", p.scope.from, until); err != nil {
		return fmt.Errorf("borrar estampas: %w", err)
	}
	if res.Unseen, err = deleteByAthletes(tx, &dbs.GamificationUnseen{}, p.athletes, "created_at", p.scope.from, until); err != nil {
		return fmt.Errorf("borrar unseen: %w", err)
	}
	// streak_state no tiene fecha: es el "current" cacheado, se resetea entero.
	if res.StreakStates, err = deleteByAthletes(tx, &dbs.GamificationStreakState{}, p.athletes, "", p.scope.from, nil); err != nil {
		return fmt.Errorf("borrar rachas: %w", err)
	}
	return nil
}

// countByAthletes cuenta filas con user_id IN athletes y, si dateColumn no está
// vacío, el rango [since, until) sobre esa columna. Es el mismo predicado que
// deleteByAthletes, para que el dry-run muestre exactamente lo que se borraría.
func countByAthletes(db *gorm.DB, model any, athletes []int64, dateColumn string, since time.Time, until *time.Time) (int64, error) {
	var total int64
	for _, chunk := range chunkIDs(athletes) {
		q := db.Model(model).Where("user_id IN ?", chunk)
		if dateColumn != "" {
			q = q.Where(dateColumn+" >= ?", since)
			if until != nil {
				q = q.Where(dateColumn+" < ?", *until)
			}
		}
		var n int64
		if err := q.Count(&n).Error; err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// deleteByAthletes es el espejo exacto de countByAthletes para --apply.
func deleteByAthletes(tx *gorm.DB, model any, athletes []int64, dateColumn string, since time.Time, until *time.Time) (int64, error) {
	var total int64
	for _, chunk := range chunkIDs(athletes) {
		q := tx.Where("user_id IN ?", chunk)
		if dateColumn != "" {
			q = q.Where(dateColumn+" >= ?", since)
			if until != nil {
				q = q.Where(dateColumn+" < ?", *until)
			}
		}
		res := q.Delete(model)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
	}
	return total, nil
}

func attendanceIDs(rows []attendanceRow) []int64 {
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func runnerSessionIDs(rows []runnerSessionRow) []int64 {
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func printApplied(res *appliedCounts) {
	fmt.Println("\nBorrado aplicado (una transacción, sin rollback parcial):")
	fmt.Printf("  feedback=%d · puntos GPS=%d · asistencias=%d · runner_session=%d\n",
		res.Feedback, res.Points, res.Attendances, res.RunnerSessions)
	fmt.Printf("  días de calendario=%d · session_instances=%d · exercise_instances=%d · vínculos=%d\n",
		res.Days, res.Instances, res.Exercises, res.Links)
	fmt.Printf("  gamificación: logros=%d · estampas=%d · unseen=%d · rachas=%d\n",
		res.Achievements, res.Stamps, res.Unseen, res.StreakStates)
}
