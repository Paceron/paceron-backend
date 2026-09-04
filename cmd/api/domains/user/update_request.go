package user

type UserUpdateRequest struct {
	Name                 *string `json:"name,omitempty"`
	Surname              *string `json:"surname,omitempty"`
	Email                *string `json:"email,omitempty"`
	Phone                *string `json:"phone,omitempty"`
	PhoneContact         *string `json:"phone_contact,omitempty"`
	Country              *string `json:"country,omitempty"`
	Province             *string `json:"province,omitempty"`
	City                 *string `json:"city,omitempty"`
	Street               *string `json:"street,omitempty"`
	Number               *string `json:"number,omitempty"`
	Dni                  *string `json:"dni,omitempty"`
	BirthDate            *string `json:"birth_date,omitempty"`
	BankAlias            *string `json:"bank_alias,omitempty"`
	DefaultTheme         *string `json:"default_theme,omitempty"`
	AllowTeamInvitations *bool   `json:"allow_team_invitations,omitempty"`
}
