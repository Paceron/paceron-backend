package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

// Tests de conteo para los batches de las optimizaciones P7/P8/P9:
// push en 1 request por usuario, roles de login/refresh en 1 query,
// validateExercises en 1 query para N ejercicios.

func TestSendPushToUser_BatchesAllTokensInOneRequest(t *testing.T) {
	tokens := []dbs.PushToken{{Token: "ExponentPushToken[a]"}, {Token: "ExponentPushToken[b]"}}
	pushTokenDao := &mockPushTokenDao{mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
		return tokens, nil
	}}
	pushClient := &mockExpoPushClient{}

	sendPushToUser(nil, pushTokenDao, pushClient, 1, "Título", "Cuerpo", "invitation_received", "/invitations")

	assert.Equal(t, 1, pushClient.sendCallCount)
	require.Len(t, pushClient.lastTokens, 2)
	assert.Equal(t, "ExponentPushToken[a]", pushClient.lastTokens[0])
	assert.Equal(t, "ExponentPushToken[b]", pushClient.lastTokens[1])
	assert.Equal(t, "invitation_received", pushClient.lastData["type"])
	assert.Equal(t, "/invitations", pushClient.lastData["route"])
}

// countingRoleDaoEnvuelve mockRoleDao para contar queries por método.
type countingRoleDao struct {
	*mockRoleDao
	findByIDCalls  int
	findByIDsCalls int
	rolesByIDs     map[int64]string
}

func (c *countingRoleDao) FindByID(ctx *gin.Context, id int64) (*dbs.Role, error) {
	c.findByIDCalls++
	return c.mockRoleDao.FindByID(ctx, id)
}

func (c *countingRoleDao) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Role, error) {
	c.findByIDsCalls++
	roles := make([]dbs.Role, 0, len(c.rolesByIDs))
	for _, id := range ids {
		if name, ok := c.rolesByIDs[id]; ok {
			roles = append(roles, dbs.Role{ID: id, Name: name})
		}
	}
	return roles, nil
}

func TestRoleNamesForUser_SingleQueryForAllRoles(t *testing.T) {
	userRoleDao := &mockUserRoleDao{
		findByUserIDFn: func(ctx *gin.Context, userID int64) ([]dbs.UserRole, error) {
			return []dbs.UserRole{{RoleID: 1}, {RoleID: 2}}, nil
		},
	}
	roleDao := &countingRoleDao{
		mockRoleDao: &mockRoleDao{},
		rolesByIDs:  map[int64]string{1: "corredor", 2: "entrenador"},
	}
	svc := NewAuthService(&mockAuthDao{}, userRoleDao, roleDao, &mockRefreshTokenDao{}, nil).(*authService)

	names, err := svc.roleNamesForUser(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, []string{"corredor", "entrenador"}, names)
	assert.Equal(t, 1, roleDao.findByIDsCalls)
	assert.Equal(t, 0, roleDao.findByIDCalls)
}

// countingExerciseDaoEnvuelve mockExerciseDao para contar queries por método.
type countingExerciseDao struct {
	*mockExerciseDao
	findByIDCalls  int
	findByIDsCalls int
}

func (c *countingExerciseDao) FindByID(ctx *gin.Context, id int64) (*dbs.Exercise, error) {
	c.findByIDCalls++
	return c.mockExerciseDao.FindByID(ctx, id)
}

func (c *countingExerciseDao) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Exercise, error) {
	c.findByIDsCalls++
	return c.mockExerciseDao.FindByIDs(ctx, ids)
}

func TestValidateExercises_SingleQueryForAllExercises(t *testing.T) {
	exerciseDao := &countingExerciseDao{mockExerciseDao: &mockExerciseDao{}}
	svc := NewSessionService(&mockSessionDao{}, &mockSessionExerciseDao{}, exerciseDao).(*sessionService)

	items := append(validSessionExercises(), []session.SessionExerciseRequest{{ExerciseID: 4, Role: "warmup"}}...)

	err := svc.validateExercises(nil, items)

	require.NoError(t, err)
	assert.Equal(t, 1, exerciseDao.findByIDsCalls)
	assert.Equal(t, 0, exerciseDao.findByIDCalls)
}

func TestSendPushToUser_NoTokens_NoRequest(t *testing.T) {
	pushTokenDao := &mockPushTokenDao{mockFindByUserID: func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
		return nil, nil
	}}
	pushClient := &mockExpoPushClient{}

	sendPushToUser(nil, pushTokenDao, pushClient, 1, "Título", "Cuerpo", "invitation_received", "/invitations")

	assert.Equal(t, 0, pushClient.sendCallCount)
}

// countingSessionDao envuelve mockSessionDao para contar queries por método.
type countingSessionDao struct {
	*mockSessionDao
	findByIDCalls  int
	findByIDsCalls int
}

func (c *countingSessionDao) FindByID(ctx *gin.Context, id int64) (*dbs.Session, error) {
	c.findByIDCalls++
	return c.mockSessionDao.FindByID(ctx, id)
}

func (c *countingSessionDao) FindByIDs(ctx *gin.Context, ids []int64) ([]dbs.Session, error) {
	c.findByIDsCalls++
	return c.mockSessionDao.FindByIDs(ctx, ids)
}

func TestValidateAndBuildDays_SingleQueryForAllSessions(t *testing.T) {
	sessionDao := &countingSessionDao{mockSessionDao: &mockSessionDao{}}
	svc := NewTrainingPlanService(&mockTrainingPlanDao{}, &mockPlanDayDao{}, sessionDao, &mockGroupCalendarDao{}).(*trainingPlanService)

	sessionID1, sessionID2 := int64(1), int64(2)
	days := []trainingplan.PlanDayRequest{
		{SequenceNo: 1, Kind: "training", SessionID: &sessionID1},
		{SequenceNo: 2, Kind: "training", SessionID: &sessionID2},
		{SequenceNo: 3, Kind: "training", SessionID: &sessionID1},
		{SequenceNo: 4, Kind: "rest"},
	}

	rows, err := svc.validateAndBuildDays(nil, days)

	require.NoError(t, err)
	require.Len(t, rows, 4)
	assert.Equal(t, 1, sessionDao.findByIDsCalls)
	assert.Equal(t, 0, sessionDao.findByIDCalls)
}
