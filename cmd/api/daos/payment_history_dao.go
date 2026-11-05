package daos

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
)

// DefaultMarketplaceFeePercent es la comisión que se asume para un cobro de
// equipo sin marketplace_fee guardado: el mismo valor por defecto que usa
// resolveTeamSplitConfig.
const DefaultMarketplaceFeePercent = 5.0

// feeAmountSQL es la comisión de Paceron de un cobro de equipo.
var feeAmountSQL = fmt.Sprintf("COALESCE(p.marketplace_fee, ROUND((p.amount * %g / 100)::numeric, 2))", DefaultMarketplaceFeePercent)

// netAmountExpr es el neto de un cobro aprobado: bruto menos la comisión de
// Paceron (change cobros-pendientes-y-vencimientos, design D2). Reemplaza al
// neto que informaba Mercado Pago, que solo llegaba por webhook y además
// descontaba su costo de procesamiento. Un cobro no aprobado no tiene neto.
var netAmountExpr = "CASE WHEN p.status = 'approved' THEN (p.amount - " + feeAmountSQL + ")::float8 END AS net_amount, " +
	feeAmountSQL + "::float8 AS fee_amount"

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
	FeeAmount         float64   `gorm:"column:fee_amount"`
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

// HistoryPaymentFilters son los filtros opcionales del historial del usuario.
// Type es "subscription" o "trainer_payment" ("" = ambos).
type HistoryPaymentFilters struct {
	Type     string
	Statuses []string
}

// HistoryPaymentRow es un pago que hizo el usuario, con su cuota resuelta hacia
// el tier (si es una suscripción) o hacia el equipo y su entrenador (si es una
// membresía).
type HistoryPaymentRow struct {
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
	SubscriptionID    *int64     `gorm:"column:subscription_id"`
	TierID            *int64     `gorm:"column:tier_id"`
	TierName          *string    `gorm:"column:tier_name"`
	TierRoleName      *string    `gorm:"column:tier_role_name"`
	TeamID            *int64     `gorm:"column:team_id"`
	TeamName          *string    `gorm:"column:team_name"`
	TrainerID         *int64     `gorm:"column:trainer_id"`
	TrainerName       *string    `gorm:"column:trainer_name"`
	TrainerSurname    *string    `gorm:"column:trainer_surname"`
}

// FeeMembershipRow es la membresía con cuota de un corredor en un equipo vivo,
// con lo necesario para calcular sus obligaciones (change
// cobros-pendientes-y-vencimientos, design D3). El corredor y el entrenador
// vienen resueltos según quién consulte.
type FeeMembershipRow struct {
	TeamID           int64     `gorm:"column:team_id"`
	TeamName         *string   `gorm:"column:team_name"`
	UserID           int64     `gorm:"column:user_id"`
	UserName         *string   `gorm:"column:user_name"`
	UserSurname      *string   `gorm:"column:user_surname"`
	UserEmail        *string   `gorm:"column:user_email"`
	TrainerID        *int64    `gorm:"column:trainer_id"`
	TrainerName      *string   `gorm:"column:trainer_name"`
	TrainerSurname   *string   `gorm:"column:trainer_surname"`
	AssignmentDate   time.Time `gorm:"column:assignment_date"`
	InitAmount       float64   `gorm:"column:init_amount"`
	PaidInstallments int       `gorm:"column:paid_installments"`
}

// TierSubscriptionRow es una suscripción de tier con cuota de un usuario.
type TierSubscriptionRow struct {
	ID               int64     `gorm:"column:id"`
	TierID           int64     `gorm:"column:tier_id"`
	TierName         *string   `gorm:"column:tier_name"`
	TierRoleName     *string   `gorm:"column:tier_role_name"`
	StartDate        time.Time `gorm:"column:start_date"`
	InitAmount       float64   `gorm:"column:init_amount"`
	PaidInstallments int       `gorm:"column:paid_installments"`
}

// PaymentHistoryDaoInterface es el acceso de solo lectura al historial de
// pagos. Vive aparte de PaymentDaoInterface para no tocar sus mocks (design D1).
type PaymentHistoryDaoInterface interface {
	ListReceived(ctx *gin.Context, sellerID int64, filters ReceivedPaymentFilters, page, pageSize int) ([]ReceivedPaymentRow, bool, error)
	ListReceivedBetween(ctx *gin.Context, sellerID int64, from, to time.Time) ([]ReceivedPaymentRow, error)
	EarliestReceivedAt(ctx *gin.Context, sellerID int64) (*time.Time, error)
	ListHistory(ctx *gin.Context, userID int64, filters HistoryPaymentFilters, page, pageSize int) ([]HistoryPaymentRow, bool, error)
	ListTrainerFeeMemberships(ctx *gin.Context, ownerID int64) ([]FeeMembershipRow, error)
	ListUserFeeMemberships(ctx *gin.Context, userID int64) ([]FeeMembershipRow, error)
	ListUserTierSubscriptions(ctx *gin.Context, userID int64) ([]TierSubscriptionRow, error)
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

// ListReceivedBetween devuelve todos los cobros de [from, to), sin paginar,
// para armar el resumen de una ventana de meses (design D9 y D14).
func (d *paymentHistoryDao) ListReceivedBetween(ctx *gin.Context, sellerID int64, from, to time.Time) ([]ReceivedPaymentRow, error) {
	var rows []ReceivedPaymentRow
	err := d.receivedBaseQuery(sellerID).
		Where("p.created_at >= ? AND p.created_at < ?", from, to).
		Order("p.created_at DESC, p.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing received payments between dates: %w", err)
	}
	return rows, nil
}

// EarliestReceivedAt devuelve la fecha del primer cobro del vendedor, con los
// mismos filtros que el listado, o nil si no tiene ninguno. El frontend la usa
// para saber hasta dónde deja correr la ventana del gráfico (design D14).
func (d *paymentHistoryDao) EarliestReceivedAt(ctx *gin.Context, sellerID int64) (*time.Time, error) {
	var earliest sql.NullTime
	if err := d.receivedBaseQuery(sellerID).Select("MIN(p.created_at)").Row().Scan(&earliest); err != nil {
		return nil, fmt.Errorf("error getting earliest received payment: %w", err)
	}
	if !earliest.Valid {
		return nil, nil
	}
	return &earliest.Time, nil
}

// ListHistory lista los pagos que hizo un usuario: suscripciones de tier y
// membresías de equipo. Se resuelve por la cuota (installments.user_id) y no por
// payments.user_id ni por concept: el primero no es confiable y los pagos de
// tier suelen quedar guardados como "order". El entrenador sale de
// seller_user_id (quien cobró) y, si falta, del dueño actual del equipo.
func (d *paymentHistoryDao) ListHistory(ctx *gin.Context, userID int64, filters HistoryPaymentFilters, page, pageSize int) ([]HistoryPaymentRow, bool, error) {
	query := d.DB.Table("payments AS p").
		Select(`p.id, p.payment_id AS mp_payment_id, p.status, p.status_detail, p.amount,
			p.currency_id, p.payment_method_id, p.created_at, p.installment_id,
			i.installment_number, i.due_date, i.subscription_id, i.team_id,
			tr.id AS tier_id, tr.name AS tier_name, tr.role_name AS tier_role_name,
			t.name AS team_name, o.id AS trainer_id, o.name AS trainer_name, o.surname AS trainer_surname`).
		Joins("JOIN installments i ON i.id = p.installment_id").
		Joins("LEFT JOIN user_role_tier_subscriptions s ON s.id = i.subscription_id").
		Joins("LEFT JOIN tiers tr ON tr.id = s.tier_id").
		Joins("LEFT JOIN teams t ON t.id = i.team_id").
		Joins("LEFT JOIN users o ON o.id = COALESCE(p.seller_user_id, t.owner_id) AND i.team_id IS NOT NULL").
		Where("i.user_id = ?", userID).
		Where(withMercadoPagoPaymentID)
	switch filters.Type {
	case string(constants.PaymentHistoryTypeSubscription):
		query = query.Where("i.subscription_id IS NOT NULL")
	case string(constants.PaymentHistoryTypeTrainerPayment):
		query = query.Where("i.team_id IS NOT NULL")
	}
	if len(filters.Statuses) > 0 {
		query = query.Where("p.status IN ?", filters.Statuses)
	}

	var rows []HistoryPaymentRow
	offset := (page - 1) * pageSize
	if err := query.Order("p.created_at DESC, p.id DESC").Offset(offset).Limit(pageSize + 1).Scan(&rows).Error; err != nil {
		return nil, false, fmt.Errorf("error listing payment history: %w", err)
	}
	return trimPage(rows, pageSize)
}

// feeMembershipsQuery son las membresías vivas de corredores con cuota en
// equipos vivos: las únicas que generan obligaciones (design D3).
func (d *paymentHistoryDao) feeMembershipsQuery() *gorm.DB {
	return d.DB.Table("team_users AS tu").
		Select(`tu.team_id, t.name AS team_name, tu.user_id, u.name AS user_name, u.surname AS user_surname,
			u.email AS user_email, o.id AS trainer_id, o.name AS trainer_name, o.surname AS trainer_surname,
			tu.assignment_date, tu.init_amount, tu.paid_installments`).
		Joins("JOIN teams t ON t.id = tu.team_id AND t.deleted_at IS NULL").
		Joins("LEFT JOIN users u ON u.id = tu.user_id").
		Joins("LEFT JOIN users o ON o.id = t.owner_id").
		Where("tu.deleted_at IS NULL AND tu.role_in_team = ? AND tu.init_amount > 0", string(constants.TeamUserRoleCorredor))
}

// ListTrainerFeeMemberships lista las membresías con cuota de los equipos vivos
// del entrenador. Se filtra por teams.owner_id: lo que se va a cobrar es del
// dueño actual, a diferencia del historial, que usa seller_user_id.
func (d *paymentHistoryDao) ListTrainerFeeMemberships(ctx *gin.Context, ownerID int64) ([]FeeMembershipRow, error) {
	var rows []FeeMembershipRow
	if err := d.feeMembershipsQuery().Where("t.owner_id = ?", ownerID).
		Order("tu.team_id, tu.user_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error listing trainer fee memberships: %w", err)
	}
	return rows, nil
}

// ListUserFeeMemberships lista las membresías con cuota del usuario como corredor.
func (d *paymentHistoryDao) ListUserFeeMemberships(ctx *gin.Context, userID int64) ([]FeeMembershipRow, error) {
	var rows []FeeMembershipRow
	if err := d.feeMembershipsQuery().Where("tu.user_id = ?", userID).
		Order("tu.team_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error listing user fee memberships: %w", err)
	}
	return rows, nil
}

// ListUserTierSubscriptions lista las suscripciones de tier con cuota del
// usuario que siguen en curso: activas o esperando el primer pago.
func (d *paymentHistoryDao) ListUserTierSubscriptions(ctx *gin.Context, userID int64) ([]TierSubscriptionRow, error) {
	var rows []TierSubscriptionRow
	err := d.DB.Table("user_role_tier_subscriptions AS s").
		Select(`s.id, s.tier_id, tr.name AS tier_name, tr.role_name AS tier_role_name,
			s.start_date, s.init_amount, s.paid_installments`).
		Joins("LEFT JOIN tiers tr ON tr.id = s.tier_id").
		Where("s.user_id = ? AND s.status IN ? AND s.init_amount > 0", userID,
			[]string{string(constants.SubscriptionStatusActive), string(constants.SubscriptionStatusFirstPaymentPending)}).
		Order("s.start_date, s.id").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("error listing user tier subscriptions: %w", err)
	}
	return rows, nil
}

func trimPage[T any](rows []T, pageSize int) ([]T, bool, error) {
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	return rows, hasMore, nil
}
