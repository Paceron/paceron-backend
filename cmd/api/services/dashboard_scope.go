package services

import (
	"errors"
	"fmt"
	"sort"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// ErrDashboardForbidden es 403: el recurso existe pero está fuera del scope del
// solicitante. El 404 queda reservado para lo que no existe (spec "El scope se
// aplica en la consulta, no sobre el resultado").
var ErrDashboardForbidden = errors.New("dashboard: fuera del scope del solicitante")

// ErrDashboardNotFound es 404.
var ErrDashboardNotFound = errors.New("dashboard: recurso inexistente")

// DashboardAthleteOutOfScope nombra el athlete_user_id que rompió el scope.
// /comparisons valida cada corredor por separado y responde 403 identificando
// cuál falló, sin devolver los datos de los válidos.
type DashboardAthleteOutOfScope struct {
	AthleteUserID int64
}

func (e *DashboardAthleteOutOfScope) Error() string {
	return fmt.Sprintf("dashboard: el corredor %d está fuera del scope del solicitante", e.AthleteUserID)
}

func (e *DashboardAthleteOutOfScope) Unwrap() error { return ErrDashboardForbidden }

// DashboardScope es el scope del solicitante, resuelto UNA vez por request y
// pasado ya resuelto a los DAOs: ningún DAO del dashboard acepta un group_id
// crudo, así que no hay forma de que una consulta se escape del scope.
//
// El criterio de "entrenador" NO es un user_roles: es teams.owner_id (los grupos
// que me pertenecen). Un user_roles sería otro concepto —poder ser entrenador
// sin tener grupos— y dejaría la pantalla vacía sin explicación.
type DashboardScope struct {
	CallerID int64
	// Teams son los equipos del scope: los que el solicitante administra más los
	// de los grupos donde es miembro.
	Teams map[int64]dbs.Team
	// Groups son los grupos del scope: los de los equipos que administra más los
	// donde es miembro.
	Groups map[int64]dbs.Group
	// OwnedTeamIDs es el subconjunto de Teams con teams.owner_id == CallerID. Es
	// lo que decide el 403 de leaderboard/comparisons.
	OwnedTeamIDs map[int64]struct{}
	// OwnedGroupIDs es el subconjunto de Groups que cuelga de un equipo propio.
	OwnedGroupIDs map[int64]struct{}
}

// DashboardScopeServiceInterface resuelve el scope del solicitante.
type DashboardScopeServiceInterface interface {
	Resolve(ctx *gin.Context, callerID int64) (*DashboardScope, error)
	FindGroup(ctx *gin.Context, id int64) (*dbs.Group, error)
}

type dashboardScopeService struct {
	groupDao     daos.GroupDaoInterface
	groupUserDao daos.GroupUserDaoInterface
	teamDao      daos.TeamDaoInterface
}

// GroupDao expone el DAO de grupos para el service del dashboard, que necesita
// distinguir "el grupo no existe" (404) de "existe pero es ajeno" (403).
func (s *dashboardScopeService) FindGroup(ctx *gin.Context, id int64) (*dbs.Group, error) {
	return s.groupDao.FindByID(ctx, id)
}

func NewDashboardScopeService(
	groupDao daos.GroupDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	teamDao daos.TeamDaoInterface,
) DashboardScopeServiceInterface {
	return &dashboardScopeService{groupDao: groupDao, groupUserDao: groupUserDao, teamDao: teamDao}
}

// Resolve arma el scope del solicitante con los criterios que ya usa el resto del
// repo (isGroupOwner/isGroupOwnerOrMember de calendar_service.go): los grupos
// que administra vía teams.owner_id, más los grupos donde es miembro.
func (s *dashboardScopeService) Resolve(ctx *gin.Context, callerID int64) (*DashboardScope, error) {
	scope := &DashboardScope{
		CallerID:      callerID,
		Teams:         map[int64]dbs.Team{},
		Groups:        map[int64]dbs.Group{},
		OwnedTeamIDs:  map[int64]struct{}{},
		OwnedGroupIDs: map[int64]struct{}{},
	}

	ownedGroups, err := s.groupDao.FindByOwnerID(ctx, callerID)
	if err != nil {
		customlogger.Error(ctx, "error resolving dashboard scope by owner", err,
			customlogger.TagMethod("DashboardScope.Resolve"))
		return nil, fmt.Errorf("error al resolver el scope del dashboard")
	}
	for _, group := range ownedGroups {
		scope.Groups[group.ID] = group
		scope.OwnedGroupIDs[group.ID] = struct{}{}
		if err := s.addTeam(ctx, scope, group.TeamID, true); err != nil {
			return nil, err
		}
	}

	memberships, err := s.groupUserDao.FindByUserID(ctx, callerID)
	if err != nil {
		customlogger.Error(ctx, "error resolving dashboard scope by membership", err,
			customlogger.TagMethod("DashboardScope.Resolve"))
		return nil, fmt.Errorf("error al resolver el scope del dashboard")
	}
	memberGroupIDs := make([]int64, 0, len(memberships))
	seen := map[int64]struct{}{}
	for _, membership := range memberships {
		if _, dup := seen[membership.GroupID]; dup {
			continue
		}
		seen[membership.GroupID] = struct{}{}
		memberGroupIDs = append(memberGroupIDs, membership.GroupID)
	}
	if len(memberGroupIDs) > 0 {
		groups, err := s.groupDao.FindByIDs(ctx, memberGroupIDs)
		if err != nil {
			customlogger.Error(ctx, "error resolving dashboard member groups", err,
				customlogger.TagMethod("DashboardScope.Resolve"))
			return nil, fmt.Errorf("error al resolver el scope del dashboard")
		}
		for _, group := range groups {
			if _, already := scope.Groups[group.ID]; already {
				continue
			}
			scope.Groups[group.ID] = group
			if err := s.addTeam(ctx, scope, group.TeamID, false); err != nil {
				return nil, err
			}
		}
	}

	return scope, nil
}

func (s *dashboardScopeService) addTeam(ctx *gin.Context, scope *DashboardScope, teamID int64, owned bool) error {
	if _, loaded := scope.Teams[teamID]; loaded {
		return nil
	}
	team, err := s.teamDao.FindByID(ctx, teamID)
	if err != nil {
		customlogger.Error(ctx, "error finding team for dashboard scope", err,
			customlogger.Tag("team_id", fmt.Sprintf("%d", teamID)),
			customlogger.TagMethod("DashboardScope.Resolve"))
		return fmt.Errorf("error al resolver el scope del dashboard")
	}
	if team == nil {
		return nil
	}
	scope.Teams[teamID] = *team
	if owned || team.OwnerID == scope.CallerID {
		scope.OwnedTeamIDs[teamID] = struct{}{}
	}
	return nil
}

// OwnsTeam decide el 403 de /leaderboard, /comparisons y /comparisons/periods.
// La regla vive en el backend y no en el front: si el corredor no debe ver las
// métricas de sus compañeros, dejarlo en un `if` del front se erosiona con el
// tiempo — el próximo que agrega un botón lo rompe sin darse cuenta.
func (s *DashboardScope) OwnsTeam(teamID int64) bool {
	_, ok := s.OwnedTeamIDs[teamID]
	return ok
}

// CanReadGroup dice si el grupo está en el scope.
func (s *DashboardScope) CanReadGroup(groupID int64) bool {
	_, ok := s.Groups[groupID]
	return ok
}

// TeamIDs y GroupIDs devuelven los ids ordenados: el ORDER BY de las respuestas
// y de las queries no puede depender del orden de un map.
func (s *DashboardScope) TeamIDs() []int64 {
	return sortedKeys(s.Teams)
}

func (s *DashboardScope) GroupIDs() []int64 {
	return sortedKeys(s.Groups)
}

func (s *DashboardScope) GroupsForTeam(teamID int64) []int64 {
	ids := make([]int64, 0, len(s.Groups))
	for id, group := range s.Groups {
		if group.TeamID == teamID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedKeys[T any](m map[int64]T) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}
