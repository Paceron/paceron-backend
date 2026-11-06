package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type TrainerHomeKpisQuery struct {
	Period        string
	ReferenceDate string
}

type TrainerHomeKpisServiceInterface interface {
	Get(ctx *gin.Context, trainerID int64, query TrainerHomeKpisQuery) (*dashboard.TrainerHomeKpisResponse, error)
}

type trainerHomeKpisService struct {
	dao               daos.TrainerHomeKpisDaoInterface
	paymentHistoryDao daos.PaymentHistoryDaoInterface
	now               func() time.Time
}

func NewTrainerHomeKpisService(
	dao daos.TrainerHomeKpisDaoInterface,
	paymentHistoryDao daos.PaymentHistoryDaoInterface,
) TrainerHomeKpisServiceInterface {
	return &trainerHomeKpisService{
		dao: dao, paymentHistoryDao: paymentHistoryDao, now: time.Now,
	}
}

func (s *trainerHomeKpisService) Get(ctx *gin.Context, trainerID int64, query TrainerHomeKpisQuery) (*dashboard.TrainerHomeKpisResponse, error) {
	reference := s.now()
	if raw := strings.TrimSpace(query.ReferenceDate); raw != "" {
		parsed, err := time.Parse(dashboard.DateLayoutISO, raw)
		if err != nil {
			return nil, newDashboardParamError(constants.ErrorCodeInvalidDate,
				"reference_date debe ser una fecha real en formato YYYY-MM-DD")
		}
		reference = parsed
	}
	periodName := strings.TrimSpace(query.Period)
	if periodName == "" {
		periodName = "month"
	}
	period, err := dashboard.ResolveRunnerPeriod(periodName, reference)
	if err != nil {
		return nil, newDashboardParamError(constants.ErrorCodeInvalidPeriod,
			"period debe ser uno de: month, week, day")
	}

	sessions, err := s.dao.SessionCounts(ctx, trainerID, period.From, period.To)
	if err != nil {
		customlogger.Error(ctx, "error counting trainer home sessions", err,
			customlogger.TagMethod("TrainerHomeKpis.Get"))
		return nil, fmt.Errorf("error al obtener el resumen del entrenador")
	}
	structure, err := s.dao.StructureCounts(ctx, trainerID)
	if err != nil {
		customlogger.Error(ctx, "error counting trainer home structure", err,
			customlogger.TagMethod("TrainerHomeKpis.Get"))
		return nil, fmt.Errorf("error al obtener el resumen del entrenador")
	}
	collections, err := s.collectionsForPeriod(ctx, trainerID, period.From, period.To)
	if err != nil {
		customlogger.Error(ctx, "error calculating trainer home collections", err,
			customlogger.TagMethod("TrainerHomeKpis.Get"))
		return nil, fmt.Errorf("error al obtener el resumen del entrenador")
	}

	return &dashboard.TrainerHomeKpisResponse{
		Period:         periodName,
		ReferenceDate:  dashboard.FormatDate(period.ReferenceDate),
		From:           dashboard.FormatDate(period.From),
		To:             dashboard.FormatDate(period.To),
		DaysConsidered: period.DaysConsidered,
		Sessions: dashboard.TrainerHomeSessions{
			Scheduled: sessions.Scheduled, Realized: sessions.Realized,
			PresencialScheduled: sessions.PresencialScheduled,
			PresencialRealized:  sessions.PresencialRealized,
		},
		Structure: dashboard.TrainerHomeStructure{
			ActiveTeams: structure.ActiveTeams, ActiveRunners: structure.ActiveRunners,
		},
		Collections: collections,
	}, nil
}

func (s *trainerHomeKpisService) collectionsForPeriod(ctx *gin.Context, trainerID int64, from, to time.Time) (dashboard.TrainerHomeCollections, error) {
	result := dashboard.TrainerHomeCollections{CurrencyID: "ARS"}
	fromLocal := periodDateInArgentina(from)
	toExclusive := periodDateInArgentina(to).AddDate(0, 0, 1)
	payments, err := s.paymentHistoryDao.ListReceivedBetween(ctx, trainerID, fromLocal, toExclusive)
	if err != nil {
		return result, fmt.Errorf("error listing received payments for period: %w", err)
	}
	for _, row := range payments {
		if row.CurrencyID != "" {
			result.CurrencyID = row.CurrencyID
		}
		if row.Status != string(constants.PaymentStatusApproved) {
			continue
		}
		if row.NetAmount != nil {
			result.NetCollected += *row.NetAmount
		} else {
			// Un pago aprobado siempre debería tener net_amount. Mantener el mismo
			// contrato neto si una fila histórica excepcional llega sin esa columna.
			result.NetCollected += row.GrossAmount - row.FeeAmount
		}
	}
	result.NetCollected = round2(result.NetCollected)

	memberships, err := s.paymentHistoryDao.ListTrainerFeeMemberships(ctx, trainerID)
	if err != nil {
		return result, fmt.Errorf("error listing trainer memberships for period: %w", err)
	}
	periodStart := periodDateInArgentina(from)
	periodEnd := periodDateInArgentina(to)
	for _, membership := range memberships {
		cycle := obligationCycle{
			Start: membership.AssignmentDate, Amount: membership.InitAmount, Paid: membership.PaidInstallments,
		}
		for _, due := range obligationsUntil(cycle, periodEnd) {
			dueDate := argentinaDay(due.DueDate)
			if dueDate.Before(periodStart) || dueDate.After(periodEnd) {
				continue
			}
			result.PendingAmount += due.Amount
			result.PendingCount++
		}
	}
	// pending_amount se presenta bruto, igual que MonthlyAmount.PendingAmount.
	result.PendingAmount = round2(result.PendingAmount)
	return result, nil
}

func periodDateInArgentina(value time.Time) time.Time {
	// ResolveRunnerPeriod representa fechas calendario a medianoche UTC. Para
	// llevar ese mismo día al rango de timestamps de cobros no hay que convertir
	// el instante UTC (que sería 21:00 del día anterior en ART), sino preservar
	// sus campos de fecha.
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, argentinaTZ)
}
