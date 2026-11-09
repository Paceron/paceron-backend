package services

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
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
	userDao           daos.UserDaoInterface
	userRoleDao       daos.UserRoleDaoInterface
	roleDao           daos.RoleDaoInterface
	tierDao           daos.TierDaoInterface
	tierPermissionDao daos.TierPermissionDaoInterface
	permissionDao     daos.PermissionDaoInterface
	tierSubDao        daos.TierSubscriptionDaoInterface
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

	roleIDs := make([]int64, 0, len(userRoles))
	for _, ur := range userRoles {
		roleIDs = append(roleIDs, ur.RoleID)
	}

	roleRows, err := s.roleDao.FindByIDs(ctx, roleIDs)
	if err != nil {
		// La lectura batch no puede atribuir el error a una fila: se reportan
		// todos los roles como no configurados (mismo orden de user_roles), que
		// es lo que hacía el FindByID por rol al fallar.
		customlogger.Error(ctx, "error finding roles for permissions query", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetUserPermissions"))
		missingData := make([]string, 0, len(userRoles))
		for _, ur := range userRoles {
			missingData = append(missingData, fmt.Sprintf("rol_id=%d no configurado", ur.RoleID))
		}
		return nil, fmt.Errorf("datos faltantes: %s", strings.Join(missingData, "; "))
	}
	rolesByID := make(map[int64]dbs.Role, len(roleRows))
	for _, role := range roleRows {
		rolesByID[role.ID] = role
	}

	subs, err := s.tierSubDao.FindActiveByUserRoles(ctx, userID, roleIDs, string(constants.SubscriptionStatusActive))
	if err != nil {
		customlogger.Error(ctx, "error finding active tier subscriptions", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("GetUserPermissions"))
		return nil, fmt.Errorf("error al obtener permisos")
	}
	activeTierByRole := make(map[int64]int64, len(subs))
	for _, sub := range subs {
		activeTierByRole[sub.RoleID] = sub.TierID
	}

	// El tier efectivo se resuelve desde la sub ACTIVA (ya pagada) del ledger,
	// igual que GetCurrentSubscription(period=current). Sin sub activa, el
	// usuario está en el tier base de su rol (menor jerarquía en `tiers`),
	// como en el resto de la resolución de tier inicial (user_role_service,
	// AssignRole). NO se usa user_roles.tier_id como fallback: es un caché
	// que puede conservar un tier pago de una activación vieja sin que hoy
	// exista sub que lo respalde — el estado real sin sub activa es base.
	// Una sub first_payment_pending tampoco cuenta: la cuota #1 impaga no
	// habilita el acceso al tier pago (ver docs/STATE_MACHINES.md). Durante
	// la ventana pending la sub del anterior sigue active (ChangeTier ya no la
	// cierra al crear el pendiente), así que esta rama sigue reportando el tier
	// previo; el cierre a ended ocurre en applyApprovedInstallment, cuando la
	// cuota #1 confirma la nueva.
	baseRoleIDs := make([]int64, 0, len(userRoles))
	seenBaseRole := make(map[int64]struct{}, len(userRoles))
	for _, ur := range userRoles {
		if _, ok := rolesByID[ur.RoleID]; !ok {
			continue
		}
		if _, hasSub := activeTierByRole[ur.RoleID]; hasSub {
			continue
		}
		if _, seen := seenBaseRole[ur.RoleID]; seen {
			continue
		}
		seenBaseRole[ur.RoleID] = struct{}{}
		baseRoleIDs = append(baseRoleIDs, ur.RoleID)
	}

	baseTierByRole := make(map[int64]dbs.Tier, len(baseRoleIDs))
	if len(baseRoleIDs) > 0 {
		baseTiers, err := s.tierDao.FindLowestByRoles(ctx, baseRoleIDs)
		if err != nil {
			customlogger.Error(ctx, "error finding base tiers by roles", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.TagMethod("GetUserPermissions"))
			return nil, fmt.Errorf("error al obtener permisos")
		}
		for _, tier := range baseTiers {
			if _, exists := baseTierByRole[tier.RoleID]; !exists {
				baseTierByRole[tier.RoleID] = tier
			}
		}
	}

	type rolePlan struct {
		role    *dbs.Role
		tierID  int64
		missing string
	}
	plans := make([]rolePlan, len(userRoles))
	tierIDSet := map[int64]struct{}{}
	for i, ur := range userRoles {
		role, ok := rolesByID[ur.RoleID]
		if !ok {
			plans[i].missing = fmt.Sprintf("rol_id=%d no configurado", ur.RoleID)
			continue
		}
		roleCopy := role
		plans[i].role = &roleCopy

		if tierID, hasSub := activeTierByRole[ur.RoleID]; hasSub {
			plans[i].tierID = tierID
		} else {
			baseTier, ok := baseTierByRole[ur.RoleID]
			if !ok {
				plans[i].missing = fmt.Sprintf("tier base no configurado para el rol %s", role.Name)
				continue
			}
			plans[i].tierID = baseTier.ID
		}
		tierIDSet[plans[i].tierID] = struct{}{}
	}

	tierIDs := make([]int64, 0, len(tierIDSet))
	for id := range tierIDSet {
		tierIDs = append(tierIDs, id)
	}
	sort.Slice(tierIDs, func(i, j int) bool { return tierIDs[i] < tierIDs[j] })

	tiersByID := make(map[int64]dbs.Tier, len(tierIDs))
	tierReadFailed := false
	if len(tierIDs) > 0 {
		tierRows, err := s.tierDao.FindByIDs(ctx, tierIDs)
		if err != nil {
			// Igual que el FindByID por rol: el error se refleja como
			// "tier_id=... no configurado" por cada rol afectado (ver loop de
			// plans), no como un 500 genérico.
			customlogger.Error(ctx, "error finding tiers by ids", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.TagMethod("GetUserPermissions"))
			tierReadFailed = true
		} else {
			for _, tier := range tierRows {
				tiersByID[tier.ID] = tier
			}
		}
	}

	// Happy path: una sola lectura batch de permisos. Si falla, se cae al
	// ListPermissionNamesByTier por rol/tier (misma semántica que antes: un
	// error de la lectura de un tier omite su fila) para no perder todos los
	// roles por un fallo puntual del batch.
	batchPermissionsOK := true
	permissionsByTier := make(map[int64][]daos.TierPermissionNameByTier)
	if len(tierIDs) > 0 && !tierReadFailed {
		permissionRows, err := s.tierPermissionDao.ListPermissionNamesByTiers(ctx, tierIDs)
		if err != nil {
			customlogger.Error(ctx, "error finding tier permissions in batch, falling back to per-tier", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.TagMethod("GetUserPermissions"))
			batchPermissionsOK = false
		} else {
			for _, row := range permissionRows {
				permissionsByTier[row.TierID] = append(permissionsByTier[row.TierID], row)
			}
		}
	}

	var missingData []string
	var roles []RolePermission
	for i := range plans {
		plan := plans[i]
		if plan.missing != "" {
			missingData = append(missingData, plan.missing)
			continue
		}

		tier, ok := tiersByID[plan.tierID]
		if !ok {
			missingData = append(missingData, fmt.Sprintf("tier_id=%d no configurado para el rol %s", plan.tierID, plan.role.Name))
			customlogger.Error(ctx, "tier not found for user role assignment", nil,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.Tag("tier_id", fmt.Sprintf("%d", plan.tierID)),
				customlogger.TagMethod("GetUserPermissions"))
			continue
		}

		tierPermissions := permissionsByTier[tier.ID]
		if !batchPermissionsOK {
			names, err := s.tierPermissionDao.ListPermissionNamesByTier(ctx, tier.ID)
			if err != nil {
				customlogger.Error(ctx, "error finding tier permissions", err,
					customlogger.Tag("tier_id", fmt.Sprintf("%d", tier.ID)),
					customlogger.TagMethod("GetUserPermissions"))
				continue
			}
			tierPermissions = make([]daos.TierPermissionNameByTier, 0, len(names))
			for _, name := range names {
				permissionName := name.PermissionName
				tierPermissions = append(tierPermissions, daos.TierPermissionNameByTier{
					TierID:         tier.ID,
					PermissionID:   name.PermissionID,
					PermissionName: &permissionName,
				})
			}
		}
		if len(tierPermissions) == 0 {
			missingData = append(missingData, fmt.Sprintf("tier '%s' del rol '%s' no tiene permisos asociados", tier.Name, plan.role.Name))
			customlogger.Warn(ctx, "tier has no permissions",
				customlogger.Tag("tier_id", fmt.Sprintf("%d", tier.ID)),
				customlogger.Tag("tier_name", tier.Name),
				customlogger.TagMethod("GetUserPermissions"))
		}

		var permissionNames []string
		for _, tp := range tierPermissions {
			if tp.PermissionName == nil {
				missingData = append(missingData, fmt.Sprintf("permiso_id=%d no configurado", tp.PermissionID))
				continue
			}
			permissionNames = append(permissionNames, *tp.PermissionName)
		}

		roles = append(roles, RolePermission{
			ID:          plan.role.ID,
			Name:        plan.role.Name,
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
