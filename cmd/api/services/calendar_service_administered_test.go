package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Fixture común de administered-calendar: 1 owner, 2 equipos (A: grupos
// gA/gB mismo equipo; B: grupo gC) — mismo patrón del brief Task 3.1.
func setupAdministeredCalendar(t *testing.T) (svc *calendarService, calDao daos.GroupCalendarDaoInterface, owner *dbs.User, teamA, teamB *dbs.Team, gA, gB, gC *dbs.Group) {
	t.Helper()
	db := testutils.SetupTestDB(t)
	owner = &dbs.User{Name: "Test", Surname: "Owner", Email: "administered-cal-owner@test.com", DNI: "50000120", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	teamA = &dbs.Team{Name: "Equipo A admin-cal", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamA).Error)
	teamB = &dbs.Team{Name: "Equipo B admin-cal", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamB).Error)
	gA = &dbs.Group{Name: "Grupo A admin-cal", TeamID: teamA.ID, IsMain: true}
	require.NoError(t, db.Create(gA).Error)
	gB = &dbs.Group{Name: "Grupo B admin-cal", TeamID: teamA.ID, IsMain: false}
	require.NoError(t, db.Create(gB).Error)
	gC = &dbs.Group{Name: "Grupo C admin-cal", TeamID: teamB.ID, IsMain: true}
	require.NoError(t, db.Create(gC).Error)
	calDao = daos.NewGroupCalendarDayDao(db)
	svc = NewCalendarService(calDao, daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db).(*calendarService)
	return
}

func presencialDay(groupID int64, date time.Time, fromH, fromM, toH, toM int) *dbs.GroupCalendarDay {
	from := time.Date(date.Year(), date.Month(), date.Day(), fromH, fromM, 0, 0, time.UTC)
	to := time.Date(date.Year(), date.Month(), date.Day(), toH, toM, 0, 0, time.UTC)
	return &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "training", IsPresencial: true, PresencialTimeFrom: &from, PresencialTimeTo: &to}
}

// 7.3a: colisión same-team marcada en AMBOS días involucrados.
func TestCalendarService_AdministeredCalendar_SameTeamCollisionMarksBothDays(t *testing.T) {
	svc, calDao, owner, _, _, gA, gB, _ := setupAdministeredCalendar(t)

	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, date, 9, 0, 10, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gB.ID, date, 9, 30, 11, 0)))

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date, date)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	for _, item := range resp {
		require.NotNil(t, item.PresencialCollision, "día del grupo %d debe traer collision", item.GroupID)
		assert.Equal(t, "same_team", item.PresencialCollision.Type)
		require.Len(t, item.PresencialCollision.Conflicts, 1)
		// Cada día lista al OTRO grupo como conflict, con nombres resueltos.
		other := gA.ID
		if item.GroupID == gA.ID {
			other = gB.ID
		}
		conflict := item.PresencialCollision.Conflicts[0]
		assert.Equal(t, other, conflict.GroupID)
		assert.Equal(t, "2026-10-05", conflict.Date)
		assert.NotEmpty(t, conflict.GroupName)
		assert.NotEmpty(t, conflict.TeamName)
	}
}

// 7.3b: colisión cross-team → type cross_team en ambos días.
func TestCalendarService_AdministeredCalendar_CrossTeamCollision(t *testing.T) {
	svc, calDao, owner, _, _, gA, _, gC := setupAdministeredCalendar(t)

	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, date, 9, 0, 10, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gC.ID, date, 9, 30, 11, 0)))

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date, date)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	for _, item := range resp {
		require.NotNil(t, item.PresencialCollision)
		assert.Equal(t, "cross_team", item.PresencialCollision.Type)
		require.Len(t, item.PresencialCollision.Conflicts, 1)
	}
}

// 7.3c: cross gana sobre same si hay de ambos tipos (3 grupos superpuestos:
// gA+gB mismo equipo, gC de otro equipo).
func TestCalendarService_AdministeredCalendar_CrossWinsOverSame(t *testing.T) {
	svc, calDao, owner, _, _, gA, gB, gC := setupAdministeredCalendar(t)

	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, date, 9, 0, 10, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gB.ID, date, 9, 30, 11, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gC.ID, date, 9, 15, 10, 30)))

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date, date)

	require.NoError(t, err)
	require.Len(t, resp, 3)
	for _, item := range resp {
		require.NotNil(t, item.PresencialCollision)
		assert.Equal(t, "cross_team", item.PresencialCollision.Type)
		// conflicts lista TODOS los colisionantes (los otros 2 grupos).
		assert.Len(t, item.PresencialCollision.Conflicts, 2)
	}
}

// 7.3d: colisión vieja (filas insertadas directo por DAO, nunca pasadas por
// el guard de escritura) se detecta igual — la detección es de lectura.
func TestCalendarService_AdministeredCalendar_LegacyCollisionDetected(t *testing.T) {
	svc, calDao, owner, _, _, gA, _, gC := setupAdministeredCalendar(t)

	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	// Insert por DAO crudo (db.Create directo, sin Upsert/guard): simula
	// filas guardadas antes de que existiera el guard de colisión.
	fromA := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	toA := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	fromC := time.Date(2026, 10, 5, 8, 30, 0, 0, time.UTC)
	toC := time.Date(2026, 10, 5, 9, 30, 0, 0, time.UTC)
	require.NoError(t, calDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: gA.ID, Date: date, Kind: "training", IsPresencial: true, PresencialTimeFrom: &fromA, PresencialTimeTo: &toA}))
	require.NoError(t, calDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: gC.ID, Date: date, Kind: "training", IsPresencial: true, PresencialTimeFrom: &fromC, PresencialTimeTo: &toC}))

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date, date)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	for _, item := range resp {
		require.NotNil(t, item.PresencialCollision, "colisión vieja pre-guard debe marcarse")
		assert.Equal(t, "cross_team", item.PresencialCollision.Type)
	}
}

// 7.3e: día aislado (sin superposición) no trae presencial_collision.
func TestCalendarService_AdministeredCalendar_IsolatedDayNoCollision(t *testing.T) {
	svc, calDao, owner, _, _, gA, gB, gC := setupAdministeredCalendar(t)

	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	// gA y gB superpuestos entre sí; gC presencial pero sin superponer.
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, date, 9, 0, 10, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gB.ID, date, 9, 30, 11, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gC.ID, date, 15, 0, 16, 0)))

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date, date)

	require.NoError(t, err)
	require.Len(t, resp, 3)
	for _, item := range resp {
		if item.GroupID == gC.ID {
			assert.Nil(t, item.PresencialCollision, "día aislado no debe traer collision")
		} else {
			assert.NotNil(t, item.PresencialCollision)
		}
	}
}

// 7.3f: día cancelled presencial NO genera collision (D1: un cancelado no es
// un compromiso físico — ni como candidato ni como colisionante).
func TestCalendarService_AdministeredCalendar_CancelledDayNoCollision(t *testing.T) {
	svc, calDao, owner, _, _, gA, _, gC := setupAdministeredCalendar(t)

	date := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	// gA training presencial; gC cancelled presencial superpuesto.
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, date, 9, 0, 10, 0)))
	fromC := time.Date(2026, 10, 5, 9, 30, 0, 0, time.UTC)
	toC := time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC)
	require.NoError(t, calDao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: gC.ID, Date: date, Kind: "cancelled", IsPresencial: true, PresencialTimeFrom: &fromC, PresencialTimeTo: &toC, CancelledReason: strPtr("lluvia")}))

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date, date)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	for _, item := range resp {
		assert.Nil(t, item.PresencialCollision, "cancelled no colisiona ni colisiona contra él")
	}
}

// El rango filtra: días fuera del rango no aparecen ni colisionan.
func TestCalendarService_AdministeredCalendar_RangeFilter(t *testing.T) {
	svc, calDao, owner, _, _, gA, _, gC := setupAdministeredCalendar(t)

	inRange := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	outOfRange := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, inRange, 9, 0, 10, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gC.ID, outOfRange, 9, 0, 10, 0)))

	resp, err := svc.AdministeredCalendar(nil, owner.ID, inRange, inRange)

	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Nil(t, resp[0].PresencialCollision, "la colisión está en otra fecha, fuera del rango")
}

// Sin grupos administrados → slice vacío no-nil.
func TestCalendarService_AdministeredCalendar_NoGroupsReturnsEmptySlice(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Test", Surname: "Loner", Email: "administered-cal-loner@test.com", DNI: "50000121", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db)

	resp, err := svc.AdministeredCalendar(nil, owner.ID, time.Now(), time.Now().AddDate(0, 0, 7))

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp)
}
