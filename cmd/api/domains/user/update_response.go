package user

type UserUpdateResponse struct {
	UserID               int64   `json:"user_id"`
	Name                 string  `json:"name"`
	Surname              string  `json:"surname"`
	Email                string  `json:"email"`
	Phone                string  `json:"phone,omitempty"`
	PhoneContact         string  `json:"phone_contact,omitempty"`
	Country              string  `json:"country,omitempty"`
	Province             string  `json:"province,omitempty"`
	City                 string  `json:"city,omitempty"`
	Street               string  `json:"street,omitempty"`
	Number               string  `json:"number,omitempty"`
	Dni                  string  `json:"dni"`
	BirthDate            string  `json:"birth_date"`
	Status               string  `json:"status"`
	BankAlias            *string `json:"bank_alias,omitempty"`
	PhotoURL             *string `json:"photo_url"`
	DefaultTheme         string  `json:"default_theme"`
	AllowTeamInvitations bool    `json:"allow_team_invitations"`
}
