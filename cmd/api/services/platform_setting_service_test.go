package services

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/platformsettings"
)

type mockPlatformSettingDao struct {
	getFn func(ctx *gin.Context, key string, defaultValue float64) (float64, *dbs.PlatformSetting, error)
	setFn func(ctx *gin.Context, key string, value float64, updatedBy int64) (*dbs.PlatformSetting, error)
}

func (m *mockPlatformSettingDao) Get(ctx *gin.Context, key string, defaultValue float64) (float64, *dbs.PlatformSetting, error) {
	if m.getFn != nil {
		return m.getFn(ctx, key, defaultValue)
	}
	return defaultValue, &dbs.PlatformSetting{Key: key}, nil
}

func (m *mockPlatformSettingDao) Set(ctx *gin.Context, key string, value float64, updatedBy int64) (*dbs.PlatformSetting, error) {
	if m.setFn != nil {
		return m.setFn(ctx, key, value, updatedBy)
	}
	return nil, nil
}

func TestGetMarketplaceFee_Success(t *testing.T) {
	ctx := &gin.Context{}
	updatedAt := time.Now().AddDate(0, 0, -1)

	settingDao := &mockPlatformSettingDao{
		getFn: func(ctx *gin.Context, key string, defaultValue float64) (float64, *dbs.PlatformSetting, error) {
			assert.Equal(t, "marketplace_fee_percent", key)
			assert.Equal(t, 5.0, defaultValue)
			return 8.5, &dbs.PlatformSetting{Key: key, UpdatedAt: updatedAt}, nil
		},
	}
	svc := NewPlatformSettingService(settingDao, &mockUserDaoForUserRole{})

	resp, err := svc.GetMarketplaceFee(ctx)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 8.5, resp.MarketplaceFeePercent)
	assert.Equal(t, updatedAt, *resp.UpdatedAt)
}

func TestGetMarketplaceFee_DaoError(t *testing.T) {
	ctx := &gin.Context{}
	settingDao := &mockPlatformSettingDao{
		getFn: func(ctx *gin.Context, key string, defaultValue float64) (float64, *dbs.PlatformSetting, error) {
			return 0, nil, assert.AnError
		},
	}
	svc := NewPlatformSettingService(settingDao, &mockUserDaoForUserRole{})

	resp, err := svc.GetMarketplaceFee(ctx)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error consultando comisión")
}

func TestUpdateMarketplaceFee_Success(t *testing.T) {
	ctx := &gin.Context{}
	updatedAt := time.Now().AddDate(0, 0, -1)

	userDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID}, nil
		},
	}
	updatedBy := int64(7)
	settingDao := &mockPlatformSettingDao{
		setFn: func(ctx *gin.Context, key string, value float64, by int64) (*dbs.PlatformSetting, error) {
			assert.Equal(t, "marketplace_fee_percent", key)
			assert.Equal(t, 10.0, value)
			assert.Equal(t, updatedBy, by)
			return &dbs.PlatformSetting{Key: key, UpdatedBy: &updatedBy, UpdatedAt: updatedAt}, nil
		},
	}
	svc := NewPlatformSettingService(settingDao, userDao)

	resp, err := svc.UpdateMarketplaceFee(ctx, updatedBy, &platformsettings.UpdateMarketplaceFeeRequest{MarketplaceFeePercent: 10.0})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, 10.0, resp.MarketplaceFeePercent)
	assert.Equal(t, updatedAt, *resp.UpdatedAt)
}

func TestUpdateMarketplaceFee_OutOfRange(t *testing.T) {
	ctx := &gin.Context{}
	svc := NewPlatformSettingService(&mockPlatformSettingDao{}, &mockUserDaoForUserRole{})

	for _, invalid := range []float64{-1, 101, 100.1} {
		resp, err := svc.UpdateMarketplaceFee(ctx, 1, &platformsettings.UpdateMarketplaceFeeRequest{MarketplaceFeePercent: invalid})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.EqualError(t, err, "comisión debe estar entre 0 y 100")
	}
}

func TestUpdateMarketplaceFee_UserDaoError(t *testing.T) {
	ctx := &gin.Context{}
	userDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, assert.AnError
		},
	}
	svc := NewPlatformSettingService(&mockPlatformSettingDao{}, userDao)

	resp, err := svc.UpdateMarketplaceFee(ctx, 1, &platformsettings.UpdateMarketplaceFeeRequest{MarketplaceFeePercent: 10.0})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error consultando usuario")
}

func TestUpdateMarketplaceFee_UserNotFound(t *testing.T) {
	ctx := &gin.Context{}
	userDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return nil, nil
		},
	}
	svc := NewPlatformSettingService(&mockPlatformSettingDao{}, userDao)

	resp, err := svc.UpdateMarketplaceFee(ctx, 1, &platformsettings.UpdateMarketplaceFeeRequest{MarketplaceFeePercent: 10.0})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "usuario no encontrado")
}

func TestUpdateMarketplaceFee_SetDaoError(t *testing.T) {
	ctx := &gin.Context{}
	userDao := &mockUserDaoForUserRole{
		findByIDFn: func(ctx *gin.Context, userID int64) (*dbs.User, error) {
			return &dbs.User{ID: userID}, nil
		},
	}
	settingDao := &mockPlatformSettingDao{
		setFn: func(ctx *gin.Context, key string, value float64, by int64) (*dbs.PlatformSetting, error) {
			return nil, assert.AnError
		},
	}
	svc := NewPlatformSettingService(settingDao, userDao)

	resp, err := svc.UpdateMarketplaceFee(ctx, 1, &platformsettings.UpdateMarketplaceFeeRequest{MarketplaceFeePercent: 10.0})
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.EqualError(t, err, "error actualizando comisión")
}