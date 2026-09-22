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

func presencialServiceTimes(hourFrom, hourTo int) (*time.Time, *time.Time) {
	from := time.Date(0, 1, 1, hourFrom, 0, 0, 0, time.UTC)
	to := time.Date(0, 1, 1, hourTo, 0, 0, 0, time.UTC)
	return &from, &to
}

func seedPresencialServiceDay(t *testing.T, db *gorm.DB, groupID int64, date time.Time, fromH, toH int) int64 {
	t.Helper()
	from, to := presencialServiceTimes(fromH, toH)
	day := &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "training", IsPresencial: true, PresencialTimeFrom: from, PresencialTimeTo: to}
	require.NoError(t, daos.NewGroupCalendarDayDao(db).Upsert(nil, day))
	return day.ID
}

func extraOwnerTeamGroup(t *testing.T, db *gorm.DB, teamID int64, name string) *dbs.Group {
	t.Helper()
	group := &dbs.Group{Name: name + " group", TeamID: teamID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	return group
}

func extraOwnerTeam(t *testing.T, db *gorm.DB, ownerID int64, name string) *dbs.Team {
	t.Helper()
	team := &dbs.Team{Name: name + " team", MaxMembers: 10, OwnerID: ownerID}
	require.NoError(t, db.Create(team).Error)
	return team
}

func presencialCandidate(groupID int64, date time.Time, fromH, toH int) dbs.GroupCalendarDay {
	from, to := presencialServiceTimes(fromH, toH)
	return dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: "training", IsPresencial: true, PresencialTimeFrom: from, PresencialTimeTo: to}
}

func TestFindPresencialCollisions_ClassifiesCrossAndSame(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "prescls1")
	groupA2 := extraOwnerTeamGroup(t, db, groupA.TeamID, "prescls1-a2") // mismo equipo que groupA
	teamB := extraOwnerTeam(t, db, owner.ID, "prescls1-b")              // equipo distinto
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "prescls1-b")
	groupB2 := extraOwnerTeamGroup(t, db, teamB.ID, "prescls1-b2") // otro grupo de teamB (misma fecha, franja que toca el borde)
	date := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	seedPresencialServiceDay(t, db, groupB.ID, date, 9, 10)   // cross esperado
	seedPresencialServiceDay(t, db, groupA2.ID, date, 10, 11) // same esperado
	seedPresencialServiceDay(t, db, groupB2.ID, date, 11, 12) // borde que toca: no choca con 9-11
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db).(*calendarService)

	cross, same, err := svc.findPresencialCollisions(nil, db, owner.ID, &groupA.ID, nil, []time.Time{date}, []dbs.GroupCalendarDay{
		presencialCandidate(groupA.ID, date, 9, 11),
	})

	require.NoError(t, err)
	require.Len(t, cross, 1)
	assert.Equal(t, groupB.ID, cross[0].GroupID)
	assert.NotEqual(t, groupA.TeamID, cross[0].TeamID)
	require.Len(t, same, 1)
	assert.Equal(t, groupA2.ID, same[0].GroupID)
	assert.Equal(t, groupA.TeamID, same[0].TeamID)
	assert.Equal(t, "09:00", cross[0].PresencialTimeFrom)
	assert.Equal(t, "10:00", cross[0].PresencialTimeTo)
}

func TestFindPresencialCollisions_ExcludesSelfGroupAndMovedDayIDs(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupA := task3OwnerGroup(t, db, "prescls2")
	teamB := extraOwnerTeam(t, db, owner.ID, "prescls2-b")
	groupB := extraOwnerTeamGroup(t, db, teamB.ID, "prescls2-b")
	date := time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)
	seedPresencialServiceDay(t, db, groupB.ID, date, 9, 10)
	movedID := seedPresencialServiceDay(t, db, groupB.ID, date, 15, 16)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db).(*calendarService)

	cross, same, err := svc.findPresencialCollisions(nil, db, owner.ID, &groupA.ID, []int64{movedID}, []time.Time{date}, []dbs.GroupCalendarDay{
		presencialCandidate(groupA.ID, date, 9, 10),
		presencialCandidate(groupA.ID, date, 15, 16),
	})

	require.NoError(t, err)
	assert.Empty(t, cross, "el día movido (excludeDayIDs) no debe reportarse")
	assert.Empty(t, same)
}

func TestFindPresencialCollisions_DedupKeepsCrossAndSameForSameDay(t *testing.T) {
	db := testutils.SetupTestDB(t)
	owner, groupRef := task3OwnerGroup(t, db, "prescls3")
	groupX := extraOwnerTeamGroup(t, db, groupRef.TeamID, "prescls3-x") // mismo equipo que groupY
	groupY := extraOwnerTeamGroup(t, db, groupRef.TeamID, "prescls3-y")
	teamZ := extraOwnerTeam(t, db, owner.ID, "prescls3-z") // equipo distinto
	groupZ := extraOwnerTeamGroup(t, db, teamZ.ID, "prescls3-z")
	date := time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC)
	seedPresencialServiceDay(t, db, groupX.ID, date, 9, 10)
	svc := NewCalendarService(daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db), daos.NewGroupUserDao(db), nil, nil, nil, nil, db).(*calendarService)

	// groupY es del mismo equipo que groupX; groupZ de otro. El mismo día
	// colisionante debe aparecer en ambas clasificaciones: el dedup no puede
	// suprimir el cross solo porque un candidato same-team pasó primero.
	cross, same, err := svc.findPresencialCollisions(nil, db, owner.ID, nil, nil, []time.Time{date}, []dbs.GroupCalendarDay{
		presencialCandidate(groupY.ID, date, 9, 10),
		presencialCandidate(groupZ.ID, date, 9, 10),
	})

	require.NoError(t, err)
	require.Len(t, cross, 1, "la colisión cross no debe ser suprimida por el dedup")
	assert.Equal(t, groupX.ID, cross[0].GroupID)
	require.Len(t, same, 1)
	assert.Equal(t, groupX.ID, same[0].GroupID)
}
