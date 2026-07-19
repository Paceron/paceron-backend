package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewPermissionDao(t *testing.T) {
	dao := NewPermissionDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestPermissionDao_ImplementsInterface(t *testing.T) {
	dao := NewPermissionDao(&gorm.DB{})
	var iface PermissionDaoInterface = dao
	_ = iface
}

func TestPermissionDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewPermissionDao(&gorm.DB{})
	})
}
