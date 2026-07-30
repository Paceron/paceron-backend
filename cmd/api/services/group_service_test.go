package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/group"
)

type mockTeamUserDaoGroup struct {
	findByTeamAndUserFn func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error)
}

func (m *mockTeamUserDaoGroup) Create(ctx *gin.Context, tu *dbs.TeamUser) error { return nil }
func (m *mockTeamUserDaoGroup) FindByTeamAndUser(ctx *gin.Context, t, u int64) (*dbs.TeamUser, error) {
	if m.findByTeamAndUserFn != nil {
		return m.findByTeamAndUserFn(ctx, t, u)
	}
	return nil, nil
}
func (m *mockTeamUserDaoGroup) FindByTeamID(ctx *gin.Context, t int64) ([]dbs.TeamUser, error) {
	return nil, nil
}
func (m *mockTeamUserDaoGroup) FindByUserID(ctx *gin.Context, u int64) ([]dbs.TeamUser, error) {
	return nil, nil
}
func (m *mockTeamUserDaoGroup) CountActiveByTeam(ctx *gin.Context, t int64) (int64, error) {
	return 0, nil
}
func (m *mockTeamUserDaoGroup) CountActiveByTeamExcludingUser(ctx *gin.Context, t, u int64) (int64, error) {
	return 0, nil
}
func (m *mockTeamUserDaoGroup) HasOwnerByTeam(ctx *gin.Context, t int64) (bool, error) {
	return false, nil
}
func (m *mockTeamUserDaoGroup) SoftDelete(ctx *gin.Context, id int64) error { return nil }
func (m *mockTeamUserDaoGroup) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	return nil
}

type mockGroupDao struct {
	createFn             func(ctx *gin.Context, g *dbs.Group) error
	findByIDFn           func(ctx *gin.Context, id int64) (*dbs.Group, error)
	findByIDAndTeamIDFn  func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error)
	getAllFn             func(ctx *gin.Context) ([]dbs.Group, error)
	getByTeamIDFn        func(ctx *gin.Context, teamID int64) ([]dbs.Group, error)
	updateFn             func(ctx *gin.Context, g *dbs.Group) error
	softDeleteFn         func(ctx *gin.Context, id int64) error
	softDeleteByTeamIDFn func(ctx *gin.Context, teamID int64) error
}

func (m *mockGroupDao) Create(ctx *gin.Context, g *dbs.Group) error {
	if m.createFn != nil {
		return m.createFn(ctx, g)
	}
	return nil
}

func (m *mockGroupDao) FindByID(ctx *gin.Context, id int64) (*dbs.Group, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockGroupDao) FindByIDAndTeamID(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
	if m.findByIDAndTeamIDFn != nil {
		return m.findByIDAndTeamIDFn(ctx, groupID, teamID)
	}
	return nil, nil
}

func (m *mockGroupDao) GetAll(ctx *gin.Context) ([]dbs.Group, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return nil, nil
}

func (m *mockGroupDao) GetByTeamID(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
	if m.getByTeamIDFn != nil {
		return m.getByTeamIDFn(ctx, teamID)
	}
	return nil, nil
}

func (m *mockGroupDao) Update(ctx *gin.Context, g *dbs.Group) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, g)
	}
	return nil
}

func (m *mockGroupDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	if m.softDeleteByTeamIDFn != nil {
		return m.softDeleteByTeamIDFn(ctx, teamID)
	}
	return nil
}

func (m *mockGroupDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestGroupService_Create_Success(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		createFn: func(ctx *gin.Context, g *dbs.Group) error {
			g.ID = 1
			return nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, Name: "Alpha"}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, mockTeamDao, &mockTeamUserDaoGroup{})
	resp, err := svc.Create(nil, &group.CreateGroupRequest{
		Name:   "Grupo 1",
		TeamID: 1,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Grupo 1", resp.Name)
	assert.Equal(t, int64(1), resp.TeamID)
}

func TestGroupService_Create_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewGroupService(&mockGroupDao{}, mockTeamDao, &mockTeamUserDaoGroup{})
	_, err := svc.Create(nil, &group.CreateGroupRequest{
		Name:   "Grupo 1",
		TeamID: 999,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el equipo no existe")
}

func TestGroupService_Update_Success(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, Name: "Old"}, nil
		},
		updateFn: func(ctx *gin.Context, g *dbs.Group) error {
			return nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	newName := "New"
	resp, err := svc.Update(nil, 1, &group.UpdateGroupRequest{
		Name: &newName,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "New", resp.Name)
}

func TestGroupService_Update_GroupNotFound(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	_, err := svc.Update(nil, 999, &group.UpdateGroupRequest{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grupo no encontrado")
}

func TestGroupService_Delete_Success(t *testing.T) {
	softDeleteCalled := false
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, TeamID: 1}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	mockTU := &mockTeamUserDaoGroup{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 1, RoleInTeam: "entrenador"}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, mockTU)
	err := svc.Delete(nil, 1, 1)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
}

func TestGroupService_Delete_GroupNotFound(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	err := svc.Delete(nil, 999, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grupo no encontrado")
}

func TestGroupService_Delete_NotMember(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, TeamID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDaoGroup{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, mockTU)
	err := svc.Delete(nil, 1, 99)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no pertenece al equipo de este grupo")
}

func TestGroupService_Delete_NotEntrenador(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, TeamID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDaoGroup{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 2, RoleInTeam: "corredor"}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, mockTU)
	err := svc.Delete(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "solo el entrenador puede eliminar el grupo")
}

func TestGroupService_GetByID_Success(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, Name: "Grupo 1", TeamID: 1}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	resp, err := svc.GetByID(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Grupo 1", resp.Name)
}

func TestGroupService_GetByID_GroupNotFound(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	_, err := svc.GetByID(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grupo no encontrado")
}

func TestGroupService_GetAll_Success(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Group, error) {
			return []dbs.Group{
				{ID: 1, Name: "Grupo A"},
				{ID: 2, Name: "Grupo B"},
			}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	resp, err := svc.GetAll(nil, nil, nil)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
}

func TestGroupService_Create_DAOError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		createFn: func(ctx *gin.Context, g *dbs.Group) error {
			return errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, mockTeamDao, &mockTeamUserDaoGroup{})
	_, err := svc.Create(nil, &group.CreateGroupRequest{
		Name:   "Grupo 1",
		TeamID: 999,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear grupo")
}

func TestGroupService_Create_TeamFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupService(&mockGroupDao{}, mockTeamDao, &mockTeamUserDaoGroup{})
	_, err := svc.Create(nil, &group.CreateGroupRequest{
		Name:   "Grupo 1",
		TeamID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear grupo")
}

func TestGroupService_Update_FindByIDError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	newName := "New"
	_, err := svc.Update(nil, 1, &group.UpdateGroupRequest{Name: &newName})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar grupo")
}

func TestGroupService_Update_DAOUpdateError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, Name: "Old"}, nil
		},
		updateFn: func(ctx *gin.Context, g *dbs.Group) error {
			return errors.New("db error")
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	newName := "New"
	_, err := svc.Update(nil, 1, &group.UpdateGroupRequest{Name: &newName})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar grupo")
}

func TestGroupService_Delete_FindByIDError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	err := svc.Delete(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar grupo")
}

func TestGroupService_Delete_SoftDeleteError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, TeamID: 1}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}
	mockTU := &mockTeamUserDaoGroup{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 1, RoleInTeam: "entrenador"}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, mockTU)
	err := svc.Delete(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar grupo")
}

func TestGroupService_GetByID_FindByIDError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	_, err := svc.GetByID(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener grupo")
}

func TestGroupService_GetAll_GetAllError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Group, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	_, err := svc.GetAll(nil, nil, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener grupos")
}

func TestGroupService_Update_AllOptionalFields(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, Name: "Old", Description: "Old Desc", IsMain: false}, nil
		},
		updateFn: func(ctx *gin.Context, g *dbs.Group) error {
			return nil
		},
	}

	svc := NewGroupService(mockGroupDao, &mockTeamDao{}, &mockTeamUserDaoGroup{})
	newName := "New Name"
	newDesc := "New Desc"
	newIsMain := true
	resp, err := svc.Update(nil, 1, &group.UpdateGroupRequest{
		Name:        &newName,
		Description: &newDesc,
		IsMain:      &newIsMain,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "New Name", resp.Name)
	assert.Equal(t, "New Desc", resp.Description)
	assert.Equal(t, true, resp.IsMain)
}

func TestGroupService_GetAll_WithTeamID_MemberExists(t *testing.T) {
	teamID := int64(1)
	userID := int64(10)
	mockGroupDao := &mockGroupDao{
		getByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.Group, error) {
			return []dbs.Group{{ID: 1, Name: "Grupo A"}}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDaoGroup{
		findByTeamAndUserFn: func(ctx *gin.Context, t, u int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{TeamID: 1, UserID: 10, RoleInTeam: "corredor"}, nil
		},
	}

	svc := NewGroupService(mockGroupDao, mockTeamDao, mockTU)
	resp, err := svc.GetAll(nil, &teamID, &userID)

	assert.NoError(t, err)
	assert.Len(t, resp, 1)
}

func TestGroupService_GetAll_WithTeamID_MemberNotFound(t *testing.T) {
	teamID := int64(1)
	userID := int64(99)
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDaoGroup{
		findByTeamAndUserFn: func(ctx *gin.Context, t, u int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}

	svc := NewGroupService(&mockGroupDao{}, mockTeamDao, mockTU)
	_, err := svc.GetAll(nil, &teamID, &userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no pertenece a este equipo")
}

func TestGroupService_GetAll_WithTeamID_TeamNotFound(t *testing.T) {
	teamID := int64(999)
	userID := int64(10)
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewGroupService(&mockGroupDao{}, mockTeamDao, &mockTeamUserDaoGroup{})
	_, err := svc.GetAll(nil, &teamID, &userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestGroupService_GetAll_WithTeamID_MembershipDAOError(t *testing.T) {
	teamID := int64(1)
	userID := int64(10)
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}
	mockTU := &mockTeamUserDaoGroup{
		findByTeamAndUserFn: func(ctx *gin.Context, t, u int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupService(&mockGroupDao{}, mockTeamDao, mockTU)
	_, err := svc.GetAll(nil, &teamID, &userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener grupos")
}
