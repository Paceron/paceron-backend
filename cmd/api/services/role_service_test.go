package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/role"
)

type mockRoleDao struct {
	createFn    func(ctx *gin.Context, r *dbs.Role) error
	findByIDFn  func(ctx *gin.Context, id int64) (*dbs.Role, error)
	findByNameFn func(ctx *gin.Context, name string) (*dbs.Role, error)
	getAllFn    func(ctx *gin.Context) ([]dbs.Role, error)
	updateFn    func(ctx *gin.Context, r *dbs.Role) error
	softDeleteFn func(ctx *gin.Context, id int64) error
}

func (m *mockRoleDao) Create(ctx *gin.Context, r *dbs.Role) error {
	if m.createFn != nil {
		return m.createFn(ctx, r)
	}
	return nil
}

func (m *mockRoleDao) FindByID(ctx *gin.Context, id int64) (*dbs.Role, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRoleDao) FindByName(ctx *gin.Context, name string) (*dbs.Role, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockRoleDao) GetAll(ctx *gin.Context) ([]dbs.Role, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return []dbs.Role{}, nil
}

func (m *mockRoleDao) Update(ctx *gin.Context, r *dbs.Role) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, r)
	}
	return nil
}

func (m *mockRoleDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestRoleService_Create_Success(t *testing.T) {
	mockDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, r *dbs.Role) error {
			r.ID = 1
			return nil
		},
	}

	svc := NewRoleService(mockDao)
	resp, err := svc.Create(nil, &role.CreateRoleRequest{
		Name:        "corredor",
		Description: "Rol de corredor",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "corredor", resp.Name)
	assert.Equal(t, "Rol de corredor", resp.Description)
}

func TestRoleService_Create_DuplicateName(t *testing.T) {
	mockDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: name}, nil
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.Create(nil, &role.CreateRoleRequest{
		Name: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre del rol ya existe")
}

func TestRoleService_Create_EmptyName(t *testing.T) {
	svc := NewRoleService(&mockRoleDao{})
	_, err := svc.Create(nil, &role.CreateRoleRequest{
		Name: "",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre es requerido")
}

func TestRoleService_Update_Success(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor", Description: "desc"}, nil
		},
		updateFn: func(ctx *gin.Context, r *dbs.Role) error {
			return nil
		},
	}

	svc := NewRoleService(mockDao)
	newName := "corredor_v2"
	resp, err := svc.Update(nil, 1, &role.UpdateRoleRequest{
		Name: &newName,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "corredor_v2", resp.Name)
}

func TestRoleService_Update_NotFound(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewRoleService(mockDao)
	newName := "nuevo"
	_, err := svc.Update(nil, 999, &role.UpdateRoleRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol no encontrado")
}

func TestRoleService_Delete_Success(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return nil
		},
	}

	svc := NewRoleService(mockDao)
	resp, err := svc.Delete(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Rol eliminado correctamente", resp.Message)
}

func TestRoleService_Delete_NotFound(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.Delete(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol no encontrado")
}

func TestRoleService_Create_FindByNameError(t *testing.T) {
	mockDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.Create(nil, &role.CreateRoleRequest{
		Name: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear rol")
}

func TestRoleService_Update_DuplicateName(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: name}, nil
		},
	}

	svc := NewRoleService(mockDao)
	duplicateName := "existente"
	_, err := svc.Update(nil, 1, &role.UpdateRoleRequest{
		Name: &duplicateName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre del rol ya existe")
}

func TestRoleService_Create_DAOCreateError(t *testing.T) {
	mockDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, r *dbs.Role) error {
			return errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.Create(nil, &role.CreateRoleRequest{
		Name: "corredor",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear rol")
}

func TestRoleService_Update_FindByIDError(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	newName := "nuevo"
	_, err := svc.Update(nil, 1, &role.UpdateRoleRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar rol")
}

func TestRoleService_Update_FindByNameError(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "old_name"}, nil
		},
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	newName := "new_name"
	_, err := svc.Update(nil, 1, &role.UpdateRoleRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar rol")
}

func TestRoleService_Update_EmptyName(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewRoleService(mockDao)
	emptyName := "   "
	_, err := svc.Update(nil, 1, &role.UpdateRoleRequest{
		Name: &emptyName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre no puede estar vacío")
}

func TestRoleService_Update_DAOUpdateError(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor", Description: "desc"}, nil
		},
		updateFn: func(ctx *gin.Context, r *dbs.Role) error {
			return errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	newDesc := "updated desc"
	_, err := svc.Update(nil, 1, &role.UpdateRoleRequest{
		Description: &newDesc,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar rol")
}

func TestRoleService_Delete_FindByIDError(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.Delete(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar rol")
}

func TestRoleService_Delete_SoftDeleteError(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.Delete(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar rol")
}

func TestRoleService_GetByID_Success(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor", Description: "desc"}, nil
		},
	}

	svc := NewRoleService(mockDao)
	resp, err := svc.GetByID(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "corredor", resp.Name)
}

func TestRoleService_GetByID_NotFound(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.GetByID(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol no encontrado")
}

func TestRoleService_GetByID_Error(t *testing.T) {
	mockDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.GetByID(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener rol")
}

func TestRoleService_GetByName_Success(t *testing.T) {
	mockDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: name, Description: "desc"}, nil
		},
	}

	svc := NewRoleService(mockDao)
	resp, err := svc.GetByName(nil, "corredor")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "corredor", resp.Name)
}

func TestRoleService_GetByName_NotFound(t *testing.T) {
	mockDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.GetByName(nil, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol no encontrado")
}

func TestRoleService_GetByName_Error(t *testing.T) {
	mockDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.GetByName(nil, "corredor")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener rol")
}

func TestRoleService_GetAll_Success(t *testing.T) {
	mockDao := &mockRoleDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Role, error) {
			return []dbs.Role{
				{ID: 1, Name: "corredor", Description: "desc1"},
				{ID: 2, Name: "entrenador", Description: "desc2"},
			}, nil
		},
	}

	svc := NewRoleService(mockDao)
	resp, err := svc.GetAll(nil)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "corredor", resp[0].Name)
	assert.Equal(t, "entrenador", resp[1].Name)
}

func TestRoleService_GetAll_Empty(t *testing.T) {
	mockDao := &mockRoleDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Role, error) {
			return []dbs.Role{}, nil
		},
	}

	svc := NewRoleService(mockDao)
	resp, err := svc.GetAll(nil)

	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestRoleService_GetAll_Error(t *testing.T) {
	mockDao := &mockRoleDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewRoleService(mockDao)
	_, err := svc.GetAll(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener roles")
}
