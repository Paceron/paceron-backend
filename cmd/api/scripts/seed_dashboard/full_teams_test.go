package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/invitation"
	"simple-arq-golang/cmd/api/domains/team"
	"simple-arq-golang/cmd/api/infrastructure/crypto"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const (
	fullTeamsTestClientID = "seed-full-test-client"
	fullTeamsTestKey      = "seed-full-test-encryption-key-32"
)

func TestFullSeederSeedTeams_SeedsMatrixAndRelations(t *testing.T) {
	db := testutils.SetupTestDB(t)
	oldClientID, oldKey := config.MyMP.OAuthClientID, config.TokenEncryptionKey
	config.MyMP.OAuthClientID, config.TokenEncryptionKey = fullTeamsTestClientID, fullTeamsTestKey
	t.Cleanup(func() { config.MyMP.OAuthClientID, config.TokenEncryptionKey = oldClientID, oldKey })

	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.July, 18, 15, 0, 0, 0, gametime.Location)
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())
	assert.Equal(t, map[string]int64{
		"team_a": s.scope.teamAID, "team_b": s.scope.teamBID, "team_c": s.scope.teamCID,
		"team_d": s.scope.teamDID, "team_e": s.scope.teamEID, "team_f": s.scope.teamFID,
	}, s.ids.teams)
	assert.Equal(t, map[string]int64{
		"group_a_main": s.scope.groupAMainID, "group_a_secondary": s.scope.groupASecondaryID,
		"group_b": s.scope.groupBID, "group_c": s.scope.groupCID, "group_d": s.scope.groupDID,
		"group_e": s.scope.groupEID, "group_f": s.scope.groupFID,
	}, s.ids.groups)

	teams := make(map[int64]dbs.Team)
	for _, id := range []int64{s.scope.teamAID, s.scope.teamBID, s.scope.teamCID, s.scope.teamDID, s.scope.teamEID, s.scope.teamFID} {
		var row dbs.Team
		require.NoError(t, db.First(&row, id).Error)
		teams[id] = row
		assert.Contains(t, row.Description, fullSeedMarker)
		assert.NotNil(t, row.Lat)
		assert.NotNil(t, row.Lng)
	}
	assert.Equal(t, s.ids.users["trainer_1"], teams[s.scope.teamAID].OwnerID)
	assert.Equal(t, s.ids.users["trainer_2"], teams[s.scope.teamBID].OwnerID)
	assert.Equal(t, s.ids.users["trainer_1"], teams[s.scope.teamFID].OwnerID)
	assert.Zero(t, teams[s.scope.teamAID].MembershipFee)
	assert.Positive(t, teams[s.scope.teamBID].MembershipFee)
	assert.True(t, teams[s.scope.teamAID].Visible)
	assert.False(t, teams[s.scope.teamDID].Visible)
	assert.False(t, teams[s.scope.teamEID].IsPublic)

	var teamUsers []dbs.TeamUser
	require.NoError(t, db.Where("team_id IN ? AND deleted_at IS NULL", []int64{s.scope.teamAID, s.scope.teamBID, s.scope.teamCID, s.scope.teamDID, s.scope.teamEID, s.scope.teamFID}).Find(&teamUsers).Error)
	assert.GreaterOrEqual(t, len(teamUsers), 20)
	assertTeamMember(t, teamUsers, s.scope.teamAID, s.ids.users["trainer_1"], string(constants.TeamUserRoleEntrenador), "")
	assertTeamMember(t, teamUsers, s.scope.teamBID, s.ids.users["r1"], string(constants.TeamUserRoleCorredor), string(constants.SubscriptionStatusFirstPaymentPending))
	assertTeamMember(t, teamUsers, s.scope.teamBID, s.ids.users["r6"], string(constants.TeamUserRoleCorredor), string(constants.SubscriptionStatusFirstPaymentPending))
	assertTeamMember(t, teamUsers, s.scope.teamBID, s.ids.users["r11"], string(constants.TeamUserRoleCorredor), string(constants.SubscriptionStatusFirstPaymentPending))
	assertTeamMember(t, teamUsers, s.scope.teamCID, s.ids.users["r10"], string(constants.TeamUserRoleCorredor), string(constants.SubscriptionStatusActive))
	assertTeamMember(t, teamUsers, s.scope.teamFID, s.ids.users["r10"], string(constants.TeamUserRoleCorredor), string(constants.SubscriptionStatusActive))

	var groupUsers []dbs.GroupUser
	require.NoError(t, db.Where("group_id IN ? AND deleted_at IS NULL", []int64{s.scope.groupAMainID, s.scope.groupASecondaryID, s.scope.groupBID, s.scope.groupCID, s.scope.groupDID, s.scope.groupEID, s.scope.groupFID}).Find(&groupUsers).Error)
	assertGroupMembership(t, groupUsers, s.scope.groupAMainID, s.ids.users["r1"], s.now.AddDate(0, 0, -60), nil)
	ended := s.now.AddDate(0, 0, -7)
	assertGroupMembership(t, groupUsers, s.scope.groupASecondaryID, s.ids.users["r8"], s.now.AddDate(0, 0, -60), &ended)
	future := s.now.AddDate(0, 0, 7)
	assertGroupMembership(t, groupUsers, s.scope.groupASecondaryID, s.ids.users["r9"], future, nil)
	assertGroupMembership(t, groupUsers, s.scope.groupBID, s.ids.users["r1"], s.now.AddDate(0, 0, -30), nil)
	assertGroupMembership(t, groupUsers, s.scope.groupBID, s.ids.users["r11"], s.now.AddDate(0, 0, -30), nil)

	var invites []dbs.Invitation
	require.NoError(t, db.Where("team_id IN ?", []int64{s.scope.teamAID, s.scope.teamBID}).Find(&invites).Error)
	assertInvitationStatus(t, invites, s.scope.teamAID, s.ids.users["r10"], string(constants.InvitationStatusPending), nil)
	assertInvitationStatus(t, invites, s.scope.teamBID, s.ids.users["r9"], string(constants.InvitationStatusPending), nil)
	assertInvitationStatus(t, invites, s.scope.teamBID, s.ids.users["r11"], string(constants.InvitationStatusAccepted), &s.now)
	assertInvitationStatus(t, invites, s.scope.teamAID, s.ids.users["invitations_off"], string(constants.InvitationStatusRejected), &s.now)
	var expired dbs.Invitation
	require.NoError(t, db.Where("team_id = ? AND invitee_id = ?", s.scope.teamAID, s.ids.users["r12"]).Take(&expired).Error)
	assert.Equal(t, string(constants.InvitationStatusPending), expired.Status)
	assert.True(t, expired.ExpiresAt.Before(s.now))

	var requests []dbs.JoinRequest
	require.NoError(t, db.Where("team_id IN ?", []int64{s.scope.teamAID, s.scope.teamBID, s.scope.teamCID, s.scope.teamEID}).Find(&requests).Error)
	assertJoinRequestStatus(t, requests, s.scope.teamAID, s.ids.users["r12"], string(constants.InvitationStatusPending))
	assertJoinRequestStatus(t, requests, s.scope.teamEID, s.ids.users["r10"], string(constants.InvitationStatusPending))
	assertJoinRequestStatus(t, requests, s.scope.teamCID, s.ids.users["r11"], string(constants.InvitationStatusPending))
	assertJoinRequestStatus(t, requests, s.scope.teamBID, s.ids.users["r12"], string(constants.InvitationStatusRejected))

	var connection dbs.SellerConnection
	require.NoError(t, db.Where("user_id = ? AND client_id = ?", s.ids.users["trainer_1"], fullTeamsTestClientID).Take(&connection).Error)
	assert.Equal(t, string(constants.SellerConnectionStatusAuthorized), connection.Status)
	assert.NotEmpty(t, connection.PublicKey)
	encryptor := crypto.NewAESGCMEncryptor(fullTeamsTestKey)
	decryptedAccess, err := encryptor.Decrypt(connection.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, fullSyntheticSellerAccessToken, decryptedAccess)
	decryptedRefresh, err := encryptor.Decrypt(connection.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, fullSyntheticSellerRefreshToken, decryptedRefresh)
	var trainerTwoConnectionCount int64
	require.NoError(t, db.Model(&dbs.SellerConnection{}).Where("user_id = ?", s.ids.users["trainer_2"]).Count(&trainerTwoConnectionCount).Error)
	assert.Zero(t, trainerTwoConnectionCount)

	before := fullTeamsTableCounts(t, db)
	firstAccessToken := connection.AccessToken
	firstRefreshToken := connection.RefreshToken
	require.NoError(t, s.seedTeams())
	assert.Equal(t, before, fullTeamsTableCounts(t, db))
	require.NoError(t, db.Where("user_id = ? AND client_id = ?", s.ids.users["trainer_1"], fullTeamsTestClientID).Take(&connection).Error)
	assert.Equal(t, firstAccessToken, connection.AccessToken)
	assert.Equal(t, firstRefreshToken, connection.RefreshToken)
}

func TestFullSeederSeedTeams_RealTeamContracts(t *testing.T) {
	db := testutils.SetupTestDB(t)
	oldClientID, oldKey := config.MyMP.OAuthClientID, config.TokenEncryptionKey
	config.MyMP.OAuthClientID, config.TokenEncryptionKey = fullTeamsTestClientID, fullTeamsTestKey
	t.Cleanup(func() { config.MyMP.OAuthClientID, config.TokenEncryptionKey = oldClientID, oldKey })

	s := newFullSeeder(db, options{})
	s.now = time.Date(2026, time.July, 18, 15, 0, 0, 0, gametime.Location)
	require.NoError(t, s.seedIdentity())
	require.NoError(t, s.seedTeams())

	teamService := services.NewTeamService(
		daos.NewTeamDao(db), daos.NewUserDao(db), daos.NewUserRoleDao(db), daos.NewRoleDao(db),
		daos.NewTeamUserDao(db), daos.NewGroupDao(db), daos.NewGroupUserDao(db), daos.NewInvitationDao(db), nil,
		daos.NewSellerConnectionDao(db),
	)
	visible, err := teamService.Search(nil, s.ids.users["no_role"], team.SearchFilters{Name: "Semilla Full"}, 1)
	require.NoError(t, err)
	visibleIDs := make(map[int64]team.TeamSearchResult, len(visible.Teams))
	for _, result := range visible.Teams {
		visibleIDs[result.ID] = result
	}
	for _, id := range []int64{s.scope.teamAID, s.scope.teamBID, s.scope.teamCID, s.scope.teamEID, s.scope.teamFID} {
		assert.Contains(t, visibleIDs, id)
	}
	assert.NotContains(t, visibleIDs, s.scope.teamDID)
	assert.True(t, visibleIDs[s.scope.teamAID].CanReceivePayments)
	assert.False(t, visibleIDs[s.scope.teamBID].CanReceivePayments)
	assert.Equal(t, visibleIDs[s.scope.teamCID].MaxMembers, visibleIDs[s.scope.teamCID].MemberCount)

	ownedSearch, err := teamService.Search(nil, s.ids.users["trainer_1"], team.SearchFilters{Name: "Semilla Full"}, 1)
	require.NoError(t, err)
	for _, result := range ownedSearch.Teams {
		assert.NotContains(t, []int64{s.scope.teamAID, s.scope.teamCID, s.scope.teamFID}, result.ID)
	}

	joinService := services.NewJoinRequestService(
		daos.NewJoinRequestDao(db), daos.NewTeamDao(db), daos.NewTeamUserDao(db), daos.NewUserDao(db),
		daos.NewGroupDao(db), daos.NewGroupUserDao(db), daos.NewInstallmentDao(db), db,
	)
	_, err = joinService.Create(nil, s.scope.teamCID, s.ids.users["r12"])
	assert.ErrorIs(t, err, services.ErrTeamFull)
	_, err = joinService.Create(nil, s.scope.teamEID, s.ids.users["r12"])
	assert.ErrorIs(t, err, services.ErrTeamNotPublic)

	var invitationCount int64
	require.NoError(t, db.Model(&dbs.Invitation{}).Where("team_id = ? AND invitee_id = ?", s.scope.teamAID, s.ids.users["invitations_off"]).Count(&invitationCount).Error)
	inviteService := services.NewInvitationService(
		daos.NewUserDao(db), daos.NewTeamDao(db), daos.NewInvitationDao(db), daos.NewTeamUserDao(db),
		daos.NewGroupDao(db), daos.NewGroupUserDao(db), nil, nil, nil, daos.NewInstallmentDao(db), db,
		daos.NewSellerConnectionDao(db),
	)
	_, err = inviteService.InviteRunner(nil, s.scope.teamAID, s.ids.users["trainer_1"], &invitation.InviteRunnerRequest{
		Email: "invitations-off.seed-full@seed-full.local",
	})
	require.EqualError(t, err, "el usuario no acepta invitaciones a equipos")
	var invitationCountAfter int64
	require.NoError(t, db.Model(&dbs.Invitation{}).Where("team_id = ? AND invitee_id = ?", s.scope.teamAID, s.ids.users["invitations_off"]).Count(&invitationCountAfter).Error)
	assert.Equal(t, invitationCount, invitationCountAfter)
}

func TestFullSeederSeedTeams_RequiresIdentityAndSellerConfiguration(t *testing.T) {
	db := testutils.SetupTestDB(t)
	s := newFullSeeder(db, options{})
	err := s.seedTeams()
	require.ErrorContains(t, err, "seedIdentity")

	oldClientID, oldKey := config.MyMP.OAuthClientID, config.TokenEncryptionKey
	config.MyMP.OAuthClientID, config.TokenEncryptionKey = fullTeamsTestClientID, fullTeamsTestKey
	t.Cleanup(func() { config.MyMP.OAuthClientID, config.TokenEncryptionKey = oldClientID, oldKey })
	require.NoError(t, s.seedIdentity())

	config.MyMP.OAuthClientID = ""
	err = s.seedTeams()
	require.ErrorContains(t, err, "MP_OAUTH_CLIENT_ID")
	config.MyMP.OAuthClientID = fullTeamsTestClientID
	config.TokenEncryptionKey = "invalid-key-length"
	err = s.seedTeams()
	require.ErrorContains(t, err, "invalid key size")

	var teamCount int64
	require.NoError(t, db.Model(&dbs.Team{}).Where("id BETWEEN ? AND ?", s.scope.teamAID, s.scope.teamFID).Count(&teamCount).Error)
	assert.Zero(t, teamCount)
}

func TestFullSeederSeedTeams_RejectsReservedIDsWithoutMarkerAtomically(t *testing.T) {
	db := testutils.SetupTestDB(t)
	oldClientID, oldKey := config.MyMP.OAuthClientID, config.TokenEncryptionKey
	config.MyMP.OAuthClientID, config.TokenEncryptionKey = fullTeamsTestClientID, fullTeamsTestKey
	t.Cleanup(func() { config.MyMP.OAuthClientID, config.TokenEncryptionKey = oldClientID, oldKey })

	s := newFullSeeder(db, options{})
	require.NoError(t, s.seedIdentity())
	foreign := dbs.Team{ID: s.scope.teamDID, Name: "foreign", Description: "not a full seed"}
	require.NoError(t, db.Create(&foreign).Error)

	err := s.seedTeams()
	require.ErrorContains(t, err, "sin marker")
	var seededTeamCount int64
	require.NoError(t, db.Model(&dbs.Team{}).Where("id IN ?", []int64{s.scope.teamAID, s.scope.teamBID, s.scope.teamCID, s.scope.teamEID, s.scope.teamFID}).Count(&seededTeamCount).Error)
	assert.Zero(t, seededTeamCount)
}

func assertTeamMember(t *testing.T, rows []dbs.TeamUser, teamID, userID int64, role, subscriptionStatus string) {
	t.Helper()
	for _, row := range rows {
		if row.TeamID == teamID && row.UserID == userID {
			assert.Equal(t, role, row.RoleInTeam)
			assert.Equal(t, "active", row.Status)
			assert.Equal(t, subscriptionStatus, row.SubscriptionStatus)
			return
		}
	}
	t.Fatalf("no se encontró team_user team=%d user=%d", teamID, userID)
}

func assertGroupMembership(t *testing.T, rows []dbs.GroupUser, groupID, userID int64, start time.Time, end *time.Time) {
	t.Helper()
	for _, row := range rows {
		if row.GroupID == groupID && row.UserID == userID {
			assert.True(t, row.DateStart.Equal(start))
			if end == nil {
				assert.Nil(t, row.DateEnd)
			} else {
				require.NotNil(t, row.DateEnd)
				assert.True(t, row.DateEnd.Equal(*end))
			}
			return
		}
	}
	t.Fatalf("no se encontró group_user group=%d user=%d", groupID, userID)
}

func assertInvitationStatus(t *testing.T, rows []dbs.Invitation, teamID, inviteeID int64, status string, respondedAt *time.Time) {
	t.Helper()
	for _, row := range rows {
		if row.TeamID == teamID && row.InviteeID == inviteeID && row.ExpiresAt.After(time.Time{}) {
			if row.Status == status {
				if respondedAt == nil {
					assert.Nil(t, row.RespondedAt)
				} else {
					require.NotNil(t, row.RespondedAt)
					assert.True(t, row.RespondedAt.Equal(*respondedAt))
				}
				return
			}
		}
	}
	t.Fatalf("no se encontró invitation team=%d invitee=%d status=%s", teamID, inviteeID, status)
}

func assertJoinRequestStatus(t *testing.T, rows []dbs.JoinRequest, teamID, runnerID int64, status string) {
	t.Helper()
	for _, row := range rows {
		if row.TeamID == teamID && row.RunnerID == runnerID {
			assert.Equal(t, status, row.Status)
			return
		}
	}
	t.Fatalf("no se encontró join_request team=%d runner=%d", teamID, runnerID)
}

func fullTeamsTableCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	models := map[string]any{
		"teams": &dbs.Team{}, "groups": &dbs.Group{}, "team_users": &dbs.TeamUser{},
		"group_users": &dbs.GroupUser{}, "invitations": &dbs.Invitation{},
		"join_requests": &dbs.JoinRequest{}, "seller_connections": &dbs.SellerConnection{},
	}
	counts := make(map[string]int64, len(models))
	for table, model := range models {
		var count int64
		require.NoError(t, db.Model(model).Count(&count).Error)
		counts[table] = count
	}
	return counts
}
