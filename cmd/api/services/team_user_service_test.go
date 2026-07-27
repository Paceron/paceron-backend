package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/teamuser"
)

type mockTeamUserDao struct {
	createFn          func(ctx *gin.Context, tu *dbs.TeamUser) error
	findByTeamAndUserFn func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error)
	findByTeamIDFn    func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error)
	findByUserIDFn    func(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error)
	countActiveByTeamFn func(ctx *gin.Context, teamID int64) (int64, error)
	hasOwnerByTeamFn  func(ctx *gin.Context, teamID int64) (bool, error)
	softDeleteFn      func(ctx *gin.Context, id int64) error
}

func (m *mockTeamUserDao) Create(ctx *gin.Context, tu *dbs.TeamUser) error {
	if m.createFn != nil {
		return m.createFn(ctx, tu)
	}
	return nil
}

func (m *mockTeamUserDao) FindByTeamAndUser(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
	if m.findByTeamAndUserFn != nil {
		return m.findByTeamAndUserFn(ctx, teamID, userID)
	}
	return nil, nil
}

func (m *mockTeamUserDao) FindByTeamID(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
	if m.findByTeamIDFn != nil {
		return m.findByTeamIDFn(ctx, teamID)
	}
	return nil, nil
}

func (m *mockTeamUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockTeamUserDao) CountActiveByTeam(ctx *gin.Context, teamID int64) (int64, error) {
	if m.countActiveByTeamFn != nil {
		return m.countActiveByTeamFn(ctx, teamID)
	}
	return 0, nil
}

func (m *mockTeamUserDao) HasOwnerByTeam(ctx *gin.Context, teamID int64) (bool, error) {
	if m.hasOwnerByTeamFn != nil {
		return m.hasOwnerByTeamFn(ctx, teamID)
	}
	return false, nil
}

func (m *mockTeamUserDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestTeamUserService_AddUser_Success(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			tu.ID = 1
			return nil
		},
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 2, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao)
	resp, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, "corredor", resp.RoleInTeam)
}

func TestTeamUserService_AddUser_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{})
	_, err := svc.AddUser(nil, 999, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamUserService_AddUser_InvalidRole(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 20}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "User"}, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "entrenador",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol inválido")
}

func TestTeamUserService_AddUser_UserNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     999,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestTeamUserService_AddUser_AlreadyMember(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario ya pertenece a este equipo")
}

func TestTeamUserService_AddUser_MaxMembersReached(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 10, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "máximo de 10 miembros")
}

func TestTeamUserService_RemoveUser_Success(t *testing.T) {
	softDeleteCalled := false
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
}

func TestTeamUserService_RemoveUser_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 999, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamUserService_RemoveUser_NotMember(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no pertenece a este equipo")
}

func TestTeamUserService_RemoveUser_CannotRemoveOwner(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "entrenador"}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se puede quitar al entrenador")
}

func TestTeamUserService_AddUser_TeamFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{})
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_AddUser_UserFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_AddUser_FindByTeamAndUserError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_AddUser_CreateError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 2, nil
		},
		createFn: func(ctx *gin.Context, tu *dbs.TeamUser) error {
			return errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_RemoveUser_TeamFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del equipo")
}

func TestTeamUserService_RemoveUser_FindByTeamAndUserError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del equipo")
}

func TestTeamUserService_RemoveUser_SoftDeleteError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return &dbs.TeamUser{ID: 1, RoleInTeam: "corredor"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del equipo")
}

func TestTeamUserService_AddUser_CountError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamAndUserFn: func(ctx *gin.Context, teamID, userID int64) (*dbs.TeamUser, error) {
			return nil, nil
		},
		countActiveByTeamFn: func(ctx *gin.Context, teamID int64) (int64, error) {
			return 0, errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1, MaxMembers: 10}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, &teamuser.AddTeamUserRequest{
		UserID:     1,
		RoleInTeam: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al equipo")
}

func TestTeamUserService_GetUsersByTeam_Success(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{
				{ID: 1, TeamID: 1, UserID: 10, RoleInTeam: "entrenador", Status: "active"},
				{ID: 2, TeamID: 1, UserID: 20, RoleInTeam: "corredor", Status: "active"},
			}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	resp, err := svc.GetUsersByTeam(nil, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, int64(10), resp[0].UserID)
	assert.Equal(t, "entrenador", resp[0].RoleInTeam)
}

func TestTeamUserService_GetUsersByTeam_TeamNotFound(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, nil
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{})
	_, err := svc.GetUsersByTeam(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "equipo no encontrado")
}

func TestTeamUserService_GetUsersByTeam_TeamFindByIDError(t *testing.T) {
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTeamUserService(&mockTeamUserDao{}, mockTeamDao, &mockUserDaoForUserRole{})
	_, err := svc.GetUsersByTeam(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener usuarios del equipo")
}

func TestTeamUserService_GetUsersByTeam_FindByTeamIDError(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	_, err := svc.GetUsersByTeam(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener usuarios del equipo")
}

func TestTeamUserService_GetUsersByTeam_Empty(t *testing.T) {
	mockTeamUserDao := &mockTeamUserDao{
		findByTeamIDFn: func(ctx *gin.Context, teamID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{}, nil
		},
	}
	mockTeamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			return &dbs.Team{ID: 1}, nil
		},
	}

	svc := NewTeamUserService(mockTeamUserDao, mockTeamDao, &mockUserDaoForUserRole{})
	resp, err := svc.GetUsersByTeam(nil, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 0)
}
