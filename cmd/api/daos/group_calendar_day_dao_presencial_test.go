package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// día instanciado: instancia + día de calendario que la referencia (Gap 26).
func presencialDaySeed(t *testing.T, db *gorm.DB, tag string, isPresencial bool) (*dbs.SessionInstance, *dbs.GroupCalendarDay) {
	t.Helper()
	group := setupCalendarGroup(t, db, tag)
	inst := &dbs.SessionInstance{Name: "Fartlek " + tag}
	require.NoError(t, db.Create(inst).Error)
	day := &dbs.GroupCalendarDay{
		GroupID:           group.ID,
		Date:              time.Date(2027, 3, 10, 0, 0, 0, 0, time.UTC),
		Kind:              "training",
		SessionInstanceID: &inst.ID,
		IsPresencial:      isPresencial,
	}
	require.NoError(t, db.Create(day).Error)
	return inst, day
}

func TestGroupCalendarDayDao_FindBySessionInstanceID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	inst, day := presencialDaySeed(t, db, "fbi1", true)

	found, err := dao.FindBySessionInstanceID(nil, inst.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, day.ID, found.ID)
}

func TestGroupCalendarDayDao_FindBySessionInstanceID_Orphan(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	inst := &dbs.SessionInstance{Name: "huérfana sin día"}
	require.NoError(t, db.Create(inst).Error)

	found, err := dao.FindBySessionInstanceID(nil, inst.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGroupCalendarDayDao_SetPresencialOpenedAt(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbo1", true)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)

	mutated, err := dao.SetPresencialOpenedAt(nil, day.ID, at)
	require.NoError(t, err)
	assert.True(t, mutated)

	// Guard: ya abierta → no re-escribe ni marca mutación (idempotente).
	mutated2, err := dao.SetPresencialOpenedAt(nil, day.ID, at.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, mutated2)

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found.PresencialOpenedAt)
	assert.WithinDuration(t, at, *found.PresencialOpenedAt, 0)
	require.Nil(t, found.PresencialClosedAt)
}

func TestGroupCalendarDayDao_SetPresencialClosedAt(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbc1", true)
	opened := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	closed := opened.Add(2 * time.Hour)
	_, err := dao.SetPresencialOpenedAt(nil, day.ID, opened)
	require.NoError(t, err)

	mutated, err := dao.SetPresencialClosedAt(nil, day.ID, closed)
	require.NoError(t, err)
	assert.True(t, mutated)

	mutated2, err := dao.SetPresencialClosedAt(nil, day.ID, closed.Add(time.Hour))
	require.NoError(t, err)
	assert.False(t, mutated2, "cierre final: sin re-escritura")

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found.PresencialClosedAt)
	assert.WithinDuration(t, closed, *found.PresencialClosedAt, 0)
}

func TestGroupCalendarDayDao_SetPresencial_DayInexistente(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)

	mutated, err := dao.SetPresencialOpenedAt(nil, 999999999, at)
	require.NoError(t, err)
	assert.False(t, mutated)

	mutated, err = dao.SetPresencialClosedAt(nil, 999999999, at)
	require.NoError(t, err)
	assert.False(t, mutated)
}

// Gap 26 D5: reasignar el día a otra instancia reinicia el estado presencial;
// editar el mismo día con la MISMA instancia lo conserva.
func TestGroupCalendarDayDao_Upsert_ReasignacionReseteaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbu1", true)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	_, err := dao.SetPresencialOpenedAt(nil, day.ID, at)
	require.NoError(t, err)

	newInst := *day.SessionInstanceID + 500
	require.NoError(t, db.Create(&dbs.SessionInstance{ID: newInst, Name: "otra"}).Error)
	reassigned := &dbs.GroupCalendarDay{
		GroupID:           day.GroupID,
		Date:              day.Date,
		Kind:              day.Kind,
		SessionInstanceID: &newInst,
	}
	require.NoError(t, dao.Upsert(nil, reassigned))

	found, findErr := dao.FindBySessionInstanceID(nil, newInst)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Nil(t, found.PresencialOpenedAt, "día reasignado arranca sin apertura")
}

func TestGroupCalendarDayDao_Upsert_MismaInstanciaConservaEstado(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	_, day := presencialDaySeed(t, db, "fbu2", true)
	at := time.Date(2027, 3, 10, 17, 5, 0, 0, time.UTC)
	_, err := dao.SetPresencialOpenedAt(nil, day.ID, at)
	require.NoError(t, err)

	// Re-upsert con la misma instancia (edición de contenido) conserva la apertura.
	edited := &dbs.GroupCalendarDay{
		GroupID:           day.GroupID,
		Date:              day.Date,
		Kind:              day.Kind,
		SessionInstanceID: day.SessionInstanceID,
		IsPresencial:      false,
	}
	require.NoError(t, dao.Upsert(nil, edited))

	found, findErr := dao.FindBySessionInstanceID(nil, *day.SessionInstanceID)
	require.NoError(t, findErr)
	require.NotNil(t, found.PresencialOpenedAt, "edición con misma instancia no borra la apertura")
	assert.WithinDuration(t, at, *found.PresencialOpenedAt, 0)
}
