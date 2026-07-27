package constants

// TeamUserRole define los roles posibles de un usuario dentro de un equipo.
type TeamUserRole string

const (
	TeamUserRoleEntrenador TeamUserRole = "entrenador" // Entrenador/propietario del equipo
	TeamUserRoleCorredor   TeamUserRole = "corredor"   // Corredor (miembro regular del equipo)
)

// GetValidTeamUserRoles devuelve la lista de roles válidos dentro de un equipo.
func GetValidTeamUserRoles() []string {
	return []string{
		string(TeamUserRoleEntrenador),
		string(TeamUserRoleCorredor),
	}
}

// GetAddableTeamUserRoles devuelve los roles que se pueden asignar al agregar un usuario a un equipo.
// El rol "entrenador" se asigna automáticamente al crear el equipo.
func GetAddableTeamUserRoles() []string {
	return []string{
		string(TeamUserRoleCorredor),
	}
}

// IsValidTeamUserRole valida si un string es un rol de equipo válido.
func IsValidTeamUserRole(role string) bool {
	for _, r := range GetValidTeamUserRoles() {
		if r == role {
			return true
		}
	}
	return false
}

// IsValidAddableTeamUserRole valida si un string es un rol asignable al agregar un usuario.
func IsValidAddableTeamUserRole(role string) bool {
	for _, r := range GetAddableTeamUserRoles() {
		if r == role {
			return true
		}
	}
	return false
}
