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

func batchreadTestUser(t *testing.T, db *gorm.DB, name, email string, dni string) *dbs.User {
	t.Helper()
	u := &dbs.User{Name: name, Surname: "Biz", Email: email, DNI: dni, BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(u).Error)
	return u
}

// D9 batch: las 3 invitaciones vigentes (2 equipos, 3 users distintos
// invitee+inviters) resuelven en 3 queries: pendientes + teams IN + users IN.
// La invitación vencida no agrega queries ni aparece.
func TestInvitationService_ListPendingInvitationsForUser_BatchQueries(t *testing.T) {
	db := testutils.SetupTestDB(t)
	invitee := batchreadTestUser(t, db, "Ivy", "batchread-ivy@test.com", "55000201")
	inviter1 := batchreadTestUser(t, db, "Carl", "batchread-carl@test.com", "55000202")
	inviter2 := batchreadTestUser(t, db, "Dana", "batchread-dana@test.com", "55000203")
	teamA := &dbs.Team{Name: "Team A batchread", MaxMembers: 10, OwnerID: inviter1.ID, MembershipFee: 1500}
	require.NoError(t, db.Create(teamA).Error)
	teamB := &dbs.Team{Name: "Team B batchread", MaxMembers: 10, OwnerID: inviter2.ID}
	require.NoError(t, db.Create(teamB).Error)

	invDao := daos.NewInvitationDao(db)
	pending := time.Now().Add(24 * time.Hour)
	expired := time.Now().Add(-24 * time.Hour)
	for _, inv := range []dbs.Invitation{
		{TeamID: teamA.ID, InviterID: inviter1.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: pending},
		{TeamID: teamB.ID, InviterID: inviter2.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: pending},
		{TeamID: teamA.ID, InviterID: inviter2.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: pending},
		{TeamID: teamA.ID, InviterID: inviter1.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: expired},
	} {
		require.NoError(t, invDao.Create(nil, &inv))
	}

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewInvitationService(daos.NewUserDao(cdb), daos.NewTeamDao(cdb), daos.NewInvitationDao(cdb), daos.NewTeamUserDao(cdb), nil, nil, nil, nil, nil, nil, cdb, &mockSellerConnectionDao{})

	resp, err := svc.ListPendingInvitationsForUser(nil, invitee.ID)

	require.NoError(t, err)
	require.Len(t, resp, 3)
	assert.Equal(t, 3, counter, "pendientes + teams IN + users IN; la vencida no participa en queries")
	assert.Equal(t, "Ivy", resp[0].InviteeName)
	assert.Equal(t, "batchread-ivy@test.com", resp[0].InviteeEmail)
	assert.Equal(t, inviter1.Name, resp[0].InviterName)
	assert.Equal(t, "Team A batchread", resp[0].TeamName)
	assert.Equal(t, float64(1500), resp[0].MembershipFee)
	assert.False(t, resp[0].CanReceivePayments)
	assert.Equal(t, "Team B batchread", resp[1].TeamName)
	assert.Equal(t, inviter2.Name, resp[1].InviterName)
	assert.Equal(t, "Team A batchread", resp[2].TeamName)
}

// Migración batch del listado del equipo (D9): 3 queries — team del caller +
// pendientes + users IN. Antes: team + pendientes + 2 users por invitación.
func TestInvitationService_ListPendingInvitations_BatchQueries(t *testing.T) {
	db := testutils.SetupTestDB(t)
	team := &dbs.Team{Name: "Team batchread eq", MaxMembers: 10, OwnerID: 0, MembershipFee: 700}
	require.NoError(t, db.Create(team).Error)
	invitee := batchreadTestUser(t, db, "Nora", "batchread-nora@test.com", "55000204")
	inviter := batchreadTestUser(t, db, "Omar", "batchread-omar@test.com", "55000205")
	require.NoError(t, db.Create(&dbs.TeamUser{TeamID: team.ID, UserID: inviter.ID, RoleInTeam: "entrenador", Status: "active", AssignmentDate: time.Now()}).Error)

	invDao := daos.NewInvitationDao(db)
	inv := dbs.Invitation{TeamID: team.ID, InviterID: inviter.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: time.Now().Add(24 * time.Hour)}
	require.NoError(t, invDao.Create(nil, &inv))

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewInvitationService(daos.NewUserDao(cdb), daos.NewTeamDao(cdb), daos.NewInvitationDao(cdb), daos.NewTeamUserDao(cdb), nil, nil, nil, nil, nil, nil, cdb, &mockSellerConnectionDao{})

	resp, err := svc.ListPendingInvitations(nil, team.ID, inviter.ID)

	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, 4, counter, "equipo + rol caller + pendientes + users IN (antes: +2 users por invitación)")
	assert.Equal(t, "Nora", resp[0].InviteeName)
	assert.Equal(t, "Omar", resp[0].InviterName)
	assert.Equal(t, "Team batchread eq", resp[0].TeamName)
}

// Degrade igual al FindByID puntual: team irrelevable e inviter inexistente no
// rompen el listado — team_name/inviter_name vacíos, fee 0, can_receive false;
// el invitee se resuelve igual.
func TestInvitationService_ListPendingInvitationsForUser_TeamEInviterNoResueltos(t *testing.T) {
	db := testutils.SetupTestDB(t)
	invitee := batchreadTestUser(t, db, "Pia", "batchread-pia@test.com", "55000206")

	invDao := daos.NewInvitationDao(db)
	inv := dbs.Invitation{TeamID: 987654321, InviterID: 987654322, InviteeID: invitee.ID, Status: "pending", ExpiresAt: time.Now().Add(24 * time.Hour)}
	require.NoError(t, invDao.Create(nil, &inv))

	counter := 0
	cdb := countingDB(t, db, &counter)
	svc := NewInvitationService(daos.NewUserDao(cdb), daos.NewTeamDao(cdb), daos.NewInvitationDao(cdb), daos.NewTeamUserDao(cdb), nil, nil, nil, nil, nil, nil, cdb, &mockSellerConnectionDao{})

	resp, err := svc.ListPendingInvitationsForUser(nil, invitee.ID)

	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, "", resp[0].TeamName)
	assert.Equal(t, "", resp[0].InviterName)
	assert.Equal(t, 0.0, resp[0].MembershipFee)
	assert.False(t, resp[0].CanReceivePayments)
	assert.Equal(t, "Pia", resp[0].InviteeName)
	assert.Equal(t, "batchread-pia@test.com", resp[0].InviteeEmail)
	assert.Equal(t, 3, counter, "pendientes + teams IN + users IN team fantasma incluido")
}
