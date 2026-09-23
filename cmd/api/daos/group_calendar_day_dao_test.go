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

func TestGroupCalendarDayDao_ImplementsInterface(t *testing.T) {
	dao := NewGroupCalendarDayDao(&gorm.DB{})
	var iface GroupCalendarDaoInterface = dao
	_ = iface
}

// setupCalendarGroup crea un owner + team + group real para los tests de
// calendario (no depende de team_dao_test.go, pero reusa persistUser).
func setupCalendarGroup(t *testing.T, db *gorm.DB, emailSuffix string) *dbs.Group {
	t.Helper()
	owner := persistUser(db, "cal-owner-"+emailSuffix+"@test.com", "70000"+emailSuffix)
	team := testTeam(db, "equipo_calendario_"+emailSuffix, owner.ID)
	group := &dbs.Group{Name: "grupo_calendario_" + emailSuffix, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	return group
}

func TestGroupCalendarDayDao_UpsertAndFindByGroupAndDate(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "1")
	date := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	err := dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest"})

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	require.NotNil(t, found)
	assert.Equal(t, "rest", found.Kind)
}

func TestGroupCalendarDayDao_Upsert_ReplacesExistingDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "2")
	date := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest"}))

	err := dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "other", OtherName: strPtrCal("Elongación")})

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Equal(t, "other", found.Kind)
	require.NotNil(t, found.OtherName)
	assert.Equal(t, "Elongación", *found.OtherName)
}

func TestGroupCalendarDayDao_FindByGroupAndRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "3")
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), Kind: "rest"}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), Kind: "rest"}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), Kind: "rest"}))

	results, err := dao.FindByGroupAndRange(nil, group.ID, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGroupCalendarDayDao_Delete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "4")
	date := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest"}))

	err := dao.Delete(nil, group.ID, date)

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}

func TestGroupCalendarDayDao_DeleteByDates(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "5")
	d1 := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d1, Kind: "rest"}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: d2, Kind: "rest"}))

	err := dao.DeleteByDates(nil, group.ID, []time.Time{d1, d2})

	require.NoError(t, err)
	results, findErr := dao.FindByGroupAndRange(nil, group.ID, d1, d2)
	require.NoError(t, findErr)
	assert.Empty(t, results)
}

func TestGroupCalendarDayDao_FindNextForGroupsByKind(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "6")
	// Usar UTC para evitar problemas de zona horaria con columnas DATE de Postgres
	past := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	future := time.Now().UTC().AddDate(0, 0, 3).Truncate(24 * time.Hour)
	sessionID := int64(1)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "training", SessionInstanceID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", SessionInstanceID: &sessionID}))

	today := time.Now().UTC().Truncate(24 * time.Hour)
	found, err := dao.FindNextForGroupsByKind(nil, []int64{group.ID}, "training", today, time.Now().UTC().Format("15:04"))

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Date.Equal(future))
}

func TestGroupCalendarDayDao_FindNextForGroupsByKind_TodayPresencialStartedExcluded(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "6b")
	// Un presencial de hoy cuyo horario ya arrancó NO cuenta; el próximo
	// elegible es el futuro. nowHHMM sintético → determinista.
	started := "00:00"
	today := time.Now().UTC().Truncate(24 * time.Hour)
	future := today.AddDate(0, 0, 3)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday(started), PresencialTimeTo: utcTimeToday("23:59")}))

	found, err := dao.FindNextForGroupsByKind(nil, []int64{group.ID}, "training", today, "12:00")

	require.NoError(t, err)
	assert.Nil(t, found)

	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training"}))
	found, err = dao.FindNextForGroupsByKind(nil, []int64{group.ID}, "training", today, "12:00")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Date.Equal(future))
}

func TestGroupCalendarDayDao_FindNextPresencialForGroups(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "7")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	future := today.AddDate(0, 0, 3)
	// Async y cancelled nunca son candidatos.
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training"}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today.AddDate(0, 0, 1), Kind: "cancelled", IsPresencial: true, PresencialTimeFrom: utcTimeToday("09:00"), PresencialTimeTo: utcTimeToday("10:00")}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday("09:00"), PresencialTimeTo: utcTimeToday("10:00")}))

	found, err := dao.FindNextPresencialForGroups(nil, []int64{group.ID}, today, "12:00")

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Date.Equal(future))
}

func TestGroupCalendarDayDao_FindNextPresencialForGroups_TodayPendingCounts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "8")
	today := time.Now().UTC().Truncate(24 * time.Hour)
	pending := "12:00"
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday(pending), PresencialTimeTo: utcTimeToday("23:59")}))

	found, err := dao.FindNextPresencialForGroups(nil, []int64{group.ID}, today, "06:00")

	require.NoError(t, err)
	require.NotNil(t, found, "hoy presencial por arrancar cuenta")
	assert.True(t, found.Date.Equal(today))

	started := "00:00"
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: today, Kind: "training", IsPresencial: true, PresencialTimeFrom: utcTimeToday(started), PresencialTimeTo: utcTimeToday("23:59")}))
	found, err = dao.FindNextPresencialForGroups(nil, []int64{group.ID}, today, "06:00")
	require.NoError(t, err)
	assert.Nil(t, found, "hoy presencial ya arrancado no cuenta")
}

// utcTimeToday arma un *time.Time de HOY a la HH:MM dada, en UTC (convención
// de persistencia de los horarios presenciales, ver dbs.GroupCalendarDay).
func utcTimeToday(hhmm string) *time.Time {
	hm, err := time.Parse("15:04", hhmm)
	if err != nil {
		return nil
	}
	now := time.Now().UTC()
	t := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), 0, 0, time.UTC)
	return &t
}

func TestGroupCalendarDayDao_ClearSourcePlan(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "9")
	planID := int64(99)
	date := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "rest", SourcePlanID: &planID}))

	err := dao.ClearSourcePlan(nil, planID)

	require.NoError(t, err)
	found, findErr := dao.FindByGroupAndDate(nil, group.ID, date)
	require.NoError(t, findErr)
	assert.Nil(t, found.SourcePlanID)
}

func TestGroupCalendarDayDao_UpdateDatesForShift(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "12")
	original := time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: original, Kind: "rest"}))

	err := dao.UpdateDatesForShift(nil, group.ID, original, original.AddDate(0, 0, 3))

	require.NoError(t, err)
	oldFound, _ := dao.FindByGroupAndDate(nil, group.ID, original)
	assert.Nil(t, oldFound)
	newFound, findErr := dao.FindByGroupAndDate(nil, group.ID, original.AddDate(0, 0, 3))
	require.NoError(t, findErr)
	require.NotNil(t, newFound)
}

func strPtrCal(s string) *string { return &s }
