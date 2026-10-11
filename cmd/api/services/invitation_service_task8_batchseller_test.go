package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Task 8: batch can_receive — 1 query de connections authorized para TODOS
// los owners del listado; quien tiene conexión authorized con public_key
// recibe true, el resto false. Sin N+1 por invitación.
func TestInvitationService_ListPendingInvitationsForUser_CanReceiveBatch(t *testing.T) {
	db := testutils.SetupTestDB(t)
	invitee := batchreadTestUser(t, db, "Uve", "batch-seller-uve@test.com", "55000207")
	ownerA := batchreadTestUser(t, db, "Wally", "batch-seller-wally@test.com", "55000208")
	ownerB := batchreadTestUser(t, db, "Xavi", "batch-seller-xavi@test.com", "55000209")
	teamA := &dbs.Team{Name: "Team seller A", MaxMembers: 10, OwnerID: ownerA.ID}
	require.NoError(t, db.Create(teamA).Error)
	teamB := &dbs.Team{Name: "Team seller B", MaxMembers: 10, OwnerID: ownerB.ID}
	require.NoError(t, db.Create(teamB).Error)

	var batchSeenOwners []int64
	seller := &mockSellerConnectionDao{
		findByUserAndClFn: func(ctx *gin.Context, userID int64, clientID string) (*dbs.SellerConnection, error) {
			t.Error("el listado no debe tirar un FindByUserAndClient por invitación (N+1)")
			return nil, nil
		},
		findAuthorizedByIDsFn: func(ctx *gin.Context, userIDs []int64, clientID string) ([]dbs.SellerConnection, error) {
			batchSeenOwners = append(batchSeenOwners, userIDs...)
			return []dbs.SellerConnection{
				{UserID: ownerA.ID, Status: string(constants.SellerConnectionStatusAuthorized), PublicKey: "PK_A"},
			}, nil
		},
	}

	invDao := daos.NewInvitationDao(db)
	for _, inv := range []dbs.Invitation{
		{TeamID: teamA.ID, InviterID: ownerA.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: time.Now().Add(24 * time.Hour)},
		{TeamID: teamB.ID, InviterID: ownerB.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: time.Now().Add(24 * time.Hour)},
	} {
		require.NoError(t, invDao.Create(nil, &inv))
	}

	svc := NewInvitationService(daos.NewUserDao(db), daos.NewTeamDao(db), daos.NewInvitationDao(db), daos.NewTeamUserDao(db), nil, nil, nil, nil, nil, nil, db, seller)

	resp, err := svc.ListPendingInvitationsForUser(nil, invitee.ID)

	require.NoError(t, err)
	require.Len(t, resp, 2)
	assert.True(t, resp[0].CanReceivePayments, "owner con conexión authorized + public_key → true")
	assert.False(t, resp[1].CanReceivePayments, "owner sin conexión → false, el resto del listado no se rompe")
	assert.ElementsMatch(t, []int64{ownerA.ID, ownerB.ID}, batchSeenOwners, "los dos owners entran juntos a la misma query")
}

// Fallo honesto de la query batch: el listado sigue vivo y can_receive queda
// false para todas las filas (misma degradación que el FindByID puntual).
func TestInvitationService_ListPendingInvitationsForUser_SellerBatchErrorDegradaAFalse(t *testing.T) {
	db := testutils.SetupTestDB(t)
	invitee := batchreadTestUser(t, db, "Yana", "batch-seller-yana@test.com", "55000210")
	owner := batchreadTestUser(t, db, "Zeta", "batch-seller-zeta@test.com", "55000211")
	team := &dbs.Team{Name: "Team seller err", MaxMembers: 10, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)

	seller := &mockSellerConnectionDao{findAuthorizedByIDsFn: func(ctx *gin.Context, userIDs []int64, clientID string) ([]dbs.SellerConnection, error) {
		return nil, errors.New("boom seller batch")
	}}
	invDao := daos.NewInvitationDao(db)
	inv := dbs.Invitation{TeamID: team.ID, InviterID: owner.ID, InviteeID: invitee.ID, Status: "pending", ExpiresAt: time.Now().Add(24 * time.Hour)}
	require.NoError(t, invDao.Create(nil, &inv))

	svc := NewInvitationService(daos.NewUserDao(db), daos.NewTeamDao(db), daos.NewInvitationDao(db), daos.NewTeamUserDao(db), nil, nil, nil, nil, nil, nil, db, seller)

	resp, err := svc.ListPendingInvitationsForUser(nil, invitee.ID)

	require.NoError(t, err, "el fallo de la conexión del owner no rompe el listado")
	require.Len(t, resp, 1)
	assert.False(t, resp[0].CanReceivePayments)
	assert.Equal(t, "Team seller err", resp[0].TeamName)
}
