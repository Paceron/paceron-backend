package payment

type CreatePreferenceRequest struct {
	Items          []PreferenceItem `json:"items" binding:"required"`
	Concept        string           `json:"concept" binding:"required"`
	SellerID       *int64           `json:"seller_id"`
	Description    string           `json:"description"`
	InstallmentID  *int64           `json:"installment_id"` // cuota a la que se vincula el pago
}

type PreferenceItem struct {
	Title     string  `json:"title" binding:"required"`
	Quantity  int     `json:"quantity" binding:"required,min=1"`
	UnitPrice float64 `json:"unit_price" binding:"required,min=0"`
}

type CreatePreferenceResponse struct {
	PreferenceID string `json:"preference_id"`
	PublicKey    string `json:"public_key"`
}

type ProcessPaymentRequest struct {
	Token             string  `json:"token" binding:"required"`
	TransactionAmount float64 `json:"transaction_amount" binding:"required,min=0"`
	PaymentMethodID   string  `json:"payment_method_id" binding:"required"`
	Installments      int     `json:"installments" binding:"required,min=1"`
	PayerEmail        string  `json:"payer_email" binding:"required,email"`
	PreferenceID      string  `json:"preference_id"`
	InstallmentID     *int64  `json:"installment_id"` // cuota a la que se vincula el pago
}

type PaymentResponse struct {
	ID              int64   `json:"id"`
	PreferenceID    string  `json:"preference_id"`
	PaymentID       string  `json:"payment_id"`
	ExternalRef     string  `json:"external_reference"`
	Concept         string  `json:"concept"`
	Description     string  `json:"description"`
	Amount          float64 `json:"amount"`
	CurrencyID      string  `json:"currency_id"`
	Status          string  `json:"status"`
	StatusDetail    string  `json:"status_detail"`
	PaymentMethodID string  `json:"payment_method_id"`
	Installments    int     `json:"installments"`
	PayerEmail      string  `json:"payer_email"`
	CreatedAt       string  `json:"created_at"`
}

type WebhookNotification struct {
	ID       int    `json:"id"`
	LiveMode bool   `json:"live_mode"`
	Type     string `json:"type"`
	Action   string `json:"action"`
	Data     struct {
		ID string `json:"id"`
	} `json:"data"`
}

type TestCardTokenRequest struct {
	CardNumber           string `json:"card_number" binding:"required"`
	ExpirationMonth      string `json:"expiration_month" binding:"required"`
	ExpirationYear       string `json:"expiration_year" binding:"required"`
	SecurityCode         string `json:"security_code" binding:"required"`
	CardholderName       string `json:"cardholder_name" binding:"required"`
	IdentificationType   string `json:"identification_type" binding:"required"`
	IdentificationNumber string `json:"identification_number" binding:"required"`
}

type TestCardTokenResponse struct {
	Token string `json:"token"`
}

type MPPaymentStatusResponse struct {
	ID                        int                        `json:"id"`
	Status                    string                     `json:"status"`
	StatusDetail              string                     `json:"status_detail"`
	OperationType             string                     `json:"operation_type"`
	Description               string                     `json:"description"`
	ExternalReference         string                     `json:"external_reference"`
	TransactionAmount         float64                    `json:"transaction_amount"`
	TransactionAmountRefunded float64                    `json:"transaction_amount_refunded"`
	NetAmount                 float64                    `json:"net_amount"`
	CouponAmount              float64                    `json:"coupon_amount"`
	CurrencyID                string                     `json:"currency_id"`
	PaymentMethodID           string                     `json:"payment_method_id"`
	PaymentTypeID             string                     `json:"payment_type_id"`
	Installments              int                        `json:"installments"`
	IssuerID                  string                     `json:"issuer_id"`
	LiveMode                  bool                       `json:"live_mode"`
	Captured                  bool                       `json:"captured"`
	DateCreated               string                     `json:"date_created"`
	DateApproved              string                     `json:"date_approved"`
	DateLastUpdated           string                     `json:"date_last_updated"`
	Payer                     MPPaymentStatusPayer       `json:"payer"`
	Card                      MPPaymentStatusCard        `json:"card"`
	FeeDetails                []MPPaymentStatusFeeDetail `json:"fee_details"`
	TransactionDetails        MPPaymentStatusTransaction `json:"transaction_details"`
}

type MPPaymentStatusPayer struct {
	ID             string                 `json:"id"`
	Email          string                 `json:"email"`
	FirstName      string                 `json:"first_name"`
	LastName       string                 `json:"last_name"`
	Type           string                 `json:"type"`
	Identification MPPaymentStatusIdentif `json:"identification"`
	Phone          MPPaymentStatusPhone   `json:"phone"`
}

type MPPaymentStatusIdentif struct {
	Type   string `json:"type"`
	Number string `json:"number"`
}

type MPPaymentStatusPhone struct {
	AreaCode string `json:"area_code"`
	Number   string `json:"number"`
}

type MPPaymentStatusCard struct {
	ID              string `json:"id"`
	LastFourDigits  string `json:"last_four_digits"`
	FirstSixDigits  string `json:"first_six_digits"`
	ExpirationMonth string `json:"expiration_month"`
	ExpirationYear  string `json:"expiration_year"`
	CardholderName  string `json:"cardholder_name"`
}

type MPPaymentStatusFeeDetail struct {
	Type     string  `json:"type"`
	FeePayer string  `json:"fee_payer"`
	Amount   float64 `json:"amount"`
}

type MPPaymentStatusTransaction struct {
	NetReceivedAmount float64 `json:"net_received_amount"`
	TotalPaidAmount   float64 `json:"total_paid_amount"`
	InstallmentAmount float64 `json:"installment_amount"`
	OverpaidAmount    float64 `json:"overpaid_amount"`
}
