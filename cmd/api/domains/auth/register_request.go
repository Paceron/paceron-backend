package auth

type RegisterRequest struct {
	Name         string `json:"name" binding:"required"`
	Surname      string `json:"surname" binding:"required"`
	Email        string `json:"email" binding:"required"`
	Phone        string `json:"phone,omitempty"`
	PhoneContact string `json:"phone_contact,omitempty"`
	Country      string `json:"country,omitempty"`
	Province     string `json:"province,omitempty"`
	City         string `json:"city,omitempty"`
	Street       string `json:"street,omitempty"`
	Number       string `json:"number,omitempty"`
	Dni          string `json:"dni" binding:"required"`
	BirthDate    string `json:"birth_date" binding:"required"`
	Password     string `json:"password" binding:"required"`
}
