package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewInvitationDao(t *testing.T) {
	dao := NewInvitationDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestInvitationDao_ImplementsInterface(t *testing.T) {
	dao := NewInvitationDao(&gorm.DB{})
	var iface InvitationDaoInterface = dao
	_ = iface
}

func TestInvitationDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewInvitationDao(&gorm.DB{})
	})
}

func testInvitation(db *gorm.DB, teamID, inviterID, inviteeID int64) *dbs.Invitation {
	inv := &dbs.Invitation{
		TeamID:    teamID,
		InviterID: inviterID,
		InviteeID: inviteeID,
		Status:    string(constants.InvitationStatusPending),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	db.Create(inv)
	return inv
}

func TestInvitationDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)
	owner := persistUser(db, "inv-create-owner@test.com", "60000001")
	invitee := persistUser(db, "inv-create-invitee@test.com", "60000002")
	team := testTeam(db, "equipo_inv_create", owner.ID)

	inv := &dbs.Invitation{TeamID: team.ID, InviterID: owner.ID, InviteeID: invitee.ID, Status: string(constants.InvitationStatusPending), ExpiresAt: time.Now().Add(24 * time.Hour)}
	err := dao.Create(nil, inv)

	require.NoError(t, err)
	assert.NotZero(t, inv.ID)
}

func TestInvitationDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)
	owner := persistUser(db, "inv-findid-owner@test.com", "60000003")
	invitee := persistUser(db, "inv-findid-invitee@test.com", "60000004")
	team := testTeam(db, "equipo_inv_findid", owner.ID)
	inv := testInvitation(db, team.ID, owner.ID, invitee.ID)

	found, err := dao.FindByID(nil, inv.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, inv.ID, found.ID)
}

func TestInvitationDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestInvitationDao_FindPendingByTeamAndInvitee_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)
	owner := persistUser(db, "inv-pendteaminvitee-owner@test.com", "60000005")
	invitee := persistUser(db, "inv-pendteaminvitee-invitee@test.com", "60000006")
	team := testTeam(db, "equipo_inv_pendteaminvitee", owner.ID)
	inv := testInvitation(db, team.ID, owner.ID, invitee.ID)

	found, err := dao.FindPendingByTeamAndInvitee(nil, team.ID, invitee.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, inv.ID, found.ID)
}

func TestInvitationDao_FindPendingByTeamAndInvitee_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)

	found, err := dao.FindPendingByTeamAndInvitee(nil, 999999, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestInvitationDao_FindPendingByTeamID_ExcludesNonPending(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)
	owner := persistUser(db, "inv-pendteam-owner@test.com", "60000007")
	invitee1 := persistUser(db, "inv-pendteam-invitee1@test.com", "60000008")
	invitee2 := persistUser(db, "inv-pendteam-invitee2@test.com", "60000009")
	team := testTeam(db, "equipo_inv_pendteam", owner.ID)
	pending := testInvitation(db, team.ID, owner.ID, invitee1.ID)
	accepted := testInvitation(db, team.ID, owner.ID, invitee2.ID)
	require.NoError(t, dao.UpdateStatus(nil, accepted.ID, string(constants.InvitationStatusAccepted), time.Now()))

	found, err := dao.FindPendingByTeamID(nil, team.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, pending.ID)
	assert.NotContains(t, ids, accepted.ID)
}

func TestInvitationDao_FindPendingByInviteeID_ExcludesNonPending(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)
	owner := persistUser(db, "inv-pendinvitee-owner@test.com", "60000010")
	invitee := persistUser(db, "inv-pendinvitee-invitee@test.com", "60000011")
	team1 := testTeam(db, "equipo_inv_pendinvitee1", owner.ID)
	team2 := testTeam(db, "equipo_inv_pendinvitee2", owner.ID)
	pending := testInvitation(db, team1.ID, owner.ID, invitee.ID)
	rejected := testInvitation(db, team2.ID, owner.ID, invitee.ID)
	require.NoError(t, dao.UpdateStatus(nil, rejected.ID, string(constants.InvitationStatusRejected), time.Now()))

	found, err := dao.FindPendingByInviteeID(nil, invitee.ID)

	require.NoError(t, err)
	ids := make([]int64, 0, len(found))
	for _, f := range found {
		ids = append(ids, f.ID)
	}
	assert.Contains(t, ids, pending.ID)
	assert.NotContains(t, ids, rejected.ID)
}

func TestInvitationDao_UpdateStatus_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)
	owner := persistUser(db, "inv-updatestatus-owner@test.com", "60000012")
	invitee := persistUser(db, "inv-updatestatus-invitee@test.com", "60000013")
	team := testTeam(db, "equipo_inv_updatestatus", owner.ID)
	inv := testInvitation(db, team.ID, owner.ID, invitee.ID)

	respondedAt := time.Now()
	err := dao.UpdateStatus(nil, inv.ID, string(constants.InvitationStatusAccepted), respondedAt)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, inv.ID)
	require.NoError(t, findErr)
	assert.Equal(t, string(constants.InvitationStatusAccepted), found.Status)
	assert.NotNil(t, found.RespondedAt)
}

func TestInvitationDao_SoftDeleteByTeamID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewInvitationDao(db)
	owner := persistUser(db, "inv-softdeleteteam-owner@test.com", "60000014")
	invitee := persistUser(db, "inv-softdeleteteam-invitee@test.com", "60000015")
	team := testTeam(db, "equipo_inv_softdeleteteam", owner.ID)
	inv := testInvitation(db, team.ID, owner.ID, invitee.ID)

	err := dao.SoftDeleteByTeamID(nil, team.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, inv.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found)
}
