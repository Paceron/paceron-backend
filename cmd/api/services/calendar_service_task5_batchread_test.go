package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Los inserts van sobre el handle base para no contar setup como queries.
func setupAdministeredCalendarCounting(t *testing.T) (*dbs.User, *dbs.Group, *dbs.Group, *dbs.Group, *gorm.DB) {
	t.Helper()
	db := testutils.SetupTestDB(t)
	owner := &dbs.User{Name: "Count", Surname: "Owner", Email: "batch-read-owner@test.com", DNI: "55000110", BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(owner).Error)
	teamA := &dbs.Team{Name: "Equipo A batchread", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamA).Error)
	teamB := &dbs.Team{Name: "Equipo B batchread", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(teamB).Error)
	gA := &dbs.Group{Name: "Grupo A batchread", TeamID: teamA.ID, IsMain: true}
	require.NoError(t, db.Create(gA).Error)
	gB := &dbs.Group{Name: "Grupo B batchread", TeamID: teamA.ID, IsMain: false}
	require.NoError(t, db.Create(gB).Error)
	gC := &dbs.Group{Name: "Grupo C batchread", TeamID: teamB.ID, IsMain: true}
	require.NoError(t, db.Create(gC).Error)
	return owner, gA, gB, gC, db
}

// D8 batch: 3 fechas con colisión (cross y same mezcladas) resuelven la
// detección 100% en memoria sobre las filas de FindForGroupsInRange —
// exactamente 3 queries (grupos + equipos + rango), ninguna de colisiones.
// Antes de la migración eran 3 queries adicionales por fecha colisionante (9).
func TestCalendarService_AdministeredCalendar_BatchCollisionQueries(t *testing.T) {
	owner, gA, gB, gC, db := setupAdministeredCalendarCounting(t)
	calDao := daos.NewGroupCalendarDayDao(db)

	dates := [3]time.Time{}
	for i := range dates {
		dates[i] = time.Date(2026, 10, 6+i, 0, 0, 0, 0, time.UTC)
	}
	// Fecha 1: same (gA vs gB) + cross (gC choca con gA) — cross gana en type.
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, dates[0], 9, 0, 10, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gB.ID, dates[0], 9, 30, 11, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gC.ID, dates[0], 9, 15, 10, 30)))
	// Fecha 2: solo same (gA vs gB).
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, dates[1], 8, 0, 9, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gB.ID, dates[1], 8, 15, 9, 30)))
	// Fecha 3: solo cross (gA vs gC).
	require.NoError(t, calDao.Upsert(nil, presencialDay(gA.ID, dates[2], 15, 0, 16, 0)))
	require.NoError(t, calDao.Upsert(nil, presencialDay(gC.ID, dates[2], 15, 30, 17, 0)))

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb), daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb)

	resp, err := svc.AdministeredCalendar(nil, owner.ID, dates[0], dates[2])

	require.NoError(t, err)
	require.Len(t, resp, 7)
	assert.Equal(t, 3, counter, "grupos + equipos + rango; 0 queries de colisiones para las 3 fechas colisionantes")

	collisions := make(map[int64]bool)
	for _, item := range resp {
		switch {
		case item.GroupID == gA.ID && item.Date == dates[0].Format("2006-01-02") ||
			item.GroupID == gB.ID && item.Date == dates[0].Format("2006-01-02") ||
			item.GroupID == gC.ID && item.Date == dates[0].Format("2006-01-02"):
			require.NotNil(t, item.PresencialCollision)
			assert.Equal(t, "cross_team", item.PresencialCollision.Type)
			require.Len(t, item.PresencialCollision.Conflicts, 2)
			collisions[item.GroupID] = true
		case item.Date == dates[1].Format("2006-01-02"):
			require.NotNil(t, item.PresencialCollision)
			assert.Equal(t, "same_team", item.PresencialCollision.Type)
			collisions[item.GroupID] = true
		case item.Date == dates[2].Format("2006-01-02"):
			require.NotNil(t, item.PresencialCollision)
			assert.Equal(t, "cross_team", item.PresencialCollision.Type)
			collisions[item.GroupID] = true
		default:
			assert.Nil(t, item.PresencialCollision)
		}
	}
	require.True(t, collisions[gA.ID] && collisions[gB.ID] && collisions[gC.ID])
}
