package tiersubscription

// ChangeTierRequest es el body de PUT /api/v1/users/:id/roles/:role_id/tier.
type ChangeTierRequest struct {
	TierID int64 `json:"tier_id" binding:"required"` // ID del tier target (mismo role_id que la asignación)
}