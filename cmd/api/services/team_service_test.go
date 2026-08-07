package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/team"
)

// entrenadorMockTeamUserDao devuelve un mockTeamUserDao que reporta a cualquier
// (team, user) como miembro entrenador — atajo para tests que no ejercitan la
// autorización en sí, solo necesitan que el chequeo de caller pase.
func entrenadorMockTeamUserDao() *mockTeamUserDao {
	return &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: "entrenador"}, nil
		},
	}
}

type mockTeamDao struct {
	createFn           func(ctx *gin.Context, t *dbs.Team) error
	findByIDFn         func(ctx *gin.Context, id int64) (*dbs.Team, error)
	getAllFn           func(ctx *gin.Context) ([]dbs.Team, error)
	getAllByOwnerIDFn  func(ctx *gin.Context, ownerID int64) ([]dbs.Team, error)
	getAllByMemberIDFn func(ctx *gin.Context, memberID int64) ([]dbs.Team, error)
	updateFn           func(ctx *gin.Context, t *dbs.Team) error
	softDeleteFn       func(ctx *gin.Context, id int64) error
}

func (m *mockTeamDao) Create(ctx *gin.Context, t *dbs.Team) error {
	if m.createFn != nil {
		return m.createFn(ctx, t)
	}
	return nil
}

func (m *mockTeamDao) FindByID(ctx *gin.Context, id int64) (*dbs.Team, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockTeamDao) GetAll(ctx *gin.Context) ([]dbs.Team, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return nil, nil
}

func (m *mockTeamDao) GetAllByOwnerID(ctx *gin.Context, ownerID int64) ([]dbs.Team, error) {
	if m.getAllByOwnerIDFn != nil {
		return m.getAllByOwnerIDFn(ctx, ownerID)
	}
	return nil, nil
}

func (m *mockTeamDao) GetAllByMemberID(ctx *gin.Context, memberID int64) ([]dbs.Team, error) {
	if m.getAllByMemberIDFn != nil {
		return m.getAllByMemberIDFn(ctx, memberID)
	}
	return nil, nil
}

func (m *mockTeamDao) Update(ctx *gin.Context, t *dbs.Team) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, t)
	}
	return nil
}

func (m *mockTeamDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestTeamService_Create_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		createFn: func(ctx *gin.Context, t *dbs.Team) error {
			t.ID = 1
			return nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Coach"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}

	teamUserCreateCalled := false
	mockTeamUserDao := &mockTeamUserDao{
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			teamUserCreateCalled = true
			assert.Equal(t, int64(1), tu.TeamID)
			assert.Equal(t, int64(1), tu.UserID)
			assert.Equal(t, "entrenador", tu.RoleInTeam)
			assert.Equal(t, "active", tu.Status)
			return nil
		},
	}

	svc := NewTeamService(mockTeamDao, mockUserDao, mockUserRoleDao, mockRoleDao, mockTeamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	resp, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Equipo Alpha", resp.Name)
	assert.Equal(t, int64(20), resp.MaxMembers)
	assert.Equal(t, int64(1), resp.OwnerID)
	assert.Equal(t, "active", resp.Status)
	assert.False(t, resp.ShowGroupsToRunners)
	assert.True(t, teamUserCreateCalled)
}

func TestTeamService_Create_ShowGroupsToRunners_True(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		createFn: func(ctx *gin.Context, t *dbs.Team) error {
			t.ID = 1
			return nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Coach"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, mockUserDao, mockUserRoleDao, mockRoleDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	showGroups := true
	resp, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:                "Equipo Alpha",
		MaxMembers:          20,
		ShowGroupsToRunners: &showGroups,
	})

	assert.NoError(t, err)
	assert.True(t, resp.ShowGroupsToRunners)
}

func TestTeamService_Create_TeamUserDaoCreateError_StillSucceeds(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		createFn: func(ctx *gin.Context, t *dbs.Team) error {
			t.ID = 1
			return nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Coach"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			return errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, mockUserDao, mockUserRoleDao, mockRoleDao, mockTeamUserDao, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	resp, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestTeamService_Create_OwnerNotFound(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewTeamService(&mockTeamDao{}, mockUserDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Create(nil, 999, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario owner no existe")
}

func TestTeamService_Create_OwnerNoEntrenadorRole(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "User"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewTeamService(&mockTeamDao{}, mockUserDao, mockUserRoleDao, mockRoleDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el owner debe tener el rol 'entrenador'")
}

func TestTeamService_Create_UserFindByIDError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(&mockTeamDao{}, mockUserDao, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear equipo")
}

func TestTeamService_Create_DAOError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		createFn: func(ctx *gin.Context, t *dbs.Team) error {
			return errors.New("db error")
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Coach"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, mockUserDao, mockUserRoleDao, mockRoleDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear equipo")
}

func TestTeamService_Update_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Old Name", Status: "active"}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Team) error {
			return nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, entrenadorMockTeamUserDao(), &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	newName := "New Name"
	resp, err := svc.Update(nil, 1, 1, &team.UpdateTeamRequest{
		Name: &newName,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "New Name", resp.Name)
}

func TestTeamService_Update_ShowGroupsToRunners(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Old Name", Status: "active", ShowGroupsToRunners: false}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Team) error {
			return nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, entrenadorMockTeamUserDao(), &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	showGroups := true
	resp, err := svc.Update(nil, 1, 1, &team.UpdateTeamRequest{
		ShowGroupsToRunners: &showGroups,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.ShowGroupsToRunners)
}

func TestTeamService_Update_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Update(nil, 999, 1, &team.UpdateTeamRequest{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamService_Update_NotEntrenador(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Old Name"}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: "corredor"}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	newName := "New Name"
	_, err := svc.Update(nil, 1, 2, &team.UpdateTeamRequest{Name: &newName})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede actualizar el equipo")
}

func TestTeamService_UpdateAddress_NotEntrenador(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: teamID, UserID: userID, RoleInTeam: "corredor"}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.UpdateAddress(nil, 1, 2, &team.UpdateTeamAddressRequest{Country: "Argentina"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede actualizar el equipo")
}

func TestTeamService_Delete_Success(t *testing.T) {
	softDeleteCalled := false
	cascadeTeamUsersCalled := false
	cascadeGroupsCalled := false
	cascadeGroupUsersCalled := false
	cascadeInvitationsCalled := false
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 1, RoleInTeam: "entrenador"}, nil
		},
		countActiveByTeamExcludingUserFn: func(ctx *gin.Context, teamID, excludeUserID int64) (int64, error) {
			return 0, nil
		},
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			cascadeTeamUsersCalled = true
			assert.Equal(t, int64(1), teamID)
			return nil
		},
	}
	mockGroup := &mockGroupDao{
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			cascadeGroupsCalled = true
			return nil
		},
	}
	mockGroupUser := &mockGroupUserDao{
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			cascadeGroupUsersCalled = true
			return nil
		},
	}
	mockInvitation := &mockInvitationDao{
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			cascadeInvitationsCalled = true
			return nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, mockGroup, mockGroupUser, mockInvitation)
	err := svc.Delete(nil, 1, 1)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
	assert.True(t, cascadeTeamUsersCalled)
	assert.True(t, cascadeGroupsCalled)
	assert.True(t, cascadeGroupUsersCalled)
	assert.True(t, cascadeInvitationsCalled)
}

func TestTeamService_Delete_CascadeErrors_StillSucceeds(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 1, RoleInTeam: "entrenador"}, nil
		},
		countActiveByTeamExcludingUserFn: func(ctx *gin.Context, teamID, excludeUserID int64) (int64, error) {
			return 0, nil
		},
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			return errors.New("db error")
		},
	}
	mockGroup := &mockGroupDao{
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			return errors.New("db error")
		},
	}
	mockGroupUser := &mockGroupUserDao{
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			return errors.New("db error")
		},
	}
	mockInvitation := &mockInvitationDao{
		softDeleteByTeamIDFn: func(ctx *gin.Context, teamID int64) error {
			return errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, mockGroup, mockGroupUser, mockInvitation)
	err := svc.Delete(nil, 1, 1)

	assert.NoError(t, err)
}

func TestTeamService_Delete_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	err := svc.Delete(nil, 999, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamService_Delete_NotMember(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	err := svc.Delete(nil, 1, 99)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no pertenece a este equipo")
}

func TestTeamService_Delete_NotEntrenador(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 2, RoleInTeam: "corredor"}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	err := svc.Delete(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede eliminar el equipo")
}

func TestTeamService_Delete_HasMembers(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 1, RoleInTeam: "entrenador"}, nil
		},
		countActiveByTeamExcludingUserFn: func(ctx *gin.Context, teamID, excludeUserID int64) (int64, error) {
			return 3, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	err := svc.Delete(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se puede eliminar un equipo con miembros activos")
}

func TestTeamService_GetByID_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Equipo Alpha", Status: "active", MaxMembers: 20}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	resp, err := svc.GetByID(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Equipo Alpha", resp.Name)
}

func TestTeamService_GetByID_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.GetByID(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamService_GetAll_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Team, error) {
			return []dbs.Team{
				{ID: 1, Name: "Alpha"},
				{ID: 2, Name: "Beta"},
			}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	resp, err := svc.GetAll(nil, nil, nil)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "Alpha", resp[0].Name)
	assert.Equal(t, "Beta", resp[1].Name)
}

func TestTeamService_UpdateAddress_Success(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Team) error {
			return nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, entrenadorMockTeamUserDao(), &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	resp, err := svc.UpdateAddress(nil, 1, 1, &team.UpdateTeamAddressRequest{
		Country:  "Argentina",
		Province: "Córdoba",
		City:     "Córdoba",
		Street:   "Av. General Paz",
		Number:   "1234",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Argentina", resp.Country)
	assert.Equal(t, "Córdoba", resp.Province)
}

func TestTeamService_UpdateAddress_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.UpdateAddress(nil, 999, 1, &team.UpdateTeamAddressRequest{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamService_Create_UserRoleFindByIDError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Coach"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(&mockTeamDao{}, mockUserDao, mockUserRoleDao, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear equipo")
}

func TestTeamService_Update_FindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	newName := "New"
	_, err := svc.Update(nil, 1, 1, &team.UpdateTeamRequest{Name: &newName})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar equipo")
}

func TestTeamService_Update_CallerRoleCheckError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Old"}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	newName := "New"
	_, err := svc.Update(nil, 1, 1, &team.UpdateTeamRequest{Name: &newName})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar equipo")
}

func TestTeamService_UpdateAddress_CallerRoleCheckError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.UpdateAddress(nil, 1, 1, &team.UpdateTeamAddressRequest{Country: "Argentina"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar dirección")
}

func TestTeamService_Update_DAOUpdateError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Old"}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Team) error {
			return errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, entrenadorMockTeamUserDao(), &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	newName := "New"
	_, err := svc.Update(nil, 1, 1, &team.UpdateTeamRequest{Name: &newName})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar equipo")
}

func TestTeamService_Delete_FindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	err := svc.Delete(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar equipo")
}

func TestTeamService_Delete_SoftDeleteError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 1, RoleInTeam: "entrenador"}, nil
		},
		countActiveByTeamExcludingUserFn: func(ctx *gin.Context, teamID, excludeUserID int64) (int64, error) {
			return 0, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	err := svc.Delete(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar equipo")
}

func TestTeamService_GetByID_FindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.GetByID(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener equipo")
}

func TestTeamService_GetAll_GetAllError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.GetAll(nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener equipos")
}

func TestTeamService_GetAll_ByOwnerID(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		getAllByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Team, error) {
			assert.Equal(t, int64(5), ownerID)
			return []dbs.Team{{ID: 1, Name: "Alpha", OwnerID: 5}}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	ownerID := int64(5)
	resp, err := svc.GetAll(nil, &ownerID, nil)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Alpha", resp[0].Name)
}

func TestTeamService_GetAll_ByMemberID(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		getAllByMemberIDFn: func(ctx *gin.Context, memberID int64) ([]dbs.Team, error) {
			assert.Equal(t, int64(7), memberID)
			return []dbs.Team{{ID: 2, Name: "Beta"}}, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	memberID := int64(7)
	resp, err := svc.GetAll(nil, nil, &memberID)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Beta", resp[0].Name)
}

func TestTeamService_GetAll_ByOwnerIDAndMemberID_FiltersInMemory(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		getAllByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Team, error) {
			return []dbs.Team{{ID: 1, Name: "Alpha"}, {ID: 2, Name: "Beta"}}, nil
		},
	}
	mockTU := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			if teamID == 1 {
				return &dbs.TeamUser{TeamID: 1, UserID: userID}, nil
			}
			return nil, nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, mockTU, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	ownerID := int64(5)
	memberID := int64(7)
	resp, err := svc.GetAll(nil, &ownerID, &memberID)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Equal(t, "Alpha", resp[0].Name)
}

func TestTeamService_GetAll_ByMemberID_DaoError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		getAllByMemberIDFn: func(ctx *gin.Context, memberID int64) ([]dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	memberID := int64(7)
	_, err := svc.GetAll(nil, nil, &memberID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener equipos")
}

func TestTeamService_UpdateAddress_FindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.UpdateAddress(nil, 1, 1, &team.UpdateTeamAddressRequest{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar dirección")
}

func TestTeamService_UpdateAddress_DAOUpdateError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Team) error {
			return errors.New("db error")
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, entrenadorMockTeamUserDao(), &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.UpdateAddress(nil, 1, 1, &team.UpdateTeamAddressRequest{
		Country: "Argentina",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar dirección")
}

func TestTeamService_Create_RoleFindByIDError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Coach"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamService(&mockTeamDao{}, mockUserDao, mockUserRoleDao, mockRoleDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el owner debe tener el rol 'entrenador'")
}

func TestTeamService_Create_RoleNil(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "Coach"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewTeamService(&mockTeamDao{}, mockUserDao, mockUserRoleDao, mockRoleDao, &mockTeamUserDao{}, &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	_, err := svc.Create(nil, 1, &team.CreateTeamRequest{
		Name:       "Equipo Alpha",
		MaxMembers: 20,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el owner debe tener el rol 'entrenador'")
}

func TestTeamService_Update_AllOptionalFields(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Old", Description: "Old Desc", Level: "low", MaxMembers: 5, Requirements: "none"}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Team) error {
			return nil
		},
	}

	svc := NewTeamService(mockTeamDao, &mockUserDaoForUserRole{}, &mockUserRoleDao{}, &mockRoleDao{}, entrenadorMockTeamUserDao(), &mockGroupDao{}, &mockGroupUserDao{}, &mockInvitationDao{})
	newName := "New Name"
	newDesc := "New Desc"
	newLevel := "high"
	newMax := int64(20)
	newReqs := "some reqs"
	resp, err := svc.Update(nil, 1, 1, &team.UpdateTeamRequest{
		Name:         &newName,
		Description:  &newDesc,
		Level:        &newLevel,
		MaxMembers:   &newMax,
		Requirements: &newReqs,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "New Name", resp.Name)
	assert.Equal(t, "New Desc", resp.Description)
	assert.Equal(t, "high", resp.Level)
	assert.Equal(t, int64(20), resp.MaxMembers)
	assert.Equal(t, "some reqs", resp.Requirements)
}
