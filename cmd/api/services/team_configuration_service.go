package services

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/teamconfiguration"
)

// TeamConfigurationServiceInterface devuelve la configuración de creación/
// edición de equipos según el tier del entrenador (rol "entrenador").
type TeamConfigurationServiceInterface interface {
	GetTeamConfiguration(ctx *gin.Context, userID int64) (*teamconfiguration.TeamConfiguration, error)
}

type teamConfigurationService struct {
	roleDao     daos.RoleDaoInterface
	userRoleDao daos.UserRoleDaoInterface
	tierSubDao  daos.TierSubscriptionDaoInterface
	tierDao     daos.TierDaoInterface
}

func NewTeamConfigurationService(
	roleDao daos.RoleDaoInterface,
	userRoleDao daos.UserRoleDaoInterface,
	tierSubDao daos.TierSubscriptionDaoInterface,
	tierDao daos.TierDaoInterface,
) TeamConfigurationServiceInterface {
	return &teamConfigurationService{
		roleDao:     roleDao,
		userRoleDao: userRoleDao,
		tierSubDao:  tierSubDao,
		tierDao:     tierDao,
	}
}

// GetTeamConfiguration resuelve la config de equipo del entrenador (identidad
// tomada del access token) según su tier. El tier se resuelve igual que
// GetCurrentSubscription (suscripción vigente → user_roles.tier_id); si no hay
// tier resuelto o el tier no está en el mapa, devuelve el default.
func (s *teamConfigurationService) GetTeamConfiguration(ctx *gin.Context, userID int64) (*teamconfiguration.TeamConfiguration, error) {
	tier, err := s.resolveEntrenadorTier(ctx, userID)
	if err != nil {
		return nil, err
	}
	if tier == nil {
		cfg := teamconfiguration.ForTier("")
		return &cfg, nil
	}

	cfg := teamconfiguration.ForTier(tier.Name)
	return &cfg, nil
}

// resolveEntrenadorTier devuelve el tier vigente del usuario para el rol
// "entrenador", o nil si no se puede resolver (rol inexistente, sin asignación
// ni suscripción). Mismo criterio que GetCurrentSubscription: sub vigente
// primero, después user_roles.tier_id.
func (s *teamConfigurationService) resolveEntrenadorTier(ctx *gin.Context, userID int64) (*dbs.Tier, error) {
	role, err := s.roleDao.FindByName(ctx, teamOwnerRoleName)
	if err != nil {
		return nil, fmt.Errorf("error al obtener la configuración")
	}
	if role == nil {
		return nil, nil
	}

	sub, err := s.tierSubDao.FindActiveByUserRole(ctx, userID, role.ID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener la configuración")
	}

	var tierID int64
	if sub != nil {
		tierID = sub.TierID
	} else {
		ur, err := s.userRoleDao.FindByUserAndRole(ctx, userID, role.ID)
		if err != nil {
			return nil, fmt.Errorf("error al obtener la configuración")
		}
		if ur == nil {
			return nil, nil
		}
		tierID = ur.TierID
	}

	tier, err := s.tierDao.FindByID(ctx, tierID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener la configuración")
	}
	return tier, nil
}
