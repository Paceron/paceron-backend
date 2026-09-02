package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
)

type mockUserDaoForQuery struct {
	findByIDFn func(ctx *gin.Context, userID int64) (*dbs.User, error)
}

func (m *mockUserDaoForQuery) GetByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForQuery) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockUserDaoForQuery) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForQuery) Update(ctx *gin.Context, user *dbs.User) error {
	return nil
}

func (m *mockUserDaoForQuery) UpdateStatus(ctx *gin.Context, userID int64, status string) error {
	return nil
}

func (m *mockUserDaoForQuery) SearchActive(ctx *gin.Context, query string, limit int) ([]*dbs.User, error) {
	return nil, nil
}

func (m *mockUserDaoForQuery) FindByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	return nil, nil
}

type mockUserRoleDaoForQuery struct {
	findByUserIDFn func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error)
}

func (m *mockUserRoleDaoForQuery) Create(ctx *gin.Context, ur *dbs.UserRole) error {
	return nil
}

func (m *mockUserRoleDaoForQuery) FindByUserAndRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
	return nil, nil
}

func (m *mockUserRoleDaoForQuery) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
	if m.findByUserIDFn != nil {
		return m.findByUserIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockUserRoleDaoForQuery) SoftDelete(ctx *gin.Context, id int64) error {
	return nil
}

func (m *mockUserRoleDaoForQuery) UpdateTier(ctx *gin.Context, userID, roleID, tierID int64) error {
	return nil
}

type mockRoleDaoForQuery struct {
	findByIDFn func(ctx *gin.Context, id int64) (*dbs.Role, error)
}

func (m *mockRoleDaoForQuery) Create(ctx *gin.Context, r *dbs.Role) error {
	return nil
}

func (m *mockRoleDaoForQuery) FindByID(ctx *gin.Context, id int64) (*dbs.Role, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRoleDaoForQuery) FindByName(ctx *gin.Context, name string) (*dbs.Role, error) {
	return nil, nil
}

func (m *mockRoleDaoForQuery) Update(ctx *gin.Context, r *dbs.Role) error {
	return nil
}

func (m *mockRoleDaoForQuery) SoftDelete(ctx *gin.Context, id int64) error {
	return nil
}

func (m *mockRoleDaoForQuery) GetAll(ctx *gin.Context) ([]dbs.Role, error) {
	return nil, nil
}

type mockTierDaoForQuery struct {
	findByIDFn func(ctx *gin.Context, id int64) (*dbs.Tier, error)
}

func (m *mockTierDaoForQuery) Create(ctx *gin.Context, t *dbs.Tier) error {
	return nil
}

func (m *mockTierDaoForQuery) FindByID(ctx *gin.Context, id int64) (*dbs.Tier, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockTierDaoForQuery) FindByNameAndRole(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
	return nil, nil
}

func (m *mockTierDaoForQuery) Update(ctx *gin.Context, t *dbs.Tier) error {
	return nil
}

func (m *mockTierDaoForQuery) SoftDelete(ctx *gin.Context, id int64) error {
	return nil
}

func (m *mockTierDaoForQuery) FindByName(ctx *gin.Context, name string) (*dbs.Tier, error) {
	return nil, nil
}

func (m *mockTierDaoForQuery) FindLowestByRole(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
	return nil, nil
}

func (m *mockTierDaoForQuery) GetAll(ctx *gin.Context) ([]dbs.Tier, error) {
	return nil, nil
}

type mockTierPermissionDaoForQuery struct {
	findByTierIDFn func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error)
}

func (m *mockTierPermissionDaoForQuery) Create(ctx *gin.Context, tp *dbs.TierPermission) error {
	return nil
}

func (m *mockTierPermissionDaoForQuery) FindByTierAndPermission(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
	return nil, nil
}

func (m *mockTierPermissionDaoForQuery) FindByTierID(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
	if m.findByTierIDFn != nil {
		return m.findByTierIDFn(ctx, tierID)
	}
	return nil, nil
}

func (m *mockTierPermissionDaoForQuery) SoftDelete(ctx *gin.Context, id int64) error {
	return nil
}

type mockPermissionDaoForQuery struct {
	findByIDFn func(ctx *gin.Context, id int64) (*dbs.Permission, error)
}

func (m *mockPermissionDaoForQuery) Create(ctx *gin.Context, p *dbs.Permission) error {
	return nil
}

func (m *mockPermissionDaoForQuery) FindByID(ctx *gin.Context, id int64) (*dbs.Permission, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockPermissionDaoForQuery) FindByName(ctx *gin.Context, name string) (*dbs.Permission, error) {
	return nil, nil
}

func (m *mockPermissionDaoForQuery) Update(ctx *gin.Context, p *dbs.Permission) error {
	return nil
}

func (m *mockPermissionDaoForQuery) SoftDelete(ctx *gin.Context, id int64) error {
	return nil
}

func (m *mockPermissionDaoForQuery) GetAll(ctx *gin.Context) ([]dbs.Permission, error) {
	return nil, nil
}

func TestPermissionsQueryService_GetUserPermissions_Success(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: 1, PermissionID: 1},
			}, nil
		},
	}
	permDao := &mockPermissionDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao)
	resp, err := svc.GetUserPermissions(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Len(t, resp.Roles, 1)
	assert.Equal(t, "corredor", resp.Roles[0].Name)
	assert.Equal(t, "base", resp.Roles[0].Tier)
	assert.Contains(t, resp.Roles[0].Permissions, "crear_venta")
}

func TestPermissionsQueryService_GetUserPermissions_UserNotFound(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, &mockUserRoleDaoForQuery{}, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestPermissionsQueryService_GetUserPermissions_NoRoles(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	resp, err := svc.GetUserPermissions(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.UserID)
	assert.Len(t, resp.Roles, 0)
}

func TestPermissionsQueryService_GetUserPermissions_MissingRole(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}

func TestPermissionsQueryService_GetUserPermissions_MissingTier(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}

func TestPermissionsQueryService_GetUserPermissions_MissingPermission(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: 1, PermissionID: 1},
			}, nil
		},
	}
	permDao := &mockPermissionDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao)
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}

func TestPermissionsQueryService_GetUserPermissions_UserFindError(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionsQueryService(userDao, &mockUserRoleDaoForQuery{}, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener permisos")
}

func TestPermissionsQueryService_GetUserPermissions_EmptyTierPermissions(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}

func TestPermissionsQueryService_GetUserPermissions_UserRolesFindError(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener permisos")
}

func TestPermissionsQueryService_GetUserPermissions_RoleFindByIDError(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}

func TestPermissionsQueryService_GetUserPermissions_TierFindByIDError(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}

func TestPermissionsQueryService_GetUserPermissions_TierPermissionFindByTierIDError(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, &mockPermissionDaoForQuery{})
	resp, err := svc.GetUserPermissions(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Roles, 0)
}

func TestPermissionsQueryService_GetUserPermissions_PermissionFindByIDError(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 1},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: 1, PermissionID: 1},
			}, nil
		},
	}
	permDao := &mockPermissionDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao)
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}
