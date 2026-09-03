package platformsettings

import "time"

// UpdateMarketplaceFeeRequest es el body de PUT /api/v1/platform-settings/marketplace-fee (D8).
type UpdateMarketplaceFeeRequest struct {
	MarketplaceFeePercent float64 `json:"marketplace_fee_percent" binding:"required,min=0,max=100"`
}

// MarketplaceFeeResponse es la respuesta de GET/PUT marketplace-fee (D8).
type MarketplaceFeeResponse struct {
	MarketplaceFeePercent float64    `json:"marketplace_fee_percent"`
	UpdatedAt             *time.Time `json:"updated_at"`
}