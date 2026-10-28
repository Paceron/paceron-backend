package userrole

// AssignRoleRequest es el DTO para asignar un rol a un usuario.
type AssignRoleRequest struct {
	RoleID int64 `json:"role_id" binding:"required"` // ID del rol a asignar (requerido)
	TierID int64 `json:"tier_id"`                    // ID del tier (opcional, default: "base" del rol)
}

// ActivateEntrenadorRequest es el DTO para que el usuario autenticado active su propio
// rol entrenador. Exige la contraseña actual. bank_alias es opcional y, si se envía no
// vacío, debe tener un formato válido.
type ActivateEntrenadorRequest struct {
	Password  string  `json:"password" binding:"required"` // Contraseña actual, confirma que es una acción deliberada del dueño de la cuenta
	BankAlias *string `json:"bank_alias,omitempty"`        // Alias bancario opcional
}
