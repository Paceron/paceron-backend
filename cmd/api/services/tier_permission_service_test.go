package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/tierpermission"
)

type mockTierPermissionDao struct {
	createFn                     func(ctx *gin.Context, tp *dbs.TierPermission) error
	findByTierAndPermissionFn    func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error)
	findByTierIDFn               func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error)
	listPermissionNamesByTierFn  func(ctx *gin.Context, tierID int64) ([]daos.TierPermissionName, error)
	listPermissionNamesByTiersFn func(ctx *gin.Context, tierIDs []int64) ([]daos.TierPermissionNameByTier, error)
	softDeleteFn                 func(ctx *gin.Context, id int64) error
}

func (m *mockTierPermissionDao) Create(ctx *gin.Context, tp *dbs.TierPermission) error {
	if m.createFn != nil {
		return m.createFn(ctx, tp)
	}
	return nil
}

func (m *mockTierPermissionDao) FindByTierAndPermission(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
	if m.findByTierAndPermissionFn != nil {
		return m.findByTierAndPermissionFn(ctx, tierID, permissionID)
	}
	return nil, nil
}

func (m *mockTierPermissionDao) FindByTierID(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
	if m.findByTierIDFn != nil {
		return m.findByTierIDFn(ctx, tierID)
	}
	return nil, nil
}

func (m *mockTierPermissionDao) ListPermissionNamesByTier(ctx *gin.Context, tierID int64) ([]daos.TierPermissionName, error) {
	if m.listPermissionNamesByTierFn != nil {
		return m.listPermissionNamesByTierFn(ctx, tierID)
	}
	return nil, nil
}

func (m *mockTierPermissionDao) ListPermissionNamesByTiers(ctx *gin.Context, tierIDs []int64) ([]daos.TierPermissionNameByTier, error) {
	if m.listPermissionNamesByTiersFn != nil {
		return m.listPermissionNamesByTiersFn(ctx, tierIDs)
	}
	return nil, nil
}

func (m *mockTierPermissionDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func tierDaoWithTier() *mockTierDao {
	return &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: id, RoleID: 1}, nil
		},
	}
}

func TestTierPermissionService_Assign_Success(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, tp *dbs.TierPermission) error {
			tp.ID = 1
			return nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao, &mockRoleDaoForTier{})
	resp, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{
		PermissionID: 1,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.TierID)
	assert.Equal(t, int64(1), resp.PermissionID)
}

func TestTierPermissionService_Assign_TierNotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.Assign(nil, 999, &tierpermission.AssignPermissionRequest{
		PermissionID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierPermissionService_Assign_PermissionNotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, nil
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, mockPermDao, &mockRoleDaoForTier{})
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{
		PermissionID: 999,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permiso no encontrado")
}

func TestTierPermissionService_Assign_AlreadyAssigned(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return &dbs.TierPermission{ID: 1, TierID: tierID, PermissionID: permissionID}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao, &mockRoleDaoForTier{})
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{
		PermissionID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el permiso ya está asignado a este tier")
}

func TestTierPermissionService_Unassign_Success(t *testing.T) {
	now := time.Now()
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return &dbs.TierPermission{ID: 1, TierID: tierID, PermissionID: permissionID, AsignationDate: now}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, tierDaoWithTier(), &mockPermissionDao{}, &mockRoleDaoForTier{})
	resp, err := svc.Unassign(nil, 1, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Permiso desasignado del tier correctamente", resp.Message)
}

func TestTierPermissionService_Unassign_NotFound(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return nil, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, tierDaoWithTier(), &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.Unassign(nil, 1, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "asignación no encontrada")
}

func TestTierPermissionService_Unassign_SoftDeleteError(t *testing.T) {
	now := time.Now()
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return &dbs.TierPermission{ID: 1, TierID: tierID, PermissionID: permissionID, AsignationDate: now}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, tierDaoWithTier(), &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.Unassign(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al desasignar permiso")
}

func TestTierPermissionService_Assign_TierFindByIDError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{
		PermissionID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar permiso")
}

func TestTierPermissionService_Assign_PermissionFindByIDError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, mockPermDao, &mockRoleDaoForTier{})
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{
		PermissionID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar permiso")
}

func TestTierPermissionService_Assign_FindByTierAndPermissionError(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return nil, errors.New("db error")
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao, &mockRoleDaoForTier{})
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{
		PermissionID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar permiso")
}

func TestTierPermissionService_Assign_CreateError(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, tp *dbs.TierPermission) error {
			return errors.New("db error")
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao, &mockRoleDaoForTier{})
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{
		PermissionID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar permiso")
}

func TestTierPermissionService_Unassign_FindByTierAndPermissionError(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, tierDaoWithTier(), &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.Unassign(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al desasignar permiso")
}

func TestTierPermissionService_List_Success(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		listPermissionNamesByTierFn: func(ctx *gin.Context, tierID int64) ([]daos.TierPermissionName, error) {
			return []daos.TierPermissionName{
				{PermissionID: 3, PermissionName: "perm_c"},
				{PermissionID: 7, PermissionName: "perm_g"},
			}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	resp, err := svc.List(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, resp.Permissions, 2)
	assert.Equal(t, int64(3), resp.Permissions[0].PermissionID)
	assert.Equal(t, "perm_c", resp.Permissions[0].PermissionName)
	assert.Equal(t, int64(7), resp.Permissions[1].PermissionID)
	assert.Equal(t, "perm_g", resp.Permissions[1].PermissionName)
}

func TestTierPermissionService_List_TierNotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.List(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierPermissionService_List_Empty(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		listPermissionNamesByTierFn: func(ctx *gin.Context, tierID int64) ([]daos.TierPermissionName, error) {
			return []daos.TierPermissionName{}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	resp, err := svc.List(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Permissions)
	assert.Empty(t, resp.Permissions)
}

// El filtro de permiso soft-deleted ahora vive en el JOIN del DAO; a nivel
// service basta verificar que el shape de la respuesta no incluye la fila
// excluida (el caso real lo cubre el test del DAO con DB).
func TestTierPermissionService_List_SkipsDeletedPermission(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		listPermissionNamesByTierFn: func(ctx *gin.Context, tierID int64) ([]daos.TierPermissionName, error) {
			return []daos.TierPermissionName{
				{PermissionID: 3, PermissionName: "perm_c"},
			}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	resp, err := svc.List(nil, 1)

	assert.NoError(t, err)
	assert.Len(t, resp.Permissions, 1)
	assert.Equal(t, int64(3), resp.Permissions[0].PermissionID)
}

func TestTierPermissionService_List_QueryError(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		listPermissionNamesByTierFn: func(ctx *gin.Context, tierID int64) ([]daos.TierPermissionName, error) {
			return nil, errors.New("db error")
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.List(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar permisos")
}

func TestTierPermissionService_List_TierFindByIDError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.List(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar permisos")
}

func protectedRoleDao(roleID int64) *mockRoleDaoForTier {
	return &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: roleID, Name: protectedRoleName}, nil
		},
	}
}

func TestTierPermissionService_Assign_ProtectedRoleByLiveRole(t *testing.T) {
	createCalled := false
	mockTierPermDao := &mockTierPermissionDao{
		createFn: func(ctx *gin.Context, tp *dbs.TierPermission) error {
			createCalled = true
			return nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, RoleID: 10, RoleName: ""}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao, protectedRoleDao(10))
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{PermissionID: 1})

	assert.Error(t, err)
	assert.Equal(t, "el rol corredor no admite tiers", err.Error())
	assert.False(t, createCalled)
}

func TestTierPermissionService_Unassign_ProtectedRoleByLiveRole(t *testing.T) {
	softDeleteCalled := false
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return &dbs.TierPermission{ID: 1, TierID: tierID, PermissionID: permissionID}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, RoleID: 10, RoleName: ""}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{}, protectedRoleDao(10))
	_, err := svc.Unassign(nil, 1, 1)

	assert.Error(t, err)
	assert.Equal(t, "el rol corredor no admite tiers", err.Error())
	assert.False(t, softDeleteCalled)
}

func TestTierPermissionService_List_ProtectedRoleByLiveRole(t *testing.T) {
	listCalled := false
	mockTierPermDao := &mockTierPermissionDao{
		listPermissionNamesByTierFn: func(ctx *gin.Context, tierID int64) ([]daos.TierPermissionName, error) {
			listCalled = true
			return nil, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, RoleID: 10, RoleName: ""}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{}, protectedRoleDao(10))
	_, err := svc.List(nil, 1)

	assert.Error(t, err)
	assert.Equal(t, "el rol corredor no admite tiers", err.Error())
	assert.False(t, listCalled)
}

func TestTierPermissionService_Assign_RoleLookupError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, RoleID: 10}, nil
		},
	}
	mockRoleDao := &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{}, mockRoleDao)
	_, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{PermissionID: 1})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar permiso")
}

func TestTierPermissionService_Unassign_TierNotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{}, &mockRoleDaoForTier{})
	_, err := svc.Unassign(nil, 999, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierPermissionService_Assign_StaleRoleNameButLiveNonProtected(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierAndPermissionFn: func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, tp *dbs.TierPermission) error {
			tp.ID = 1
			return nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, RoleID: 20, RoleName: protectedRoleName}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao, &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 20, Name: "entrenador"}, nil
		},
	})
	resp, err := svc.Assign(nil, 1, &tierpermission.AssignPermissionRequest{PermissionID: 1})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}
