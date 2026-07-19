package services

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/role"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type RoleServiceInterface interface {
	Create(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error)
	Update(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error)
	Delete(ctx *gin.Context, id int64) (*role.DeleteRoleResponse, error)
	GetByID(ctx *gin.Context, id int64) (*role.RoleResponse, error)
	GetByName(ctx *gin.Context, name string) (*role.RoleResponse, error)
	GetAll(ctx *gin.Context) ([]role.RoleResponse, error)
}

type roleService struct {
	roleDao daos.RoleDaoInterface
}

func NewRoleService(roleDao daos.RoleDaoInterface) RoleServiceInterface {
	return &roleService{
		roleDao: roleDao,
	}
}

func (s *roleService) Create(ctx *gin.Context, req *role.CreateRoleRequest) (*role.RoleResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("el nombre es requerido")
	}

	existing, err := s.roleDao.FindByName(ctx, name)
	if err != nil {
		customlogger.Error(ctx, "error checking role name", err,
			customlogger.Tag("name", name),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear rol")
	}
	if existing != nil {
		return nil, fmt.Errorf("el nombre del rol ya existe")
	}

	r := &dbs.Role{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
	}

	if err := s.roleDao.Create(ctx, r); err != nil {
		customlogger.Error(ctx, "error creating role", err,
			customlogger.Tag("name", name),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear rol")
	}

	customlogger.Info(ctx, "role created successfully",
		customlogger.Tag("role_id", fmt.Sprintf("%d", r.ID)),
		customlogger.Tag("name", r.Name),
		customlogger.TagMethod("Create"))

	return &role.RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func (s *roleService) Update(ctx *gin.Context, id int64, req *role.UpdateRoleRequest) (*role.RoleResponse, error) {
	r, err := s.roleDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding role for update", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar rol")
	}
	if r == nil {
		return nil, fmt.Errorf("rol no encontrado")
	}

	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return nil, fmt.Errorf("el nombre no puede estar vacío")
		}
		if newName != r.Name {
			existing, err := s.roleDao.FindByName(ctx, newName)
			if err != nil {
				customlogger.Error(ctx, "error checking role name", err,
					customlogger.Tag("name", newName),
					customlogger.TagMethod("Update"))
				return nil, fmt.Errorf("error al actualizar rol")
			}
			if existing != nil {
				return nil, fmt.Errorf("el nombre del rol ya existe")
			}
			r.Name = newName
		}
	}

	if req.Description != nil {
		r.Description = strings.TrimSpace(*req.Description)
	}

	if err := s.roleDao.Update(ctx, r); err != nil {
		customlogger.Error(ctx, "error updating role", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar rol")
	}

	customlogger.Info(ctx, "role updated successfully",
		customlogger.Tag("role_id", fmt.Sprintf("%d", r.ID)),
		customlogger.Tag("name", r.Name),
		customlogger.TagMethod("Update"))

	return &role.RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func (s *roleService) Delete(ctx *gin.Context, id int64) (*role.DeleteRoleResponse, error) {
	r, err := s.roleDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding role for delete", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return nil, fmt.Errorf("error al eliminar rol")
	}
	if r == nil {
		return nil, fmt.Errorf("rol no encontrado")
	}

	if err := s.roleDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error deleting role", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return nil, fmt.Errorf("error al eliminar rol")
	}

	customlogger.Info(ctx, "role deleted successfully",
		customlogger.Tag("role_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("Delete"))

	return &role.DeleteRoleResponse{
		Message: "Rol eliminado correctamente",
	}, nil
}

func (s *roleService) GetByID(ctx *gin.Context, id int64) (*role.RoleResponse, error) {
	r, err := s.roleDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding role by id", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("GetByID"))
		return nil, fmt.Errorf("error al obtener rol")
	}
	if r == nil {
		return nil, fmt.Errorf("rol no encontrado")
	}

	return &role.RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func (s *roleService) GetByName(ctx *gin.Context, name string) (*role.RoleResponse, error) {
	r, err := s.roleDao.FindByName(ctx, name)
	if err != nil {
		customlogger.Error(ctx, "error finding role by name", err,
			customlogger.Tag("name", name),
			customlogger.TagMethod("GetByName"))
		return nil, fmt.Errorf("error al obtener rol")
	}
	if r == nil {
		return nil, fmt.Errorf("rol no encontrado")
	}

	return &role.RoleResponse{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

func (s *roleService) GetAll(ctx *gin.Context) ([]role.RoleResponse, error) {
	roles, err := s.roleDao.GetAll(ctx)
	if err != nil {
		customlogger.Error(ctx, "error getting all roles", err,
			customlogger.TagMethod("GetAll"))
		return nil, fmt.Errorf("error al obtener roles")
	}

	var responses []role.RoleResponse
	for _, r := range roles {
		responses = append(responses, role.RoleResponse{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		})
	}

	if responses == nil {
		responses = []role.RoleResponse{}
	}

	return responses, nil
}
