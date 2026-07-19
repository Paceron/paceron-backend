package tierpermission

// AssignPermissionRequest es el DTO para asignar un permiso a un tier.
type AssignPermissionRequest struct {
	PermissionID int64 `json:"permission_id" binding:"required"` // ID del permiso a asignar (requerido)
}
