package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewUserRoleDao(t *testing.T) {
	dao := NewUserRoleDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestUserRoleDao_ImplementsInterface(t *testing.T) {
	dao := NewUserRoleDao(&gorm.DB{})
	var iface UserRoleDaoInterface = dao
	_ = iface
}

func TestUserRoleDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewUserRoleDao(&gorm.DB{})
	})
}
