package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/testutils"
)

func TestResolveCleanupScope_ValidaEquipoGrupoYVentana(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "cleanup-scope-owner@test.com", "76001")
	team := cleanupTestTeam(t, db, "equipo_cleanup_scope", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_cleanup_scope", team.ID)

	scope, err := resolveCleanupScope(db, options{
		teamID: team.ID, groupID: group.ID, start: "2026-09-28",
	})
	require.NoError(t, err)
	assert.Equal(t, team.ID, scope.TeamID)
	assert.Equal(t, group.ID, scope.GroupID)
	assert.Equal(t, owner.ID, scope.OwnerID)
	assert.Equal(t, "2026-09-28", scope.From.Format(dateLayout))
	assert.Equal(t, "2026-10-11", scope.To.Format(dateLayout))

	_, err = resolveCleanupScope(db, options{teamID: 999999, groupID: group.ID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no existe")

	_, err = resolveCleanupScope(db, options{teamID: team.ID, groupID: 999999})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pertenece")

	_, err = resolveCleanupScope(db, options{teamID: team.ID, groupID: group.ID, start: "2026-10-13"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lunes")
}

func TestResolveCleanupScope_ErrorDeDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })

	_, err := resolveCleanupScope(failing, options{teamID: 1, groupID: 2})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al leer el equipo")
}

func TestRunCleanupMode_SinResiduosYConDryRun(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "cleanup-mode-owner@test.com", "76002")
	team := cleanupTestTeam(t, db, "equipo_cleanup_mode", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_cleanup_mode", team.ID)
	opts := options{teamID: team.ID, groupID: group.ID, start: "2026-09-28", dryRun: true}

	require.NotPanics(t, func() { runCleanupMode(db, opts) })
}

func TestRunCleanupMode_ConResiduosDryRunYConfirmado(t *testing.T) {
	fixture := newCleanupFixture(t, "mode")
	opts := options{
		teamID: fixture.scope.TeamID, groupID: fixture.scope.GroupID,
		start: "2026-09-28", dryRun: true,
	}

	require.NotPanics(t, func() { runCleanupMode(fixture.db, opts) })
	assert.Equal(t, int64(1), countRows(t, fixture.db, "group_calendar_days", "id = ?", fixture.seedDay))

	opts.dryRun = false
	opts.yes = true
	require.NotPanics(t, func() { runCleanupMode(fixture.db, opts) })

	assert.Zero(t, countRows(t, fixture.db, "group_calendar_days", "id = ?", fixture.seedDay))
	assert.Equal(t, int64(1), countRows(t, fixture.db, "group_calendar_days", "id = ?", fixture.foreignDay))
}

func TestResolveWindow_UsaRelojParaDefault(t *testing.T) {
	window, err := resolveWindow("")
	require.NoError(t, err)
	assert.True(t, window.Start.Before(time.Now().Add(24*time.Hour)))
}
