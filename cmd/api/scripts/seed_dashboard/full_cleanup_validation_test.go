package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestFullCleanup_EarlyReturnsSinEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)

	require.NoError(t, ensureNoUnownedFullReferences(db, &fullCleanupState{}))
	require.NoError(t, validateFullCatalogReferences(db, &fullCleanupState{}))
	require.NoError(t, collectFullCalendarRows(db, &fullCleanupState{}))
	require.NoError(t, collectFullExtraCalendarDays(db, &fullCleanupState{}))
	require.NoError(t, collectFullCompetitionRows(db, &fullCleanupState{}))
	require.NoError(t, collectFullActivityRows(db, &fullCleanupState{}))
	require.NoError(t, collectFullMessages(db, &fullCleanupState{}))
	require.NoError(t, collectFullPayments(db, &fullCleanupState{}))
	require.NoError(t, collectFullSellerConnection(db, &fullCleanupState{}))
}

func TestCollectFullCalendarRows_SinAnclaNoAgregaDias(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "full-calendar-owner@test.com", "73001")
	team := cleanupTestTeam(t, db, "equipo_full_calendar", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_full_calendar", team.ID)

	state := &fullCleanupState{fullGroupIDs: map[string]int64{"group_a_main": group.ID}}
	require.NoError(t, collectFullCalendarRows(db, state))
	assert.Empty(t, state.calendarDays)
}

func TestValidateFullCatalogReferences_DetectaReferenciasAjenas(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "full-catalog-owner@test.com", "73002")
	session := &dbs.Session{OwnerID: owner.ID, Name: "sesion"}
	require.NoError(t, db.Create(session).Error)
	exercise := &dbs.Exercise{OwnerID: owner.ID, Name: "ejercicio", Kind: "liviano"}
	require.NoError(t, db.Create(exercise).Error)

	state := &fullCleanupState{sessions: []int64{session.ID}}
	require.NoError(t, validateFullCatalogReferences(db, state))

	require.NoError(t, db.Create(&dbs.PlanDay{
		PlanID: 999, SequenceNo: 1, Kind: "training", SessionID: &session.ID,
	}).Error)
	err := validateFullCatalogReferences(db, state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan_days ajenos")

	state = &fullCleanupState{exercises: []int64{exercise.ID}}
	require.NoError(t, validateFullCatalogReferences(db, state))
	otherSession := &dbs.Session{OwnerID: owner.ID, Name: "sesion ajena"}
	require.NoError(t, db.Create(otherSession).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{
		SessionID: otherSession.ID, ExerciseID: exercise.ID, Role: "main",
	}).Error)
	err = validateFullCatalogReferences(db, state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sesiones ajenas")
}

func TestFullCleanupHelpers_ErroresDeDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	selectFails := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })

	state := &fullCleanupState{
		fullGroupIDs: map[string]int64{"group_a_main": 1},
	}
	require.Error(t, collectFullCalendarRows(selectFails, state))
	require.Error(t, collectFullMessages(selectFails, &fullCleanupState{calendarInstances: []int64{1}}))
	require.Error(t, collectFullPayments(selectFails, &fullCleanupState{users: []int64{1}}))
	require.Error(t, collectFullSellerConnection(selectFails, &fullCleanupState{
		fullUserIDs: map[string]int64{"trainer_1": 1},
	}))
}
