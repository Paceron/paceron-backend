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

func TestGroupCalendarDayDao_FindNextSessionForGroups(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "6")
	// Usar UTC para evitar problemas de zona horaria con columnas DATE de Postgres
	past := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)
	future := time.Now().UTC().AddDate(0, 0, 3).Truncate(24 * time.Hour)
	sessionID := int64(1)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: past, Kind: "training", SessionInstanceID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: future, Kind: "training", SessionInstanceID: &sessionID}))

	found, err := dao.FindNextSessionForGroups(nil, []int64{group.ID}, time.Now().UTC().Truncate(24*time.Hour))

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Date.Equal(future))
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

func TestGroupCalendarDayDao_RepointSessionForGroups(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	groupA := setupCalendarGroup(t, db, "10")
	groupB := setupCalendarGroup(t, db, "11")
	oldSessionID := int64(5)
	newSessionID := int64(6)
	dateA := time.Date(2026, 12, 5, 0, 0, 0, 0, time.UTC)
	dateB := time.Date(2026, 12, 6, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupA.ID, Date: dateA, Kind: "training", SessionInstanceID: &oldSessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: groupB.ID, Date: dateB, Kind: "training", SessionInstanceID: &oldSessionID}))

	err := dao.RepointSessionForGroups(nil, []int64{groupA.ID}, oldSessionID, newSessionID)

	require.NoError(t, err)
	foundA, _ := dao.FindByGroupAndDate(nil, groupA.ID, dateA)
	require.NotNil(t, foundA.SessionInstanceID)
	assert.Equal(t, newSessionID, *foundA.SessionInstanceID)
	foundB, _ := dao.FindByGroupAndDate(nil, groupB.ID, dateB)
	require.NotNil(t, foundB.SessionInstanceID)
	assert.Equal(t, oldSessionID, *foundB.SessionInstanceID, "groupB no estaba en la lista a repuntear, debe quedar intacto")
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

func TestGroupCalendarDayDao_FindBySessionID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group1 := setupCalendarGroup(t, db, "13")
	group2 := setupCalendarGroup(t, db, "14")
	sessionID := int64(77)
	otherSessionID := int64(78)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group1.ID, Date: time.Date(2027, 2, 1, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group2.ID, Date: time.Date(2027, 2, 2, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group1.ID, Date: time.Date(2027, 2, 3, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &otherSessionID}))

	found, err := dao.FindBySessionID(nil, sessionID)

	require.NoError(t, err)
	assert.Len(t, found, 2)
}

func TestGroupCalendarDayDao_RepointDaysByID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "15")
	oldSessionID := int64(80)
	newSessionID := int64(81)
	date1 := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2027, 3, 2, 0, 0, 0, 0, time.UTC)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date1, Kind: "training", SessionInstanceID: &oldSessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date2, Kind: "training", SessionInstanceID: &oldSessionID}))
	day1, err := dao.FindByGroupAndDate(nil, group.ID, date1)
	require.NoError(t, err)

	err = dao.RepointDaysByID(nil, []int64{day1.ID}, newSessionID)

	require.NoError(t, err)
	found1, _ := dao.FindByGroupAndDate(nil, group.ID, date1)
	require.NotNil(t, found1.SessionInstanceID)
	assert.Equal(t, newSessionID, *found1.SessionInstanceID)
	found2, _ := dao.FindByGroupAndDate(nil, group.ID, date2)
	require.NotNil(t, found2.SessionInstanceID)
	assert.Equal(t, oldSessionID, *found2.SessionInstanceID, "el día no listado en dayIDs debe quedar intacto, aunque comparta group_id y session_id viejo")
}

func TestGroupCalendarDayDao_FindByExerciseID(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "16")
	sessionID := int64(90)
	otherSessionID := int64(91)
	exerciseID := int64(500)
	otherExerciseID := int64(501)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: sessionID, ExerciseID: exerciseID, Role: "main", RepeatCount: 1, RestMinutes: 0}).Error)
	require.NoError(t, db.Create(&dbs.SessionExercise{SessionID: otherSessionID, ExerciseID: otherExerciseID, Role: "main", RepeatCount: 1, RestMinutes: 0}).Error)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2027, 4, 1, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &sessionID}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2027, 4, 2, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &otherSessionID}))

	found, err := dao.FindByExerciseID(nil, exerciseID)

	require.NoError(t, err)
	require.Len(t, found, 1)
	require.NotNil(t, found[0].SessionInstanceID)
	assert.Equal(t, sessionID, *found[0].SessionInstanceID)
}

func strPtrCal(s string) *string { return &s }
