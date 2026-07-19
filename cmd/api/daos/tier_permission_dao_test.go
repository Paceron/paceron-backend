package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewTierPermissionDao(t *testing.T) {
	dao := NewTierPermissionDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTierPermissionDao_ImplementsInterface(t *testing.T) {
	dao := NewTierPermissionDao(&gorm.DB{})
	var iface TierPermissionDaoInterface = dao
	_ = iface
}

func TestTierPermissionDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTierPermissionDao(&gorm.DB{})
	})
}
