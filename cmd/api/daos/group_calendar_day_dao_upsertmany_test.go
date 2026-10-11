package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestGroupCalendarDayDao_UpsertMany_InsertMapeaOrdenAIds(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "umn1")
	days := []*dbs.GroupCalendarDay{
		{GroupID: group.ID, Date: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Kind: "rest"},
		{GroupID: group.ID, Date: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), Kind: "other", OtherName: strPtrCal("Elongación")},
		{GroupID: group.ID, Date: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), Kind: "rest"},
	}

	results, err := dao.UpsertMany(nil, days)

	require.NoError(t, err)
	require.Len(t, results, 3)
	for i := range results {
		require.NotZero(t, results[i].ID)
		if i > 0 {
			assert.NotEqual(t, results[i-1].ID, results[i].ID)
		}
	}
	for i, day := range days {
		days[i].ID = results[i].ID
		found, findErr := dao.FindByGroupAndDate(nil, group.ID, day.Date)
		require.NoError(t, findErr)
		require.NotNil(t, found)
		assert.Equal(t, results[i].ID, found.ID, "result[%d] es la fila de %s", i, day.Date.Format("2006-01-02"))
		assert.Equal(t, day.Kind, found.Kind)
	}
	require.NotNil(t, days[1].OtherName)
	assert.False(t, results[0].CreatedAt.IsZero())
	assert.False(t, results[0].UpdatedAt.IsZero())
}

func TestGroupCalendarDayDao_UpsertMany_ConflictActualizaYConservaId(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "umn2")
	date := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	existingID := int64(0)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest"}))
	found, _ := dao.FindByGroupAndDate(nil, group.ID, date)
	existingID = found.ID

	results, err := dao.UpsertMany(nil, []*dbs.GroupCalendarDay{
		{GroupID: group.ID, Date: date, Kind: "other", OtherName: strPtrCal("Cambió")},
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, existingID, results[0].ID, "el conflicto actualiza la misma fila")
	updated, _ := dao.FindByGroupAndDate(nil, group.ID, date)
	assert.Equal(t, "other", updated.Kind)
	require.NotNil(t, updated.OtherName)
	assert.Equal(t, "Cambió", *updated.OtherName)
}

func TestGroupCalendarDayDao_UpsertMany_MismaInstanciaConservaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	inst, day := presencialDaySeed(t, db, "umn3", true)
	opened := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	_, err := dao.SetPresencialOpenedAt(nil, day.ID, opened)
	require.NoError(t, err)

	sameInst := &dbs.GroupCalendarDay{
		GroupID: day.GroupID, Date: day.Date, Kind: day.Kind,
		SessionInstanceID: day.SessionInstanceID, IsPresencial: true,
	}
	_, err = dao.UpsertMany(nil, []*dbs.GroupCalendarDay{sameInst})
	require.NoError(t, err)

	found, findErr := dao.FindBySessionInstanceID(nil, inst.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found.PresencialOpenedAt, "misma instancia presencial conserva la apertura")
	assert.Equal(t, opened.UTC(), (*found.PresencialOpenedAt).UTC())
}

func TestGroupCalendarDayDao_UpsertMany_CambioInstanciaLimpiaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "umn4", true)
	opened := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	_, err := dao.SetPresencialOpenedAt(nil, day.ID, opened)
	require.NoError(t, err)

	newInst := &dbs.SessionInstance{Name: "otra instancia"}
	require.NoError(t, db.Create(newInst).Error)
	reassigned := &dbs.GroupCalendarDay{
		GroupID: day.GroupID, Date: day.Date, Kind: day.Kind,
		SessionInstanceID: &newInst.ID, IsPresencial: true,
	}
	_, err = dao.UpsertMany(nil, []*dbs.GroupCalendarDay{reassigned})
	require.NoError(t, err)

	found, findErr := dao.FindBySessionInstanceID(nil, newInst.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Nil(t, found.PresencialOpenedAt, "cambio de instancia limpia la apertura")
}

func TestGroupCalendarDayDao_UpsertMany_ApagaPresencialLimpiaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	inst, day := presencialDaySeed(t, db, "umn5", true)
	opened := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	closed := opened.Add(2 * time.Hour)
	_, err := dao.SetPresencialOpenedAt(nil, day.ID, opened)
	require.NoError(t, err)
	_, err = dao.SetPresencialClosedAt(nil, day.ID, closed)
	require.NoError(t, err)

	off := &dbs.GroupCalendarDay{
		GroupID: day.GroupID, Date: day.Date, Kind: day.Kind,
		SessionInstanceID: day.SessionInstanceID, IsPresencial: false,
	}
	_, err = dao.UpsertMany(nil, []*dbs.GroupCalendarDay{off})
	require.NoError(t, err)

	found, findErr := dao.FindBySessionInstanceID(nil, inst.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.False(t, found.IsPresencial)
	assert.Nil(t, found.PresencialOpenedAt)
	assert.Nil(t, found.PresencialClosedAt)
}
