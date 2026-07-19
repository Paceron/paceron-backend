package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewRoleDao(t *testing.T) {
	dao := NewRoleDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestRoleDao_ImplementsInterface(t *testing.T) {
	dao := NewRoleDao(&gorm.DB{})
	var iface RoleDaoInterface = dao
	_ = iface
}

func TestRoleDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewRoleDao(&gorm.DB{})
	})
}
