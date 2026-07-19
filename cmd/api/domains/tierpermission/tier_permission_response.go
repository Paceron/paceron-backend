package tierpermission

import "time"

// TierPermissionResponse es el DTO de respuesta para una asignación de permiso a tier.
type TierPermissionResponse struct {
	ID             int64     `json:"id"`               // ID de la asignación
	TierID         int64     `json:"tier_id"`          // ID del tier
	PermissionID   int64     `json:"permission_id"`    // ID del permiso
	AsignationDate time.Time `json:"asignation_date"`  // Fecha de asignación
}

// DeleteTierPermissionResponse es el DTO de respuesta para desasignación de permiso.
type DeleteTierPermissionResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
