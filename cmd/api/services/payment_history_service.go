package services

import (
	"errors"
	"fmt"
	"math"
	"sort"
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

// Ventana del resumen de cobros, en meses (el actual incluido).
const (
	PaymentSummaryDefaultMonths = 6
	paymentSummaryMinMonths     = 2
	paymentSummaryMaxMonths     = 12
)

// argentinaTZ corta los meses del resumen en hora argentina. Zona fija porque
// Argentina no tiene horario de verano desde 2009, y así no se depende de que
// el container tenga tzdata (design D6).
var argentinaTZ = time.FixedZone("ART", -3*3600)

// PaymentHistoryServiceInterface expone el historial de pagos y cobros del
// usuario autenticado (change historial-pagos-cobros-entrenador).
type PaymentHistoryServiceInterface interface {
	ListReceived(ctx *gin.Context, sellerID int64, teamID *int64, statusGroup string, page int) (*payment.ReceivedPaymentsResponse, error)
	ListMyTierPayments(ctx *gin.Context, userID int64, roleName string, page int) (*payment.TierPaymentsResponse, error)
	GetReceivedSummary(ctx *gin.Context, sellerID int64, months int) (*payment.ReceivedSummaryResponse, error)
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

// GetReceivedSummary resume los cobros de la ventana de meses. La agregación se
// hace acá y no en SQL: el volumen por entrenador es chico y así la lógica queda
// testeable con mocks (design D9).
func (s *paymentHistoryService) GetReceivedSummary(ctx *gin.Context, sellerID int64, months int) (*payment.ReceivedSummaryResponse, error) {
	if months < paymentSummaryMinMonths || months > paymentSummaryMaxMonths {
		return nil, fmt.Errorf("%w: months debe estar entre %d y %d", ErrInvalidPaymentHistoryQuery, paymentSummaryMinMonths, paymentSummaryMaxMonths)
	}

	now := s.now().In(argentinaTZ)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, argentinaTZ).AddDate(0, -(months - 1), 0)

	rows, err := s.dao.ListReceivedSince(ctx, sellerID, start)
	if err != nil {
		customlogger.Error(ctx, "error listing received payments for summary", err, customlogger.TagMethod("GetReceivedSummary"))
		return nil, fmt.Errorf("error al obtener el resumen de cobros: %w", err)
	}

	monthly := make([]payment.MonthlyAmount, months)
	monthIndex := make(map[string]int, months)
	for i := 0; i < months; i++ {
		key := start.AddDate(0, i, 0).Format("2006-01")
		monthly[i] = payment.MonthlyAmount{Month: key}
		monthIndex[key] = i
	}

	teams := map[int64]*payment.TeamAmount{}
	teamNet := map[int64]float64{}
	monthNet := make([]float64, months)
	installments := map[int64]*installmentAttempts{}
	currency := "ARS"

	for _, r := range rows {
		if r.CurrencyID != "" {
			currency = r.CurrencyID
		}
		team, ok := teams[r.TeamID]
		if !ok {
			team = &payment.TeamAmount{TeamID: r.TeamID, TeamName: deref(r.TeamName)}
			teams[r.TeamID] = team
		}

		approved := r.Status == string(constants.PaymentStatusApproved)
		if approved {
			team.GrossAmount += r.GrossAmount
			team.ApprovedCount++
			if r.NetAmount != nil {
				teamNet[r.TeamID] += *r.NetAmount
				team.NetKnownCount++
			}
			if i, inWindow := monthIndex[r.CreatedAt.In(argentinaTZ).Format("2006-01")]; inWindow {
				monthly[i].GrossAmount += r.GrossAmount
				monthly[i].ApprovedCount++
				if r.NetAmount != nil {
					monthNet[i] += *r.NetAmount
					monthly[i].NetKnownCount++
				}
			}
		}

		attempts, ok := installments[r.InstallmentID]
		if !ok {
			attempts = &installmentAttempts{}
			installments[r.InstallmentID] = attempts
		}
		attempts.add(r, approved)
	}

	pending, rejected := 0, 0
	for _, a := range installments {
		if a.anyApproved {
			continue
		}
		switch constants.GroupOfPaymentStatus(a.latest.Status) {
		case constants.PaymentStatusGroupPending:
			pending++
			teams[a.latest.TeamID].PendingCount++
		case constants.PaymentStatusGroupRejected:
			rejected++
			teams[a.latest.TeamID].RejectedCount++
		}
	}

	for i := range monthly {
		monthly[i].GrossAmount = round2(monthly[i].GrossAmount)
		monthly[i].NetAmount = knownNet(monthNet[i], monthly[i].NetKnownCount)
	}

	byTeam := make([]payment.TeamAmount, 0, len(teams))
	for id, t := range teams {
		t.GrossAmount = round2(t.GrossAmount)
		t.NetAmount = knownNet(teamNet[id], t.NetKnownCount)
		byTeam = append(byTeam, *t)
	}
	sort.Slice(byTeam, func(i, j int) bool {
		if byTeam[i].GrossAmount != byTeam[j].GrossAmount {
			return byTeam[i].GrossAmount > byTeam[j].GrossAmount
		}
		if byTeam[i].TeamName != byTeam[j].TeamName {
			return byTeam[i].TeamName < byTeam[j].TeamName
		}
		return byTeam[i].TeamID < byTeam[j].TeamID
	})

	return &payment.ReceivedSummaryResponse{
		CurrencyID:    currency,
		Months:        months,
		Monthly:       monthly,
		ByTeam:        byTeam,
		PendingCount:  pending,
		RejectedCount: rejected,
		GeneratedAt:   formatUTC(s.now()),
	}, nil
}

// installmentAttempts guarda el último intento de pago de una cuota y si alguno
// fue aprobado: pendientes y rechazados se cuentan por cuota (design D8).
type installmentAttempts struct {
	latest      daos.ReceivedPaymentRow
	anyApproved bool
	seen        bool
}

func (a *installmentAttempts) add(r daos.ReceivedPaymentRow, approved bool) {
	if approved {
		a.anyApproved = true
	}
	if !a.seen || r.CreatedAt.After(a.latest.CreatedAt) ||
		(r.CreatedAt.Equal(a.latest.CreatedAt) && r.ID > a.latest.ID) {
		a.latest = r
		a.seen = true
	}
}

// knownNet devuelve nil si no hubo ningún neto informado por Mercado Pago.
func knownNet(sum float64, knownCount int) *float64 {
	if knownCount == 0 {
		return nil
	}
	v := round2(sum)
	return &v
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
