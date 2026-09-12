package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/teamconfiguration"
)

// ErrTeamNotOwner indica que el usuario no es el dueño del equipo.
var ErrTeamNotOwner = errors.New("solo el dueño del equipo puede consultar su configuración")

// TeamConfigurationServiceInterface devuelve la configuración de creación/
// edición de equipos según el tier del entrenador (rol "entrenador").
type TeamConfigurationServiceInterface interface {
	GetTeamConfiguration(ctx *gin.Context, userID, teamID int64) (*teamconfiguration.TeamConfiguration, error)
}

type teamConfigurationService struct {
	teamDao     daos.TeamDaoInterface
	roleDao     daos.RoleDaoInterface
	userRoleDao daos.UserRoleDaoInterface
	tierSubDao  daos.TierSubscriptionDaoInterface
	tierDao     daos.TierDaoInterface
}

func NewTeamConfigurationService(
	teamDao daos.TeamDaoInterface,
	roleDao daos.RoleDaoInterface,
	userRoleDao daos.UserRoleDaoInterface,
	tierSubDao daos.TierSubscriptionDaoInterface,
	tierDao daos.TierDaoInterface,
) TeamConfigurationServiceInterface {
	return &teamConfigurationService{
		teamDao:     teamDao,
		roleDao:     roleDao,
		userRoleDao: userRoleDao,
		tierSubDao:  tierSubDao,
		tierDao:     tierDao,
	}
}

// GetTeamConfiguration resuelve la config de equipo del entrenador: valida que
// sea dueño del equipo y devuelve el hashmap según su tier. El tier se resuelve
// igual que GetCurrentSubscription (suscripción vigente → user_roles.tier_id);
// si no hay tier resuelto o el tier no está en el mapa, devuelve el default.
func (s *teamConfigurationService) GetTeamConfiguration(ctx *gin.Context, userID, teamID int64) (*teamconfiguration.TeamConfiguration, error) {
	team, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("error al obtener la configuración")
	}
	if team == nil {
		return nil, ErrTeamNotFound
	}
	if team.OwnerID != userID {
		return nil, ErrTeamNotOwner
	}

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
