package services

import (
	"fmt"
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
