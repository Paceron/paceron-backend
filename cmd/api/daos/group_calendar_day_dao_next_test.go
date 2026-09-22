package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// nowHHMM se pasa como parámetro, así que este grupo de tests es determinista
// y no depende del reloj real (a diferencia de los tests de banner que usan
// time.Now() real por convención del repo).
func TestGroupCalendarDayDao_HoyPresencialBordeExactoExcluido(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "border")
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// from == nowHHMM (borde exacto) NO cuenta.
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday("12:00"), PresencialTimeTo: utcTimeToday("13:00")}))

	found, err := dao.FindNextForGroupsByKind(nil, []int64{group.ID}, "training", today, "12:00")
	require.NoError(t, err)
	require.Nil(t, found)

	// from > nowHHMM sí cuenta.
	found, err = dao.FindNextForGroupsByKind(nil, []int64{group.ID}, "training", today, "10:00")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Date.Equal(today))

	// Mismo criterio en la variante presencial.
	found, err = dao.FindNextPresencialForGroups(nil, []int64{group.ID}, today, "12:00")
	require.NoError(t, err)
	require.Nil(t, found)

	found, err = dao.FindNextPresencialForGroups(nil, []int64{group.ID}, today, "10:00")
	require.NoError(t, err)
	require.NotNil(t, found)
}

func TestGroupCalendarDayDao_FindNextForGroupsByKind_DesempateMismaFecha(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	groupA := setupCalendarGroup(t, db, "tie-a") // id menor
	groupB := setupCalendarGroup(t, db, "tie-b") // id mayor
	require.Less(t, groupA.ID, groupB.ID)
	future := time.Now().UTC().AddDate(0, 0, 2).Truncate(24 * time.Hour)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// groupB (id mayor) arranca más temprano: gana por horario aunque pierda por id.
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupA.ID, Date: future, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday("15:00"), PresencialTimeTo: utcTimeToday("16:00")}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupB.ID, Date: future, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday("09:00"), PresencialTimeTo: utcTimeToday("10:00")}))

	found, err := dao.FindNextPresencialForGroups(nil, []int64{groupA.ID, groupB.ID}, today, "00:00")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, groupB.ID, found.GroupID)

	// A igual horario en la otra fecha, el id menor desempata. Primero se
	// limpian las filas del escenario anterior para que la query sea la
	// más temprana elegible.
	require.NoError(t, dao.DeleteByDates(nil, groupA.ID, []time.Time{future}))
	require.NoError(t, dao.DeleteByDates(nil, groupB.ID, []time.Time{future}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupA.ID, Date: future.AddDate(0, 0, 1), Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday("09:00"), PresencialTimeTo: utcTimeToday("10:00")}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupB.ID, Date: future.AddDate(0, 0, 1), Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday("09:00"), PresencialTimeTo: utcTimeToday("10:00")}))

	found, err = dao.FindNextPresencialForGroups(nil, []int64{groupA.ID, groupB.ID}, today, "23:59")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, groupA.ID, found.GroupID)
	assert.Equal(t, future.AddDate(0, 0, 1).Format("2006-01-02"), found.Date.Format("2006-01-02"))
}
