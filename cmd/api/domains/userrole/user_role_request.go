package userrole

// AssignRoleRequest es el DTO para asignar un rol a un usuario.
type AssignRoleRequest struct {
	RoleID int64 `json:"role_id" binding:"required"` // ID del rol a asignar (requerido)
	TierID int64 `json:"tier_id"`                      // ID del tier (opcional, default: "base" del rol)
}
