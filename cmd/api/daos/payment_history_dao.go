package daos

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
)

// netAmountExpr extrae el neto real que informó Mercado Pago. Solo el
// payment.Response que guarda el webhook trae transaction_details; el
// PaymentResult que guarda ProcessPayment no. Un pago no aprobado puede traer
// el neto en 0, por eso se exige approved y > 0 (design D5).
const netAmountExpr = `CASE WHEN p.status = 'approved'
	AND jsonb_typeof(p.raw_response->'transaction_details'->'net_received_amount') = 'number'
	AND (p.raw_response->'transaction_details'->>'net_received_amount')::numeric > 0
	THEN (p.raw_response->'transaction_details'->>'net_received_amount')::float8 END AS net_amount`

// withMercadoPagoPaymentID excluye las filas que nunca llegaron a MP: las que
// deja CreatePreference y los intentos donde CreatePayment falló (design D7).
const withMercadoPagoPaymentID = "COALESCE(p.payment_id, '') <> ''"

// ReceivedPaymentFilters son los filtros opcionales del listado de cobros.
type ReceivedPaymentFilters struct {
	TeamID   *int64
	Statuses []string
}

// ReceivedPaymentRow es un cobro de membresía de equipo con su cuota, equipo y
// corredor resueltos.
type ReceivedPaymentRow struct {
	ID                int64     `gorm:"column:id"`
	MPPaymentID       string    `gorm:"column:mp_payment_id"`
	Status            string    `gorm:"column:status"`
	StatusDetail      string    `gorm:"column:status_detail"`
	GrossAmount       float64   `gorm:"column:gross_amount"`
	NetAmount         *float64  `gorm:"column:net_amount"`
	CurrencyID        string    `gorm:"column:currency_id"`
	PaymentMethodID   string    `gorm:"column:payment_method_id"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	InstallmentID     int64     `gorm:"column:installment_id"`
	InstallmentNumber int       `gorm:"column:installment_number"`
	TeamID            int64     `gorm:"column:team_id"`
	TeamName          *string   `gorm:"column:team_name"`
	PayerID           int64     `gorm:"column:payer_id"`
	PayerName         *string   `gorm:"column:payer_name"`
	PayerSurname      *string   `gorm:"column:payer_surname"`
	PayerEmail        *string   `gorm:"column:payer_email"`
}

// TierPaymentRow es un pago de suscripción de tier con su cuota y su tier.
type TierPaymentRow struct {
	ID                int64      `gorm:"column:id"`
	MPPaymentID       string     `gorm:"column:mp_payment_id"`
	Status            string     `gorm:"column:status"`
	StatusDetail      string     `gorm:"column:status_detail"`
	Amount            float64    `gorm:"column:amount"`
	CurrencyID        string     `gorm:"column:currency_id"`
	PaymentMethodID   string     `gorm:"column:payment_method_id"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	InstallmentID     int64      `gorm:"column:installment_id"`
	InstallmentNumber int        `gorm:"column:installment_number"`
	DueDate           *time.Time `gorm:"column:due_date"`
	SubscriptionID    int64      `gorm:"column:subscription_id"`
	TierID            *int64     `gorm:"column:tier_id"`
	TierName          *string    `gorm:"column:tier_name"`
	TierRoleName      *string    `gorm:"column:tier_role_name"`
}

// PaymentHistoryDaoInterface es el acceso de solo lectura al historial de
// pagos. Vive aparte de PaymentDaoInterface para no tocar sus mocks (design D1).
type PaymentHistoryDaoInterface interface {
	ListReceived(ctx *gin.Context, sellerID int64, filters ReceivedPaymentFilters, page, pageSize int) ([]ReceivedPaymentRow, bool, error)
	ListReceivedSince(ctx *gin.Context, sellerID int64, since time.Time) ([]ReceivedPaymentRow, error)
	ListMyTierPayments(ctx *gin.Context, userID int64, roleName string, page, pageSize int) ([]TierPaymentRow, bool, error)
}

type paymentHistoryDao struct {
	DB *gorm.DB
}

func NewPaymentHistoryDao(database *gorm.DB) PaymentHistoryDaoInterface {
	return &paymentHistoryDao{DB: database}
}

// receivedBaseQuery arma la consulta de cobros de un vendedor. Se filtra por
// seller_user_id y no por teams.owner_id, que puede cambiar (design D11). Los
// equipos se unen sin filtrar deleted_at: el historial conserva los borrados.
func (d *paymentHistoryDao) receivedBaseQuery(sellerID int64) *gorm.DB {
	return d.DB.Table("payments AS p").
		Select(`p.id, p.payment_id AS mp_payment_id, p.status, p.status_detail,
			p.amount AS gross_amount, `+netAmountExpr+`, p.currency_id, p.payment_method_id,
			p.created_at, p.installment_id, i.installment_number, i.team_id, t.name AS team_name,
			i.user_id AS payer_id, u.name AS payer_name, u.surname AS payer_surname, u.email AS payer_email`).
		Joins("JOIN installments i ON i.id = p.installment_id AND i.team_id IS NOT NULL").
		Joins("LEFT JOIN teams t ON t.id = i.team_id").
		Joins("LEFT JOIN users u ON u.id = i.user_id").
		Where("p.seller_user_id = ? AND p.concept = ?", sellerID, string(constants.PaymentConceptTeamSubscription)).
		Where(withMercadoPagoPaymentID)
}

// ListReceived pide pageSize+1 filas para derivar hasMore sin un COUNT(*).
func (d *paymentHistoryDao) ListReceived(ctx *gin.Context, sellerID int64, filters ReceivedPaymentFilters, page, pageSize int) ([]ReceivedPaymentRow, bool, error) {
	query := d.receivedBaseQuery(sellerID)
	if filters.TeamID != nil {
		query = query.Where("i.team_id = ?", *filters.TeamID)
	}
	if len(filters.Statuses) > 0 {
		query = query.Where("p.status IN ?", filters.Statuses)
	}

	var rows []ReceivedPaymentRow
	offset := (page - 1) * pageSize
	if err := query.Order("p.created_at DESC, p.id DESC").Offset(offset).Limit(pageSize + 1).Scan(&rows).Error; err != nil {
		return nil, false, fmt.Errorf("error listing received payments: %w", err)
	}
	return trimPage(rows, pageSize)
}

// ListReceivedSince devuelve todos los cobros desde since, sin paginar, para
// armar el resumen (design D9).
func (d *paymentHistoryDao) ListReceivedSince(ctx *gin.Context, sellerID int64, since time.Time) ([]ReceivedPaymentRow, error) {
	var rows []ReceivedPaymentRow
	err := d.receivedBaseQuery(sellerID).
		Where("p.created_at >= ?", since).
		Order("p.created_at DESC, p.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing received payments since date: %w", err)
	}
	return rows, nil
}

// ListMyTierPayments lista los pagos de suscripción de tier de un usuario. Se
// resuelve por la cuota y no por payments.user_id ni por concept: el primero no
// es confiable y los pagos de tier suelen quedar guardados como "order".
func (d *paymentHistoryDao) ListMyTierPayments(ctx *gin.Context, userID int64, roleName string, page, pageSize int) ([]TierPaymentRow, bool, error) {
	query := d.DB.Table("payments AS p").
		Select(`p.id, p.payment_id AS mp_payment_id, p.status, p.status_detail, p.amount,
			p.currency_id, p.payment_method_id, p.created_at, p.installment_id,
			i.installment_number, i.due_date, i.subscription_id,
			tr.id AS tier_id, tr.name AS tier_name, tr.role_name AS tier_role_name`).
		Joins("JOIN installments i ON i.id = p.installment_id AND i.subscription_id IS NOT NULL").
		Joins("LEFT JOIN user_role_tier_subscriptions s ON s.id = i.subscription_id").
		Joins("LEFT JOIN tiers tr ON tr.id = s.tier_id").
		Where("i.user_id = ?", userID).
		Where(withMercadoPagoPaymentID)
	if roleName != "" {
		query = query.Where("tr.role_name = ?", roleName)
	}

	var rows []TierPaymentRow
	offset := (page - 1) * pageSize
	if err := query.Order("p.created_at DESC, p.id DESC").Offset(offset).Limit(pageSize + 1).Scan(&rows).Error; err != nil {
		return nil, false, fmt.Errorf("error listing tier payments: %w", err)
	}
	return trimPage(rows, pageSize)
}

func trimPage[T any](rows []T, pageSize int) ([]T, bool, error) {
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	return rows, hasMore, nil
}
