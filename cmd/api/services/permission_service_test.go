package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/permission"
)

type mockPermissionDao struct {
	createFn    func(ctx *gin.Context, p *dbs.Permission) error
	findByIDFn  func(ctx *gin.Context, id int64) (*dbs.Permission, error)
	findByNameFn func(ctx *gin.Context, name string) (*dbs.Permission, error)
	getAllFn    func(ctx *gin.Context) ([]dbs.Permission, error)
	updateFn    func(ctx *gin.Context, p *dbs.Permission) error
	softDeleteFn func(ctx *gin.Context, id int64) error
}

func (m *mockPermissionDao) Create(ctx *gin.Context, p *dbs.Permission) error {
	if m.createFn != nil {
		return m.createFn(ctx, p)
	}
	return nil
}

func (m *mockPermissionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Permission, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockPermissionDao) FindByName(ctx *gin.Context, name string) (*dbs.Permission, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockPermissionDao) GetAll(ctx *gin.Context) ([]dbs.Permission, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return []dbs.Permission{}, nil
}

func (m *mockPermissionDao) Update(ctx *gin.Context, p *dbs.Permission) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, p)
	}
	return nil
}

func (m *mockPermissionDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestPermissionService_Create_Success(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, p *dbs.Permission) error {
			p.ID = 1
			return nil
		},
	}

	svc := NewPermissionService(mockDao)
	resp, err := svc.Create(nil, &permission.CreatePermissionRequest{
		Name:        "crear_venta",
		Description: "Permiso para crear ventas",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "crear_venta", resp.Name)
	assert.Equal(t, "Permiso para crear ventas", resp.Description)
}

func TestPermissionService_Create_DuplicateName(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: name}, nil
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.Create(nil, &permission.CreatePermissionRequest{
		Name: "crear_venta",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre del permiso ya existe")
}

func TestPermissionService_Create_EmptyName(t *testing.T) {
	mockDao := &mockPermissionDao{}

	svc := NewPermissionService(mockDao)
	_, err := svc.Create(nil, &permission.CreatePermissionRequest{
		Name: "",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre es requerido")
}

func TestPermissionService_Create_FindByNameError(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.Create(nil, &permission.CreatePermissionRequest{
		Name: "crear_venta",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear permiso")
}

func TestPermissionService_Update_Success(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta", Description: "desc"}, nil
		},
		updateFn: func(ctx *gin.Context, p *dbs.Permission) error {
			return nil
		},
	}

	svc := NewPermissionService(mockDao)
	newName := "crear_venta_v2"
	resp, err := svc.Update(nil, 1, &permission.UpdatePermissionRequest{
		Name: &newName,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "crear_venta_v2", resp.Name)
}

func TestPermissionService_Update_NotFound(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, nil
		},
	}

	svc := NewPermissionService(mockDao)
	newName := "nuevo_nombre"
	_, err := svc.Update(nil, 999, &permission.UpdatePermissionRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permiso no encontrado")
}

func TestPermissionService_Update_DuplicateName(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 2, Name: name}, nil
		},
	}

	svc := NewPermissionService(mockDao)
	duplicateName := "existente"
	_, err := svc.Update(nil, 1, &permission.UpdatePermissionRequest{
		Name: &duplicateName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre del permiso ya existe")
}

func TestPermissionService_Delete_Success(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return nil
		},
	}

	svc := NewPermissionService(mockDao)
	resp, err := svc.Delete(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Permiso eliminado correctamente", resp.Message)
}

func TestPermissionService_Delete_NotFound(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, nil
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.Delete(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permiso no encontrado")
}

func TestPermissionService_Create_TrimSpaces(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, p *dbs.Permission) error {
			p.ID = 1
			return nil
		},
	}

	svc := NewPermissionService(mockDao)
	resp, err := svc.Create(nil, &permission.CreatePermissionRequest{
		Name:        "  crear_venta  ",
		Description: "  desc  ",
	})

	assert.NoError(t, err)
	assert.Equal(t, "crear_venta", resp.Name)
}

func TestPermissionService_Create_DAOCreateError(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, p *dbs.Permission) error {
			return errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.Create(nil, &permission.CreatePermissionRequest{
		Name: "crear_venta",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear permiso")
}

func TestPermissionService_Update_FindByIDError(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	newName := "nuevo_nombre"
	_, err := svc.Update(nil, 1, &permission.UpdatePermissionRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar permiso")
}

func TestPermissionService_Update_FindByNameError(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "old_name"}, nil
		},
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	newName := "new_name"
	_, err := svc.Update(nil, 1, &permission.UpdatePermissionRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar permiso")
}

func TestPermissionService_Update_DAOUpdateError(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta", Description: "desc"}, nil
		},
		updateFn: func(ctx *gin.Context, p *dbs.Permission) error {
			return errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	newDesc := "updated desc"
	_, err := svc.Update(nil, 1, &permission.UpdatePermissionRequest{
		Description: &newDesc,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar permiso")
}

func TestPermissionService_Update_EmptyName(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
	}

	svc := NewPermissionService(mockDao)
	emptyName := "   "
	_, err := svc.Update(nil, 1, &permission.UpdatePermissionRequest{
		Name: &emptyName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre no puede estar vacío")
}

func TestPermissionService_Delete_FindByIDError(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.Delete(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar permiso")
}

func TestPermissionService_Delete_SoftDeleteError(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.Delete(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar permiso")
}

func TestPermissionService_GetByID_Success(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: "crear_venta", Description: "desc"}, nil
		},
	}

	svc := NewPermissionService(mockDao)
	resp, err := svc.GetByID(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "crear_venta", resp.Name)
}

func TestPermissionService_GetByID_NotFound(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, nil
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.GetByID(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permiso no encontrado")
}

func TestPermissionService_GetByID_Error(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.GetByID(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener permiso")
}

func TestPermissionService_GetByName_Success(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return &dbs.Permission{ID: 1, Name: name, Description: "desc"}, nil
		},
	}

	svc := NewPermissionService(mockDao)
	resp, err := svc.GetByName(nil, "crear_venta")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "crear_venta", resp.Name)
}

func TestPermissionService_GetByName_NotFound(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return nil, nil
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.GetByName(nil, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permiso no encontrado")
}

func TestPermissionService_GetByName_Error(t *testing.T) {
	mockDao := &mockPermissionDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.GetByName(nil, "crear_venta")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener permiso")
}

func TestPermissionService_GetAll_Success(t *testing.T) {
	mockDao := &mockPermissionDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Permission, error) {
			return []dbs.Permission{
				{ID: 1, Name: "crear_venta", Description: "desc1"},
				{ID: 2, Name: "ver_venta", Description: "desc2"},
			}, nil
		},
	}

	svc := NewPermissionService(mockDao)
	resp, err := svc.GetAll(nil)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "crear_venta", resp[0].Name)
	assert.Equal(t, "ver_venta", resp[1].Name)
}

func TestPermissionService_GetAll_Empty(t *testing.T) {
	mockDao := &mockPermissionDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Permission, error) {
			return []dbs.Permission{}, nil
		},
	}

	svc := NewPermissionService(mockDao)
	resp, err := svc.GetAll(nil)

	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestPermissionService_GetAll_Error(t *testing.T) {
	mockDao := &mockPermissionDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Permission, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewPermissionService(mockDao)
	_, err := svc.GetAll(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener permisos")
}
