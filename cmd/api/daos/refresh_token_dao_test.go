package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewRefreshTokenDao(t *testing.T) {
	dao := NewRefreshTokenDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestRefreshTokenDao_ImplementsInterface(t *testing.T) {
	dao := NewRefreshTokenDao(&gorm.DB{})
	var iface RefreshTokenDaoInterface = dao
	_ = iface
}

func TestRefreshTokenDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewRefreshTokenDao(&gorm.DB{})
	})
}
