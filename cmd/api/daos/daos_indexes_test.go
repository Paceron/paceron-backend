package daos

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/testutils"
)

type expectedIndex struct {
	name     string
	table    string
	contains []string
	excludes []string
}

// TestDaos_HotTablesIndexes verifica que los tags index de las tablas calientes
// y los índices creados con SQL en postgres.go existan en Postgres, validando
// composición y parcialidad sobre pg_indexes.indexdef (no el modelo GORM: el tag
// describe lo que AutoMigrate crearía en una base nueva, no la base real).
func TestDaos_HotTablesIndexes(t *testing.T) {
	db := testutils.SetupTestDB(t)

	expected := []expectedIndex{
		{name: "idx_group_users_user_id", table: "group_users", contains: []string{"(user_id)"}},
		{name: "idx_group_users_group_id_user_id", table: "group_users", contains: []string{"(group_id, user_id)"}},
		{name: "idx_team_users_user_id", table: "team_users", contains: []string{"(user_id)"}},
		{name: "idx_team_users_team_id_user_id", table: "team_users", contains: []string{"(team_id, user_id)"}},
		{name: "idx_groups_team_id", table: "groups"},
		{name: "idx_exercise_instances_source_exercise_id", table: "exercise_instances"},
		{name: "idx_session_exercise_instances_session_instance_id", table: "session_exercise_instances"},
		{name: "idx_session_exercise_instances_exercise_instance_id", table: "session_exercise_instances"},
		{name: "idx_workout_feedback_feedback_owner_user_id", table: "workout_feedback"},
		{name: "idx_workout_feedback_points_feedback_id", table: "workout_feedback_points"},
		{name: "idx_workout_feedback_points_session_instance_id", table: "workout_feedback_points"},
		{name: "idx_runner_session_athlete", table: "runner_session", contains: []string{"(athlete_user_id)"}, excludes: []string{"where"}},
		{name: "idx_gamification_unseen_user_section", table: "gamification_unseen"},
		{name: "gamification_streak_state_pkey", table: "gamification_streak_state"},
		{name: "idx_user_achievement_progress_user", table: "user_achievement_progress"},
		{name: "user_achievement_progress_pkey", table: "user_achievement_progress"},
		{name: "gamification_stamp_pkey", table: "gamification_stamp"},
		{name: "idx_gamification_stamp_user", table: "gamification_stamp"},
		{name: "idx_competition_team", table: "competition"},
		{name: "idx_competition_participant_user", table: "competition_participant"},
		{name: "idx_competition_winner_user", table: "competition_winner"},
		{name: "idx_user_roles_user_id", table: "user_roles"},
		{name: "idx_installments_user_id", table: "installments"},
		{name: "idx_installments_team_id_user_id", table: "installments"},
		{name: "idx_installments_subscription_id", table: "installments"},
		{name: "idx_payments_installment_id", table: "payments"},
		{name: "idx_session_exercises_session_id", table: "session_exercises"},
		{name: "idx_plan_days_plan_id", table: "plan_days"},
		{name: "idx_join_requests_team_id_status", table: "join_requests"},
		{name: "idx_join_requests_runner_id", table: "join_requests"},
		{name: "idx_user_role_tier_subscriptions_user_id_role_id", table: "user_role_tier_subscriptions"},
		{name: "idx_sessions_owner_id", table: "sessions"},
		{name: "idx_exercises_owner_id", table: "exercises"},
		{name: "idx_training_plans_owner_id", table: "training_plans"},
		{name: "idx_runner_session_competition", table: "runner_session", contains: []string{"(competition_id)", "where (competition_id is not null)"}},
		{name: "uq_competition_route_session_instance", table: "competition", contains: []string{"(route_session_instance_id)", "where (route_session_instance_id is not null)"}},
		{name: "idx_feedback_assigned_exercise", table: "workout_feedback", contains: []string{"(assigned_exercise_id)", "where (deleted_at is null)"}},
		{name: "idx_group_calendar_days_source_plan_id", table: "group_calendar_days", contains: []string{"(source_plan_id)", "where (source_plan_id is not null)"}},
	}

	for _, exp := range expected {
		var defs []string
		err := db.Raw(`SELECT indexdef FROM pg_indexes WHERE tablename = ? AND indexname = ?`, exp.table, exp.name).Scan(&defs).Error
		require.NoError(t, err)
		require.Len(t, defs, 1, "falta el índice %s en la tabla %s", exp.name, exp.table)

		def := strings.ToLower(defs[0])
		for _, want := range exp.contains {
			require.Contains(t, def, strings.ToLower(want), "indexdef inesperado para %s: %s", exp.name, defs[0])
		}
		for _, unwanted := range exp.excludes {
			require.NotContains(t, def, strings.ToLower(unwanted), "indexdef inesperado para %s: %s", exp.name, defs[0])
		}
	}

	// El duplicado y los prefijos redundantes ya no deben existir.
	for _, name := range []string{
		"idx_runner_session_athlete_user_id",
		"idx_group_users_group_id",
		"idx_team_users_team_id",
	} {
		var count int64
		err := db.Raw(`SELECT count(*) FROM pg_indexes WHERE indexname = ?`, name).Scan(&count).Error
		require.NoError(t, err)
		require.Zero(t, count, "el índice redundante %s no debería existir", name)
	}

	// Un solo índice simple sobre runner_session(athlete_user_id): el UNIQUE
	// compuesto (session_instance_id, athlete_user_id) no matchea este patrón.
	var athleteIndexes int64
	err := db.Raw(`SELECT count(*) FROM pg_indexes
		WHERE tablename = 'runner_session' AND indexdef LIKE '%USING btree (athlete_user_id)%'`).Scan(&athleteIndexes).Error
	require.NoError(t, err)
	require.Equal(t, int64(1), athleteIndexes, "debe quedar un solo índice simple sobre runner_session(athlete_user_id)")
}
