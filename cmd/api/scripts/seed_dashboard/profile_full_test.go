package main

import (
	"flag"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestParseOptions_DefaultProfilePreservesDashboardScope(t *testing.T) {
	fs := flag.NewFlagSet("seed_dashboard", flag.ContinueOnError)
	opts, err := parseOptionsArgs(fs, nil)

	require.NoError(t, err)
	assert.Equal(t, dashboardProfile, opts.profile)
	assert.Equal(t, int64(29), opts.teamID)
	assert.Equal(t, int64(39), opts.groupID)
}

func TestParseOptions_RejectsUnknownProfile(t *testing.T) {
	fs := flag.NewFlagSet("seed_dashboard", flag.ContinueOnError)
	_, err := parseOptionsArgs(fs, []string{"--profile", "other"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dashboard|full")
}

func TestParseOptions_AcceptsFullProfile(t *testing.T) {
	fs := flag.NewFlagSet("seed_dashboard", flag.ContinueOnError)
	opts, err := parseOptionsArgs(fs, []string{"--profile=full"})

	require.NoError(t, err)
	assert.Equal(t, fullProfile, opts.profile)
}

func TestDispatchSeedMode_FullRequiresExplicitMode(t *testing.T) {
	err := dispatchSeedMode(nil, options{profile: fullProfile})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "--seed, --verify o --cleanup")
}

func TestFullScopeGuard_RejectsOccupiedIDsWithoutMarker(t *testing.T) {
	t.Run("team", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		trainerID := createFullGuardTrainer(t, db, "seed-full-guard-team@example.test", "990001001")
		require.NoError(t, db.Create(&dbs.Team{
			ID: 1001, Name: "Ocupado", OwnerID: trainerID, MaxMembers: 10,
			Status: "active", Description: "[seed_fullish] dato ajeno",
		}).Error)

		err := newFullSeeder(db, options{}).guardScope()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reservado 1001")
	})

	t.Run("group", func(t *testing.T) {
		db := testutils.SetupTestDB(t)
		trainerID := createFullGuardTrainer(t, db, "seed-full-guard-group@example.test", "990001002")
		require.NoError(t, db.Create(&dbs.Team{
			ID: 1001, Name: "Seed", OwnerID: trainerID, MaxMembers: 10,
			Status: "active", Description: fullSeedMarker,
		}).Error)
		require.NoError(t, db.Create(&dbs.Group{
			ID: 2001, Name: "Ocupado", TeamID: 1001, IsMain: true,
			Description: "dato ajeno",
		}).Error)

		err := newFullSeeder(db, options{}).guardScope()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reservado 2001")
	})
}

func TestFullScopeGuard_AllowsMarkedReservedRows(t *testing.T) {
	db := testutils.SetupTestDB(t)
	trainerID := createFullGuardTrainer(t, db, "seed-full-guard-marked@example.test", "990001003")
	require.NoError(t, db.Create(&dbs.Team{
		ID: 1001, Name: "Seed", OwnerID: trainerID, MaxMembers: 10,
		Status: "active", Description: "Team A " + fullSeedMarker,
	}).Error)
	require.NoError(t, db.Create(&dbs.Group{
		ID: 2001, Name: "Seed", TeamID: 1001, IsMain: true,
		Description: "Grupo principal " + fullSeedMarker,
	}).Error)

	err := newFullSeeder(db, options{}).guardScope()

	assert.NoError(t, err)
}

func TestFullScope_UsesReservedTeamAndMainGroupIDs(t *testing.T) {
	s := newFullSeeder(nil, options{})

	assert.Equal(t, int64(1001), s.scope.teamAID)
	assert.Equal(t, int64(2001), s.scope.groupAMainID)
}

func createFullGuardTrainer(t *testing.T, db *gorm.DB, email, dni string) int64 {
	t.Helper()
	trainer := dbs.User{
		Name: "Seed", Surname: "Guard", Email: email, DNI: dni,
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Password:  "test", Status: "active",
	}
	require.NoError(t, db.Create(&trainer).Error)
	return trainer.ID
}

func TestRunFullSeed_ValidacionesYBaseVacia(t *testing.T) {
	db := testutils.SetupTestDB(t)

	require.Error(t, runFullSeed(db, options{profile: "nope"}))
	require.ErrorContains(t, runFullSeed(db, options{profile: fullProfile, cleanup: true}), "--yes")

	require.NoError(t, runFullSeed(db, options{profile: fullProfile, seed: true, dryRun: true}))
	require.NoError(t, runFullSeed(db, options{profile: fullProfile, cleanup: true, dryRun: true, yes: true}))
	require.NoError(t, runFullSeed(db, options{profile: fullProfile, cleanup: true, yes: true}))
}
