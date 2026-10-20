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
		"idx_workout_feedback_points_session_instance_id":     "workout_feedback_points",
		"idx_runner_session_athlete_user_id":                  "runner_session",
		"idx_gamification_unseen_user_section":                "gamification_unseen",
		"gamification_streak_state_pkey":                      "gamification_streak_state",
		"idx_user_achievement_progress_user":                  "user_achievement_progress",
		"user_achievement_progress_pkey":                      "user_achievement_progress",
		"gamification_stamp_pkey":                             "gamification_stamp",
		"idx_gamification_stamp_user":                         "gamification_stamp",
		"idx_competition_team":                                "competition",
		"idx_competition_participant_user":                    "competition_participant",
		"idx_competition_winner_user":                         "competition_winner",
		"idx_user_roles_user_id":                              "user_roles",
		"idx_installments_user_id":                            "installments",
		"idx_installments_team_id_user_id":                    "installments",
		"idx_installments_subscription_id":                    "installments",
		"idx_payments_installment_id":                         "payments",
		"idx_session_exercises_session_id":                    "session_exercises",
		"idx_plan_days_plan_id":                               "plan_days",
		"idx_join_requests_team_id_status":                    "join_requests",
		"idx_join_requests_runner_id":                         "join_requests",
		"idx_user_role_tier_subscriptions_user_id_role_id":    "user_role_tier_subscriptions",
		"idx_sessions_owner_id":                               "sessions",
		"idx_exercises_owner_id":                              "exercises",
		"idx_training_plans_owner_id":                         "training_plans",
		"idx_runner_session_competition":                      "runner_session",
		"uq_competition_route_session_instance":               "competition",
	}

	for indexName, table := range expected {
		var n int64
		err := db.Raw(`SELECT count(*) FROM pg_indexes WHERE tablename = ? AND indexname = ?`, table, indexName).Scan(&n).Error
		require.NoError(t, err)
		require.Equal(t, int64(1), n, "falta el índice %s en la tabla %s", indexName, table)
	}
}
