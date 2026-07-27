package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/groupuser"
)

type mockGroupUserDao struct {
	createFn            func(ctx *gin.Context, gu *dbs.GroupUser) error
	findByGroupAndUserFn func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error)
	findByGroupIDFn     func(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error)
	findByUserIDFn      func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error)
	softDeleteFn        func(ctx *gin.Context, id int64) error
}

func (m *mockGroupUserDao) Create(ctx *gin.Context, gu *dbs.GroupUser) error {
	if m.createFn != nil {
		return m.createFn(ctx, gu)
	}
	return nil
}

func (m *mockGroupUserDao) FindByGroupAndUser(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
	if m.findByGroupAndUserFn != nil {
		return m.findByGroupAndUserFn(ctx, groupID, userID)
	}
	return nil, nil
}

func (m *mockGroupUserDao) FindByGroupID(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error) {
	if m.findByGroupIDFn != nil {
		return m.findByGroupIDFn(ctx, groupID)
	}
	return nil, nil
}

func (m *mockGroupUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockGroupUserDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestGroupUserService_AddUser_Success(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			gu.ID = 1
			return nil
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, Name: "Grupo 1", TeamID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, mockUserDao)
	resp, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID: 1,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, int64(1), resp.GroupID)
}

func TestGroupUserService_AddUser_GroupNotFound(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return nil, nil
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, &mockUserDaoForUserRole{})
	_, err := svc.AddUser(nil, 1, 999, &groupuser.AddGroupUserRequest{
		UserID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grupo no encontrado en este equipo")
}

func TestGroupUserService_AddUser_UserNotFound(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID: 999,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestGroupUserService_AddUser_AlreadyMember(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return &dbs.GroupUser{ID: 1}, nil
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario ya pertenece a este grupo")
}

func TestGroupUserService_RemoveUser_Success(t *testing.T) {
	softDeleteCalled := false
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return &dbs.GroupUser{ID: 1}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
}

func TestGroupUserService_RemoveUser_GroupNotFound(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, nil
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 999, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grupo no encontrado")
}

func TestGroupUserService_RemoveUser_NotMember(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no pertenece a este grupo")
}

func TestGroupUserService_AddUser_GroupFindByIDError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, &mockUserDaoForUserRole{})
	_, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al grupo")
}

func TestGroupUserService_AddUser_UserFindByIDError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al grupo")
}

func TestGroupUserService_AddUser_FindByGroupAndUserError(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, errors.New("db error")
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al grupo")
}

func TestGroupUserService_AddUser_CreateError(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			return errors.New("db error")
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, mockUserDao)
	_, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al agregar usuario al grupo")
}

func TestGroupUserService_RemoveUser_GroupFindByIDError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del grupo")
}

func TestGroupUserService_RemoveUser_FindByGroupAndUserError(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return nil, errors.New("db error")
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del grupo")
}

func TestGroupUserService_RemoveUser_SoftDeleteError(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupAndUserFn: func(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
			return &dbs.GroupUser{ID: 1}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, &mockUserDaoForUserRole{})
	err := svc.RemoveUser(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al quitar usuario del grupo")
}

func TestGroupUserService_AddUser_WithDateStart(t *testing.T) {
	dateStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mockGroupUserDao := &mockGroupUserDao{
		createFn: func(ctx *gin.Context, gu *dbs.GroupUser) error {
			gu.ID = 1
			return nil
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDAndTeamIDFn: func(ctx *gin.Context, groupID, teamID int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1, Name: "Grupo 1", TeamID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, mockUserDao)
	resp, err := svc.AddUser(nil, 1, 1, &groupuser.AddGroupUserRequest{
		UserID:    1,
		DateStart: &dateStart,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, dateStart, resp.DateStart)
}

func TestGroupUserService_GetUsersByGroup_Success(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupIDFn: func(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{
				{ID: 1, GroupID: 1, UserID: 10},
				{ID: 2, GroupID: 1, UserID: 20},
			}, nil
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, &mockUserDaoForUserRole{})
	resp, err := svc.GetUsersByGroup(nil, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, int64(10), resp[0].UserID)
}

func TestGroupUserService_GetUsersByGroup_GroupNotFound(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, nil
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, &mockUserDaoForUserRole{})
	_, err := svc.GetUsersByGroup(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "grupo no encontrado")
}

func TestGroupUserService_GetUsersByGroup_GroupFindByIDError(t *testing.T) {
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewGroupUserService(&mockGroupUserDao{}, mockGroupDao, &mockUserDaoForUserRole{})
	_, err := svc.GetUsersByGroup(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener usuarios del grupo")
}

func TestGroupUserService_GetUsersByGroup_FindByGroupIDError(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupIDFn: func(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error) {
			return nil, errors.New("db error")
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, &mockUserDaoForUserRole{})
	_, err := svc.GetUsersByGroup(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener usuarios del grupo")
}

func TestGroupUserService_GetUsersByGroup_Empty(t *testing.T) {
	mockGroupUserDao := &mockGroupUserDao{
		findByGroupIDFn: func(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{}, nil
		},
	}
	mockGroupDao := &mockGroupDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Group, error) {
			return &dbs.Group{ID: 1}, nil
		},
	}

	svc := NewGroupUserService(mockGroupUserDao, mockGroupDao, &mockUserDaoForUserRole{})
	resp, err := svc.GetUsersByGroup(nil, 1)

	assert.NoError(t, err)
	assert.Len(t, resp, 0)
}
