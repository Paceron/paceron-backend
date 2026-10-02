package user

// SearchResultItem trae solo los datos discretos necesarios para sugerir un usuario al
// invitar a un equipo (autocompletar) — no expone datos sensibles (DNI, teléfono, dirección).
type SearchResultItem struct {
	UserID  int64  `json:"user_id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
	// null explícito para usuarios sin foto: sin omitempty a propósito.
	PhotoURL *string `json:"photo_url"`
}

type SearchResponse struct {
	Results []SearchResultItem `json:"results"`
}
