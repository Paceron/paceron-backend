package services

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/userrole"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

const defaultTierName = "base"

// protectedRoleName es el rol base que todo usuario de la app tiene por defecto —
// no se puede dar de baja vía RemoveRole, sea cual sea el usuario.
const protectedRoleName = "corredor"

type UserRoleServiceInterface interface {
	AssignRole(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error)
	RemoveRole(ctx *gin.Context, userID, roleID int64) error
	ActivateEntrenador(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error)
	DeactivateEntrenador(ctx *gin.Context, userID int64) error
}

type userRoleService struct {
	userRoleDao daos.UserRoleDaoInterface
	roleDao     daos.RoleDaoInterface
	tierDao     daos.TierDaoInterface
	userDao     daos.UserDaoInterface
	teamUserDao daos.TeamUserDaoInterface
}

func NewUserRoleService(
	userRoleDao daos.UserRoleDaoInterface,
	roleDao daos.RoleDaoInterface,
	tierDao daos.TierDaoInterface,
	userDao daos.UserDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
) UserRoleServiceInterface {
	return &userRoleService{
		userRoleDao: userRoleDao,
		roleDao:     roleDao,
		tierDao:     tierDao,
		userDao:     userDao,
		teamUserDao: teamUserDao,
	}
}

func (s *userRoleService) AssignRole(ctx *gin.Context, userID int64, req *userrole.AssignRoleRequest) (*userrole.UserRoleResponse, error) {
	user, err := s.userDao.FindByID(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding user for role assignment", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("AssignRole"))
		return nil, fmt.Errorf("error al asignar rol")
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	role, err := s.roleDao.FindByID(ctx, req.RoleID)
	if err != nil {
		customlogger.Error(ctx, "error finding role for assignment", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
			customlogger.TagMethod("AssignRole"))
		return nil, fmt.Errorf("error al asignar rol")
	}
	if role == nil {
		return nil, fmt.Errorf("rol no encontrado")
	}

	existing, err := s.userRoleDao.FindByUserAndRole(ctx, userID, req.RoleID)
	if err != nil {
		customlogger.Error(ctx, "error checking existing role assignment", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
			customlogger.TagMethod("AssignRole"))
		return nil, fmt.Errorf("error al asignar rol")
	}
	if existing != nil {
		return nil, fmt.Errorf("el usuario ya tiene asignado este rol")
	}

	tierID := req.TierID
	if tierID == 0 {
		defaultTier, err := s.tierDao.FindByNameAndRole(ctx, defaultTierName, req.RoleID)
		if err != nil {
			customlogger.Error(ctx, "error finding default tier", err,
				customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
				customlogger.Tag("tier_name", defaultTierName),
				customlogger.TagMethod("AssignRole"))
			return nil, fmt.Errorf("error al asignar rol")
		}
		if defaultTier == nil {
			return nil, fmt.Errorf("el tier por defecto 'base' no existe para este rol")
		}
		tierID = defaultTier.ID
	} else {
		tier, err := s.tierDao.FindByID(ctx, tierID)
		if err != nil {
			customlogger.Error(ctx, "error finding tier for assignment", err,
				customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
				customlogger.TagMethod("AssignRole"))
			return nil, fmt.Errorf("error al asignar rol")
		}
		if tier == nil {
			return nil, fmt.Errorf("tier no encontrado")
		}
		if tier.RoleID != req.RoleID {
			return nil, fmt.Errorf("el tier no pertenece al rol especificado")
		}
	}

	ur := &dbs.UserRole{
		UserID:         userID,
		RoleID:         req.RoleID,
		TierID:         tierID,
		AssignmentDate: time.Now(),
		Status:         "active",
	}

	if err := s.userRoleDao.Create(ctx, ur); err != nil {
		customlogger.Error(ctx, "error assigning role to user", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
			customlogger.TagMethod("AssignRole"))
		return nil, fmt.Errorf("error al asignar rol")
	}

	customlogger.Info(ctx, "role assigned to user successfully",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.Tag("role_id", fmt.Sprintf("%d", req.RoleID)),
		customlogger.Tag("tier_id", fmt.Sprintf("%d", tierID)),
		customlogger.TagMethod("AssignRole"))

	return &userrole.UserRoleResponse{
		ID:             ur.ID,
		UserID:         ur.UserID,
		RoleID:         ur.RoleID,
		TierID:         ur.TierID,
		AssignmentDate: ur.AssignmentDate,
		Status:         ur.Status,
	}, nil
}

func (s *userRoleService) RemoveRole(ctx *gin.Context, userID, roleID int64) error {
	role, err := s.roleDao.FindByID(ctx, roleID)
	if err != nil {
		customlogger.Error(ctx, "error finding role for removal check", err,
			customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
			customlogger.TagMethod("RemoveRole"))
		return fmt.Errorf("error al eliminar rol")
	}
	if role != nil && role.Name == protectedRoleName {
		customlogger.Warn(ctx, "attempt to remove protected role",
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
			customlogger.TagMethod("RemoveRole"))
		return fmt.Errorf("el rol '%s' no se puede eliminar, es el rol base de todo usuario", protectedRoleName)
	}

	existing, err := s.userRoleDao.FindByUserAndRole(ctx, userID, roleID)
	if err != nil {
		customlogger.Error(ctx, "error finding role assignment to remove", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
			customlogger.TagMethod("RemoveRole"))
		return fmt.Errorf("error al eliminar rol")
	}
	if existing == nil {
		return fmt.Errorf("el usuario no tiene asignado este rol")
	}

	if err := s.userRoleDao.SoftDelete(ctx, existing.ID); err != nil {
		customlogger.Error(ctx, "error removing role from user", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
			customlogger.TagMethod("RemoveRole"))
		return fmt.Errorf("error al eliminar rol")
	}

	customlogger.Info(ctx, "role removed from user successfully",
		customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
		customlogger.Tag("role_id", fmt.Sprintf("%d", roleID)),
		customlogger.TagMethod("RemoveRole"))

	return nil
}

// ActivateEntrenador activa el rol "entrenador" del usuario autenticado sobre sí mismo.
// Exige confirmar la contraseña actual (evita que un token robado alcance para
// auto-promoverse) y un alias bancario válido, propio o recién provisto — internamente
// reutiliza AssignRole para no duplicar la lógica de tier por defecto.
func (s *userRoleService) ActivateEntrenador(ctx *gin.Context, userID int64, req *userrole.ActivateEntrenadorRequest) (*userrole.UserRoleResponse, error) {
	userDB, err := s.userDao.FindByID(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error finding user for entrenador activation", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("ActivateEntrenador"))
		return nil, fmt.Errorf("error al activar rol entrenador")
	}
	if userDB == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(userDB.Password), []byte(req.Password)); err != nil {
		customlogger.Warn(ctx, "invalid current password for entrenador activation",
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)))
		return nil, fmt.Errorf("contraseña actual incorrecta")
	}

	bankAlias := userDB.BankAlias
	if req.BankAlias != nil {
		trimmed := strings.TrimSpace(*req.BankAlias)
		if !bankAliasRegex.MatchString(trimmed) {
			return nil, fmt.Errorf(bankAliasFormatError)
		}
		bankAlias = &trimmed
	}
	if bankAlias == nil || *bankAlias == "" {
		return nil, fmt.Errorf("se requiere un alias bancario para activar el rol entrenador")
	}

	role, err := s.roleDao.FindByName(ctx, teamOwnerRoleName)
	if err != nil {
		customlogger.Error(ctx, "error finding entrenador role", err,
			customlogger.TagMethod("ActivateEntrenador"))
		return nil, fmt.Errorf("error al activar rol entrenador")
	}
	if role == nil {
		customlogger.Error(ctx, "entrenador role not found in catalog", nil,
			customlogger.TagMethod("ActivateEntrenador"))
		return nil, fmt.Errorf("error al activar rol entrenador")
	}

	if req.BankAlias != nil {
		userDB.BankAlias = bankAlias
		if err := s.userDao.Update(ctx, userDB); err != nil {
			customlogger.Error(ctx, "error saving bank alias for entrenador activation", err,
				customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
				customlogger.TagMethod("ActivateEntrenador"))
			return nil, fmt.Errorf("error al activar rol entrenador")
		}
	}

	return s.AssignRole(ctx, userID, &userrole.AssignRoleRequest{RoleID: role.ID})
}

// DeactivateEntrenador desactiva el rol "entrenador" del usuario autenticado sobre sí
// mismo. Bloquea la desactivación si todavía lidera algún equipo activo (mismo criterio
// que team_service.Delete con "no se puede eliminar un equipo con miembros activos") —
// primero debe transferir o eliminar esos equipos.
func (s *userRoleService) DeactivateEntrenador(ctx *gin.Context, userID int64) error {
	role, err := s.roleDao.FindByName(ctx, teamOwnerRoleName)
	if err != nil {
		customlogger.Error(ctx, "error finding entrenador role", err,
			customlogger.TagMethod("DeactivateEntrenador"))
		return fmt.Errorf("error al desactivar rol entrenador")
	}
	if role == nil {
		customlogger.Error(ctx, "entrenador role not found in catalog", nil,
			customlogger.TagMethod("DeactivateEntrenador"))
		return fmt.Errorf("error al desactivar rol entrenador")
	}

	leadsActiveTeam, err := s.leadsAnyActiveTeam(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error checking active team leadership", err,
			customlogger.Tag("user_id", fmt.Sprintf("%d", userID)),
			customlogger.TagMethod("DeactivateEntrenador"))
		return fmt.Errorf("error al desactivar rol entrenador")
	}
	if leadsActiveTeam {
		return fmt.Errorf("no podés desactivar el rol entrenador mientras lideres equipos activos")
	}

	return s.RemoveRole(ctx, userID, role.ID)
}

// leadsAnyActiveTeam indica si el usuario es entrenador (RoleInTeam) de algún equipo activo.
func (s *userRoleService) leadsAnyActiveTeam(ctx *gin.Context, userID int64) (bool, error) {
	teamUsers, err := s.teamUserDao.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, tu := range teamUsers {
		if tu.RoleInTeam == teamOwnerRoleName {
			return true, nil
		}
	}
	return false, nil
}
