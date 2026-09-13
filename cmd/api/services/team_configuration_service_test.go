package services

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/teamconfiguration"
)

func TestTeamConfigurationService_PremiumViaActiveSubscription(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	mockSubDao := &mockTierSubscriptionDao{
		findActiveFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRoleTierSubscription, error) {
			return &dbs.UserRoleTierSubscription{TierID: 3}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: id, Name: "premium"}, nil
		},
	}

	svc := NewTeamConfigurationService(mockRoleDao, &mockUserRoleDao{}, mockSubDao, mockTierDao)
	cfg, err := svc.GetTeamConfiguration(nil, 7)

	require.NoError(t, err)
	assert.Equal(t, &teamconfiguration.TeamConfiguration{MaxMembers: 50, MinimumFee: 20000}, cfg)
}

func TestTeamConfigurationService_BaseViaUserRoleTier(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID, TierID: 1}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: id, Name: "base"}, nil
		},
	}

	svc := NewTeamConfigurationService(mockRoleDao, mockUserRoleDao, &mockTierSubscriptionDao{}, mockTierDao)
	cfg, err := svc.GetTeamConfiguration(nil, 1)

	require.NoError(t, err)
	assert.Equal(t, &teamconfiguration.TeamConfiguration{MaxMembers: 10, MinimumFee: 20000}, cfg)
}

func TestTeamConfigurationService_RoleEntrenadorMissingReturnsDefault(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return nil, nil
		},
	}
	svc := NewTeamConfigurationService(mockRoleDao, &mockUserRoleDao{}, &mockTierSubscriptionDao{}, &mockTierDao{})

	cfg, err := svc.GetTeamConfiguration(nil, 5)

	require.NoError(t, err)
	assert.Equal(t, &teamconfiguration.TeamConfiguration{MaxMembers: 10, MinimumFee: 20000}, cfg)
}

func TestTeamConfigurationService_NoSubNoUserRoleReturnsDefault(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}

	svc := NewTeamConfigurationService(mockRoleDao, &mockUserRoleDao{}, &mockTierSubscriptionDao{}, &mockTierDao{})
	cfg, err := svc.GetTeamConfiguration(nil, 5)

	require.NoError(t, err)
	assert.Equal(t, &teamconfiguration.TeamConfiguration{MaxMembers: 10, MinimumFee: 20000}, cfg)
}

func TestTeamConfigurationService_UnknownTierReturnsDefault(t *testing.T) {
	mockRoleDao := &mockRoleDao{
		findByNameFn: func(ctx *gin.Context, name string) (*dbs.Role, error) {
			return &dbs.Role{ID: 1, Name: "entrenador"}, nil
		},
	}
	mockUserRoleDao := &mockUserRoleDao{
		findByUserAndRoleFn: func(ctx *gin.Context, userID, roleID int64) (*dbs.UserRole, error) {
			return &dbs.UserRole{UserID: userID, RoleID: roleID, TierID: 9}, nil
		},
	}
	mockTierDao := &mockTierDao{
		findByIDFn: func(ctx *gin.Context, id int64) (*dbs.Tier, error) {
			return &dbs.Tier{ID: id, Name: "tier_desconocido"}, nil
		},
	}
	svc := NewTeamConfigurationService(mockRoleDao, mockUserRoleDao, &mockTierSubscriptionDao{}, mockTierDao)

	cfg, err := svc.GetTeamConfiguration(nil, 5)

	require.NoError(t, err)
	assert.Equal(t, &teamconfiguration.TeamConfiguration{MaxMembers: 10, MinimumFee: 20000}, cfg)
}
