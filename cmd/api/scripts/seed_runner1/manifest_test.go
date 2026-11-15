package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "manifest.json")
	prior := &streakStateSnapshot{StrictCurrent: 3, FlexibleCurrent: 2, UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	m := &manifest{
		Email: "runner@example.com", UserID: 42, Anchor: "2026-11-15",
		PriorStreakState:   prior,
		NewAchievementIDs:  []string{"first_session"},
		NewUnseenIDs:       []int64{7},
		SessionInstanceIDs: []int64{10, 11},
	}
	require.NoError(t, writeManifest(path, m))

	loaded, err := loadManifest(path)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, manifestVersion, loaded.Version)
	assert.Equal(t, m.Email, loaded.Email)
	assert.Equal(t, m.UserID, loaded.UserID)
	assert.Equal(t, m.Anchor, loaded.Anchor)
	assert.Equal(t, prior, loaded.PriorStreakState)
	assert.Equal(t, m.NewAchievementIDs, loaded.NewAchievementIDs)
	assert.Equal(t, m.NewUnseenIDs, loaded.NewUnseenIDs)
	assert.Equal(t, m.SessionInstanceIDs, loaded.SessionInstanceIDs)
}

func TestLoadManifestMissingEsNil(t *testing.T) {
	m, err := loadManifest(filepath.Join(t.TempDir(), "missing.json"))
	require.NoError(t, err)
	assert.Nil(t, m)
}

func TestLoadManifestSinModeEsEstrictoYRequiereMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1,"email":"r@example.com"}`), 0o600))
	m, err := loadManifest(path)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.Equal(t, manifestModeStrict, m.Mode)
	assert.NoError(t, requireManifestMode(m, manifestModeStrict))
	assert.Error(t, requireManifestMode(m, manifestModeMerge))
}

func TestMergeManifestPreservaEvidenciaPrevia(t *testing.T) {
	cleaned := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	previous := &manifest{
		Version: manifestVersion, Email: "r@example.com", UserID: 1, Anchor: "2026-11-15",
		CreatedAt:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LastCleanupAt:      &cleaned,
		PriorStreakState:   &streakStateSnapshot{StrictCurrent: 2, FlexibleCurrent: 1},
		NewAchievementIDs:  []string{"first_session"},
		NewUnseenIDs:       []int64{1, 2},
		SessionInstanceIDs: []int64{10},
	}
	current := &manifest{
		Version: manifestVersion, Email: "r@example.com", UserID: 1, Anchor: "2026-11-15",
		NewAchievementIDs:  []string{"streak_3", "first_session"},
		NewUnseenIDs:       []int64{2, 3},
		SessionInstanceIDs: []int64{10, 11},
	}
	merged := mergeManifest(previous, current)
	require.NotNil(t, merged)
	assert.Nil(t, merged.LastCleanupAt)
	assert.Equal(t, previous.PriorStreakState, merged.PriorStreakState)
	assert.Equal(t, []string{"first_session", "streak_3"}, merged.NewAchievementIDs)
	assert.Equal(t, []int64{1, 2, 3}, merged.NewUnseenIDs)
	assert.Equal(t, []int64{10, 11}, merged.SessionInstanceIDs)
}

func TestMergeManifestConNil(t *testing.T) {
	m := &manifest{Email: "r@example.com"}
	assert.Equal(t, m, mergeManifest(nil, m))
	assert.Equal(t, m, mergeManifest(m, nil))
}

func TestDiffHelpers(t *testing.T) {
	assert.Equal(t, []string{"b"}, diffStrings([]string{"a", "c"}, []string{"a", "b", "c"}))
	assert.Equal(t, []int64{2}, diffInt64s([]int64{1, 3}, []int64{1, 2, 3}))
	assert.Empty(t, diffInt64s([]int64{1, 2}, []int64{1, 2}))
}

func TestUniqueHelpers(t *testing.T) {
	assert.Equal(t, []int64{1, 2, 3}, uniqueInt64s([]int64{3, 1, 2, 1, 0, -1}))
	assert.Equal(t, []string{"a", "b"}, uniqueStrings([]string{"b", "a", "b"}))
}
