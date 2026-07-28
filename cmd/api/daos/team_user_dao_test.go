package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewTeamUserDao(t *testing.T) {
	dao := NewTeamUserDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTeamUserDao_ImplementsInterface(t *testing.T) {
	dao := NewTeamUserDao(&gorm.DB{})
	var iface TeamUserDaoInterface = dao
	_ = iface
}

func TestTeamUserDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTeamUserDao(&gorm.DB{})
	})
}
