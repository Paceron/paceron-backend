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

func task4User(t *testing.T, db *gorm.DB, email, dni string) *dbs.User {
	t.Helper()
	user := &dbs.User{Name: "Task4", Surname: "User", Email: email, DNI: dni, BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(user).Error)
	return user
}

func task4TeamGroup(t *testing.T, db *gorm.DB, owner *dbs.User, teamName, groupName string) (*dbs.Team, *dbs.Group) {
	t.Helper()
	team := &dbs.Team{Name: teamName, MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: groupName, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	return team, group
}

func task4SoftDeleteGroup(t *testing.T, db *gorm.DB, groupID int64) {
	t.Helper()
	require.NoError(t, db.Model(&dbs.Group{}).Where("id = ?", groupID).Update("deleted_at", gorm.Expr("NOW()")).Error)
}

// CalendarSummary resuelve membresías+grupos en una sola consulta, conserva el
// orden de las membresías y omite grupos soft-deleted.
func TestCalendarService_CalendarSummary_SingleQueryOrderAndSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := task4User(t, db, "task4-summary-owner@test.com", "56000101")
	runner := task4User(t, db, "task4-summary-runner@test.com", "56000102")
	team, groupA := task4TeamGroup(t, db, owner, "Equipo summary task4", "Grupo A summary task4")
	groupB := &dbs.Group{Name: "Grupo B summary task4", TeamID: team.ID}
	require.NoError(t, db.Create(groupB).Error)
	groupDeleted := &dbs.Group{Name: "Grupo borrado summary task4", TeamID: team.ID}
	require.NoError(t, db.Create(groupDeleted).Error)

	// Orden de inserción de membresías: B, A, borrado.
	for _, groupID := range []int64{groupB.ID, groupA.ID, groupDeleted.ID} {
		require.NoError(t, db.Create(&dbs.GroupUser{GroupID: groupID, UserID: runner.ID, DateStart: time.Now()}).Error)
	}
	task4SoftDeleteGroup(t, db, groupDeleted.ID)

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(nil, nil, nil, daos.NewGroupUserDao(cdb), nil, nil, nil, nil, nil)

	items, err := svc.CalendarSummary(nil, runner.ID)

	require.NoError(t, err)
	assert.Equal(t, 1, counter, "memberships + grupos en una sola consulta")
	require.Len(t, items, 2, "el grupo soft-deleted no aparece")
	assert.Equal(t, groupB.ID, items[0].GroupID)
	assert.Equal(t, groupB.Name, items[0].GroupName)
	assert.Equal(t, groupA.ID, items[1].GroupID)
	assert.Equal(t, groupA.Name, items[1].GroupName)
}

func TestCalendarService_CalendarSummary_NoGroupsStillSingleQuery(t *testing.T) {
	db := testutils.SetupTestDB(t)
	runner := task4User(t, db, "task4-summary-empty@test.com", "56000103")

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(nil, nil, nil, daos.NewGroupUserDao(cdb), nil, nil, nil, nil, nil)

	items, err := svc.CalendarSummary(nil, runner.ID)

	require.NoError(t, err)
	assert.Equal(t, 1, counter)
	require.NotNil(t, items)
	assert.Empty(t, items)
}

// MemberCalendar resuelve grupo+equipo en una sola consulta con JOIN: 3 queries
// en total (memberships, rango, nombres) y nombres correctos.
func TestCalendarService_MemberCalendar_GroupTeamSingleQuery(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := task4User(t, db, "task4-member-owner@test.com", "56000104")
	runner := task4User(t, db, "task4-member-runner@test.com", "56000105")
	team, group := task4TeamGroup(t, db, owner, "Equipo member task4", "Grupo member task4")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Now()}).Error)
	date := time.Now().AddDate(0, 0, 3)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training"}))

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb), daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb)

	resp, err := svc.MemberCalendar(nil, runner.ID, date.AddDate(0, 0, -1), date.AddDate(0, 0, 1))

	require.NoError(t, err)
	assert.Equal(t, 3, counter, "memberships + rango + grupos/equipos (JOIN)")
	require.Len(t, resp, 1)
	assert.Equal(t, group.ID, resp[0].GroupID)
	assert.Equal(t, group.Name, resp[0].GroupName)
	assert.Equal(t, team.ID, resp[0].TeamID)
	assert.Equal(t, team.Name, resp[0].TeamName)
}

// Un grupo soft-deleted con membresía y día sigue produciendo la fila del día,
// pero sin datos de grupo/equipo (misma paridad que el batch separado anterior).
func TestCalendarService_MemberCalendar_SoftDeletedGroupKeepsDayWithoutNames(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := task4User(t, db, "task4-member-del-owner@test.com", "56000106")
	runner := task4User(t, db, "task4-member-del-runner@test.com", "56000107")
	_, group := task4TeamGroup(t, db, owner, "Equipo member del task4", "Grupo member del task4")
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Now()}).Error)
	date := time.Now().AddDate(0, 0, 3)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training"}))
	task4SoftDeleteGroup(t, db, group.ID)

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb), daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb)

	resp, err := svc.MemberCalendar(nil, runner.ID, date.AddDate(0, 0, -1), date.AddDate(0, 0, 1))

	require.NoError(t, err)
	assert.Equal(t, 3, counter)
	require.Len(t, resp, 1)
	assert.Equal(t, int64(0), resp[0].GroupID)
	assert.Empty(t, resp[0].GroupName)
	assert.Empty(t, resp[0].TeamName)
}

// AdministeredCalendar resuelve grupos+equipos en una sola consulta y el rango
// en otra: 2 queries en total.
func TestCalendarService_AdministeredCalendar_GroupTeamSingleQuery(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := task4User(t, db, "task4-admin-owner@test.com", "56000108")
	team, group := task4TeamGroup(t, db, owner, "Equipo admin task4", "Grupo admin task4")
	date := time.Now().AddDate(0, 0, 3)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training"}))

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb), daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb)

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date.AddDate(0, 0, -1), date.AddDate(0, 0, 1))

	require.NoError(t, err)
	assert.Equal(t, 2, counter, "grupos/equipos (JOIN) + rango")
	require.Len(t, resp, 1)
	assert.Equal(t, group.ID, resp[0].GroupID)
	assert.Equal(t, group.Name, resp[0].GroupName)
	assert.Equal(t, team.ID, resp[0].TeamID)
	assert.Equal(t, team.Name, resp[0].TeamName)
}

func TestCalendarService_AdministeredCalendar_SoftDeletedGroupExcluded(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner := task4User(t, db, "task4-admin-del-owner@test.com", "56000109")
	_, group := task4TeamGroup(t, db, owner, "Equipo admin del task4", "Grupo admin del task4")
	date := time.Now().AddDate(0, 0, 3)
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, &dbs.GroupCalendarDay{GroupID: group.ID, Date: date, Kind: "training"}))
	task4SoftDeleteGroup(t, db, group.ID)

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(cdb), daos.NewGroupDao(cdb), daos.NewTeamDao(cdb), daos.NewGroupUserDao(cdb), nil, nil, nil, nil, cdb)

	resp, err := svc.AdministeredCalendar(nil, owner.ID, date.AddDate(0, 0, -1), date.AddDate(0, 0, 1))

	require.NoError(t, err)
	assert.Equal(t, 1, counter, "sin grupos activos no se consulta el rango")
	require.NotNil(t, resp)
	assert.Empty(t, resp)
}
