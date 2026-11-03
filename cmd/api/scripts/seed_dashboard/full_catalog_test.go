package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func TestFullSeederSeedCatalogCalendar_RequiresIdentityAndTeamScope(t *testing.T) {
	s := newFullSeeder(nil, options{})
	err := s.seedCatalogCalendar()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requiere seedIdentity")

	s.ids.users["trainer_1"], s.ids.users["trainer_2"] = 1, 2
	err = s.seedCatalogCalendar()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requiere seedTeams: falta team_a")

	s.ids.teams["team_a"], s.ids.teams["team_b"], s.ids.teams["team_f"] = 1001, 1002, 1006
	err = s.seedCatalogCalendar()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requiere seedTeams: falta group_a_main")
}

func TestEnsureFullExercise_RejectsUnmarkedReservedName(t *testing.T) {
	db := testutils.SetupTestDB(t)
	ownerID := createFullGuardTrainer(t, db, "seed-full-catalog-collision@example.test", "990001101")
	exercise := dbs.Exercise{OwnerID: ownerID, Name: "Semilla Full · Activación", Kind: "liviano"}
	require.NoError(t, db.Create(&exercise).Error)

	_, err := ensureFullExercise(db, ownerID, fullExerciseSpecs()[0], time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sin marker")
}

func TestEnsureFullExercise_ResetsFieldsOutsideItsCatalogSpec(t *testing.T) {
	db := testutils.SetupTestDB(t)
	ownerID := createFullGuardTrainer(t, db, "seed-full-catalog-reset@example.test", "990001103")
	description := fullSeedMarker
	exercise := dbs.Exercise{
		OwnerID: ownerID, Name: "Semilla Full · Activación", Description: &description, Kind: "liviano",
		Intensity: strPtr("alta"), SpeedKph: floatPtr(9.5), VideoURL: strPtr("https://example.test/video"),
	}
	require.NoError(t, db.Create(&exercise).Error)

	_, err := ensureFullExercise(db, ownerID, fullExerciseSpecs()[0], time.Now())

	require.NoError(t, err)
	var updated dbs.Exercise
	require.NoError(t, db.First(&updated, exercise.ID).Error)
	assert.Nil(t, updated.Intensity)
	assert.Nil(t, updated.SpeedKph)
	assert.Nil(t, updated.VideoURL)
}

func TestInstantiateFullSession_RejectsUnmarkedSession(t *testing.T) {
	db := testutils.SetupTestDB(t)
	ownerID := createFullGuardTrainer(t, db, "seed-full-session-unmarked@example.test", "990001102")
	session := dbs.Session{OwnerID: ownerID, Name: "ajena"}
	require.NoError(t, db.Create(&session).Error)

	_, err := instantiateFullSession(db, session.ID, ownerID, time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no pertenece al catálogo full")
}

func TestFullSeederStampFullPlan_RejectsCalendarContentOutsideItsPlan(t *testing.T) {
	db := testutils.SetupTestDB(t)
	seedFullCalendarPrerequisites(t)
	s := newFullSeeder(db, options{})
	s.now = gametime.Now().In(gametime.Location).Truncate(time.Second)
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())
	require.NoError(t, s.seedFullCatalog())
	occupied := dbs.GroupCalendarDay{
		GroupID: s.ids.groups["group_a_main"], Date: fullCalendarDate(s.now.AddDate(0, 0, -21)), Kind: "rest",
	}
	require.NoError(t, db.Create(&occupied).Error)

	err := s.stampFullPlan(fullCalendarService(db), "group_a_main", "full_21_days", s.now.AddDate(0, 0, -21))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "contenido fuera del plan full")
}
