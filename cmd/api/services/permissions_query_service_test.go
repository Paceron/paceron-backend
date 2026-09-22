package services

import (
	"errors"
	"testing"
	"time"

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

func (m *mockUserDaoForQuery) UpdatePhoto(ctx *gin.Context, userID int64, key string, updatedAt time.Time) error {
	return nil
}

func (m *mockUserDaoForQuery) ClearPhoto(ctx *gin.Context, userID int64) error {
	return nil
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
	findByIDFn         func(ctx *gin.Context, id int64) (*dbs.Tier, error)
	findLowestByRoleFn func(ctx *gin.Context, roleID int64) (*dbs.Tier, error)
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
	if m.findLowestByRoleFn != nil {
		return m.findLowestByRoleFn(ctx, roleID)
	}
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

type mockTierSubscriptionDaoForQuery struct {
	findActiveFn      func(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error)
	findLatestEndedFn func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error)
}

func (m *mockTierSubscriptionDaoForQuery) Create(ctx *gin.Context, sub *dbs.UserRoleTierSubscription) error {
	return nil
}

func (m *mockTierSubscriptionDaoForQuery) FindByID(ctx *gin.Context, id int64) (*dbs.UserRoleTierSubscription, error) {
	return nil, nil
}

func (m *mockTierSubscriptionDaoForQuery) FindActiveByUserRole(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error) {
	if m.findActiveFn != nil {
		return m.findActiveFn(ctx, userID, roleID, statuses...)
	}
	return nil, nil
}

func (m *mockTierSubscriptionDaoForQuery) FindLatestByUserRole(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
	return nil, nil
}

func (m *mockTierSubscriptionDaoForQuery) FindPendingByUserRoleTier(ctx *gin.Context, userID, roleID, tierID int64) (*dbs.UserRoleTierSubscription, error) {
	return nil, nil
}

func (m *mockTierSubscriptionDaoForQuery) FindByUserRoleTier(ctx *gin.Context, userID, roleID, tierID int64) (*dbs.UserRoleTierSubscription, error) {
	return nil, nil
}

func (m *mockTierSubscriptionDaoForQuery) SetEnded(ctx *gin.Context, id int64) error {
	return nil
}

func (m *mockTierSubscriptionDaoForQuery) SetCanceled(ctx *gin.Context, id int64) error {
	return nil
}

func (m *mockTierSubscriptionDaoForQuery) Activate(ctx *gin.Context, id int64) error {
	return nil
}

func (m *mockTierSubscriptionDaoForQuery) IncrementPaidInstallments(ctx *gin.Context, id int64) error {
	return nil
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
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao, &mockTierSubscriptionDaoForQuery{})
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

	svc := NewPermissionsQueryService(userDao, &mockUserRoleDaoForQuery{}, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
	assert.Contains(t, err.Error(), "tier_id=1 no configurado")
}

func TestPermissionsQueryService_GetUserPermissions_NoBaseTierForRole(t *testing.T) {
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
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
	assert.Contains(t, err.Error(), "tier base no configurado para el rol entrenador")
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
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao, &mockTierSubscriptionDaoForQuery{})
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

	svc := NewPermissionsQueryService(userDao, &mockUserRoleDaoForQuery{}, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, &mockRoleDaoForQuery{}, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, &mockTierDaoForQuery{}, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, &mockPermissionDaoForQuery{}, &mockTierSubscriptionDaoForQuery{})
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
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
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

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao, &mockTierSubscriptionDaoForQuery{})
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
}

func TestPermissionsQueryService_GetUserPermissions_ActiveSubBeatsUserRoleTier(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 2},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			if id == 1 {
				return &dbs.Tier{ID: 1, Name: "base"}, nil
			}
			return &dbs.Tier{ID: 2, Name: "premium_entrenador"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: tierID, PermissionID: 1},
			}, nil
		},
	}
	permDao := &mockPermissionDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_equipos"}, nil
		},
	}
	tierSubDao := &mockTierSubscriptionDaoForQuery{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{ID: 7, UserID: userID, RoleID: roleID, TierID: 1, Status: "active"}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao, tierSubDao)
	resp, err := svc.GetUserPermissions(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Roles, 1)
	assert.Equal(t, "entrenador", resp.Roles[0].Name)
	assert.Equal(t, "base", resp.Roles[0].Tier)
	assert.Contains(t, resp.Roles[0].Permissions, "crear_equipos")
}

func TestPermissionsQueryService_GetUserPermissions_PendingSubWithoutPreviousEndedFallsBackToBaseTier(t *testing.T) {
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
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			if id == 2 {
				return &dbs.Tier{ID: 2, Name: "premium_entrenador"}, nil
			}
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: tierID, PermissionID: 1},
			}, nil
		},
	}
	permDao := &mockPermissionDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_equipos"}, nil
		},
	}
	// Primer pago pendiente existente (cambio a premium en vuelo) pero SIN sub
	// previa ended (el usuario nunca pagó un tier): el tier efectivo es base —
	// la cuota #1 impaga no habilita el tier pago ni hay tier previo que
	// conservar.
	tierSubDao := &mockTierSubscriptionDaoForQuery{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error) {
			if len(statuses) == 1 && statuses[0] == "first_payment_pending" {
				return &dbs.UserRoleTierSubscription{ID: 8, UserID: userID, RoleID: roleID, TierID: 2, Status: "first_payment_pending"}, nil
			}
			return nil, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao, tierSubDao)
	resp, err := svc.GetUserPermissions(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Roles, 1)
	assert.Equal(t, "base", resp.Roles[0].Tier)
}

func TestPermissionsQueryService_GetUserPermissions_FallbackToBaseTierWhenNoActiveSub(t *testing.T) {
	userDao := &mockUserDaoForQuery{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John"}, nil
		},
	}
	userRoleDao := &mockUserRoleDaoForQuery{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{
				{ID: 1, UserID: 1, RoleID: 1, TierID: 2},
			}, nil
		},
	}
	roleDao := &mockRoleDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			if id == 2 {
				return &dbs.Tier{ID: 2, Name: "premium_entrenador"}, nil
			}
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: tierID, PermissionID: 1},
			}, nil
		},
	}
	permDao := &mockPermissionDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_equipos"}, nil
		},
	}
// Sin sub activa, sin pending y sin suscripción alguna en el ledger — el
	// caché user_roles.tier_id (premium de una activación vieja) no manda: base.
	tierSubDao := &mockTierSubscriptionDaoForQuery{}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao, tierSubDao)
	resp, err := svc.GetUserPermissions(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Roles, 1)
	assert.Equal(t, "base", resp.Roles[0].Tier)
}

func TestPermissionsQueryService_GetUserPermissions_PendingSubKeepsPreviousPaidTier(t *testing.T) {
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
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			if id == 2 {
				return &dbs.Tier{ID: 2, Name: "premium_entrenador"}, nil
			}
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
		findLowestByRoleFn: func(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	tierPermDao := &mockTierPermissionDaoForQuery{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: tierID, PermissionID: 1},
			}, nil
		},
	}
	permDao := &mockPermissionDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_equipos"}, nil
		},
	}
	// Usuario premium paga (sub active, id 5, tier 2). Cambia a medium: la
	// vieja queda ACTIVE (id 5, tier 2, ChangeTier ya no la cierra al crear el
	// pendiente) y se crea la pending (id 7, tier 3). El tier efectivo sigue
	// siendo el premium — reportado por la sub activa, como siempre — hasta que
	// la cuota #1 confirme al pagar (la vieja pasa a ended en ese momento).
	tierSubDao := &mockTierSubscriptionDaoForQuery{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error) {
			if len(statuses) == 1 && statuses[0] == "first_payment_pending" {
				return &dbs.UserRoleTierSubscription{ID: 7, UserID: userID, RoleID: roleID, TierID: 3, Status: "first_payment_pending"}, nil
			}
			return &dbs.UserRoleTierSubscription{ID: 5, UserID: userID, RoleID: roleID, TierID: 2, Status: "active"}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, tierPermDao, permDao, tierSubDao)

	resp, err := svc.GetUserPermissions(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Roles, 1)
	assert.Equal(t, "premium_entrenador", resp.Roles[0].Tier)
	assert.Contains(t, resp.Roles[0].Permissions, "crear_equipos")
}

func TestPermissionsQueryService_GetUserPermissions_SubTierNotConfigured(t *testing.T) {
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
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	tierDao := &mockTierDaoForQuery{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}
	tierSubDao := &mockTierSubscriptionDaoForQuery{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64, statuses ...string) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{ID: 9, UserID: userID, RoleID: roleID, TierID: 99, Status: "active"}, nil
		},
	}

	svc := NewPermissionsQueryService(userDao, userRoleDao, roleDao, tierDao, &mockTierPermissionDaoForQuery{}, &mockPermissionDaoForQuery{}, tierSubDao)
	_, err := svc.GetUserPermissions(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datos faltantes")
	assert.Contains(t, err.Error(), "tier_id=99")
}
