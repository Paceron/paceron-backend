package daos

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewTeamDao(t *testing.T) {
	dao := NewTeamDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTeamDao_ImplementsInterface(t *testing.T) {
	dao := NewTeamDao(&gorm.DB{})
	var iface TeamDaoInterface = dao
	_ = iface
}

func TestTeamDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTeamDao(&gorm.DB{})
	})
}

// persistUser crea y guarda un usuario válido directo por GORM (bypass del DAO),
// pensado como fixture para tests de otros DAOs que necesitan un user_id real.
func persistUser(db *gorm.DB, email, dni string) *dbs.User {
	user := testUser(email, dni)
	db.Create(user)
	return user
}

func testTeam(db *gorm.DB, name string, ownerID int64) *dbs.Team {
	team := &dbs.Team{Name: name, MaxMembers: 20, OwnerID: ownerID}
	db.Create(team)
	return team
}

func TestTeamDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-create-owner@test.com", "20000001")

	team := &dbs.Team{Name: "equipo_test", MaxMembers: 20, OwnerID: owner.ID}
	err := dao.Create(nil, team)

	require.NoError(t, err)
	assert.NotZero(t, team.ID)
}

func TestTeamDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-findid-owner@test.com", "20000002")
	team := testTeam(db, "equipo_findid_test", owner.ID)

	found, err := dao.FindByID(nil, team.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "equipo_findid_test", found.Name)
}

func TestTeamDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTeamDao_FindByID_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-deleted-owner@test.com", "20000003")
	team := testTeam(db, "equipo_borrado_test", owner.ID)
	require.NoError(t, dao.SoftDelete(nil, team.ID))

	found, err := dao.FindByID(nil, team.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestTeamDao_GetAll_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-getall-owner@test.com", "20000004")
	active := testTeam(db, "equipo_activo_test", owner.ID)
	deleted := testTeam(db, "equipo_eliminado_test", owner.ID)
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	all, err := dao.GetAll(nil)

	require.NoError(t, err)
	ids := make([]int64, 0, len(all))
	for _, tm := range all {
		ids = append(ids, tm.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestTeamDao_GetAllByOwnerID_FiltersCorrectly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner1 := persistUser(db, "team-owner1@test.com", "20000005")
	owner2 := persistUser(db, "team-owner2@test.com", "20000006")
	own := testTeam(db, "equipo_owner1_test", owner1.ID)
	testTeam(db, "equipo_owner2_test", owner2.ID)

	found, err := dao.GetAllByOwnerID(nil, owner1.ID)

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, own.ID, found[0].ID)
}

func TestTeamDao_GetAllByMemberID_JoinsTeamUsers(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-member-owner@test.com", "20000007")
	member := persistUser(db, "team-member-user@test.com", "20000008")
	team := testTeam(db, "equipo_member_test", owner.ID)

	teamUser := &dbs.TeamUser{TeamID: team.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: team.CreatedAt}
	require.NoError(t, db.Create(teamUser).Error)

	found, err := dao.GetAllByMemberID(nil, member.ID)

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, team.ID, found[0].ID)
}

func TestTeamDao_GetAllByMemberID_ExcludesSoftDeletedMembership(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-member2-owner@test.com", "20000009")
	member := persistUser(db, "team-member2-user@test.com", "20000010")
	team := testTeam(db, "equipo_member2_test", owner.ID)

	teamUser := &dbs.TeamUser{TeamID: team.ID, UserID: member.ID, RoleInTeam: "corredor", AssignmentDate: team.CreatedAt}
	require.NoError(t, db.Create(teamUser).Error)
	require.NoError(t, db.Model(&dbs.TeamUser{}).Where("id = ?", teamUser.ID).Update("deleted_at", gorm.Expr("NOW()")).Error)

	found, err := dao.GetAllByMemberID(nil, member.ID)

	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestTeamDao_Update_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-update-owner@test.com", "20000011")
	team := testTeam(db, "equipo_original_test", owner.ID)

	team.Description = "actualizado"
	require.NoError(t, dao.Update(nil, team))

	found, err := dao.FindByID(nil, team.ID)
	require.NoError(t, err)
	assert.Equal(t, "actualizado", found.Description)
}

func TestTeamDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-softdelete-owner@test.com", "20000012")
	team := testTeam(db, "equipo_a_borrar_test", owner.ID)

	err := dao.SoftDelete(nil, team.ID)

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, team.ID)
	assert.Nil(t, found)
}

func TestTeamDao_UpdateIcon_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-updateicon-owner@test.com", "20000013")
	team := testTeam(db, "equipo_icono_test", owner.ID)
	updatedAt := time.Now().UTC().Truncate(time.Second)
	key := "teams/team-icon.png"

	err := dao.UpdateIcon(nil, team.ID, key, updatedAt)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, team.ID)
	require.NoError(t, findErr)
	require.NotNil(t, found.IconKey)
	assert.Equal(t, key, *found.IconKey)
	require.NotNil(t, found.IconUpdatedAt)
	assert.WithinDuration(t, updatedAt, *found.IconUpdatedAt, time.Second)
}

func TestTeamDao_ClearIcon_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "team-clearicon-owner@test.com", "20000014")
	team := testTeam(db, "equipo_icono_borrar_test", owner.ID)
	require.NoError(t, dao.UpdateIcon(nil, team.ID, "teams/team-icon.png", time.Now()))

	err := dao.ClearIcon(nil, team.ID)

	require.NoError(t, err)
	found, findErr := dao.FindByID(nil, team.ID)
	require.NoError(t, findErr)
	assert.Nil(t, found.IconKey)
	assert.Nil(t, found.IconUpdatedAt)
}

func TestTeamDao_SearchPublic_ExcludesInvisible(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "search-owner-1@test.com", "50000001")
	caller := persistUser(db, "search-caller-1@test.com", "50000002")
	visible := testTeam(db, "equipo_visible_1", owner.ID)
	require.NoError(t, dao.Update(nil, visible))
	invisible := testTeam(db, "equipo_invisible_1", owner.ID)
	invisible.Visible = false
	require.NoError(t, dao.Update(nil, invisible))

	results, hasMore, err := dao.SearchPublic(nil, TeamSearchFilters{}, caller.ID, 1, 20)

	require.NoError(t, err)
	assert.False(t, hasMore)
	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.Name
	}
	assert.Contains(t, names, "equipo_visible_1")
	assert.NotContains(t, names, "equipo_invisible_1")
}

func TestTeamDao_SearchPublic_ExcludesCallerMembership(t *testing.T) {
	db := testutils.SetupTestDB(t)
	teamDao := NewTeamDao(db)
	teamUserDao := NewTeamUserDao(db)
	owner := persistUser(db, "search-owner-2@test.com", "50000003")
	caller := persistUser(db, "search-caller-2@test.com", "50000004")
	team := testTeam(db, "equipo_member_test", owner.ID)
	require.NoError(t, teamUserDao.Create(nil, &dbs.TeamUser{TeamID: team.ID, UserID: caller.ID, RoleInTeam: "corredor", AssignmentDate: time.Now()}))

	results, _, err := teamDao.SearchPublic(nil, TeamSearchFilters{}, caller.ID, 1, 20)

	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, team.ID, r.ID)
	}
}

func TestTeamDao_SearchPublic_FiltersByName(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "search-owner-3@test.com", "50000005")
	caller := persistUser(db, "search-caller-3@test.com", "50000006")
	testTeam(db, "runners_unicos_del_sur", owner.ID)
	testTeam(db, "otro_equipo_cualquiera", owner.ID)

	results, _, err := dao.SearchPublic(nil, TeamSearchFilters{Name: "unicos"}, caller.ID, 1, 20)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "runners_unicos_del_sur", results[0].Name)
}

func TestTeamDao_SearchPublic_HasMore(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewTeamDao(db)
	owner := persistUser(db, "search-owner-4@test.com", "50000007")
	caller := persistUser(db, "search-caller-4@test.com", "50000008")
	for i := 0; i < 3; i++ {
		testTeam(db, fmt.Sprintf("equipo_paginado_%d", i), owner.ID)
	}

	results, hasMore, err := dao.SearchPublic(nil, TeamSearchFilters{}, caller.ID, 1, 2)

	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.True(t, hasMore)
}
