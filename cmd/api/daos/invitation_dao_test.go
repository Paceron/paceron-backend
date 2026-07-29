package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewInvitationDao(t *testing.T) {
	dao := NewInvitationDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestInvitationDao_ImplementsInterface(t *testing.T) {
	dao := NewInvitationDao(&gorm.DB{})
	var iface InvitationDaoInterface = dao
	_ = iface
}

func TestInvitationDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewInvitationDao(&gorm.DB{})
	})
}
