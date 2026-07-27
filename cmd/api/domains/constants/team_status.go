package constants

// TeamStatus define los estados posibles de un equipo.
// teamStatus es un atributo del Team (no tabla separada),
// siguiendo el mismo patrón que UserStatus.
type TeamStatus string

const (
	TeamStatusActive   TeamStatus = "active"   // Equipo activo y operativo
	TeamStatusInactive TeamStatus = "inactive"  // Equipo inactivo (no acepta nuevos miembros)
	TeamStatusArchived TeamStatus = "archived"  // Equipo archivado (solo lectura)
)

// GetValidTeamStatuses devuelve la lista de estados válidos para un equipo.
func GetValidTeamStatuses() []string {
	return []string{
		string(TeamStatusActive),
		string(TeamStatusInactive),
		string(TeamStatusArchived),
	}
}

// IsValidTeamStatus valida si un string es un estado de equipo válido.
func IsValidTeamStatus(status string) bool {
	for _, s := range GetValidTeamStatuses() {
		if s == status {
			return true
		}
	}
	return false
}
