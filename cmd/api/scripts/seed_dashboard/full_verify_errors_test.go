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

func TestFullOwnedUserIDs_VacioIncompletoYFueraDeScope(t *testing.T) {
	db := testutils.SetupTestDB(t)

	ids, err := fullOwnedUserIDs(db)
	require.NoError(t, err)
	assert.Empty(t, ids)

	users := fullIdentityUsers()
	partial := &dbs.User{
		Name: "Parcial", Surname: "Full", Email: users[0].email, DNI: users[0].dni,
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "x",
	}
	require.NoError(t, db.Create(partial).Error)

	_, err = fullOwnedUserIDs(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "incompleta")

	require.NoError(t, db.Delete(&dbs.User{}, partial.ID).Error)
	require.NoError(t, db.Create(&dbs.User{
		Name: "Ajeno", Surname: "Full", Email: "ajeno@seed-full.local", DNI: "990299999",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "x",
	}).Error)

	_, err = fullOwnedUserIDs(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fuera del conjunto conocido")
}

func TestFullOwnedUserIDs_DNIMismatch(t *testing.T) {
	db := testutils.SetupTestDB(t)
	spec := fullIdentityUsers()[0]
	require.NoError(t, db.Create(&dbs.User{
		Name: "Trainer", Surname: "Uno", Email: spec.email, DNI: "000000000",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "x",
	}).Error)

	_, err := fullOwnedUserIDs(db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no coincide con la identidad full")
}

func TestEnsureFullScopeEmpty_DetectaResiduos(t *testing.T) {
	db := testutils.SetupTestDB(t)
	require.NoError(t, ensureFullScopeEmpty(db))

	require.NoError(t, db.Create(&dbs.Team{
		ID: fullTeamAID, Name: "reservado", OwnerID: 1, MaxMembers: 1,
		Description: fullSeedMarker + " equipo",
	}).Error)

	err := ensureFullScopeEmpty(db)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "teams")
}

func TestMarkedCatalogID_UnaNingunaYAmbigua(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "marked-catalog-owner@test.com", "75001")
	description := fullSeedMarker + " catalogo"

	id, err := markedCatalogID(db, "exercises", owner.ID, "Ejercicio")
	require.Error(t, err)
	assert.Zero(t, id)

	require.NoError(t, db.Create(&dbs.Exercise{
		OwnerID: owner.ID, Name: "Ejercicio", Kind: "liviano", Description: &description,
	}).Error)
	id, err = markedCatalogID(db, "exercises", owner.ID, "Ejercicio")
	require.NoError(t, err)
	assert.Positive(t, id)

	require.NoError(t, db.Create(&dbs.Exercise{
		OwnerID: owner.ID, Name: "Ejercicio", Kind: "liviano", Description: &description,
	}).Error)
	_, err = markedCatalogID(db, "exercises", owner.ID, "Ejercicio")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "se esperaba una fila marcada")
}

func TestResolveExistingFullClock_SinEntrenadorSinPlanYAmbigua(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})

	require.NoError(t, s.resolveExistingFullClock())

	spec := fullIdentityUsers()[0]
	trainer := &dbs.User{
		Name: "Trainer", Surname: "Uno", Email: spec.email, DNI: spec.dni,
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "x",
	}
	require.NoError(t, db.Create(trainer).Error)
	require.NoError(t, s.resolveExistingFullClock(), "sin plan ancla no hay nada que resolver")

	description := fullSeedMarker + " plan"
	plan := &dbs.TrainingPlan{OwnerID: trainer.ID, Name: "Semilla Full · Plan de 21 días", Description: &description}
	require.NoError(t, db.Create(plan).Error)
	require.NoError(t, s.resolveExistingFullClock())

	require.NoError(t, db.Create(&dbs.TrainingPlan{
		OwnerID: trainer.ID, Name: "Semilla Full · Plan de 21 días", Description: &description,
	}).Error)
	err := s.resolveExistingFullClock()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identidad única y marcada")
}

func TestResolveFullClockForPlan_AnclaYPresencial(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := cleanupTestUser(t, db, "full-clock-owner@test.com", "75002")
	team := cleanupTestTeam(t, db, "equipo_full_clock", owner.ID)
	group := cleanupTestGroup(t, db, "grupo_full_clock", team.ID)

	s := newFullSeeder(db, options{})
	s.scope.groupAMainID = group.ID
	s.now = time.Date(2026, time.October, 13, 15, 0, 0, 0, gametime.Location)
	description := fullSeedMarker + " plan"
	plan := &dbs.TrainingPlan{OwnerID: owner.ID, Name: "Plan", Description: &description}
	require.NoError(t, db.Create(plan).Error)

	anchor := time.Date(2026, time.September, 28, 0, 0, 0, 0, gametime.Location)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: group.ID, Date: anchor, Kind: "training", SourcePlanID: &plan.ID,
	}).Error)

	require.NoError(t, s.resolveFullClockForPlan(plan.ID))
	assert.Equal(t, "2026-10-19", s.now.Format(dateLayout))
	assert.Equal(t, 15, s.now.Hour())

	opened := time.Date(2026, time.October, 19, 6, 15, 0, 0, gametime.Location)
	require.NoError(t, db.Create(&dbs.GroupCalendarDay{
		GroupID: group.ID, Date: anchor.AddDate(0, 0, 21), Kind: "training",
		SourcePlanID: &plan.ID, PresencialOpenedAt: &opened,
	}).Error)

	require.NoError(t, s.resolveFullClockForPlan(plan.ID))
	assert.Equal(t, 6, s.now.Hour())
	assert.Equal(t, 35, s.now.Minute())
}

func TestResolveFullClockForPlan_ErrorDeDB(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})
	failing := testutils.FailingDB(t, db, func(op string) bool { return op == "select" })
	s.db = failing

	err := s.resolveFullClockForPlan(1)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolver ancla del calendario full")
}

func TestVerifyFull_BaseVaciaEsCoherente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})

	require.NoError(t, s.verifyFull())
}

func TestVerifyFullUsers_UsuarioAusenteFalla(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})
	s.ids.users = map[string]int64{"trainer_1": 999999}

	err := s.verifyFullUsers()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "usuario ausente")
}

func TestVerifyFullCalendar_SinDiaDeHoyFalla(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.October, 13, 15, 0, 0, 0, gametime.Location)
	s.scope.groupAMainID = 999999

	err := s.verifyFullCalendar()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "calendario de hoy")
}

func TestVerifyFullCompetitions_SinCompetenciasFalla(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})
	s.scope.teamAID = 999999

	err := s.verifyFullCompetitions()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "competencias")
}

func TestHydrateFullIDs_IdentidadesCompletasSinEquiposFalla(t *testing.T) {
	db := testutils.SetupTestDB(t)
	for _, spec := range fullIdentityUsers() {
		require.NoError(t, db.Create(&dbs.User{
			Name: spec.name, Surname: spec.surname, Email: spec.email, DNI: spec.dni,
			BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "x", Status: spec.status,
		}).Error)
	}
	s := newFullSeeder(db, options{})

	err := s.hydrateFullIDs()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "verificar equipo reservado")
}
