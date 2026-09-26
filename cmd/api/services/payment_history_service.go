package services

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// ErrInvalidPaymentHistoryQuery indica parámetros inválidos en las consultas
// del historial de pagos. El controller lo traduce a 400 INVALID_QUERY.
var ErrInvalidPaymentHistoryQuery = errors.New("parámetros de consulta de pagos inválidos")

// paymentHistoryPageSize es el tamaño fijo de página de los listados (design D10).
const paymentHistoryPageSize = 20

// PaymentHistoryServiceInterface expone el historial de pagos y cobros del
// usuario autenticado (change historial-pagos-cobros-entrenador).
type PaymentHistoryServiceInterface interface {
	ListReceived(ctx *gin.Context, sellerID int64, teamID *int64, statusGroup string, page int) (*payment.ReceivedPaymentsResponse, error)
	ListMyTierPayments(ctx *gin.Context, userID int64, roleName string, page int) (*payment.TierPaymentsResponse, error)
}

type paymentHistoryService struct {
	dao daos.PaymentHistoryDaoInterface
	now func() time.Time
}

// NewPaymentHistoryService depende solo del DAO de historial: no combina
// services, así que no hace falta un delegate (design D2).
func NewPaymentHistoryService(dao daos.PaymentHistoryDaoInterface) PaymentHistoryServiceInterface {
	return &paymentHistoryService{dao: dao, now: time.Now}
}

func (s *paymentHistoryService) ListReceived(ctx *gin.Context, sellerID int64, teamID *int64, statusGroup string, page int) (*payment.ReceivedPaymentsResponse, error) {
	if page < 1 {
		return nil, fmt.Errorf("%w: page debe ser mayor o igual a 1", ErrInvalidPaymentHistoryQuery)
	}
	filters := daos.ReceivedPaymentFilters{TeamID: teamID}
	if statusGroup != "" {
		statuses, ok := constants.StatusesForGroup(statusGroup)
		if !ok {
			return nil, fmt.Errorf("%w: status desconocido %q", ErrInvalidPaymentHistoryQuery, statusGroup)
		}
		filters.Statuses = statuses
	}

	rows, hasMore, err := s.dao.ListReceived(ctx, sellerID, filters, page, paymentHistoryPageSize)
	if err != nil {
		customlogger.Error(ctx, "error listing received payments", err, customlogger.TagMethod("ListReceived"))
		return nil, fmt.Errorf("error al obtener los cobros: %w", err)
	}

	items := make([]payment.ReceivedPaymentItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toReceivedPaymentItem(r))
	}
	return &payment.ReceivedPaymentsResponse{Payments: items, HasMore: hasMore}, nil
}

func (s *paymentHistoryService) ListMyTierPayments(ctx *gin.Context, userID int64, roleName string, page int) (*payment.TierPaymentsResponse, error) {
	if page < 1 {
		return nil, fmt.Errorf("%w: page debe ser mayor o igual a 1", ErrInvalidPaymentHistoryQuery)
	}

	rows, hasMore, err := s.dao.ListMyTierPayments(ctx, userID, roleName, page, paymentHistoryPageSize)
	if err != nil {
		customlogger.Error(ctx, "error listing tier payments", err, customlogger.TagMethod("ListMyTierPayments"))
		return nil, fmt.Errorf("error al obtener los pagos de suscripción: %w", err)
	}

	items := make([]payment.TierPaymentItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toTierPaymentItem(r))
	}
	return &payment.TierPaymentsResponse{Payments: items, HasMore: hasMore}, nil
}

func toReceivedPaymentItem(r daos.ReceivedPaymentRow) payment.ReceivedPaymentItem {
	return payment.ReceivedPaymentItem{
		ID:                r.ID,
		MPPaymentID:       r.MPPaymentID,
		Status:            r.Status,
		StatusGroup:       string(constants.GroupOfPaymentStatus(r.Status)),
		StatusDetail:      r.StatusDetail,
		GrossAmount:       round2(r.GrossAmount),
		NetAmount:         roundPtr(r.NetAmount),
		CurrencyID:        r.CurrencyID,
		PaymentMethodID:   r.PaymentMethodID,
		CreatedAt:         formatUTC(r.CreatedAt),
		InstallmentID:     r.InstallmentID,
		InstallmentNumber: r.InstallmentNumber,
		Team:              payment.PaymentTeamRef{ID: r.TeamID, Name: deref(r.TeamName)},
		Payer: payment.PaymentPayerRef{
			ID:      r.PayerID,
			Name:    deref(r.PayerName),
			Surname: deref(r.PayerSurname),
			Email:   deref(r.PayerEmail),
		},
	}
}

func toTierPaymentItem(r daos.TierPaymentRow) payment.TierPaymentItem {
	item := payment.TierPaymentItem{
		ID:                r.ID,
		MPPaymentID:       r.MPPaymentID,
		Status:            r.Status,
		StatusGroup:       string(constants.GroupOfPaymentStatus(r.Status)),
		StatusDetail:      r.StatusDetail,
		Amount:            round2(r.Amount),
		CurrencyID:        r.CurrencyID,
		PaymentMethodID:   r.PaymentMethodID,
		CreatedAt:         formatUTC(r.CreatedAt),
		InstallmentID:     r.InstallmentID,
		InstallmentNumber: r.InstallmentNumber,
		SubscriptionID:    r.SubscriptionID,
	}
	if r.DueDate != nil {
		due := formatUTC(*r.DueDate)
		item.DueDate = &due
	}
	if r.TierID != nil {
		item.Tier = &payment.PaymentTierRef{ID: *r.TierID, Name: deref(r.TierName), RoleName: deref(r.TierRoleName)}
	}
	return item
}

func formatUTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func roundPtr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := round2(*v)
	return &r
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
