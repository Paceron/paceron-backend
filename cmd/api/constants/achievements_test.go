package constants

// --- Catálogo de logros (gamificacion-core D5, task 3.1): pin del shape ---

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func familyCount(family string) int {
	n := 0
	for _, row := range AchievementCatalog {
		if row.Family == family {
			n++
		}
	}
	return n
}

func TestAchievementsCatalog_Pin(t *testing.T) {
	assert.Equal(t, 21, len(AchievementCatalog), "6 consistency + 4 volume + 5 sessions + 3 gps + 1 early_bird + 2 team")
	assert.Equal(t, 6, familyCount("consistency"))
	assert.Equal(t, 4, familyCount("volume"))
	assert.Equal(t, 5, familyCount("sessions"))
	assert.Equal(t, 3, familyCount("gps"))
	assert.Equal(t, 1, familyCount("early_bird"))
	assert.Equal(t, 2, familyCount("team"))
}

func TestAchievementsCatalog_Shape(t *testing.T) {
	ids := map[string]int{}
	families := map[string]bool{
		"consistency": true, "volume": true, "sessions": true,
		"gps": true, "early_bird": true, "team": true,
	}
	units := map[string]bool{}
	for _, unit := range AchievementUnits() {
		units[unit] = true
	}

	for _, row := range AchievementCatalog {
		assert.NotEmpty(t, row.ID)
		_, dup := ids[row.ID]
		assert.False(t, dup, "ids únicos: %s repetida", row.ID)
		ids[row.ID]++
		assert.True(t, families[row.Family], "familia válida en %s", row.ID)
		assert.NotEmpty(t, row.Name)
		assert.NotEmpty(t, row.Description)
		assert.NotEmpty(t, row.IconKey)
		assert.True(t, row.Threshold > 0, "threshold > 0 en %s", row.ID)
		assert.True(t, units[row.Unit], "unidad válida en %s", row.ID)
	}

	// Pin de los thresholds del spec D5.
	expect := map[string]float64{
		"first_session": 1, "streak_3": 3, "streak_7": 7, "streak_14": 14,
		"streak_30": 30, "streak_60": 60,
		"distance_10k": 10, "distance_50k": 50, "distance_100k": 100,
		"distance_42k_single": 42.2,
		"sessions_5":          5, "sessions_25": 25, "sessions_50": 50,
		"sessions_100": 100, "sessions_200": 200,
		"gps_first": 1, "gps_10": 10, "gps_50": 50,
		"early_5": 5, "team_first": 1, "team_10": 10,
	}
	for _, row := range AchievementCatalog {
		assert.Equal(t, expect[row.ID], row.Threshold)
	}

	// Filas excluídas de v1 (D5/D13): jamás deben colarse por descuido.
	for _, excluded := range []string{"early_10", "distance_250k", "distance_500k", "attendance_20"} {
		assert.NotContains(t, ids, excluded, "%s queda fuera de v1", excluded)
	}
}
