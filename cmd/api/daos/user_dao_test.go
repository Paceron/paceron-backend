package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewUserDao(t *testing.T) {
	dao := NewUserDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestUserDao_ImplementsInterface(t *testing.T) {
	dao := NewUserDao(&gorm.DB{})
	var iface UserDaoInterface = dao
	_ = iface
}

func TestUserDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewUserDao(&gorm.DB{})
	})
}

func TestUserDao_SearchActive_ImplementsInterface(t *testing.T) {
	dao := NewUserDao(&gorm.DB{})
	var iface UserDaoInterface = dao
	assert.NotNil(t, iface.SearchActive)
}
