package user

// SearchResultItem trae solo los datos discretos necesarios para sugerir un usuario al
// invitar a un equipo (autocompletar) — no expone datos sensibles (DNI, teléfono, dirección).
type SearchResultItem struct {
	UserID  int64  `json:"user_id"`
	Name    string `json:"name"`
	Surname string `json:"surname"`
	Email   string `json:"email"`
}

type SearchResponse struct {
	Results []SearchResultItem `json:"results"`
}
