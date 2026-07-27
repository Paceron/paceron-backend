package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewTeamDao(t *testing.T) {
	dao := NewTeamDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestTeamDao_ImplementsInterface(t *testing.T) {
	dao := NewTeamDao(&gorm.DB{})
	var iface TeamDaoInterface = dao
	_ = iface
}

func TestTeamDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewTeamDao(&gorm.DB{})
	})
}
