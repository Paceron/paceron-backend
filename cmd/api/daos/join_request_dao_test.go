package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestJoinRequestDao_ImplementsInterface(t *testing.T) {
	dao := NewJoinRequestDao(&gorm.DB{})
	var iface JoinRequestDaoInterface = dao
	_ = iface
}

func testJoinRequestTeamAndOwner(db *gorm.DB, suffix string) (*dbs.Team, *dbs.User) {
	owner := persistUser(db, "jr-owner-"+suffix+"@test.com", "3000000"+suffix)
	team := testTeam(db, "equipo_jr_"+suffix, owner.ID)
	return team, owner
}

func TestJoinRequestDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "1")
	runner := persistUser(db, "jr-runner-1@test.com", "40000001")

	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	err := dao.Create(nil, jr)

	require.NoError(t, err)
	assert.NotZero(t, jr.ID)
}

func TestJoinRequestDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "2")
	runner := persistUser(db, "jr-runner-2@test.com", "40000002")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))

	found, err := dao.FindByID(nil, jr.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, runner.ID, found.RunnerID)
}

func TestJoinRequestDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestJoinRequestDao_FindPendingByTeamAndUser_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "3")
	runner := persistUser(db, "jr-runner-3@test.com", "40000003")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}))

	found, err := dao.FindPendingByTeamAndUser(nil, team.ID, runner.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
}

func TestJoinRequestDao_FindPendingByTeamAndUser_IgnoresResolved(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "4")
	runner := persistUser(db, "jr-runner-4@test.com", "40000004")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))
	require.NoError(t, dao.UpdateStatus(nil, jr.ID, string(constants.InvitationStatusRejected)))

	found, err := dao.FindPendingByTeamAndUser(nil, team.ID, runner.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestJoinRequestDao_FindPendingByTeam_OnlyPending(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "5")
	runnerA := persistUser(db, "jr-runner-5a@test.com", "40000005")
	runnerB := persistUser(db, "jr-runner-5b@test.com", "40000006")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: team.ID, RunnerID: runnerA.ID, Status: string(constants.InvitationStatusPending)}))
	resolved := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runnerB.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, resolved))
	require.NoError(t, dao.UpdateStatus(nil, resolved.ID, string(constants.InvitationStatusAccepted)))

	found, err := dao.FindPendingByTeam(nil, team.ID)

	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, runnerA.ID, found[0].RunnerID)
}

func TestJoinRequestDao_FindByUser_AllStatuses(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	teamA, _ := testJoinRequestTeamAndOwner(db, "6")
	teamB, _ := testJoinRequestTeamAndOwner(db, "7")
	runner := persistUser(db, "jr-runner-6@test.com", "40000007")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: teamA.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}))
	rejected := &dbs.JoinRequest{TeamID: teamB.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, rejected))
	require.NoError(t, dao.UpdateStatus(nil, rejected.ID, string(constants.InvitationStatusRejected)))

	found, err := dao.FindByUser(nil, runner.ID)

	require.NoError(t, err)
	assert.Len(t, found, 2)
}

func TestJoinRequestDao_UpdateStatus(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "8")
	runner := persistUser(db, "jr-runner-8@test.com", "40000008")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))

	err := dao.UpdateStatus(nil, jr.ID, string(constants.InvitationStatusAccepted))

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, jr.ID)
	assert.Equal(t, string(constants.InvitationStatusAccepted), found.Status)
}

func TestJoinRequestDao_Delete(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	team, _ := testJoinRequestTeamAndOwner(db, "9")
	runner := persistUser(db, "jr-runner-9@test.com", "40000009")
	jr := &dbs.JoinRequest{TeamID: team.ID, RunnerID: runner.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, jr))

	err := dao.Delete(nil, jr.ID)

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, jr.ID)
	assert.Nil(t, found)
}

func TestJoinRequestDao_CountPendingByOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewJoinRequestDao(db)
	owner := persistUser(db, "jr-owner-count@test.com", "40000010")
	teamA := testTeam(db, "equipo_jr_count_a", owner.ID)
	teamB := testTeam(db, "equipo_jr_count_b", owner.ID)
	runnerA := persistUser(db, "jr-runner-count-a@test.com", "40000011")
	runnerB := persistUser(db, "jr-runner-count-b@test.com", "40000012")
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: teamA.ID, RunnerID: runnerA.ID, Status: string(constants.InvitationStatusPending)}))
	require.NoError(t, dao.Create(nil, &dbs.JoinRequest{TeamID: teamB.ID, RunnerID: runnerB.ID, Status: string(constants.InvitationStatusPending)}))
	resolved := &dbs.JoinRequest{TeamID: teamA.ID, RunnerID: runnerB.ID, Status: string(constants.InvitationStatusPending)}
	require.NoError(t, dao.Create(nil, resolved))
	require.NoError(t, dao.UpdateStatus(nil, resolved.ID, string(constants.InvitationStatusRejected)))

	count, err := dao.CountPendingByOwner(nil, owner.ID)

	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
