package team

import "time"

// TeamResponse es el DTO de respuesta para un equipo.
type TeamResponse struct {
	ID                  int64     `json:"id"`                     // ID del equipo
	Name                string    `json:"name"`                   // Nombre del equipo
	Description         string    `json:"description"`            // Descripción del equipo
	Level               string    `json:"level"`                  // Nivel del equipo
	MaxMembers          int64     `json:"max_members"`            // Cantidad máxima de integrantes
	Requirements        string    `json:"requirements"`           // Requerimientos para entrar
	OwnerID             int64     `json:"owner_id"`               // ID del usuario owner
	Status              string    `json:"status"`                 // Estado del equipo
	Country             string    `json:"country"`                // Dirección: país
	Province            string    `json:"province"`               // Dirección: provincia
	City                string    `json:"city"`                   // Dirección: ciudad
	Street              string    `json:"street"`                 // Dirección: calle
	Number              string    `json:"number"`                 // Dirección: número
	ShowGroupsToRunners bool      `json:"show_groups_to_runners"` // Si los corredores ven a qué grupo pertenece cada compañero
	Visible             bool      `json:"visible"`                // Si aparece en resultados de búsqueda
	IsPublic            bool      `json:"is_public"`              // Si acepta solicitudes de ingreso
	IconURL             *string   `json:"icon_url"`               // URL pública del ícono del equipo (nil = sin ícono)
	CreatedAt           time.Time `json:"created_at"`             // Fecha de creación
	UpdatedAt           time.Time `json:"updated_at"`             // Fecha de última actualización
}

// DeleteTeamResponse es el DTO de respuesta para la eliminación de un equipo.
type DeleteTeamResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
