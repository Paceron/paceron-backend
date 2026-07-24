package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewPasswordResetDao(t *testing.T) {
	dao := NewPasswordResetDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestPasswordResetDao_ImplementsInterface(t *testing.T) {
	dao := NewPasswordResetDao(&gorm.DB{})
	var iface PasswordResetDaoInterface = dao
	_ = iface
}

func TestPasswordResetDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewPasswordResetDao(&gorm.DB{})
	})
}
