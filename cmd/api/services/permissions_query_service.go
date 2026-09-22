package services

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type PermissionsQueryResponse struct {
	UserID int64            `json:"user_id"`
	Roles  []RolePermission `json:"roles"`
}

type RolePermission struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Tier        string   `json:"tier"`
	Permissions []string `json:"permissions"`
}

type PermissionsQueryServiceInterface interface {
	GetUserPermissions(ctx *gin.Context, userID int64) (*PermissionsQueryResponse, error)
}

type permissionsQueryService struct {
	userDao             daos.UserDaoInterface
	userRoleDao         daos.UserRoleDaoInterface
	roleDao             daos.RoleDaoInterface
	tierDao             daos.TierDaoInterface
	tierPermissionDao   daos.TierPermissionDaoInterface
	permissionDao       daos.PermissionDaoInterface
	tierSubDao          daos.TierSubscriptionDaoInterface
}

func NewPermissionsQueryService(
	userDao daos.UserDaoInterface,
	userRoleDao daos.UserRoleDaoInterface,
	roleDao daos.RoleDaoInterface,
	tierDao daos.TierDaoInterface,
	tierPermissionDao daos.TierPermissionDaoInterface,
	permissionDao daos.PermissionDaoInterface,
	tierSubDao daos.TierSubscriptionDaoInterface,
) PermissionsQueryServiceInterface {
	return &permissionsQueryService{
		userDao:           userDao,
		userRoleDao:       userRoleDao,
		roleDao:           roleDao,
		tierDao:           tierDao,
		tierPermissionDao: tierPermissionDao,
		permissionDao:     permissionDao,
		tierSubDao:        tierSubDao,
	}
}

func (s *permissionsQueryService) GetUserPermissions(ctx *gin.Context, userID int64) (*PermissionsQueryResponse, error) {
	user, err := s.userDao.FindByID(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding user for permissions query", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetUserPermissions"))
		return nil, fmt.Errorf("error al obtener permisos")
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	userRoles, err := s.userRoleDao.FindByUserID(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding user roles", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetUserPermissions"))
		return nil, fmt.Errorf("error al obtener permisos")
	}

	if len(userRoles) == 0 {
		return &PermissionsQueryResponse{
			UserID: userID,
			Roles:  []RolePermission{},
		}, nil
	}

	var missingData []string
	var roles []RolePermission

	for _, ur := range userRoles {
		role, err := s.roleDao.FindByID(ctx, ur.RoleID)
		if err != nil || role == nil {
			missingData = append(missingData, fmt.Sprintf("rol_id=%d no configurado", ur.RoleID))
			customlogger.Error(ctx, "role not found for user role assignment", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.Tag("role_id", fmt.Sprintf("%d", ur.RoleID)),
				customlogger.TagMethod("GetUserPermissions"))
			continue
		}

		// El tier efectivo se resuelve desde la sub ACTIVA (ya pagada) del ledger,
		// igual que GetCurrentSubscription(period=current). Sin sub activa, el
		// usuario está en el tier base de su rol (menor jerarquía en `tiers`),
		// como en el resto de la resolución de tier inicial (user_role_service,
		// AssignRole). NO se usa user_roles.tier_id como fallback: es un caché
		// que puede conservar un tier pago de una activación vieja sin que hoy
		// exista sub que lo respalde — el estado real sin sub activa es base.
		// Una sub first_payment_pending tampoco cuenta: la cuota #1 impaga no
		// habilita el acceso al tier pago (ver docs/STATE_MACHINES.md). El
		// índice único parcial de subs vigentes garantiza a lo sumo una sub.
		var tierID int64
		sub, err := s.tierSubDao.FindActiveByUserRole(ctx, userID, ur.RoleID, string(constants.SubscriptionStatusActive))
		if err != nil {
			customlogger.Error(ctx, "error finding active tier subscription", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.Tag("role_id", fmt.Sprintf("%d", ur.RoleID)),
				customlogger.TagMethod("GetUserPermissions"))
			return nil, fmt.Errorf("error al obtener permisos")
		}
		if sub != nil {
			tierID = sub.TierID
		} else {
			baseTier, err := s.tierDao.FindLowestByRole(ctx, ur.RoleID)
			if err != nil {
				customlogger.Error(ctx, "error finding base tier by role", err,
					customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
					customlogger.Tag("role_id", fmt.Sprintf("%d", ur.RoleID)),
					customlogger.TagMethod("GetUserPermissions"))
				return nil, fmt.Errorf("error al obtener permisos")
			}
			if baseTier == nil {
				missingData = append(missingData, fmt.Sprintf("tier base no configurado para el rol %s", role.Name))
				customlogger.Error(ctx, "no base tier configured for role", nil,
					customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
					customlogger.Tag("role_id", fmt.Sprintf("%d", ur.RoleID)),
					customlogger.TagMethod("GetUserPermissions"))
				continue
			}
			tierID = baseTier.ID
		}

		tier, err := s.tierDao.FindByID(ctx, tierID)
		if err != nil || tier == nil {
			missingData = append(missingData, fmt.Sprintf("tier_id=%d no configurado para el rol %s", tierID, role.Name))
			customlogger.Error(ctx, "tier not found for user role assignment", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
				customlogger.TagMethod("GetUserPermissions"))
			continue
		}

		tierPermissions, err := s.tierPermissionDao.FindByTierID(ctx, tier.ID)
		if err != nil {
			customlogger.Error(ctx, "error finding tier permissions", err,
				customlogger.Tag("tier_id", fmt.Sprintf("%d", tier.ID)),
				customlogger.TagMethod("GetUserPermissions"))
			continue
		}

		if len(tierPermissions) == 0 {
			missingData = append(missingData, fmt.Sprintf("tier '%s' del rol '%s' no tiene permisos asociados", tier.Name, role.Name))
			customlogger.Warn(ctx, "tier has no permissions",
				customlogger.Tag("tier_id", fmt.Sprintf("%d", tier.ID)),
				customlogger.Tag("tier_name", tier.Name),
				customlogger.TagMethod("GetUserPermissions"))
		}

		var permissionNames []string
		for _, tp := range tierPermissions {
			perm, err := s.permissionDao.FindByID(ctx, tp.PermissionID)
			if err != nil || perm == nil {
				missingData = append(missingData, fmt.Sprintf("permiso_id=%d no configurado", tp.PermissionID))
				continue
			}
			permissionNames = append(permissionNames, perm.Name)
		}

		roles = append(roles, RolePermission{
			ID:          role.ID,
			Name:        role.Name,
			Tier:        tier.Name,
			Permissions: permissionNames,
		})
	}

	if len(missingData) > 0 {
		customlogger.Error(ctx, "missing data found in permissions chain", nil,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("missing", strings.Join(missingData, "; ")),
			customlogger.TagMethod("GetUserPermissions"))
		return nil, fmt.Errorf("datos faltantes: %s", strings.Join(missingData, "; "))
	}

	return &PermissionsQueryResponse{
		UserID: userID,
		Roles:  roles,
	}, nil
}
