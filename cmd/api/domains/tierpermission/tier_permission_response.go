package tierpermission

import "time"

// TierPermissionResponse es el DTO de respuesta para una asignación de permiso a tier.
type TierPermissionResponse struct {
	ID             int64     `json:"id"`              // ID de la asignación
	TierID         int64     `json:"tier_id"`         // ID del tier
	PermissionID   int64     `json:"permission_id"`   // ID del permiso
	AsignationDate time.Time `json:"asignation_date"` // Fecha de asignación
}

// TierPermissionListItem es un permiso activo de un tier con su nombre resuelto.
type TierPermissionListItem struct {
	PermissionID   int64  `json:"permission_id"`   // ID del permiso
	PermissionName string `json:"permission_name"` // Nombre del permiso
}

// ListTierPermissionsResponse es el DTO de respuesta para el listado de permisos de un tier.
type ListTierPermissionsResponse struct {
	Permissions []TierPermissionListItem `json:"permissions"`
}

// DeleteTierPermissionResponse es el DTO de respuesta para desasignación de permiso.
type DeleteTierPermissionResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
}
