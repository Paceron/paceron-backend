package services

// --- POST seen (gamificacion-core D3, task 2.1): contrato de servicio con
// mock DAO (mockGamificationDao de streak_service_test.go); la integración
// del DELETE vive en daos/gamification_dao_test.go.

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/constants"
)

func TestGamificationSeenService_ForwardArgs(t *testing.T) {
	var gotUserID int64
	var gotSection string
	var gotTeam *int64
	dao := &mockGamificationDao{deleteUnseenFn: func(ctx *gin.Context, userID int64, section string, teamID *int64) (int64, error) {
		gotUserID, gotSection, gotTeam = userID, section, teamID
		return 0, nil
	}}

	team := int64(55)
	err := NewGamificationSeenService(dao).See(nil, 7, constants.GamificationSectionCompetitions, &team)

	require.NoError(t, err)
	assert.Equal(t, int64(7), gotUserID)
	assert.Equal(t, constants.GamificationSectionCompetitions, gotSection)
	require.NotNil(t, gotTeam)
	assert.Equal(t, team, *gotTeam)
}

func TestGamificationSeenService_Whitelist(t *testing.T) {
	for _, section := range constants.GetValidGamificationSections() {
		dao := &mockGamificationDao{}
		err := NewGamificationSeenService(dao).See(nil, 7, section, nil)
		require.NoError(t, err, section)
		assert.Equal(t, 1, dao.deleteUnseenCall, section)
	}
}

func TestGamificationSeenService_SectionInvalida(t *testing.T) {
	dao := &mockGamificationDao{}
	err := NewGamificationSeenService(dao).See(nil, 7, "logros", nil)

	require.ErrorIs(t, err, ErrGamificationInvalidSection)
	assert.Equal(t, 0, dao.deleteUnseenCall, "el guard corta antes del DAO")
}

func TestGamificationSeenService_TeamIdSoloCompetitions(t *testing.T) {
	team := int64(3)
	for _, section := range []string{constants.GamificationSectionStreaks, constants.GamificationSectionAchievements} {
		dao := &mockGamificationDao{}
		err := NewGamificationSeenService(dao).See(nil, 7, section, &team)
		require.ErrorIs(t, err, ErrGamificationTeamIDInvalid, section)
		assert.Equal(t, 0, dao.deleteUnseenCall, section)
	}
}

func TestGamificationSeenService_DBErrorContractual(t *testing.T) {
	dao := &mockGamificationDao{deleteUnseenFn: func(ctx *gin.Context, userID int64, section string, teamID *int64) (int64, error) {
		return 0, errors.New("SQLSTATE 23505: detalle crudo del driver")
	}}
	err := NewGamificationSeenService(dao).See(nil, 7, constants.GamificationSectionStreaks, nil)

	require.EqualError(t, err, "error al marcar novedades vistas", "mensaje contractual sin detalle del driver")
}
