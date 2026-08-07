package userrole

// AssignRoleRequest es el DTO para asignar un rol a un usuario.
type AssignRoleRequest struct {
	RoleID int64 `json:"role_id" binding:"required"` // ID del rol a asignar (requerido)
	TierID int64 `json:"tier_id"`                    // ID del tier (opcional, default: "base" del rol)
}

// ActivateEntrenadorRequest es el DTO para que el usuario autenticado active su propio
// rol entrenador. Exige la contraseña actual (mismo patrón que cambiar email/password) y
// un alias bancario válido — si el usuario ya tiene uno guardado, bank_alias es opcional.
type ActivateEntrenadorRequest struct {
	Password  string  `json:"password" binding:"required"` // Contraseña actual, confirma que es una acción deliberada del dueño de la cuenta
	BankAlias *string `json:"bank_alias"`                  // Alias bancario (requerido si el usuario no tiene uno ya guardado)
}
