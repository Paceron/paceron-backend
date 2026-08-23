package services

import (
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/pushtoken"
)

// mockPushTokenDao es el doble de prueba compartido por todos los tests de services
// que dependen de daos.PushTokenDaoInterface. Funciones no seteadas devuelven
// zero-value sin panic, para poder usarse como mockPushTokenDao{} en tests que no
// les interesa este colaborador.
type mockPushTokenDao struct {
	mockUpsert       func(ctx *gin.Context, userID int64, token, platform string) error
	mockFindByUserID func(ctx *gin.Context, userID int64) ([]dbs.PushToken, error)
}

func (m mockPushTokenDao) Upsert(ctx *gin.Context, userID int64, token, platform string) error {
	if m.mockUpsert != nil {
		return m.mockUpsert(ctx, userID, token, platform)
	}
	return nil
}

func (m mockPushTokenDao) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
	if m.mockFindByUserID != nil {
		return m.mockFindByUserID(ctx, userID)
	}
	return nil, nil
}

func TestPushTokenService_RegisterToken_Success(t *testing.T) {
	var capturedUserID int64
	var capturedToken, capturedPlatform string

	mockDao := mockPushTokenDao{
		mockUpsert: func(ctx *gin.Context, userID int64, token, platform string) error {
			capturedUserID = userID
			capturedToken = token
			capturedPlatform = platform
			return nil
		},
	}

	service := NewPushTokenService(mockDao)
	err := service.RegisterToken(nil, 1, &pushtoken.RegisterPushTokenRequest{
		Token:    "ExponentPushToken[abc]",
		Platform: "android",
	})

	require.NoError(t, err)
	assert.Equal(t, int64(1), capturedUserID)
	assert.Equal(t, "ExponentPushToken[abc]", capturedToken)
	assert.Equal(t, "android", capturedPlatform)
}

func TestPushTokenService_RegisterToken_InvalidPlatform(t *testing.T) {
	mockDao := mockPushTokenDao{}

	service := NewPushTokenService(mockDao)
	err := service.RegisterToken(nil, 1, &pushtoken.RegisterPushTokenRequest{
		Token:    "ExponentPushToken[abc]",
		Platform: "ios",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "platform inválida")
}

func TestPushTokenService_RegisterToken_DaoError(t *testing.T) {
	mockDao := mockPushTokenDao{
		mockUpsert: func(ctx *gin.Context, userID int64, token, platform string) error {
			return errors.New("dao error")
		},
	}

	service := NewPushTokenService(mockDao)
	err := service.RegisterToken(nil, 1, &pushtoken.RegisterPushTokenRequest{
		Token:    "ExponentPushToken[abc]",
		Platform: "android",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "error al registrar el token de push")
}
