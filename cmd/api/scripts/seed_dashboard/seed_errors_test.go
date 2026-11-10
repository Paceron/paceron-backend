package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestCleanupHelpers_ErroresDeDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	selectFails := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	scope := cleanupScope{OwnerID: 1, GroupID: 2, From: time.Now().AddDate(0, 0, -7), To: time.Now()}

	_, err := seedPlanIDs(selectFails, scope)
	require.Error(t, err)

	_, _, err = seedCalendarArtifacts(selectFails, scope, []int64{1})
	require.Error(t, err)

	_, err = seedExerciseInstanceIDs(selectFails, []int64{1})
	require.Error(t, err)

	_, err = seedSessionIDs(selectFails, []int64{1})
	require.Error(t, err)

	_, err = seedExerciseIDs(selectFails, scope, []int64{1})
	require.Error(t, err)

	_, err = exclusiveIDs(selectFails, []int64{1}, "SELECT 1 AS id")
	require.Error(t, err)

	_, err = countCleanupSteps(selectFails, []cleanupStep{{Table: "users", Where: "1 = 1"}})
	require.Error(t, err)

	_, err = buildCleanupSteps(selectFails, scope)
	require.Error(t, err)

	deleteFails := testutils.FailingDB(t, db, func(op string) bool { return op == "delete" })
	_, err = executeCleanup(deleteFails, []cleanupStep{{Table: "users", Where: "1 = 0"}})
	require.Error(t, err)
}

func TestSeeder_ErroresDeDBEnCatalogos(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "seed-errors-owner@test.com", "74001")

	selectFails := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	s := &seeder{db: selectFails, ownerID: owner.ID}
	require.Error(t, s.ensureCatalog())
	_, err := s.ensureSessions()
	require.Error(t, err)
	_, err = s.ensurePlans(map[string]int64{})
	require.Error(t, err)

	insertFails := testutils.FailingDB(t, db, func(op string) bool { return op == "insert" })
	s = &seeder{db: insertFails, ownerID: owner.ID, catalogIDs: map[string]int64{}}
	require.Error(t, s.ensureCatalog())
	_, err = s.ensureSessions()
	require.Error(t, err)
	_, err = s.ensurePlans(map[string]int64{})
	require.Error(t, err)
}

func TestSeeder_ErroresDeDBEnCalendarioYScope(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "seed-calendar-owner@test.com", "74002")
	team := cleanupTestTeam(t, db, "equipo_seed_calendar", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_seed_calendar", team.ID)

	selectFails := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	s := &seeder{db: selectFails, groupID: group.ID, ownerID: owner.ID}
	_, err := s.existingCalendarDays()
	require.Error(t, err)
	_, err = s.loadSeededDays(map[string]int64{"x": 1})
	require.Error(t, err)
	require.Error(t, s.buildRoster())
	require.Error(t, s.resolveScope())
}

func TestSeeder_ResolveScopeRechazaEquipoYGrupoAjenos(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "seed-scope-owner@test.com", "74003")
	team := cleanupTestTeam(t, db, "equipo_seed_scope", owner.ID)
	otherTeam := cleanupTestTeam(t, db, "equipo_seed_scope_otro", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_seed_scope", team.ID)

	s := &seeder{db: db, teamID: 999999, groupID: group.ID, opts: options{}}
	err := s.resolveScope()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no existe")

	s = &seeder{db: db, teamID: otherTeam.ID, groupID: group.ID, opts: options{}}
	err = s.resolveScope()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pertenece")
}

func TestSeeder_EnsureMembershipWindowCreaYDesplaza(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "seed-membership-owner@test.com", "74004")
	team := cleanupTestTeam(t, db, "equipo_seed_membership", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_seed_membership", team.ID)
	athlete := cleanupTestUser(t, db, "seed-membership-athlete@test.com", "74005")

	window := seedWindow{Start: time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC)}
	s := &seeder{
		db: db, groupID: group.ID, window: window, counts: map[string]int64{},
		roster: []seedAthlete{{UserID: athlete.ID}},
	}

	require.NoError(t, s.ensureMembershipWindow())
	var member dbs.GroupUser
	require.NoError(t, db.Where("group_id = ? AND user_id = ?", group.ID, athlete.ID).Take(&member).Error)
	assert.True(t, window.from().Equal(member.DateStart))

	member.DateStart = window.from().AddDate(0, 0, 7)
	require.NoError(t, db.Save(&member).Error)
	require.NoError(t, s.ensureMembershipWindow())
	require.NoError(t, db.First(&member, member.ID).Error)
	assert.True(t, window.from().Equal(member.DateStart))
}

func TestSeeder_ValidateDaysYResolveCatalogKey(t *testing.T) {
	err := validateDays(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quedaron 0")

	days := make([]seededDay, seedSessionsCount)
	err = validateDays(days)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sin bloque principal")

	for i := range days {
		days[i].Blocks = []instanceBlock{{Role: "main"}}
	}
	require.NoError(t, validateDays(days))

	sourceID := int64(10)
	key, ok := resolveCatalogKey(&sourceID, "otro", map[int64]string{10: "velocidad"}, nil)
	assert.True(t, ok)
	assert.Equal(t, "velocidad", key)

	key, ok = resolveCatalogKey(nil, "Distancia 4 km", nil, map[string]string{"distancia 4 km": "volumen"})
	assert.True(t, ok)
	assert.Equal(t, "volumen", key)

	_, ok = resolveCatalogKey(nil, "desconocido", nil, map[string]string{})
	assert.False(t, ok)
}

func TestSeeder_StampRechazaDiaAjenoYModoTestingEstricto(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "seed-stamp-owner@test.com", "74006")
	team := cleanupTestTeam(t, db, "equipo_seed_stamp", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_seed_stamp", team.ID)
	window := seedWindow{Start: time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC)}
	target := window.weekStart(0)

	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: group.ID, Date: target, Kind: "training",
	}).Error)

	s := &seeder{db: db, groupID: group.ID, ownerID: owner.ID, window: window, opts: options{}}
	_, err := s.stamp(map[string]int64{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no salió de este seed")

	planID := int64(1234)
	require.NoError(t, db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ?", group.ID, target).
		Update("source_plan_id", planID).Error)
	s = &seeder{
		db: db, groupID: group.ID, ownerID: owner.ID, window: window,
		opts: options{target: "testing"},
	}
	_, err = s.stamp(map[string]int64{plans[0].Key: planID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "idempotencia estricta")
}
