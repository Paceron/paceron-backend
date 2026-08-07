package user

// BatchLookupResponse resuelve nombre/apellido/email para varios user_id de una sola vez —
// mismo shape que SearchResultItem, pensado para el roster de equipo/grupo (que solo trae
// user_id) sin que el cliente tenga que hacer un fan-out N+1 contra GET /auth/user.
type BatchLookupResponse struct {
	Results []SearchResultItem `json:"results"`
}
