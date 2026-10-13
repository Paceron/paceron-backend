package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// Mocks del dashboard. Son deliberadamente explícitos en vez de genéricos: el
// contrato del DAO es lo que el dashboard no puede violar, y un mock que devuelve
// ceros por default esconde exactamente los 400 que tienen que rechazarse SIN
// ejecutar la consulta (tareas 6.10 y 6.11).

type mockDashboardDao struct {
	findSeriesFn     func(ctx *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error)
	countOpenFn      func(ctx *gin.Context, filter daos.DashboardScopeFilter) (daos.DashboardOpenSessions, error)
	dateRangeFn      func(ctx *gin.Context, groupIDs []int64) (*time.Time, *time.Time, error)
	unlinkedFn       func(ctx *gin.Context, groupIDs []int64) ([]daos.UnlinkedExerciseInstance, error)
	athletesFn       func(ctx *gin.Context, groupIDs []int64) ([]int64, error)
	catalogByOwnerFn func(ctx *gin.Context, ownerID int64, kinds []string) ([]daos.DashboardCatalogExercises, error)
	catalogByIDFn    func(ctx *gin.Context, id int64) (*daos.DashboardCatalogExercises, error)
	catalogByIDsFn   func(ctx *gin.Context, ids []int64) ([]daos.DashboardCatalogExercises, error)
	// calls cuenta las consultas que llegaron al DAO: los tests de rechazo de
	// parámetros asserten que queda en cero.
	calls int
}

func (m *mockDashboardDao) FindSeries(ctx *gin.Context, filter daos.DashboardScopeFilter) ([]daos.DashboardSeriesRow, error) {
	m.calls++
	if m.findSeriesFn != nil {
		return m.findSeriesFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockDashboardDao) CountOpenSessions(ctx *gin.Context, filter daos.DashboardScopeFilter) (daos.DashboardOpenSessions, error) {
	m.calls++
	if m.countOpenFn != nil {
		return m.countOpenFn(ctx, filter)
	}
	return daos.DashboardOpenSessions{}, nil
}

func (m *mockDashboardDao) ScopeDateRange(ctx *gin.Context, groupIDs []int64) (*time.Time, *time.Time, error) {
	m.calls++
	if m.dateRangeFn != nil {
		return m.dateRangeFn(ctx, groupIDs)
	}
	return nil, nil, nil
}

func (m *mockDashboardDao) UnlinkedExerciseInstances(ctx *gin.Context, groupIDs []int64) ([]daos.UnlinkedExerciseInstance, error) {
	m.calls++
	if m.unlinkedFn != nil {
		return m.unlinkedFn(ctx, groupIDs)
	}
	return nil, nil
}

func (m *mockDashboardDao) ScopeAthletes(ctx *gin.Context, groupIDs []int64) ([]int64, error) {
	m.calls++
	if m.athletesFn != nil {
		return m.athletesFn(ctx, groupIDs)
	}
	return nil, nil
}

func (m *mockDashboardDao) FindCatalogExercisesByOwner(ctx *gin.Context, ownerID int64, kinds []string) ([]daos.DashboardCatalogExercises, error) {
	m.calls++
	if m.catalogByOwnerFn != nil {
		return m.catalogByOwnerFn(ctx, ownerID, kinds)
	}
	return nil, nil
}

func (m *mockDashboardDao) FindDashboardCatalogExerciseByID(ctx *gin.Context, id int64) (*daos.DashboardCatalogExercises, error) {
	m.calls++
	if m.catalogByIDFn != nil {
		return m.catalogByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockDashboardDao) FindDashboardCatalogExercisesByIDs(ctx *gin.Context, ids []int64) ([]daos.DashboardCatalogExercises, error) {
	m.calls++
	if m.catalogByIDsFn != nil {
		return m.catalogByIDsFn(ctx, ids)
	}
	return nil, nil
}

type mockDashboardScopeService struct {
	scope   *DashboardScope
	resolve int
	findErr error
	// foreignGroupID modela un grupo que EXISTE pero es de otro scope: tiene que
	// responder 403, no 404 (spec "Recurso inexistente versus recurso ajeno").
	foreignGroupID int64
}

func (m *mockDashboardScopeService) Resolve(ctx *gin.Context, callerID int64) (*DashboardScope, error) {
	m.resolve++
	if m.scope != nil {
		return m.scope, nil
	}
	return &DashboardScope{
		CallerID:      callerID,
		Teams:         map[int64]dbs.Team{},
		Groups:        map[int64]dbs.Group{},
		OwnedTeamIDs:  map[int64]struct{}{},
		OwnedGroupIDs: map[int64]struct{}{},
	}, nil
}

func (m *mockDashboardScopeService) FindGroup(ctx *gin.Context, id int64) (*dbs.Group, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.scope != nil {
		if group, ok := m.scope.Groups[id]; ok {
			return &group, nil
		}
		if m.foreignGroupID != 0 && id == m.foreignGroupID {
			return &dbs.Group{ID: id, TeamID: 30, Name: "Grupo ajeno"}, nil
		}
		return nil, nil
	}
	return nil, nil
}

type mockDashboardUserDao struct {
	users []*dbs.User
	err   error
}

func (m *mockDashboardUserDao) Create(ctx *gin.Context, user *dbs.User) error { return nil }
func (m *mockDashboardUserDao) GetByID(ctx *gin.Context, id int64) (*dbs.User, error) {
	return m.FindByID(ctx, id)
}

func (m *mockDashboardUserDao) FindByID(ctx *gin.Context, userID int64) (*dbs.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, user := range m.users {
		if user.ID == userID {
			return user, nil
		}
	}
	return nil, nil
}

func (m *mockDashboardUserDao) FindByEmail(ctx *gin.Context, email string) (*dbs.User, error) {
	return nil, nil
}
func (m *mockDashboardUserDao) Update(ctx *gin.Context, user *dbs.User) error { return nil }
func (m *mockDashboardUserDao) UpdateStatus(ctx *gin.Context, id int64, status string) error {
	return nil
}
func (m *mockDashboardUserDao) SearchActive(ctx *gin.Context, q string, limit int) ([]*dbs.User, error) {
	return nil, nil
}
func (m *mockDashboardUserDao) FindByIDs(ctx *gin.Context, userIDs []int64) ([]*dbs.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	wanted := map[int64]struct{}{}
	for _, id := range userIDs {
		wanted[id] = struct{}{}
	}
	out := make([]*dbs.User, 0, len(userIDs))
	for _, user := range m.users {
		if _, ok := wanted[user.ID]; ok {
			out = append(out, user)
		}
	}
	return out, nil
}
func (m *mockDashboardUserDao) UpdatePhoto(ctx *gin.Context, id int64, key string, t time.Time) error {
	return nil
}
func (m *mockDashboardUserDao) ClearPhoto(ctx *gin.Context, id int64) error { return nil }

type mockDashboardGroupUserDao struct {
	members []dbs.GroupUser
	err     error
}

func (m *mockDashboardGroupUserDao) Create(ctx *gin.Context, groupUser *dbs.GroupUser) error {
	return nil
}
func (m *mockDashboardGroupUserDao) FindByGroupAndUser(ctx *gin.Context, groupID, userID int64) (*dbs.GroupUser, error) {
	return nil, nil
}
func (m *mockDashboardGroupUserDao) FindByGroupID(ctx *gin.Context, groupID int64) ([]dbs.GroupUser, error) {
	if m.err != nil {
		return nil, m.err
	}
	out := make([]dbs.GroupUser, 0, len(m.members))
	for _, member := range m.members {
		if member.GroupID == groupID {
			out = append(out, member)
		}
	}
	return out, nil
}
func (m *mockDashboardGroupUserDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
	return nil, nil
}
func (m *mockDashboardGroupUserDao) SoftDelete(ctx *gin.Context, id int64) error { return nil }
func (m *mockDashboardGroupUserDao) SoftDeleteByTeamID(ctx *gin.Context, teamID int64) error {
	return nil
}
func (m *mockDashboardGroupUserDao) IsActiveGroupMember(ctx *gin.Context, groupID, userID int64, date time.Time) (bool, error) {
	return false, nil
}
func (m *mockDashboardGroupUserDao) MissingGroupMembers(ctx *gin.Context, groupID int64, userIDs []int64, date time.Time) ([]int64, error) {
	return nil, nil
}

// dashboardOwnerID es el entrenador dueño del equipo 29 del fixture.
const dashboardOwnerID = int64(10)

// dashboardFixture arma el service con el scope del entrenador dueño del equipo 29
// (grupo 39), listo para los tests de los siete endpoints.
func dashboardFixture(t *testing.T) (*dashboardService, *mockDashboardDao, *mockDashboardScopeService) {
	t.Helper()
	return dashboardFixtureAs(t, dashboardOwnerID, true)
}

// dashboardFixtureAs arma el service con el scope de un llamador concreto:
// owner=false modela al corredor, que pertenece al equipo pero no lo administra
// (su Team entra al scope por membresía, sin pasar a OwnedTeamIDs).
func dashboardFixtureAs(t *testing.T, callerID int64, owner bool) (*dashboardService, *mockDashboardDao, *mockDashboardScopeService) {
	t.Helper()
	scope := &DashboardScope{
		CallerID:      callerID,
		Teams:         map[int64]dbs.Team{29: {ID: 29, Name: "Paceron", OwnerID: dashboardOwnerID}},
		Groups:        map[int64]dbs.Group{39: {ID: 39, TeamID: 29, Name: "Grupo A"}},
		OwnedTeamIDs:  map[int64]struct{}{},
		OwnedGroupIDs: map[int64]struct{}{},
	}
	if owner {
		scope.OwnedTeamIDs[29] = struct{}{}
		scope.OwnedGroupIDs[39] = struct{}{}
	}
	dao := &mockDashboardDao{}
	scopeService := &mockDashboardScopeService{scope: scope, foreignGroupID: 99}
	service := NewDashboardService(dao, scopeService, dashboardUserDao(), &mockDashboardGroupUserDao{})
	return service.(*dashboardService), dao, scopeService
}

// dashboardUserDao devuelve los usuarios que el fixture usa como corredores, para
// que /filters y el leaderboard puedan resolver sus nombres.
func dashboardUserDao() *mockDashboardUserDao {
	users := make([]*dbs.User, 0, 4)
	for i, name := range map[int64][2]string{
		1: {"Ana", "Gómez"}, 2: {"Beto", "Ruiz"}, 3: {"Caro", "Sosa"}, 77: {"Diego", "Paz"},
	} {
		users = append(users, &dbs.User{ID: i, Name: name[0], Surname: name[1]})
	}
	sortByID(users)
	return &mockDashboardUserDao{users: users}
}

func sortByID(users []*dbs.User) {
	for i := 1; i < len(users); i++ {
		for j := i; j > 0 && users[j].ID < users[j-1].ID; j-- {
			users[j], users[j-1] = users[j-1], users[j]
		}
	}
}

func dashboardQuery() MetricsQuery {
	return MetricsQuery{TeamID: 29, From: "2026-09-01", To: "2026-09-30"}
}
