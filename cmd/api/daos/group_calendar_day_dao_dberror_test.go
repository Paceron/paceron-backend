package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Ramas de error de DB de group_calendar_day_dao inyectadas con FailingDB por
// tipo de operación ("select"/"insert"/"delete"). Los errores se envuelven con
// %w en estos DAOs: ErrorIs con el sentinel/причина inyectado.
func nthFail(op string, n int) func(string) bool {
	calls := map[string]int{}
	return func(o string) bool {
		calls[o]++
		return o == op && calls[o] == n
	}
}

func TestGroupCalendarDayDao_DBFail_UpsertSelectError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewGroupCalendarDayDao(failing)
	d1 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	err := dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: 1, Date: d1, Kind: "rest"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding calendar day")
}

func TestGroupCalendarDayDao_DBFail_FindByGroupAndDateError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewGroupCalendarDayDao(failing)

	_, err := dao.FindByGroupAndDate(nil, 1, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding calendar day")
}

func TestGroupCalendarDayDao_DBFail_FindByGroupAndRangeError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao := NewGroupCalendarDayDao(failing)

	_, err := dao.FindByGroupAndRange(nil, 1, time.Now(), time.Now())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing calendar days")
}

// FindForGroupsInRange su two branches vacía y con error de select.
func TestGroupCalendarDayDao_DBFail_FindForGroupsInRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	from, to := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)

	// Sin grupos devuelve vacío sin tocar la DB.
	days, err := dao.FindForGroupsInRange(nil, nil, from, to)
	require.NoError(t, err)
	assert.Nil(t, days)

	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupCalendarDayDao(failing)
	_, err = dao.FindForGroupsInRange(nil, []int64{1}, from, to)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error listing calendar days")
}

// FindPresencialForGroupsInRange: error de select y set vacío.
func TestGroupCalendarDayDao_DBFail_FindPresencialForGroupsInRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	d1 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	// Sin groupIDs o sin dates devuelve vacío sin query.
	days, err := dao.FindPresencialForGroupsInRange(nil, nil, []time.Time{d1})
	require.NoError(t, err)
	assert.Nil(t, days)
	days, err = dao.FindPresencialForGroupsInRange(nil, []int64{1}, nil)
	require.NoError(t, err)
	assert.Nil(t, days)

	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupCalendarDayDao(failing)
	_, err = dao.FindPresencialForGroupsInRange(nil, []int64{1}, []time.Time{d1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding presencial days")
}

// DeleteByDates: branch vacío + error de delete con slate de fechas.
func TestGroupCalendarDayDao_DBFail_DeleteByDates(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	require.NoError(t, dao.DeleteByDates(nil, 1, nil), "lista de fechas vacía no consulta")

	failing := testutils.FailingDB(t, db, nthFail("delete", 1))
	dao = NewGroupCalendarDayDao(failing)
	err := dao.DeleteByDates(nil, 1, []time.Time{time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)})
	require.Error(t, err)
}

// FindNextForGroupsByKind: error de select real y error desconocido del row.
func TestGroupCalendarDayDao_DBFail_FindNextForGroupsByKind(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	d1 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	// Sin groups devuelve nil sin query.
	day, err := dao.FindNextForGroupsByKind(nil, nil, "rest", d1, "10:00")
	require.NoError(t, err)
	assert.Nil(t, day)

	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupCalendarDayDao(failing)
	_, err = dao.FindNextForGroupsByKind(nil, []int64{1}, "rest", d1, "10:00")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding next day by kind")
}

func TestGroupCalendarDayDao_DBFail_FindNextPresencialForGroups(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	d1 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

	// Sin groups devuelve nil sin query.
	day, err := dao.FindNextPresencialForGroups(nil, nil, d1, "10:00")
	require.NoError(t, err)
	assert.Nil(t, day)

	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	dao = NewGroupCalendarDayDao(failing)
	_, err = dao.FindNextPresencialForGroups(nil, []int64{1}, d1, "10:00")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error finding next presencial day")
}

// Upsert: SELECT encontró la fila existente y el UPDATE falla (fail "update").
func TestGroupCalendarDayDao_DBFail_UpsertUpdateError(t *testing.T) {
	db := testutils.SetupTestDB(t)
	group := setupCalendarGroup(t, db, "10")
	dao := NewGroupCalendarDayDao(db)
	d1 := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))

	failing := testutils.FailingDB(t, db, nthFail("update", 1))
	dao = NewGroupCalendarDayDao(failing)
	err := dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "training"})

	require.Error(t, err)
}
