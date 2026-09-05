package team

// SearchFilters agrupa los filtros opcionales de GET /api/v1/teams/search.
type SearchFilters struct {
	Name     string
	Level    string
	Country  string
	Province string
	City     string
}

// TeamSearchResult es una card de resultado de búsqueda de equipos.
type TeamSearchResult struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Level       string  `json:"level"`
	Country     string  `json:"country"`
	Province    string  `json:"province"`
	City        string  `json:"city"`
	MaxMembers  int64   `json:"max_members"`
	MemberCount int64   `json:"member_count"`
	OwnerName   string  `json:"owner_name"`
	IconURL     *string `json:"icon_url"`
	IsPublic    bool    `json:"is_public"`
}

// TeamSearchResponse es la respuesta paginada de GET /api/v1/teams/search.
type TeamSearchResponse struct {
	Teams   []TeamSearchResult `json:"teams"`
	HasMore bool               `json:"has_more"`
}
