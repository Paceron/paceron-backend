package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/tierpermission"
)

type mockTierPermissionDao struct {
	createFn                  func(ctx *gin.Context, tp *dbs.TierPermission) error
	findByTierAndPermissionFn func(ctx *gin.Context, tierID, permissionID int64) (*dbs.TierPermission, error)
	findByTierIDFn            func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error)
	softDeleteFn              func(ctx *gin.Context, id int64) error
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

func (m *mockTierPermissionDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
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

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao)
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

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{})
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

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, mockPermDao)
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

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao)
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

	svc := NewTierPermissionService(mockTierPermDao, &mockTierDao{}, &mockPermissionDao{})
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

	svc := NewTierPermissionService(mockTierPermDao, &mockTierDao{}, &mockPermissionDao{})
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

	svc := NewTierPermissionService(mockTierPermDao, &mockTierDao{}, &mockPermissionDao{})
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

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{})
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

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, mockPermDao)
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

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao)
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

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao)
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

	svc := NewTierPermissionService(mockTierPermDao, &mockTierDao{}, &mockPermissionDao{})
	_, err := svc.Unassign(nil, 1, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al desasignar permiso")
}

func TestTierPermissionService_List_Success(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			// desordenado a propósito: List debe ordenar por permission_id ASC
			return []dbs.TierPermission{
				{ID: 2, TierID: tierID, PermissionID: 7},
				{ID: 1, TierID: tierID, PermissionID: 3},
			}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			if id == 3 {
				return &dbs.Permission{ID: 3, Name: "perm_c"}, nil
			}
			return &dbs.Permission{ID: 7, Name: "perm_g"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao)
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

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{})
	_, err := svc.List(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierPermissionService_List_Empty(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{})
	resp, err := svc.List(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Permissions)
	assert.Empty(t, resp.Permissions)
}

func TestTierPermissionService_List_SkipsDeletedPermission(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{
				{ID: 1, TierID: tierID, PermissionID: 3},
				{ID: 2, TierID: tierID, PermissionID: 9},
			}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}
	mockPermDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			if id == 9 {
				return nil, nil // soft-deleted
			}
			return &dbs.Permission{ID: 3, Name: "perm_c"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao)
	resp, err := svc.List(nil, 1)

	assert.NoError(t, err)
	assert.Len(t, resp.Permissions, 1)
	assert.Equal(t, int64(3), resp.Permissions[0].PermissionID)
}

func TestTierPermissionService_List_FindByTierIDError(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return nil, errors.New("db error")
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
	}

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, &mockPermissionDao{})
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

	svc := NewTierPermissionService(&mockTierPermissionDao{}, mockTierDao, &mockPermissionDao{})
	_, err := svc.List(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar permisos")
}

func TestTierPermissionService_List_PermissionFindByIDError(t *testing.T) {
	mockTierPermDao := &mockTierPermissionDao{
		findByTierIDFn: func(ctx *gin.Context, tierID int64) ([]dbs.TierPermission, error) {
			return []dbs.TierPermission{{ID: 1, TierID: tierID, PermissionID: 3}}, nil
		},
	}
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

	svc := NewTierPermissionService(mockTierPermDao, mockTierDao, mockPermDao)
	_, err := svc.List(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al listar permisos")
}
