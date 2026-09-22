package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestGroupDao_FindByOwnerID_GroupsAcrossTeams(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-fbyowner@test.com", "40000020")
	teamA := testTeam(db, "equipo_OWNER_a", owner.ID)
	teamB := testTeam(db, "equipo_OWNER_b", owner.ID)
	gA1 := testGroup(db, "grupo_owner_a1", teamA.ID)
	gA2 := testGroup(db, "grupo_owner_a2", teamA.ID)
	gB1 := testGroup(db, "grupo_owner_b1", teamB.ID)

	// Equipo ajeno al owner: sus grupos no deben aparecer.
	otherOwner := persistUser(db, "group-fbyowner-other@test.com", "40000021")
	otherTeam := testTeam(db, "equipo_ajeno", otherOwner.ID)
	other := testGroup(db, "grupo_ajeno", otherTeam.ID)

	groups, err := dao.FindByOwnerID(nil, owner.ID)

	require.NoError(t, err)
	require.Len(t, groups, 3)
	byID := map[int64]int64{}
	for _, g := range groups {
		byID[g.ID] = g.TeamID
	}
	assert.Equal(t, teamA.ID, byID[gA1.ID])
	assert.Equal(t, teamA.ID, byID[gA2.ID])
	assert.Equal(t, teamB.ID, byID[gB1.ID])
	assert.NotContains(t, byID, other.ID)
}

func TestGroupDao_FindByOwnerID_SoftDeletedGroupOutOfResults(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-fbyowner2@test.com", "40000022")
	team := testTeam(db, "equipo_fbyowner2", owner.ID)
	alive := testGroup(db, "grupo_vivo", team.ID)
	deleted := testGroup(db, "grupo_borrado", team.ID)
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	groups, err := dao.FindByOwnerID(nil, owner.ID)

	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, alive.ID, groups[0].ID)
}

func TestGroupDao_FindByOwnerID_SoftDeletedTeamOutOfResults(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-fbyowner3@test.com", "40000023")
	team := testTeam(db, "equipo_fbyowner3", owner.ID)
	testGroup(db, "grupo_equipo_borrado", team.ID)

	teamDao := NewTeamDao(db)
	require.NoError(t, teamDao.SoftDelete(nil, team.ID))

	groups, err := dao.FindByOwnerID(nil, owner.ID)

	require.NoError(t, err)
	assert.Empty(t, groups)
}

func presencialCalTimes(hourFrom, hourTo int) (*time.Time, *time.Time) {
	from := time.Date(0, 1, 1, hourFrom, 0, 0, 0, time.UTC)
	to := time.Date(0, 1, 1, hourTo, 0, 0, 0, time.UTC)
	return &from, &to
}

func TestGroupCalendarDayDao_FindPresencialForGroupsInRange(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "21")
	date := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	from, to := presencialCalTimes(9, 10)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", IsPresencial: true, PresencialTimeFrom: from, PresencialTimeTo: to}))
	// Cancelado presencial: fuera (fecha en la query).
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), Kind: "cancelled", IsPresencial: true, PresencialTimeFrom: from, PresencialTimeTo: to}))
	// Training no presencial: fuera (fecha en la query).
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC), Kind: "training"}))
	// Presencial en fecha fuera de la query: fuera.
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC), Kind: "training", IsPresencial: true, PresencialTimeFrom: from, PresencialTimeTo: to}))

	days, err := dao.FindPresencialForGroupsInRange(nil, []int64{group.ID}, []time.Time{date, time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)})

	require.NoError(t, err)
	require.Len(t, days, 1)
	assert.Equal(t, date.Format("2006-01-02"), days[0].Date.Format("2006-01-02"))
	assert.Equal(t, "training", days[0].Kind)
	assert.True(t, days[0].IsPresencial)
}

func TestGroupCalendarDayDao_FindPresencialForGroupsInRange_OrdersByDateAndTime(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)
	group := setupCalendarGroup(t, db, "22")
	date := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	lateFrom, lateTo := presencialCalTimes(18, 19)
	earlyFrom, earlyTo := presencialCalTimes(8, 9)
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training", IsPresencial: true, PresencialTimeFrom: lateFrom, PresencialTimeTo: lateTo}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), Kind: "training", IsPresencial: true, PresencialTimeFrom: earlyFrom, PresencialTimeTo: earlyTo}))
	require.NoError(t, dao.Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC), Kind: "rest"}))

	days, err := dao.FindPresencialForGroupsInRange(nil, []int64{group.ID}, []time.Time{date, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)})

	require.NoError(t, err)
	require.Len(t, days, 2)
	assert.Equal(t, "2026-11-09", days[0].Date.Format("2006-01-02"))
	assert.Equal(t, date.Format("2006-01-02"), days[1].Date.Format("2006-01-02"))
	assert.Equal(t, 8, days[0].PresencialTimeFrom.UTC().Hour())
	assert.Equal(t, 18, days[1].PresencialTimeFrom.UTC().Hour())
}

func TestGroupCalendarDayDao_FindPresencialForGroupsInRange_EmptyInputs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupCalendarDayDao(db)

	days, err := dao.FindPresencialForGroupsInRange(nil, nil, []time.Time{time.Now()})
	require.NoError(t, err)
	assert.Nil(t, days)

	days, err = dao.FindPresencialForGroupsInRange(nil, []int64{1}, nil)
	require.NoError(t, err)
	assert.Nil(t, days)
}
