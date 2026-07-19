package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/userrole"
)

type mockUserRoleDao struct {
	createFn            func(ctx *gin.Context, ur *dbs.UserRole) error
	findByUserAndRoleFn func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error)
	findByUserIDFn      func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error)
	softDeleteFn        func(ctx *gin.Context, id int64) error
}

func (m *mockUserRoleDao) Create(ctx *gin.Context, ur *dbs.UserRole) error {
	if m.createFn != nil {
		return m.createFn(ctx, ur)
	}
	return nil
}

func (m *mockUserRoleDao) FindByUserAndRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
	if m.findByUserAndRoleFn != nil {
		return m.findByUserAndRoleFn(ctx, userID, roleID)
	}
	return nil, nil
}

func (m *mockUserRoleDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockUserRoleDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

type mockUserDaoForUserRole struct {
	findByIDFn func(ctx *gin.Context, userID int64) (*dbs.User, error)
}

func (m *mockUserDaoForUserRole) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForUserRole) Create(ctx *gin.Context, name, password string) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForUserRole) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockUserDaoForUserRole) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForUserRole) Update(ctx *gin.Context, user *dbs.User) error {
	return nil
}

func (m *mockUserDaoForUserRole) UpdateStatus(ctx *gin.Context, userID int64, status string) error {
	return nil
}

func TestUserRoleService_AssignRole_Success(t *testing.T) {
	now := time.Now()
	_ = now

	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, ur *dbs.UserRole) error {
			ur.ID = 1
			return nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	resp, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 1,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Equal(t, int64(1), resp.RoleID)
	assert.Equal(t, int64(1), resp.TierID)
}

func TestUserRoleService_AssignRole_UserNotFound(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao)
	_, err := svc.AssignRole(nil, 999, &userrole.AssignRoleRequest{
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestUserRoleService_AssignRole_RoleNotFound(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 999,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol no encontrado")
}

func TestUserRoleService_AssignRole_AlreadyAssigned(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{ID: 1, UserID: userID, RoleID: roleID}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, &mockTierDao{}, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario ya tiene asignado este rol")
}

func TestUserRoleService_AssignRole_DefaultTierNotFound(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 0,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el tier por defecto 'base' no existe para este rol")
}

func TestUserRoleService_AssignRole_TierNotFound(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 999,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestUserRoleService_AssignRole_TierNotBelongToRole(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 2}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el tier no pertenece al rol especificado")
}

func TestUserRoleService_AssignRole_SuccessWithDefaultTier(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, ur *dbs.UserRole) error {
			ur.ID = 1
			return nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 5, Name: "base", RoleID: roleID}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	resp, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 0,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(5), resp.TierID)
}

func TestUserRoleService_AssignRole_CreateError(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, ur *dbs.UserRole) error {
			return errors.New("db error")
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1}, nil
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}

func TestUserRoleService_AssignRole_UserFindByIDError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}

func TestUserRoleService_AssignRole_RoleFindByIDError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}

func TestUserRoleService_AssignRole_FindByUserAndRoleError(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, errors.New("db error")
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, &mockTierDao{}, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}

func TestUserRoleService_AssignRole_DefaultTierFindByNameAndRoleError(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 0,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}

func TestUserRoleService_AssignRole_TierFindByIDError(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao)
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}
