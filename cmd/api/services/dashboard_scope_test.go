package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// dashboardScopeFixture arma el scopeService con DAOs de memoria: el entrenador 10
// es dueño del equipo 29 (grupo 39), y el 77 es miembro de ese mismo grupo sin
// administrarlo.
func dashboardScopeFixture(t *testing.T) DashboardScopeServiceInterface {
	t.Helper()
	groupDao := &mockGroupDao{
		findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) {
			if ownerID != 10 {
				return nil, nil
			}
			return []dbs.Group{{ID: 39, TeamID: 29, Name: "Grupo A"}}, nil
		},
		findByIDsFn: func(ctx *gin.Context, ids []int64) ([]dbs.Group, error) {
			out := make([]dbs.Group, 0, len(ids))
			for _, id := range ids {
				if id == 39 {
					out = append(out, dbs.Group{ID: 39, TeamID: 29, Name: "Grupo A"})
				}
			}
			return out, nil
		},
	}
	groupUserDao := &mockGroupUserDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			if userID == 77 {
				return []dbs.GroupUser{{GroupID: 39, UserID: 77}}, nil
			}
			return nil, nil
		},
	}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) {
			if id == 29 {
				return &dbs.Team{ID: 29, Name: "Paceron", OwnerID: 10}, nil
			}
			return nil, nil
		},
	}
	return NewDashboardScopeService(groupDao, groupUserDao, teamDao)
}

// El scope del entrenador son los grupos que administra (teams.owner_id == caller),
// no un user_roles: un user_roles sería otro concepto —poder ser entrenador sin
// tener grupos— y dejaría la pantalla vacía sin explicación.
func TestDashboardScope_EntrenadorVeSusEquiposYGrupos(t *testing.T) {
	scopeService := dashboardScopeFixture(t)

	scope, err := scopeService.Resolve(nil, 10)

	require.NoError(t, err)
	assert.Equal(t, int64(10), scope.CallerID)
	assert.Equal(t, []int64{29}, scope.TeamIDs())
	assert.Equal(t, []int64{39}, scope.GroupIDs())
	assert.True(t, scope.OwnsTeam(29))
	assert.True(t, scope.CanReadGroup(39))
	assert.False(t, scope.OwnsTeam(30))
	assert.False(t, scope.CanReadGroup(99))
	assert.Equal(t, []int64{39}, scope.GroupsForTeam(29))
	assert.Empty(t, scope.GroupsForTeam(30))
}

// El corredor entra al scope del equipo del que es miembro, pero el equipo NO pasa
// a OwnedTeamIDs: de ahí sale el 403 de leaderboard y comparaciones.
func TestDashboardScope_CorredorEntraPorMembresiaYNoEsDueño(t *testing.T) {
	scopeService := dashboardScopeFixture(t)

	scope, err := scopeService.Resolve(nil, 77)

	require.NoError(t, err)
	assert.Equal(t, []int64{29}, scope.TeamIDs(), "el equipo del grupo donde es miembro entra al scope")
	assert.Equal(t, []int64{39}, scope.GroupIDs())
	assert.False(t, scope.OwnsTeam(29), "ser miembro no es ser dueño")
	assert.True(t, scope.CanReadGroup(39))
	assert.Empty(t, scope.OwnedTeamIDs)
	assert.Empty(t, scope.OwnedGroupIDs)
}

func TestDashboardScope_SinVinculoNoTieneScope(t *testing.T) {
	scopeService := dashboardScopeFixture(t)

	scope, err := scopeService.Resolve(nil, 500)

	require.NoError(t, err)
	assert.Empty(t, scope.TeamIDs())
	assert.Empty(t, scope.GroupIDs())
	assert.False(t, scope.OwnsTeam(29))
	assert.False(t, scope.CanReadGroup(39))
}

// Los ids salen ordenados: el ORDER BY de las respuestas y de las queries no puede
// depender del recorrido de un map.
func TestDashboardScope_IdsOrdenados(t *testing.T) {
	scope := &DashboardScope{
		Teams:  map[int64]dbs.Team{30: {}, 29: {}, 28: {}},
		Groups: map[int64]dbs.Group{9: {}, 2: {}, 7: {}},
	}

	assert.Equal(t, []int64{28, 29, 30}, scope.TeamIDs())
	assert.Equal(t, []int64{2, 7, 9}, scope.GroupIDs())
}

// El scope no puede quedar medio armado en silencio: un error de lectura se
// propaga y no se devuelve un scope vacío que parece "no tenés datos".
func TestDashboardScope_ErrorDeLecturaSePropaga(t *testing.T) {
	boom := errors.New("boom")
	groupDao := &mockGroupDao{
		findByOwnerIDFn: func(ctx *gin.Context, ownerID int64) ([]dbs.Group, error) { return nil, boom },
	}
	groupUserDao := &mockGroupUserDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) { return nil, boom },
	}
	teamDao := &mockTeamDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Team, error) { return nil, boom },
	}

	scope, err := NewDashboardScopeService(groupDao, groupUserDao, teamDao).Resolve(nil, 10)

	require.Error(t, err)
	assert.Nil(t, scope)
}