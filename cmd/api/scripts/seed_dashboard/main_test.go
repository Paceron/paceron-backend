package main

import (
	"flag"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/testutils"
)

func TestParseOptionsArgs_FlagsYPerfil(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts, err := parseOptionsArgs(fs, []string{
		"--seed", "--dry-run", "--profile=full", "--team-id=7", "--group-id=8",
		"--start=2026-10-12", "--target=testing",
	})

	require.NoError(t, err)
	assert.True(t, opts.seed)
	assert.True(t, opts.dryRun)
	assert.Equal(t, fullProfile, opts.profile)
	assert.Equal(t, int64(7), opts.teamID)
	assert.Equal(t, int64(8), opts.groupID)
	assert.Equal(t, "2026-10-12", opts.start)
	assert.Equal(t, "testing", opts.target)
}

func TestParseOptionsArgs_RechazaPerfilYFlagInvalidos(t *testing.T) {
	t.Run("perfil", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		_, err := parseOptionsArgs(fs, []string{"--profile=nope"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--profile inválido")
	})

	t.Run("flag", func(t *testing.T) {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		_, err := parseOptionsArgs(fs, []string{"--no-existe"})
		require.Error(t, err)
	})
}

func TestResolveWindow_DefaultYValidaciones(t *testing.T) {
	window, err := resolveWindow("")
	require.NoError(t, err)
	assert.Equal(t, time.Monday, window.Start.Weekday())

	_, err = resolveWindow("2026/10/12")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YYYY-MM-DD")

	_, err = resolveWindow("2026-10-13")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lunes")
}

func TestSortedKeysYReport(t *testing.T) {
	keys := sortedKeys(map[string]int64{"z": 1, "a": 2, "m": 3})
	assert.Equal(t, []string{"a", "m", "z"}, keys)
	assert.Empty(t, sortedKeys(nil))

	require.NotPanics(t, func() {
		(&seeder{
			counts:   map[string]int64{"exercises_creados": 2},
			warnings: []string{"aviso"},
		}).report()
	})
}

func TestDispatchSeedMode_ValidacionesSinEscribir(t *testing.T) {
	db := testutils.SetupTestDB(t)

	require.Error(t, dispatchSeedMode(db, options{profile: "nope"}))

	err := dispatchSeedMode(db, options{profile: dashboardProfile, verify: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--verify requiere --profile=full")

	err = dispatchSeedMode(db, options{profile: fullProfile})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requiere --seed, --verify o --cleanup")
}

func TestRunSeedMode_DryRunSoloInforma(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "run-seed-owner@test.com", "77001")
	team := cleanupTestTeam(t, db, "equipo_run_seed", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_run_seed", team.ID)

	require.NotPanics(t, func() {
		runSeedMode(db, options{
			profile: dashboardProfile, dryRun: true, teamID: team.ID, groupID: group.ID,
		})
	})
}

func TestRunDashboardSeedMode_SinSeedNoEscribe(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "dashboard-mode-owner@test.com", "77002")
	team := cleanupTestTeam(t, db, "equipo_dashboard_mode", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_dashboard_mode", team.ID)

	require.NotPanics(t, func() {
		runDashboardSeedMode(db, options{teamID: team.ID, groupID: group.ID})
	})
}

func TestParseOptions_UsaFlagCommandLine(t *testing.T) {
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("seed-dashboard-test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"seed_dashboard", "--profile=full", "--team-id=3"}
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})

	opts := parseOptions()

	assert.Equal(t, fullProfile, opts.profile)
	assert.Equal(t, int64(3), opts.teamID)
}

func TestPrintPlan_MarcaLegacyYEnProgreso(t *testing.T) {
	s := &seeder{
		teamID: 1, groupID: 2, ownerID: 3,
		window: seedWindow{Start: time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC)},
		roster: []seedAthlete{
			{Index: 4, UserID: 10, Name: "legacy", Archetype: archetype{Key: archetypeVelocista, Label: "velocista"}},
			{Index: 0, UserID: 11, Name: "progreso", Archetype: archetype{Key: archetypeEnProgreso, Label: "en progreso"}},
		},
		warnings: []string{"aviso"},
	}

	require.NotPanics(t, func() { s.printPlan() })
}
