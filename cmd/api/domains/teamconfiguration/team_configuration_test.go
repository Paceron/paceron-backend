package teamconfiguration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForTier_KnownTiers(t *testing.T) {
	assert.Equal(t, TeamConfiguration{MaxMembers: 10, MinimumFee: 20000}, ForTier("base"))
	assert.Equal(t, TeamConfiguration{MaxMembers: 25, MinimumFee: 20000}, ForTier("medium"))
	assert.Equal(t, TeamConfiguration{MaxMembers: 50, MinimumFee: 20000}, ForTier("premium"))
}

func TestForTier_UnknownTierReturnsDefault(t *testing.T) {
	assert.Equal(t, TeamConfiguration{MaxMembers: DefaultMaxMembers, MinimumFee: DefaultMinimumFee}, ForTier("desconocido"))
	assert.Equal(t, TeamConfiguration{MaxMembers: DefaultMaxMembers, MinimumFee: DefaultMinimumFee}, ForTier(""))
}

func TestForTier_DefaultConstants(t *testing.T) {
	assert.Equal(t, 10, DefaultMaxMembers)
	assert.Equal(t, float64(20000), float64(DefaultMinimumFee))
}