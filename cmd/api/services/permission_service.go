package services

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/permission"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type PermissionServiceInterface interface {
	Create(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error)
	Update(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error)
	Delete(ctx *gin.Context, id int64) (*permission.DeletePermissionResponse, error)
	GetByID(ctx *gin.Context, id int64) (*permission.PermissionResponse, error)
	GetByName(ctx *gin.Context, name string) (*permission.PermissionResponse, error)
	GetAll(ctx *gin.Context) ([]permission.PermissionResponse, error)
}

type permissionService struct {
	permissionDao daos.PermissionDaoInterface
}

func NewPermissionService(permissionDao daos.PermissionDaoInterface) PermissionServiceInterface {
	return &permissionService{
		permissionDao: permissionDao,
	}
}

func (s *permissionService) Create(ctx *gin.Context, req *permission.CreatePermissionRequest) (*permission.PermissionResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("el nombre es requerido")
	}

	existing, err := s.permissionDao.FindByName(ctx, name)
	if err != nil {
		customlogger.Error(ctx, "error checking permission name", err,
			customlogger.Tag("name", name),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear permiso")
	}
	if existing != nil {
		return nil, fmt.Errorf("el nombre del permiso ya existe")
	}

	perm := &dbs.Permission{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
	}

	if err := s.permissionDao.Create(ctx, perm); err != nil {
		customlogger.Error(ctx, "error creating permission", err,
			customlogger.Tag("name", name),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear permiso")
	}

	customlogger.Info(ctx, "permission created successfully",
		customlogger.Tag("permission_id", fmt.Sprintf("%d", perm.ID)),
		customlogger.Tag("name", perm.Name),
		customlogger.TagMethod("Create"))

	return &permission.PermissionResponse{
		ID:          perm.ID,
		Name:        perm.Name,
		Description: perm.Description,
		CreatedAt:   perm.CreatedAt,
		UpdatedAt:   perm.UpdatedAt,
	}, nil
}

func (s *permissionService) Update(ctx *gin.Context, id int64, req *permission.UpdatePermissionRequest) (*permission.PermissionResponse, error) {
	perm, err := s.permissionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding permission for update", err,
			customlogger.Tag("permission_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar permiso")
	}
	if perm == nil {
		return nil, fmt.Errorf("permiso no encontrado")
	}

	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return nil, fmt.Errorf("el nombre no puede estar vacío")
		}
		if newName != perm.Name {
			existing, err := s.permissionDao.FindByName(ctx, newName)
			if err != nil {
				customlogger.Error(ctx, "error checking permission name", err,
					customlogger.Tag("name", newName),
					customlogger.TagMethod("Update"))
				return nil, fmt.Errorf("error al actualizar permiso")
			}
			if existing != nil {
				return nil, fmt.Errorf("el nombre del permiso ya existe")
			}
			perm.Name = newName
		}
	}

	if req.Description != nil {
		perm.Description = strings.TrimSpace(*req.Description)
	}

	if err := s.permissionDao.Update(ctx, perm); err != nil {
		customlogger.Error(ctx, "error updating permission", err,
			customlogger.Tag("permission_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar permiso")
	}

	customlogger.Info(ctx, "permission updated successfully",
		customlogger.Tag("permission_id", fmt.Sprintf("%d", perm.ID)),
		customlogger.Tag("name", perm.Name),
		customlogger.TagMethod("Update"))

	return &permission.PermissionResponse{
		ID:          perm.ID,
		Name:        perm.Name,
		Description: perm.Description,
		CreatedAt:   perm.CreatedAt,
		UpdatedAt:   perm.UpdatedAt,
	}, nil
}

func (s *permissionService) Delete(ctx *gin.Context, id int64) (*permission.DeletePermissionResponse, error) {
	perm, err := s.permissionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding permission for delete", err,
			customlogger.Tag("permission_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return nil, fmt.Errorf("error al eliminar permiso")
	}
	if perm == nil {
		return nil, fmt.Errorf("permiso no encontrado")
	}

	if err := s.permissionDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error deleting permission", err,
			customlogger.Tag("permission_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return nil, fmt.Errorf("error al eliminar permiso")
	}

	customlogger.Info(ctx, "permission deleted successfully",
		customlogger.Tag("permission_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("Delete"))

	return &permission.DeletePermissionResponse{
		Message: "Permiso eliminado correctamente",
	}, nil
}

func (s *permissionService) GetByID(ctx *gin.Context, id int64) (*permission.PermissionResponse, error) {
	p, err := s.permissionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding permission by id", err,
			customlogger.Tag("permission_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("GetByID"))
		return nil, fmt.Errorf("error al obtener permiso")
	}
	if p == nil {
		return nil, fmt.Errorf("permiso no encontrado")
	}

	return &permission.PermissionResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}, nil
}

func (s *permissionService) GetByName(ctx *gin.Context, name string) (*permission.PermissionResponse, error) {
	p, err := s.permissionDao.FindByName(ctx, name)
	if err != nil {
		customlogger.Error(ctx, "error finding permission by name", err,
			customlogger.Tag("name", name),
			customlogger.TagMethod("GetByName"))
		return nil, fmt.Errorf("error al obtener permiso")
	}
	if p == nil {
		return nil, fmt.Errorf("permiso no encontrado")
	}

	return &permission.PermissionResponse{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}, nil
}

func (s *permissionService) GetAll(ctx *gin.Context) ([]permission.PermissionResponse, error) {
	permissions, err := s.permissionDao.GetAll(ctx)
	if err != nil {
		customlogger.Error(ctx, "error getting all permissions", err,
			customlogger.TagMethod("GetAll"))
		return nil, fmt.Errorf("error al obtener permisos")
	}

	var responses []permission.PermissionResponse
	for _, p := range permissions {
		responses = append(responses, permission.PermissionResponse{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	if responses == nil {
		responses = []permission.PermissionResponse{}
	}

	return responses, nil
}
