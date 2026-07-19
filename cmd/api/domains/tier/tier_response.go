package tier

import "time"

// TierResponse es el DTO de respuesta para un tier.
type TierResponse struct {
	ID          int64     `json:"id"`           // ID del tier
	Name        string    `json:"name"`         // Nombre del tier
	Description string    `json:"description"`  // Descripción del tier
	RoleID      int64     `json:"role_id"`      // ID del rol asociado
	RoleName    string    `json:"role_name"`    // Nombre del rol asociado
	CreatedAt   time.Time `json:"created_at"`   // Fecha de creación
	UpdatedAt   time.Time `json:"updated_at"`   // Fecha de última actualización
}

// DeleteTierResponse es el DTO de respuesta para eliminación de tier.
type DeleteTierResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
