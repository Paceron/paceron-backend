package daos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestNewGroupDao(t *testing.T) {
	dao := NewGroupDao(&gorm.DB{})
	assert.NotNil(t, dao)
}

func TestGroupDao_ImplementsInterface(t *testing.T) {
	dao := NewGroupDao(&gorm.DB{})
	var iface GroupDaoInterface = dao
	_ = iface
}

func TestGroupDao_NoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_ = NewGroupDao(&gorm.DB{})
	})
}
