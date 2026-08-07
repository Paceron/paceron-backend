package services

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/userrole"
)

// entrenadorTestPasswordHash es el hash bcrypt de "CorrectPass123" — compartido por los
// tests de ActivateEntrenador que necesitan simular una contraseña real guardada.
var entrenadorTestPasswordHash = func() string {
	hashed, err := bcrypt.GenerateFromPassword([]byte("CorrectPass123"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hashed)
}()

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
	updateFn   func(ctx *gin.Context, user *dbs.User) error
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
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 0,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}

func TestUserRoleService_RemoveRole_ProtectedRole(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: id, Name: "corredor"}, nil
		},
	}
	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})

	err := svc.RemoveRole(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no se puede eliminar")
}

func TestUserRoleService_RemoveRole_RoleFindByIDError(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})

	err := svc.RemoveRole(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar rol")
}

func TestUserRoleService_RemoveRole_Success(t *testing.T) {
	softDeleteCalled := false
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{ID: 7, UserID: userID, RoleID: roleID}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			assert.Equal(t, int64(7), id)
			return nil
		},
	}
	svc := NewUserRoleService(mockUserRoleDao, &mockRoleDao{}, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})

	err := svc.RemoveRole(nil, 1, 2)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
}

func TestUserRoleService_RemoveRole_NotAssigned(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
	}
	svc := NewUserRoleService(mockUserRoleDao, &mockRoleDao{}, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})

	err := svc.RemoveRole(nil, 1, 2)

	assert.Error(t, err)
	assert.Equal(t, "el usuario no tiene asignado este rol", err.Error())
}

func TestUserRoleService_RemoveRole_FindError(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, errors.New("db error")
		},
	}
	svc := NewUserRoleService(mockUserRoleDao, &mockRoleDao{}, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})

	err := svc.RemoveRole(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar rol")
}

func TestUserRoleService_RemoveRole_SoftDeleteError(t *testing.T) {
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{ID: 7}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			return errors.New("db error")
		},
	}
	svc := NewUserRoleService(mockUserRoleDao, &mockRoleDao{}, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})

	err := svc.RemoveRole(nil, 1, 2)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al eliminar rol")
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

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
	_, err := svc.AssignRole(nil, 1, &userrole.AssignRoleRequest{
		RoleID: 1,
		TierID: 1,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al asignar rol")
}

func ptrStr(s string) *string { return &s }

func TestUserRoleService_ActivateEntrenador_Success(t *testing.T) {
	bankAlias := "mi.alias-1"
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John", Password: entrenadorTestPasswordHash}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 9, Name: "base", RoleID: roleID}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, ur *dbs.UserRole) error {
			ur.ID = 1
			return nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
	resp, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{
		Password:  "CorrectPass123",
		BankAlias: &bankAlias,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(2), resp.RoleID)
}

func TestUserRoleService_ActivateEntrenador_UsesExistingBankAlias(t *testing.T) {
	existingAlias := "ya.guardado"
	updateCalled := false
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Name: "John", Password: entrenadorTestPasswordHash, BankAlias: &existingAlias}, nil
		},
		updateFn: func(ctx *gin.Context, user *dbs.User) error {
			updateCalled = true
			return nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByNameAndRoleFn: func(ctx *gin.Context, name string, roleID int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: 9, Name: "base", RoleID: roleID}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
		createFn: func(ctx *gin.Context, ur *dbs.UserRole) error {
			ur.ID = 1
			return nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, mockTierDao, mockUserDao, &mockTeamUserDao{})
	resp, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{
		Password: "CorrectPass123",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, updateCalled)
}

func TestUserRoleService_ActivateEntrenador_UserNotFound(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 999, &userrole.ActivateEntrenadorRequest{Password: "x"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usuario no encontrado")
}

func TestUserRoleService_ActivateEntrenador_UserFindByIDError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{Password: "x"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al activar rol entrenador")
}

func TestUserRoleService_ActivateEntrenador_WrongPassword(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Password: entrenadorTestPasswordHash}, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{Password: "WrongPass"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contraseña actual incorrecta")
}

func TestUserRoleService_ActivateEntrenador_MissingBankAlias(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Password: entrenadorTestPasswordHash}, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{Password: "CorrectPass123"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "se requiere un alias bancario")
}

func TestUserRoleService_ActivateEntrenador_InvalidBankAliasFormat(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Password: entrenadorTestPasswordHash}, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, &mockRoleDao{}, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{
		Password:  "CorrectPass123",
		BankAlias: ptrStr("a"),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bank_alias debe tener entre 6 y 20 caracteres")
}

func TestUserRoleService_ActivateEntrenador_RoleNotFoundInCatalog(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Password: entrenadorTestPasswordHash}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{
		Password:  "CorrectPass123",
		BankAlias: ptrStr("alias-valido"),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al activar rol entrenador")
}

func TestUserRoleService_ActivateEntrenador_RoleFindByNameError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Password: entrenadorTestPasswordHash}, nil
		},
	}
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{
		Password:  "CorrectPass123",
		BankAlias: ptrStr("alias-valido"),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al activar rol entrenador")
}

func TestUserRoleService_ActivateEntrenador_UpdateBankAliasError(t *testing.T) {
	mockUserDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: 1, Password: entrenadorTestPasswordHash}, nil
		},
		updateFn: func(ctx *gin.Context, user *dbs.User) error {
			return errors.New("db error")
		},
	}
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, mockUserDao, &mockTeamUserDao{})
	_, err := svc.ActivateEntrenador(nil, 1, &userrole.ActivateEntrenadorRequest{
		Password:  "CorrectPass123",
		BankAlias: ptrStr("alias-valido"),
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al activar rol entrenador")
}

func TestUserRoleService_DeactivateEntrenador_Success(t *testing.T) {
	softDeleteCalled := false
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{{TeamID: 1, UserID: userID, RoleInTeam: "corredor"}}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{ID: 5, UserID: userID, RoleID: roleID}, nil
		},
		softDeleteFn: func(ctx *gin.Context, id int64) error {
			softDeleteCalled = true
			return nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, mockTeamUserDao)
	err := svc.DeactivateEntrenador(nil, 1)

	assert.NoError(t, err)
	assert.True(t, softDeleteCalled)
}

func TestUserRoleService_DeactivateEntrenador_BlockedByActiveTeam(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error) {
			return []dbs.TeamUser{{TeamID: 1, UserID: userID, RoleInTeam: "entrenador"}}, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, mockTeamUserDao)
	err := svc.DeactivateEntrenador(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no podés desactivar el rol entrenador mientras lideres equipos activos")
}

func TestUserRoleService_DeactivateEntrenador_RoleNotFoundInCatalog(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, nil
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})
	err := svc.DeactivateEntrenador(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al desactivar rol entrenador")
}

func TestUserRoleService_DeactivateEntrenador_RoleFindByNameError(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})
	err := svc.DeactivateEntrenador(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al desactivar rol entrenador")
}

func TestUserRoleService_DeactivateEntrenador_TeamCheckError(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
	}
	mockTeamUserDao := &mockTeamUserDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.TeamUser, error) {
			return nil, errors.New("db error")
		},
	}

	svc := NewUserRoleService(&mockUserRoleDao{}, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, mockTeamUserDao)
	err := svc.DeactivateEntrenador(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error al desactivar rol entrenador")
}

func TestUserRoleService_DeactivateEntrenador_NotAssigned(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 2, Name: "entrenador"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return nil, nil
		},
	}

	svc := NewUserRoleService(mockUserRoleDao, mockRoleDao, &mockTierDao{}, &mockUserDaoForUserRole{}, &mockTeamUserDao{})
	err := svc.DeactivateEntrenador(nil, 1)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "el usuario no tiene asignado este rol")
}
