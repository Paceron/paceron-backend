package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewTierDao(t *testing.T) {
	dao := NewTierDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTierDao_ImplementsInterface(t *testing.T) {
	dao := NewTierDao(&gorm.DB{})
	var iface TierDaoInterface = dao
	_ = iface
}

func TestTierDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTierDao(&gorm.DB{})
	})
}
