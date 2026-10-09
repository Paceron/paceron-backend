package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/tierpermission"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type TierPermissionServiceInterface interface {
	Assign(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error)
	Unassign(ctx *gin.Context, tierID, permissionID int64) (*tierpermission.DeleteTierPermissionResponse, error)
	List(ctx *gin.Context, tierID int64) (*tierpermission.ListTierPermissionsResponse, error)
}

type tierPermissionService struct {
	tierPermissionDao daos.TierPermissionDaoInterface
	tierDao           daos.TierDaoInterface
	permissionDao     daos.PermissionDaoInterface
}

func NewTierPermissionService(
	tierPermissionDao daos.TierPermissionDaoInterface,
	tierDao daos.TierDaoInterface,
	permissionDao daos.PermissionDaoInterface,
) TierPermissionServiceInterface {
	return &tierPermissionService{
		tierPermissionDao: tierPermissionDao,
		tierDao:           tierDao,
		permissionDao:     permissionDao,
	}
}

func (s *tierPermissionService) Assign(ctx *gin.Context, tierID int64, req *tierpermission.AssignPermissionRequest) (*tierpermission.TierPermissionResponse, error) {
	t, err := s.tierDao.FindByID(ctx, tierID)
	if err != nil {
		customlogger.Error(ctx, "error finding tier for permission assignment", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
			customlogger.TagMethod("Assign"))
		return nil, fmt.Errorf("error al asignar permiso")
	}
	if t == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	perm, err := s.permissionDao.FindByID(ctx, req.PermissionID)
	if err != nil {
		customlogger.Error(ctx, "error finding permission for assignment", err,
			customlogger.Tag("permission_id", fmt.Sprintf("%d", req.PermissionID)),
			customlogger.TagMethod("Assign"))
		return nil, fmt.Errorf("error al asignar permiso")
	}
	if perm == nil {
		return nil, fmt.Errorf("permiso no encontrado")
	}

	existing, err := s.tierPermissionDao.FindByTierAndPermission(ctx, tierID, req.PermissionID)
	if err != nil {
		customlogger.Error(ctx, "error checking existing assignment", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
			customlogger.Tag("permission_id", fmt.Sprintf("%d", req.PermissionID)),
			customlogger.TagMethod("Assign"))
		return nil, fmt.Errorf("error al asignar permiso")
	}
	if existing != nil {
		return nil, fmt.Errorf("el permiso ya está asignado a este tier")
	}

	tp := &dbs.TierPermission{
		TierID:         tierID,
		PermissionID:   req.PermissionID,
		AsignationDate: time.Now(),
	}

	if err := s.tierPermissionDao.Create(ctx, tp); err != nil {
		customlogger.Error(ctx, "error assigning permission to tier", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
			customlogger.Tag("permission_id", fmt.Sprintf("%d", req.PermissionID)),
			customlogger.TagMethod("Assign"))
		return nil, fmt.Errorf("error al asignar permiso")
	}

	customlogger.Info(ctx, "permission assigned to tier successfully",
		customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
		customlogger.Tag("permission_id", fmt.Sprintf("%d", req.PermissionID)),
		customlogger.TagMethod("Assign"))

	return &tierpermission.TierPermissionResponse{
		ID:             tp.ID,
		TierID:         tp.TierID,
		PermissionID:   tp.PermissionID,
		AsignationDate: tp.AsignationDate,
	}, nil
}

func (s *tierPermissionService) Unassign(ctx *gin.Context, tierID, permissionID int64) (*tierpermission.DeleteTierPermissionResponse, error) {
	existing, err := s.tierPermissionDao.FindByTierAndPermission(ctx, tierID, permissionID)
	if err != nil {
		customlogger.Error(ctx, "error finding assignment for unassign", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
			customlogger.Tag("permission_id", fmt.Sprintf("%d", permissionID)),
			customlogger.TagMethod("Unassign"))
		return nil, fmt.Errorf("error al desasignar permiso")
	}
	if existing == nil {
		return nil, fmt.Errorf("asignación no encontrada")
	}

	if err := s.tierPermissionDao.SoftDelete(ctx, existing.ID); err != nil {
		customlogger.Error(ctx, "error unassigning permission from tier", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
			customlogger.Tag("permission_id", fmt.Sprintf("%d", permissionID)),
			customlogger.TagMethod("Unassign"))
		return nil, fmt.Errorf("error al desasignar permiso")
	}

	customlogger.Info(ctx, "permission unassigned from tier successfully",
		customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
		customlogger.Tag("permission_id", fmt.Sprintf("%d", permissionID)),
		customlogger.TagMethod("Unassign"))

	return &tierpermission.DeleteTierPermissionResponse{
		Message: "Permiso desasignado del tier correctamente",
	}, nil
}

func (s *tierPermissionService) List(ctx *gin.Context, tierID int64) (*tierpermission.ListTierPermissionsResponse, error) {
	t, err := s.tierDao.FindByID(ctx, tierID)
	if err != nil {
		customlogger.Error(ctx, "error finding tier for permission list", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
			customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar permisos")
	}
	if t == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	assignments, err := s.tierPermissionDao.FindByTierID(ctx, tierID)
	if err != nil {
		customlogger.Error(ctx, "error finding permissions of tier", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
			customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar permisos")
	}

	items := make([]tierpermission.TierPermissionListItem, 0, len(assignments))
	for _, a := range assignments {
		perm, err := s.permissionDao.FindByID(ctx, a.PermissionID)
		if err != nil {
			customlogger.Error(ctx, "error resolving permission name", err,
				customlogger.Tag("permission_id", fmt.Sprintf("%d", a.PermissionID)),
				customlogger.TagMethod("List"))
			return nil, fmt.Errorf("error al listar permisos")
		}
		// un permiso soft-deleted dejó de estar activo: se omite
		if perm == nil {
			continue
		}
		items = append(items, tierpermission.TierPermissionListItem{
			PermissionID:   a.PermissionID,
			PermissionName: perm.Name,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].PermissionID < items[j].PermissionID
	})

	return &tierpermission.ListTierPermissionsResponse{Permissions: items}, nil
}
