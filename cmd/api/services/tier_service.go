package services

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/tier"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type TierServiceInterface interface {
	Create(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error)
	Update(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error)
	Delete(ctx *gin.Context, id int64) (*tier.DeleteTierResponse, error)
	GetByID(ctx *gin.Context, id int64) (*tier.TierResponse, error)
	GetByName(ctx *gin.Context, name string) (*tier.TierResponse, error)
	GetAll(ctx *gin.Context) ([]tier.TierResponse, error)
}

type tierService struct {
	tierDao daos.TierDaoInterface
	roleDao daos.RoleDaoInterface
}

func NewTierService(tierDao daos.TierDaoInterface, roleDao daos.RoleDaoInterface) TierServiceInterface {
	return &tierService{
		tierDao: tierDao,
		roleDao: roleDao,
	}
}

// tierRulesByName devuelve la jerarquía y si debe forzarse la gratuidad según la
// primera palabra del nombre del tier (regla D11): base=1 (gratis), medium=2, premium=3.
func tierRulesByName(name string) (hierarchy int, forceFree bool) {
	first := strings.ToLower(strings.TrimSpace(name))
	if parts := strings.Fields(first); len(parts) > 0 {
		first = parts[0]
	}
	switch first {
	case "base":
		return 1, true
	case "medium":
		return 2, false
	case "premium":
		return 3, false
	default:
		return 0, false
	}
}

func (s *tierService) Create(ctx *gin.Context, req *tier.CreateTierRequest) (*tier.TierResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, fmt.Errorf("el nombre es requerido")
	}

	role, err := s.roleDao.FindByID(ctx, req.RoleID)
	if err != nil {
		customlogger.Error(ctx, "error finding role for tier", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear tier")
	}
	if role == nil {
		return nil, fmt.Errorf("rol no encontrado")
	}

	existing, err := s.tierDao.FindByNameAndRole(ctx, name, req.RoleID)
	if err != nil {
		customlogger.Error(ctx, "error checking tier name", err,
			customlogger.Tag("name", name),
			customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear tier")
	}
	if existing != nil {
		return nil, fmt.Errorf("ya existe un tier con ese nombre para este rol")
	}

	hierarchy, forceFree := tierRulesByName(name)
	paymentRequired := req.PaymentRequired
	if forceFree {
		paymentRequired = false
	}

	t := &dbs.Tier{
		Name:            name,
		Description:     strings.TrimSpace(req.Description),
		RoleID:          req.RoleID,
		RoleName:        role.Name,
		PaymentRequired: paymentRequired,
		TierAmount:      req.TierAmount,
		Hierarchy:       hierarchy,
	}

	if err := s.tierDao.Create(ctx, t); err != nil {
		customlogger.Error(ctx, "error creating tier", err,
			customlogger.Tag("name", name),
			customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
			customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear tier")
	}

	customlogger.Info(ctx, "tier created successfully",
		customlogger.Tag("tier_id", fmt.Sprintf("%d", t.ID)),
		customlogger.Tag("name", t.Name),
		customlogger.Tag("role_name", t.RoleName),
		customlogger.TagMethod("Create"))

	return &tier.TierResponse{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		RoleID:          t.RoleID,
		RoleName:        t.RoleName,
		PaymentRequired: t.PaymentRequired,
		TierAmount:      t.TierAmount,
		Hierarchy:       t.Hierarchy,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}, nil
}

func (s *tierService) Update(ctx *gin.Context, id int64, req *tier.UpdateTierRequest) (*tier.TierResponse, error) {
	t, err := s.tierDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding tier for update", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar tier")
	}
	if t == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	if req.Name != nil {
		newName := strings.TrimSpace(*req.Name)
		if newName == "" {
			return nil, fmt.Errorf("el nombre no puede estar vacío")
		}
		if newName != t.Name {
			existing, err := s.tierDao.FindByNameAndRole(ctx, newName, t.RoleID)
			if err != nil {
				customlogger.Error(ctx, "error checking tier name", err,
					customlogger.Tag("name", newName),
					customlogger.TagMethod("Update"))
				return nil, fmt.Errorf("error al actualizar tier")
			}
			if existing != nil {
				return nil, fmt.Errorf("ya existe un tier con ese nombre para este rol")
			}
			t.Name = newName
		}
	}

	if req.Description != nil {
		t.Description = strings.TrimSpace(*req.Description)
	}

	if req.PaymentRequired != nil {
		t.PaymentRequired = *req.PaymentRequired
	}

	if req.TierAmount != nil {
		t.TierAmount = *req.TierAmount
	}

	// Regla D11: mantener jerarquía y gratuidad en sincronía con el nombre final
	// (aplica incluso si el nombre no cambió en este request, para no dejar drift).
	hierarchy, forceFree := tierRulesByName(t.Name)
	if hierarchy > 0 {
		t.Hierarchy = hierarchy
		if forceFree {
			t.PaymentRequired = false
		}
	}

	if err := s.tierDao.Update(ctx, t); err != nil {
		customlogger.Error(ctx, "error updating tier", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al actualizar tier")
	}

	customlogger.Info(ctx, "tier updated successfully",
		customlogger.Tag("tier_id", fmt.Sprintf("%d", t.ID)),
		customlogger.Tag("name", t.Name),
		customlogger.TagMethod("Update"))

	return &tier.TierResponse{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		RoleID:          t.RoleID,
		RoleName:        t.RoleName,
		PaymentRequired: t.PaymentRequired,
		TierAmount:      t.TierAmount,
		Hierarchy:       t.Hierarchy,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}, nil
}

func (s *tierService) Delete(ctx *gin.Context, id int64) (*tier.DeleteTierResponse, error) {
	t, err := s.tierDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding tier for delete", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return nil, fmt.Errorf("error al eliminar tier")
	}
	if t == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	if err := s.tierDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error deleting tier", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("Delete"))
		return nil, fmt.Errorf("error al eliminar tier")
	}

	customlogger.Info(ctx, "tier deleted successfully",
		customlogger.Tag("tier_id", fmt.Sprintf("%d", id)),
		customlogger.TagMethod("Delete"))

	return &tier.DeleteTierResponse{
		Message: "Tier eliminado correctamente",
	}, nil
}

func (s *tierService) GetByID(ctx *gin.Context, id int64) (*tier.TierResponse, error) {
	t, err := s.tierDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding tier by id", err,
			customlogger.Tag("tier_id", fmt.Sprintf("%d", id)),
			customlogger.TagMethod("GetByID"))
		return nil, fmt.Errorf("error al obtener tier")
	}
	if t == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	return &tier.TierResponse{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		RoleID:          t.RoleID,
		RoleName:        t.RoleName,
		PaymentRequired: t.PaymentRequired,
		TierAmount:      t.TierAmount,
		Hierarchy:       t.Hierarchy,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}, nil
}

func (s *tierService) GetByName(ctx *gin.Context, name string) (*tier.TierResponse, error) {
	t, err := s.tierDao.FindByName(ctx, name)
	if err != nil {
		customlogger.Error(ctx, "error finding tier by name", err,
			customlogger.Tag("name", name),
			customlogger.TagMethod("GetByName"))
		return nil, fmt.Errorf("error al obtener tier")
	}
	if t == nil {
		return nil, fmt.Errorf("tier no encontrado")
	}

	return &tier.TierResponse{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		RoleID:          t.RoleID,
		RoleName:        t.RoleName,
		PaymentRequired: t.PaymentRequired,
		TierAmount:      t.TierAmount,
		Hierarchy:       t.Hierarchy,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}, nil
}

func (s *tierService) GetAll(ctx *gin.Context) ([]tier.TierResponse, error) {
	tiers, err := s.tierDao.GetAll(ctx)
	if err != nil {
		customlogger.Error(ctx, "error getting all tiers", err,
			customlogger.TagMethod("GetAll"))
		return nil, fmt.Errorf("error al obtener tiers")
	}

	var responses []tier.TierResponse
	for _, t := range tiers {
		responses = append(responses, tier.TierResponse{
			ID:              t.ID,
			Name:            t.Name,
			Description:     t.Description,
			RoleID:          t.RoleID,
			RoleName:        t.RoleName,
			PaymentRequired: t.PaymentRequired,
			TierAmount:      t.TierAmount,
			Hierarchy:       t.Hierarchy,
			CreatedAt:       t.CreatedAt,
			UpdatedAt:       t.UpdatedAt,
		})
	}

	if responses == nil {
		responses = []tier.TierResponse{}
	}

	return responses, nil
}
