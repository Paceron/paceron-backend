package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTeamUserRoleValues(t *testing.T) {
	assert.Equal(t, TeamUserRole("entrenador"), TeamUserRoleEntrenador)
	assert.Equal(t, TeamUserRole("corredor"), TeamUserRoleCorredor)
}

func TestGetValidTeamUserRoles(t *testing.T) {
	roles := GetValidTeamUserRoles()
	expected := []string{"entrenador", "corredor"}
	assert.Equal(t, expected, roles)
	assert.Len(t, roles, 2)
}

func TestGetAddableTeamUserRoles(t *testing.T) {
	roles := GetAddableTeamUserRoles()
	expected := []string{"corredor"}
	assert.Equal(t, expected, roles)
	assert.Len(t, roles, 1)
}

func TestIsValidTeamUserRole_Valid(t *testing.T) {
	assert.True(t, IsValidTeamUserRole("entrenador"))
	assert.True(t, IsValidTeamUserRole("corredor"))
}

func TestIsValidTeamUserRole_Invalid(t *testing.T) {
	assert.False(t, IsValidTeamUserRole(""))
	assert.False(t, IsValidTeamUserRole("admin"))
	assert.False(t, IsValidTeamUserRole("ENTRENADOR"))
	assert.False(t, IsValidTeamUserRole("owner"))
	assert.False(t, IsValidTeamUserRole("member"))
	assert.False(t, IsValidTeamUserRole("coach"))
}

func TestIsValidAddableTeamUserRole(t *testing.T) {
	assert.True(t, IsValidAddableTeamUserRole("corredor"))
	assert.False(t, IsValidAddableTeamUserRole("entrenador"))
	assert.False(t, IsValidAddableTeamUserRole("owner"))
	assert.False(t, IsValidAddableTeamUserRole("member"))
}

func TestIsValidTeamUserRole_CaseSensitive(t *testing.T) {
	assert.False(t, IsValidTeamUserRole("Entrenador"))
	assert.False(t, IsValidTeamUserRole("CORREDOR"))
}
