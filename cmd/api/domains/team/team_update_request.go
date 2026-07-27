package team

// UpdateTeamRequest es el DTO para actualizar un equipo existente.
// Todos los campos son opcionales (solo se actualizan los enviados).
type UpdateTeamRequest struct {
	Name         *string `json:"name"`           // Nombre del equipo (opcional)
	Description  *string `json:"description"`    // Descripción del equipo (opcional)
	Level        *string `json:"level"`          // Nivel del equipo (opcional)
	MaxMembers   *int64  `json:"max_members"`    // Cantidad máxima de integrantes (opcional)
	Requirements *string `json:"requirements"`   // Requerimientos para entrar (opcional)
}

// UpdateTeamAddressRequest es el DTO para actualizar la dirección de un equipo.
type UpdateTeamAddressRequest struct {
	Country  string `json:"country"`   // País
	Province string `json:"province"`  // Provincia
	City     string `json:"city"`      // Ciudad
	Street   string `json:"street"`    // Calle
	Number   string `json:"number"`    // Número
}
