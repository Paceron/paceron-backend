package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTeamStatusValues(t *testing.T) {
	assert.Equal(t, TeamStatus("active"), TeamStatusActive)
	assert.Equal(t, TeamStatus("inactive"), TeamStatusInactive)
	assert.Equal(t, TeamStatus("archived"), TeamStatusArchived)
}

func TestGetValidTeamStatuses(t *testing.T) {
	statuses := GetValidTeamStatuses()
	expected := []string{"active", "inactive", "archived"}
	assert.Equal(t, expected, statuses)
	assert.Len(t, statuses, 3)
}

func TestIsValidTeamStatus_Valid(t *testing.T) {
	assert.True(t, IsValidTeamStatus("active"))
	assert.True(t, IsValidTeamStatus("inactive"))
	assert.True(t, IsValidTeamStatus("archived"))
}

func TestIsValidTeamStatus_Invalid(t *testing.T) {
	assert.False(t, IsValidTeamStatus(""))
	assert.False(t, IsValidTeamStatus("unknown"))
	assert.False(t, IsValidTeamStatus("ACTIVE"))
	assert.False(t, IsValidTeamStatus("deleted"))
}

func TestIsValidTeamStatus_CaseSensitive(t *testing.T) {
	assert.False(t, IsValidTeamStatus("Active"))
	assert.False(t, IsValidTeamStatus("INACTIVE"))
}
