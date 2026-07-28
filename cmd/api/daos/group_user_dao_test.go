package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewGroupUserDao(t *testing.T) {
	dao := NewGroupUserDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestGroupUserDao_ImplementsInterface(t *testing.T) {
	dao := NewGroupUserDao(&gorm.DB{})
	var iface GroupUserDaoInterface = dao
	_ = iface
}

func TestGroupUserDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewGroupUserDao(&gorm.DB{})
	})
}
