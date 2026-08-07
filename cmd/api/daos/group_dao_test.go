package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

func TestNewGroupDao(t *testing.T) {
	dao := NewGroupDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestGroupDao_ImplementsInterface(t *testing.T) {
	dao := NewGroupDao(&gorm.DB{})
	var iface GroupDaoInterface = dao
	_ = iface
}

func TestGroupDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewGroupDao(&gorm.DB{})
	})
}

func testGroup(db *gorm.DB, name string, teamID int64) *dbs.Group {
	group := &dbs.Group{Name: name, TeamID: teamID}
	db.Create(group)
	return group
}

func TestGroupDao_Create_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-create-owner@test.com", "40000001")
	team := testTeam(db, "equipo_group_create", owner.ID)

	group := &dbs.Group{Name: "grupo_test", TeamID: team.ID}
	err := dao.Create(nil, group)

	require.NoError(t, err)
	assert.NotZero(t, group.ID)
}

func TestGroupDao_FindByID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-findid-owner@test.com", "40000002")
	team := testTeam(db, "equipo_group_findid", owner.ID)
	group := testGroup(db, "grupo_findid_test", team.ID)

	found, err := dao.FindByID(nil, group.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "grupo_findid_test", found.Name)
}

func TestGroupDao_FindByID_NotFound(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)

	found, err := dao.FindByID(nil, 999999)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGroupDao_FindByIDAndTeamID_Found(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-findidteam-owner@test.com", "40000003")
	team := testTeam(db, "equipo_group_findidteam", owner.ID)
	group := testGroup(db, "grupo_findidteam_test", team.ID)

	found, err := dao.FindByIDAndTeamID(nil, group.ID, team.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, group.ID, found.ID)
}

func TestGroupDao_FindByIDAndTeamID_WrongTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-wrongteam-owner@test.com", "40000004")
	team := testTeam(db, "equipo_group_wrongteam", owner.ID)
	otherTeam := testTeam(db, "equipo_group_otherteam", owner.ID)
	group := testGroup(db, "grupo_wrongteam_test", team.ID)

	found, err := dao.FindByIDAndTeamID(nil, group.ID, otherTeam.ID)

	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestGroupDao_GetAll_ExcludesSoftDeleted(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-getall-owner@test.com", "40000005")
	team := testTeam(db, "equipo_group_getall", owner.ID)
	active := testGroup(db, "grupo_activo_test", team.ID)
	deleted := testGroup(db, "grupo_eliminado_test", team.ID)
	require.NoError(t, dao.SoftDelete(nil, deleted.ID))

	all, err := dao.GetAll(nil)

	require.NoError(t, err)
	ids := make([]int64, 0, len(all))
	for _, g := range all {
		ids = append(ids, g.ID)
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, deleted.ID)
}

func TestGroupDao_GetByTeamID_FiltersCorrectly(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-byteam-owner@test.com", "40000006")
	team1 := testTeam(db, "equipo_group_byteam1", owner.ID)
	team2 := testTeam(db, "equipo_group_byteam2", owner.ID)
	group1 := testGroup(db, "grupo_byteam1_test", team1.ID)
	testGroup(db, "grupo_byteam2_test", team2.ID)

	found, err := dao.GetByTeamID(nil, team1.ID)

	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, group1.ID, found[0].ID)
}

func TestGroupDao_Update_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-update-owner@test.com", "40000007")
	team := testTeam(db, "equipo_group_update", owner.ID)
	group := testGroup(db, "grupo_original_test", team.ID)

	group.Description = "actualizado"
	require.NoError(t, dao.Update(nil, group))

	found, err := dao.FindByID(nil, group.ID)
	require.NoError(t, err)
	assert.Equal(t, "actualizado", found.Description)
}

func TestGroupDao_SoftDelete_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-softdelete-owner@test.com", "40000008")
	team := testTeam(db, "equipo_group_softdelete", owner.ID)
	group := testGroup(db, "grupo_a_borrar_test", team.ID)

	err := dao.SoftDelete(nil, group.ID)

	require.NoError(t, err)
	found, _ := dao.FindByID(nil, group.ID)
	assert.Nil(t, found)
}

func TestGroupDao_SoftDeleteByTeamID_Success(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewGroupDao(db)
	owner := persistUser(db, "group-softdeleteteam-owner@test.com", "40000009")
	team := testTeam(db, "equipo_group_softdeleteteam", owner.ID)
	testGroup(db, "grupo_softdeleteteam1_test", team.ID)
	testGroup(db, "grupo_softdeleteteam2_test", team.ID)

	err := dao.SoftDeleteByTeamID(nil, team.ID)

	require.NoError(t, err)
	found, findErr := dao.GetByTeamID(nil, team.ID)
	require.NoError(t, findErr)
	assert.Empty(t, found)
}
