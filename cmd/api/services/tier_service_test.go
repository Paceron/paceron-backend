package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/tier"
)

type mockTierDao struct {
	createFn            func(ctx *gin.Context, t *dbs.Tier) error
	findByIDFn          func(ctx *gin.Context, id int64) (*dbs.Tier, error)
	findByNameAndRoleFn func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error)
	findByNameFn        func(ctx *gin.Context, name string) (*dbs.Tier, error)
	findLowestByRoleFn  func(ctx *gin.Context, roleID int64) (*dbs.Tier, error)
	getAllFn            func(ctx *gin.Context) ([]dbs.Tier, error)
	updateFn            func(ctx *gin.Context, t *dbs.Tier) error
	softDeleteFn        func(ctx *gin.Context, id int64) error
}

func (m *mockTierDao) Create(ctx *gin.Context, t *dbs.Tier) error {
	if m.createFn != nil {
		return m.createFn(ctx, t)
	}
	return nil
}

func (m *mockTierDao) FindByID(ctx *gin.Context, id int64) (*dbs.Tier, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockTierDao) FindByNameAndRole(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
	if m.findByNameAndRoleFn != nil {
		return m.findByNameAndRoleFn(ctx, name, roleID)
	}
	return nil, nil
}

func (m *mockTierDao) FindByName(ctx *gin.Context, name string) (*dbs.Tier, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockTierDao) FindLowestByRole(ctx *gin.Context, roleID int64) (*dbs.Tier, error) {
	if m.findLowestByRoleFn != nil {
		return m.findLowestByRoleFn(ctx, roleID)
	}
	return nil, nil
}

func (m *mockTierDao) GetAll(ctx *gin.Context) ([]dbs.Tier, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return []dbs.Tier{}, nil
}

func (m *mockTierDao) Update(ctx *gin.Context, t *dbs.Tier) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, t)
	}
	return nil
}

func (m *mockTierDao) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

type mockRoleDaoForTier struct {
	createFn     func(ctx *gin.Context, r *dbs.Role) error
	findByIDFn   func(ctx *gin.Context, id int64) (*dbs.Role, error)
	findByNameFn func(ctx *gin.Context, name string) (*dbs.Role, error)
	getAllFn     func(ctx *gin.Context) ([]dbs.Role, error)
	updateFn     func(ctx *gin.Context, r *dbs.Role) error
	softDeleteFn func(ctx *gin.Context, id int64) error
}

func (m *mockRoleDaoForTier) Create(ctx *gin.Context, r *dbs.Role) error {
	if m.createFn != nil {
		return m.createFn(ctx, r)
	}
	return nil
}

func (m *mockRoleDaoForTier) FindByID(ctx *gin.Context, id int64) (*dbs.Role, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockRoleDaoForTier) FindByName(ctx *gin.Context, name string) (*dbs.Role, error) {
	if m.findByNameFn != nil {
		return m.findByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockRoleDaoForTier) GetAll(ctx *gin.Context) ([]dbs.Role, error) {
	if m.getAllFn != nil {
		return m.getAllFn(ctx)
	}
	return []dbs.Role{}, nil
}

func (m *mockRoleDaoForTier) Update(ctx *gin.Context, r *dbs.Role) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, r)
	}
	return nil
}

func (m *mockRoleDaoForTier) SoftDelete(ctx *gin.Context, id int64) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func TestTierService_Create_Success(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, t *dbs.Tier) error {
			t.ID = 1
			return nil
		},
	}
	mockRoleDao := &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewTierService(mockTierDao, mockRoleDao)
	resp, err := svc.Create(nil, &tier.CreateTierRequest{
		Name:        "base",
		Description: "Tier base",
		RoleID:      1,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "base", resp.Name)
	assert.Equal(t, "corredor", resp.RoleName)
}

func TestTierService_Create_RoleNotFound(t *testing.T) {
	mockRoleDao := &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewTierService(&mockTierDao{}, mockRoleDao)
	_, err := svc.Create(nil, &tier.CreateTierRequest{
		Name:   "base",
		RoleID: 999,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rol no encontrado")
}

func TestTierService_Create_DuplicateName(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: name, RoleID: roleID}, nil
		},
	}
	mockRoleDao := &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewTierService(mockTierDao, mockRoleDao)
	_, err := svc.Create(nil, &tier.CreateTierRequest{
		Name:   "base",
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya existe un tier con ese nombre para este rol")
}

func TestTierService_Update_Success(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1, RoleName: "corredor"}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Tier) error {
			return nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	newName := "premium"
	resp, err := svc.Update(nil, 1, &tier.UpdateTierRequest{
		Name: &newName,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "premium", resp.Name)
}

func TestTierService_Update_NotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	newName := "premium"
	_, err := svc.Update(nil, 999, &tier.UpdateTierRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierService_Delete_Success(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	resp, err := svc.Delete(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Tier eliminado correctamente", resp.Message)
}

func TestTierService_Delete_NotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.Delete(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierService_Create_RoleFindError(t *testing.T) {
	mockRoleDao := &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierService(&mockTierDao{}, mockRoleDao)
	_, err := svc.Create(nil, &tier.CreateTierRequest{
		Name:   "base",
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear tier")
}

func TestTierService_Create_EmptyName(t *testing.T) {
	svc := NewTierService(&mockTierDao{}, &mockRoleDaoForTier{})
	_, err := svc.Create(nil, &tier.CreateTierRequest{
		Name:   "",
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre es requerido")
}

func TestTierService_Create_FindByNameAndRoleError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}
	mockRoleDao := &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewTierService(mockTierDao, mockRoleDao)
	_, err := svc.Create(nil, &tier.CreateTierRequest{
		Name:   "base",
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear tier")
}

func TestTierService_Create_DAOCreateError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, t *dbs.Tier) error {
			return errors.New("db error")
		},
	}
	mockRoleDao := &mockRoleDaoForTier{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "corredor"}, nil
		},
	}

	svc := NewTierService(mockTierDao, mockRoleDao)
	_, err := svc.Create(nil, &tier.CreateTierRequest{
		Name:   "base",
		RoleID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al crear tier")
}

func TestTierService_Update_FindByIDError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	newName := "premium"
	_, err := svc.Update(nil, 1, &tier.UpdateTierRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar tier")
}

func TestTierService_Update_FindByNameAndRoleError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "old_name", RoleID: 1}, nil
		},
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	newName := "new_name"
	_, err := svc.Update(nil, 1, &tier.UpdateTierRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar tier")
}

func TestTierService_Update_EmptyName(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1, RoleName: "corredor"}, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	emptyName := "   "
	_, err := svc.Update(nil, 1, &tier.UpdateTierRequest{
		Name: &emptyName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el nombre no puede estar vacío")
}

func TestTierService_Update_DAOUpdateError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1, RoleName: "corredor"}, nil
		},
		updateFn: func(ctx *gin.Context, t *dbs.Tier) error {
			return errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	newDesc := "updated desc"
	_, err := svc.Update(nil, 1, &tier.UpdateTierRequest{
		Description: &newDesc,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al actualizar tier")
}

func TestTierService_Update_DuplicateName(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1}, nil
		},
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 2, Name: name, RoleID: roleID}, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	newName := "existing_name"
	_, err := svc.Update(nil, 1, &tier.UpdateTierRequest{
		Name: &newName,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ya existe un tier con ese nombre para este rol")
}

func TestTierService_Delete_FindByIDError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.Delete(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar tier")
}

func TestTierService_Delete_SoftDeleteError(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base"}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.Delete(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar tier")
}

func TestTierService_GetByID_Success(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: "base", RoleID: 1, RoleName: "corredor"}, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	resp, err := svc.GetByID(nil, 1)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.ID)
	assert.Equal(t, "base", resp.Name)
}

func TestTierService_GetByID_NotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.GetByID(nil, 999)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierService_GetByID_Error(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.GetByID(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener tier")
}

func TestTierService_GetByName_Success(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 1, Name: name, RoleID: 1, RoleName: "corredor"}, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	resp, err := svc.GetByName(nil, "base")

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "base", resp.Name)
}

func TestTierService_GetByName_NotFound(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Tier, error) {
			return nil, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.GetByName(nil, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tier no encontrado")
}

func TestTierService_GetByName_Error(t *testing.T) {
	mockTierDao := &mockTierDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.GetByName(nil, "base")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener tier")
}

func TestTierService_GetAll_Success(t *testing.T) {
	mockTierDao := &mockTierDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Tier, error) {
			return []dbs.Tier{
				{ID: 1, Name: "base", RoleID: 1, RoleName: "corredor"},
				{ID: 2, Name: "premium", RoleID: 1, RoleName: "corredor"},
			}, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	resp, err := svc.GetAll(nil)

	assert.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "base", resp[0].Name)
	assert.Equal(t, "premium", resp[1].Name)
}

func TestTierService_GetAll_Empty(t *testing.T) {
	mockTierDao := &mockTierDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Tier, error) {
			return []dbs.Tier{}, nil
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	resp, err := svc.GetAll(nil)

	assert.NoError(t, err)
	assert.Empty(t, resp)
}

func TestTierService_GetAll_Error(t *testing.T) {
	mockTierDao := &mockTierDao{
		getAllFn: func(ctx *gin.Context) ([]dbs.Tier, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewTierService(mockTierDao, &mockRoleDaoForTier{})
	_, err := svc.GetAll(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al obtener tiers")
}
