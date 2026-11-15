package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	manifestVersion    = 1
	manifestModeStrict = "strict"
	manifestModeMerge  = "merge"
)

type streakStateSnapshot struct {
	StrictCurrent   int       `json:"strict_current"`
	FlexibleCurrent int       `json:"flexible_current"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// manifest es la fuente de verdad del cleanup: contiene exactamente los IDs
// que creó el fixture (o que adoptó de una corrida previa del mismo ancla) y
// el estado previo de las filas que se actualizaron.
type manifest struct {
	Version       int        `json:"version"`
	Mode          string     `json:"mode"`
	Email         string     `json:"email"`
	UserID        int64      `json:"user_id"`
	Anchor        string     `json:"anchor"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	LastCleanupAt *time.Time `json:"last_cleanup_at,omitempty"`

	PriorStreakState *streakStateSnapshot `json:"prior_streak_state,omitempty"`

	NewAchievementIDs []string `json:"new_achievement_ids"`
	NewUnseenIDs      []int64  `json:"new_unseen_ids"`
	NewStampIDs       []int64  `json:"new_stamp_ids"`

	SessionInstanceIDs         []int64 `json:"session_instance_ids"`
	ExerciseInstanceIDs        []int64 `json:"exercise_instance_ids"`
	SessionExerciseInstanceIDs []int64 `json:"session_exercise_instance_ids"`
	CalendarDayIDs             []int64 `json:"calendar_day_ids"`
	RunnerSessionIDs           []int64 `json:"runner_session_ids"`
	AttendanceIDs              []int64 `json:"attendance_ids"`
	WorkoutFeedbackIDs         []int64 `json:"workout_feedback_ids"`
	WorkoutFeedbackPointIDs    []int64 `json:"workout_feedback_point_ids"`
}

func loadManifest(path string) (*manifest, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer manifest %s: %w", path, err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parsear manifest %s: %w", path, err)
	}
	if m.Version != manifestVersion {
		return nil, fmt.Errorf("manifest %s tiene versión %d y se esperaba %d", path, m.Version, manifestVersion)
	}
	if m.Mode == "" {
		// Manifests previos a --merge no tenían mode: eran estrictos.
		m.Mode = manifestModeStrict
	}
	return &m, nil
}

func requireManifestMode(m *manifest, expected string) error {
	mode := m.Mode
	if mode == "" {
		mode = manifestModeStrict
	}
	if mode != expected {
		return fmt.Errorf("el manifest %s es de modo %q y se esperaba %q; usá el manifest correcto o limpiá el existente", m.Email, mode, expected)
	}
	return nil
}

func writeManifest(path string, m *manifest) error {
	if m == nil {
		return fmt.Errorf("manifest vacío")
	}
	m.Version = manifestVersion
	m.UpdatedAt = time.Now().UTC()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("crear directorio del manifest: %w", err)
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar manifest: %w", err)
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".runner1-manifest-*.json")
	if err != nil {
		return fmt.Errorf("crear manifest temporal: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("escribir manifest temporal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cerrar manifest temporal: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("reemplazar manifest: %w", err)
	}
	return nil
}

// mergeManifest preserva la evidencia de la primera corrida (IDs creados,
// estado previo) y le suma lo que esta corrida creó o adoptó.
func mergeManifest(previous, current *manifest) *manifest {
	if previous == nil {
		return current
	}
	if current == nil {
		return previous
	}
	merged := *previous
	merged.UpdatedAt = current.UpdatedAt
	merged.LastCleanupAt = nil
	if previous.PriorStreakState == nil && current.PriorStreakState != nil {
		merged.PriorStreakState = current.PriorStreakState
	}
	merged.NewAchievementIDs = uniqueStrings(append(append([]string{}, previous.NewAchievementIDs...), current.NewAchievementIDs...))
	merged.NewUnseenIDs = uniqueInt64s(append(append([]int64{}, previous.NewUnseenIDs...), current.NewUnseenIDs...))
	merged.NewStampIDs = uniqueInt64s(append(append([]int64{}, previous.NewStampIDs...), current.NewStampIDs...))
	merged.SessionInstanceIDs = uniqueInt64s(append(append([]int64{}, previous.SessionInstanceIDs...), current.SessionInstanceIDs...))
	merged.ExerciseInstanceIDs = uniqueInt64s(append(append([]int64{}, previous.ExerciseInstanceIDs...), current.ExerciseInstanceIDs...))
	merged.SessionExerciseInstanceIDs = uniqueInt64s(append(append([]int64{}, previous.SessionExerciseInstanceIDs...), current.SessionExerciseInstanceIDs...))
	merged.CalendarDayIDs = uniqueInt64s(append(append([]int64{}, previous.CalendarDayIDs...), current.CalendarDayIDs...))
	merged.RunnerSessionIDs = uniqueInt64s(append(append([]int64{}, previous.RunnerSessionIDs...), current.RunnerSessionIDs...))
	merged.AttendanceIDs = uniqueInt64s(append(append([]int64{}, previous.AttendanceIDs...), current.AttendanceIDs...))
	merged.WorkoutFeedbackIDs = uniqueInt64s(append(append([]int64{}, previous.WorkoutFeedbackIDs...), current.WorkoutFeedbackIDs...))
	merged.WorkoutFeedbackPointIDs = uniqueInt64s(append(append([]int64{}, previous.WorkoutFeedbackPointIDs...), current.WorkoutFeedbackPointIDs...))
	return &merged
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func uniqueInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func diffInt64s(before, after []int64) []int64 {
	seen := make(map[int64]struct{}, len(before))
	for _, value := range before {
		seen[value] = struct{}{}
	}
	out := make([]int64, 0, len(after))
	for _, value := range after {
		if _, ok := seen[value]; !ok {
			out = append(out, value)
		}
	}
	return uniqueInt64s(out)
}

func diffStrings(before, after []string) []string {
	seen := make(map[string]struct{}, len(before))
	for _, value := range before {
		seen[value] = struct{}{}
	}
	out := make([]string, 0, len(after))
	for _, value := range after {
		if _, ok := seen[value]; !ok {
			out = append(out, value)
		}
	}
	return uniqueStrings(out)
}
