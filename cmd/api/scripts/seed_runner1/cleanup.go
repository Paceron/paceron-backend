package main

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/domains/dbs"
)

func runCleanup(db *gorm.DB, opts options) error {
	m, err := loadManifest(opts.manifest)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("no existe el manifest %s: el cleanup no adivina IDs", opts.manifest)
	}
	expectedMode := manifestModeStrict
	if opts.merge {
		expectedMode = manifestModeMerge
	}
	if err := requireManifestMode(m, expectedMode); err != nil {
		return err
	}
	if m.Email != opts.email {
		return fmt.Errorf("el manifest pertenece a %s y se pidió limpiar %s", m.Email, opts.email)
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if err := validateCleanupReferences(tx, m); err != nil {
			return err
		}
		if err := deleteIDs(tx, &dbs.WorkoutFeedbackPoint{}, m.WorkoutFeedbackPointIDs); err != nil {
			return fmt.Errorf("borrar puntos GPS: %w", err)
		}
		if err := deleteIDs(tx, &dbs.WorkoutFeedback{}, m.WorkoutFeedbackIDs); err != nil {
			return fmt.Errorf("borrar feedback: %w", err)
		}
		if err := deleteIDs(tx, &dbs.Attendance{}, m.AttendanceIDs); err != nil {
			return fmt.Errorf("borrar asistencias: %w", err)
		}
		if len(m.NewAchievementIDs) > 0 {
			if err := tx.Where("user_id = ? AND achievement_id IN ?", m.UserID, m.NewAchievementIDs).
				Delete(&dbs.UserAchievementProgress{}).Error; err != nil {
				return fmt.Errorf("borrar logros del fixture: %w", err)
			}
		}
		if len(m.NewUnseenIDs) > 0 {
			if err := tx.Where("user_id = ? AND id IN ?", m.UserID, m.NewUnseenIDs).
				Delete(&dbs.GamificationUnseen{}).Error; err != nil {
				return fmt.Errorf("borrar unseen del fixture: %w", err)
			}
		}
		if err := deleteIDs(tx, &dbs.GamificationStamp{}, m.NewStampIDs); err != nil {
			return fmt.Errorf("borrar estampas del fixture: %w", err)
		}
		if err := restoreStreakState(tx, m); err != nil {
			return err
		}
		if err := deleteIDs(tx, &dbs.RunnerSession{}, m.RunnerSessionIDs); err != nil {
			return fmt.Errorf("borrar runner_session: %w", err)
		}
		if err := deleteIDs(tx, &dbs.GroupCalendarDay{}, m.CalendarDayIDs); err != nil {
			return fmt.Errorf("borrar días de calendario: %w", err)
		}
		if err := deleteIDs(tx, &dbs.SessionExerciseInstance{}, m.SessionExerciseInstanceIDs); err != nil {
			return fmt.Errorf("borrar vínculos de ejercicios: %w", err)
		}
		if err := deleteIDs(tx, &dbs.ExerciseInstance{}, m.ExerciseInstanceIDs); err != nil {
			return fmt.Errorf("borrar exercise_instances: %w", err)
		}
		if err := deleteIDs(tx, &dbs.SessionInstance{}, m.SessionInstanceIDs); err != nil {
			return fmt.Errorf("borrar session_instances: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	cleanedAt := time.Now().UTC()
	m.LastCleanupAt = &cleanedAt
	if err := writeManifest(opts.manifest, m); err != nil {
		return fmt.Errorf("cleanup aplicado pero no se pudo actualizar el manifest: %w", err)
	}
	fmt.Println("Cleanup aplicado.")
	fmt.Printf("  session_instances=%d · exercise_instances=%d · feedback=%d · puntos=%d\n",
		len(m.SessionInstanceIDs), len(m.ExerciseInstanceIDs), len(m.WorkoutFeedbackIDs), len(m.WorkoutFeedbackPointIDs))
	fmt.Printf("  runner_session=%d · asistencias=%d · días=%d · logros nuevos=%d\n",
		len(m.RunnerSessionIDs), len(m.AttendanceIDs), len(m.CalendarDayIDs), len(m.NewAchievementIDs))
	fmt.Printf("  manifest actualizado: %s\n", opts.manifest)
	return nil
}

func validateCleanupReferences(tx *gorm.DB, m *manifest) error {
	type check struct {
		label string
		model any
		where string
		args  []any
	}
	var checks []check
	if instances := m.SessionInstanceIDs; len(instances) > 0 {
		checks = append(checks,
			check{"días de calendario", &dbs.GroupCalendarDay{}, "session_instance_id IN ? AND id NOT IN ?", []any{instances, nonEmptyOrZero(m.CalendarDayIDs)}},
			check{"asistencias", &dbs.Attendance{}, "training_session_id IN ? AND id NOT IN ?", []any{instances, nonEmptyOrZero(m.AttendanceIDs)}},
			check{"feedback", &dbs.WorkoutFeedback{}, "assigned_session_id IN ? AND id NOT IN ?", []any{instances, nonEmptyOrZero(m.WorkoutFeedbackIDs)}},
			check{"vínculos de ejercicios", &dbs.SessionExerciseInstance{}, "session_instance_id IN ? AND id NOT IN ?", []any{instances, nonEmptyOrZero(m.SessionExerciseInstanceIDs)}},
			check{"runner_session", &dbs.RunnerSession{}, "session_instance_id IN ? AND id NOT IN ?", []any{instances, nonEmptyOrZero(m.RunnerSessionIDs)}},
		)
	}
	if exercises := m.ExerciseInstanceIDs; len(exercises) > 0 {
		checks = append(checks,
			check{"vínculos de ejercicios", &dbs.SessionExerciseInstance{}, "exercise_instance_id IN ? AND id NOT IN ?", []any{exercises, nonEmptyOrZero(m.SessionExerciseInstanceIDs)}},
			check{"feedback", &dbs.WorkoutFeedback{}, "assigned_exercise_id IN ? AND id NOT IN ?", []any{exercises, nonEmptyOrZero(m.WorkoutFeedbackIDs)}},
		)
	}
	if feedback := m.WorkoutFeedbackIDs; len(feedback) > 0 {
		checks = append(checks,
			check{"puntos GPS", &dbs.WorkoutFeedbackPoint{}, "feedback_id IN ? AND id NOT IN ?", []any{feedback, nonEmptyOrZero(m.WorkoutFeedbackPointIDs)}},
		)
	}
	for _, check := range checks {
		var count int64
		if err := tx.Model(check.model).Where(check.where, check.args...).Count(&count).Error; err != nil {
			return fmt.Errorf("validar referencias de %s: %w", check.label, err)
		}
		if count > 0 {
			return fmt.Errorf("hay %d fila(s) de %s sobre las instancias del fixture que no están en el manifest; se aborta sin borrar nada", count, check.label)
		}
	}
	return nil
}

// nonEmptyOrZero evita que un slice vacío se traduzca en NOT IN (NULL), que
// en SQL no matchea nada y escondería referencias.
func nonEmptyOrZero(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{0}
	}
	return ids
}

func deleteIDs(tx *gorm.DB, model any, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Where("id IN ?", ids).Delete(model).Error
}

func restoreStreakState(tx *gorm.DB, m *manifest) error {
	if m.PriorStreakState == nil {
		return tx.Where("user_id = ?", m.UserID).Delete(&dbs.GamificationStreakState{}).Error
	}
	state := dbs.GamificationStreakState{
		UserID:          m.UserID,
		StrictCurrent:   m.PriorStreakState.StrictCurrent,
		FlexibleCurrent: m.PriorStreakState.FlexibleCurrent,
		UpdatedAt:       m.PriorStreakState.UpdatedAt,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"strict_current", "flexible_current", "updated_at"}),
	}).Create(&state).Error
}
