package constants

// SellerConnectionStatus define los estados de la conexión OAuth mp-connect de
// un entrenador. Solo `authorized` habilita a cobrar con split.
type SellerConnectionStatus string

const (
	SellerConnectionStatusAuthorized   SellerConnectionStatus = "authorized"
	SellerConnectionStatusDeauthorized SellerConnectionStatus = "deauthorized"
)
