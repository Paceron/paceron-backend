package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserStatusValues(t *testing.T) {
	assert.Equal(t, UserStatus("active"), UserStatusActive)
	assert.Equal(t, UserStatus("inactive"), UserStatusInactive)
	assert.Equal(t, UserStatus("pause"), UserStatusPause)
	assert.Equal(t, UserStatus("blocked"), UserStatusBlocked)
	assert.Equal(t, UserStatus("suspended"), UserStatusSuspended)
}

func TestGetValidUserStatuses(t *testing.T) {
	statuses := GetValidUserStatuses()
	expected := []string{"active", "inactive", "pause", "blocked", "suspended"}
	assert.Equal(t, expected, statuses)
	assert.Len(t, statuses, 5)
}

func TestIsValidUserStatus_Valid(t *testing.T) {
	assert.True(t, IsValidUserStatus("active"))
	assert.True(t, IsValidUserStatus("inactive"))
	assert.True(t, IsValidUserStatus("pause"))
	assert.True(t, IsValidUserStatus("blocked"))
	assert.True(t, IsValidUserStatus("suspended"))
}

func TestIsValidUserStatus_Invalid(t *testing.T) {
	assert.False(t, IsValidUserStatus(""))
	assert.False(t, IsValidUserStatus("unknown"))
	assert.False(t, IsValidUserStatus("ACTIVE"))
	assert.False(t, IsValidUserStatus("deleted"))
}

func TestIsValidUserStatus_CaseSensitive(t *testing.T) {
	assert.False(t, IsValidUserStatus("Active"))
	assert.False(t, IsValidUserStatus("BLOCKED"))
}
