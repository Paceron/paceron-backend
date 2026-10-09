package daos

import (
	"testing"

	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/testutils"
)

// TestDaos_HotTablesindexes verifica que los tags index de las tablas calientes
// de las tasks 2/3 resulten en índices reales en Postgres. La consulta mira
// pg_indexes y no el modelo GORM a propósito: el tag describe lo que GORM
// crearía con AutoMigrate sobre una base nueva, y no es prueba de nada sobre
// la base real.
func TestDaos_HotTablesIndexes(t *testing.T) {
	db := testutils.SetupTestDB(t)

	expected := map[string]string{
		"idx_group_users_group_id":                            "group_users",
		"idx_group_users_user_id":                             "group_users",
		"idx_team_users_team_id":                              "team_users",
		"idx_team_users_user_id":                              "team_users",
		"idx_session_exercise_instances_session_instance_id":  "session_exercise_instances",
		"idx_session_exercise_instances_exercise_instance_id": "session_exercise_instances",
		"idx_workout_feedback_points_feedback_id":             "workout_feedback_points",
	}

	for indexName, table := range expected {
		var n int64
		err := db.Raw(`SELECT count(*) FROM pg_indexes WHERE tablename = ? AND indexname = ?`, table, indexName).Scan(&n).Error
		require.NoError(t, err)
		require.Equal(t, int64(1), n, "falta el índice %s en la tabla %s", indexName, table)
	}
}
