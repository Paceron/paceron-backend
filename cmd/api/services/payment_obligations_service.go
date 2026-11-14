package services

import (
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/payment"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// Lo pendiente del resumen de cobros y los próximos vencimientos del usuario
// (change cobros-pendientes-y-vencimientos, design D3 a D6). Las obligaciones
// salen de payment_obligations.go.

// addPendingObligations suma a cada mes de la ventana las obligaciones sin
// pagar que vencen en él, y arma los dos bloques: lo que vence desde hoy hasta
// fin de mes y todo lo que ya venció sin pagar, también de meses anteriores
// (una cuota de octubre que no se pagó se sigue debiendo en noviembre).
// windowEnd es el primer día del último mes de la ventana.
func (s *paymentHistoryService) addPendingObligations(ctx *gin.Context, sellerID int64, now time.Time, monthly []payment.MonthlyAmount, monthIndex map[string]int, windowEnd time.Time) (payment.ObligationsBlock, payment.ObligationsBlock, payment.PeriodAmounts, error) {
	due := payment.ObligationsBlock{Items: []payment.ObligationItem{}}
	overdue := payment.ObligationsBlock{Items: []payment.ObligationItem{}}
	periods := payment.PeriodAmounts{}
	memberships, err := s.dao.ListTrainerFeeMemberships(ctx, sellerID)
	if err != nil {
		customlogger.Error(ctx, "error listing fee memberships for summary", err, customlogger.TagMethod("GetReceivedSummary"))
		return due, overdue, periods, fmt.Errorf("error al obtener el resumen de cobros: %w", err)
	}
	if len(memberships) == 0 {
		return due, overdue, periods, nil
	}

	feePercent := s.currentFeePercent(ctx)
	today := argentinaDay(now)
	currentMonth := today.Format("2006-01")
	currentMonthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, argentinaTZ)
	lastMonthStart := currentMonthStart.AddDate(0, -1, 0)
	quarterStart := currentMonthStart.AddDate(0, -2, 0)
	// Hasta el último día de la ventana o del mes actual, lo que llegue más lejos.
	horizon := windowEnd.AddDate(0, 1, -1)
	if monthEnd := time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, argentinaTZ); monthEnd.After(horizon) {
		horizon = monthEnd
	}

	for _, m := range memberships {
		cycle := obligationCycle{Start: m.AssignmentDate, Amount: m.InitAmount, Paid: m.PaidInstallments}
		for _, o := range obligationsUntil(cycle, horizon) {
			net := netOfFee(o.Amount, feePercent)
			month := o.DueDate.In(argentinaTZ).Format("2006-01")
			if i, ok := monthIndex[month]; ok {
				monthly[i].PendingAmount += o.Amount
				monthly[i].PendingNetAmount += net
				monthly[i].PendingCount++
			}
			// Períodos del selector de KPIs: "todas" es vencidas + mes en curso;
			// el horizonte ya recorta los meses futuros.
			dueMonth := time.Date(o.DueDate.In(argentinaTZ).Year(), o.DueDate.In(argentinaTZ).Month(), 1, 0, 0, 0, 0, argentinaTZ)
			addPeriodAmount(&periods.All, o.Amount, net)
			if dueMonth.Equal(currentMonthStart) {
				addPeriodAmount(&periods.Current, o.Amount, net)
			}
			if dueMonth.Equal(lastMonthStart) {
				addPeriodAmount(&periods.Last, o.Amount, net)
			}
			if !dueMonth.Before(quarterStart) && !dueMonth.After(currentMonthStart) {
				addPeriodAmount(&periods.Quarter, o.Amount, net)
			}

			late := isOverdue(o.DueDate, today)
			if !late && month != currentMonth {
				continue
			}
			block := &due
			if late {
				block = &overdue
			}
			block.Count++
			block.Amount += o.Amount
			block.NetAmount += net
			block.Items = append(block.Items, payment.ObligationItem{
				Team:              payment.PaymentTeamRef{ID: m.TeamID, Name: deref(m.TeamName)},
				Payer:             payment.PaymentPayerRef{ID: m.UserID, Name: deref(m.UserName), Surname: deref(m.UserSurname), Email: deref(m.UserEmail)},
				InstallmentNumber: o.Number,
				Amount:            round2(o.Amount),
				DueDate:           formatUTC(o.DueDate),
			})
		}
	}

	for i := range monthly {
		monthly[i].PendingAmount = round2(monthly[i].PendingAmount)
		monthly[i].PendingNetAmount = round2(monthly[i].PendingNetAmount)
	}
	for _, b := range []*payment.ObligationsBlock{&due, &overdue} {
		b.Amount = round2(b.Amount)
		b.NetAmount = round2(b.NetAmount)
		items := b.Items
		sort.SliceStable(items, func(i, j int) bool { return items[i].DueDate < items[j].DueDate })
	}
	roundPeriodAmounts(&periods)
	return due, overdue, periods, nil
}

// addPeriodAmount acumula una cuota en un período.
func addPeriodAmount(block *payment.AmountPeriod, gross, net float64) {
	block.Count++
	block.Amount += gross
	block.NetAmount += net
}

// roundPeriodAmounts redondea los montos de los cuatro períodos.
func roundPeriodAmounts(periods *payment.PeriodAmounts) {
	for _, block := range []*payment.AmountPeriod{&periods.All, &periods.Current, &periods.Last, &periods.Quarter} {
		block.Amount = round2(block.Amount)
		block.NetAmount = round2(block.NetAmount)
	}
}

// rejectedPeriodsFromRows cuenta, por período, las cuotas cuyo último intento
// quedó rechazado y que nunca tuvieron un intento aprobado. Se agrupa por cuota
// (design D8), no por intento.
func rejectedPeriodsFromRows(rows []daos.ReceivedPaymentRow, now time.Time) payment.PeriodAmounts {
	periods := payment.PeriodAmounts{}
	attemptsByInstallment := map[int64]*installmentAttempts{}
	for _, r := range rows {
		attempts, ok := attemptsByInstallment[r.InstallmentID]
		if !ok {
			attempts = &installmentAttempts{}
			attemptsByInstallment[r.InstallmentID] = attempts
		}
		attempts.add(r, r.Status == string(constants.PaymentStatusApproved))
	}
	today := argentinaDay(now)
	currentMonthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, argentinaTZ)
	lastMonthStart := currentMonthStart.AddDate(0, -1, 0)
	quarterStart := currentMonthStart.AddDate(0, -2, 0)
	for _, a := range attemptsByInstallment {
		if a.anyApproved || constants.GroupOfPaymentStatus(a.latest.Status) != constants.PaymentStatusGroupRejected {
			continue
		}
		gross := a.latest.GrossAmount
		net := gross - a.latest.FeeAmount
		month := time.Date(a.latest.CreatedAt.In(argentinaTZ).Year(), a.latest.CreatedAt.In(argentinaTZ).Month(), 1, 0, 0, 0, 0, argentinaTZ)
		addPeriodAmount(&periods.All, gross, net)
		if month.Equal(currentMonthStart) {
			addPeriodAmount(&periods.Current, gross, net)
		}
		if month.Equal(lastMonthStart) {
			addPeriodAmount(&periods.Last, gross, net)
		}
		if !month.Before(quarterStart) && !month.After(currentMonthStart) {
			addPeriodAmount(&periods.Quarter, gross, net)
		}
	}
	roundPeriodAmounts(&periods)
	return periods
}

// payersFromRows devuelve los corredores distintos del historial, ordenados por
// apellido y nombre para el selector de filtros.
func payersFromRows(rows []daos.ReceivedPaymentRow) []payment.PaymentPayerRef {
	seen := make(map[int64]payment.PaymentPayerRef, len(rows))
	for _, r := range rows {
		if r.PayerID == 0 {
			continue
		}
		if _, ok := seen[r.PayerID]; ok {
			continue
		}
		seen[r.PayerID] = payment.PaymentPayerRef{
			ID:      r.PayerID,
			Name:    deref(r.PayerName),
			Surname: deref(r.PayerSurname),
			Email:   deref(r.PayerEmail),
		}
	}
	payers := make([]payment.PaymentPayerRef, 0, len(seen))
	for _, p := range seen {
		payers = append(payers, p)
	}
	sort.SliceStable(payers, func(i, j int) bool {
		if payers[i].Surname != payers[j].Surname {
			return payers[i].Surname < payers[j].Surname
		}
		return payers[i].Name < payers[j].Name
	})
	return payers
}

// GetUpcoming devuelve la próxima cuota sin pagar de cada membresía con cuota y
// de cada suscripción de tier del usuario, ordenadas por vencimiento.
func (s *paymentHistoryService) GetUpcoming(ctx *gin.Context, userID int64) (*payment.UpcomingPaymentsResponse, error) {
	memberships, err := s.dao.ListUserFeeMemberships(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error listing user fee memberships", err, customlogger.TagMethod("GetUpcoming"))
		return nil, fmt.Errorf("error al obtener los próximos vencimientos: %w", err)
	}
	subscriptions, err := s.dao.ListUserTierSubscriptions(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "error listing user tier subscriptions", err, customlogger.TagMethod("GetUpcoming"))
		return nil, fmt.Errorf("error al obtener los próximos vencimientos: %w", err)
	}

	today := s.now()
	items := make([]payment.UpcomingPaymentItem, 0, len(memberships)+len(subscriptions))
	add := func(c obligationCycle, item payment.UpcomingPaymentItem) {
		ob, ok := nextObligation(c)
		if !ok {
			return
		}
		item.InstallmentNumber = ob.Number
		item.Amount = round2(ob.Amount)
		item.CurrencyID = "ARS"
		item.DueDate = formatUTC(ob.DueDate)
		item.Overdue = isOverdue(ob.DueDate, today)
		item.OverdueCount = countOverdue(c, today)
		items = append(items, item)
	}
	for _, m := range memberships {
		item := payment.UpcomingPaymentItem{
			Type: string(constants.PaymentHistoryTypeTrainerPayment),
			Team: &payment.PaymentTeamRef{ID: m.TeamID, Name: deref(m.TeamName)},
		}
		if m.TrainerID != nil {
			item.Trainer = &payment.PaymentTrainerRef{ID: *m.TrainerID, Name: deref(m.TrainerName), Surname: deref(m.TrainerSurname)}
		}
		add(obligationCycle{Start: m.AssignmentDate, Amount: m.InitAmount, Paid: m.PaidInstallments}, item)
	}
	for _, sub := range subscriptions {
		add(obligationCycle{Start: sub.StartDate, Amount: sub.InitAmount, Paid: sub.PaidInstallments}, payment.UpcomingPaymentItem{
			Type: string(constants.PaymentHistoryTypeSubscription),
			Tier: &payment.PaymentTierRef{ID: sub.TierID, Name: deref(sub.TierName), RoleName: deref(sub.TierRoleName)},
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].DueDate < items[j].DueDate })
	return &payment.UpcomingPaymentsResponse{Items: items}, nil
}

// currentFeePercent es la comisión vigente, con el mismo criterio que
// resolveTeamSplitConfig: 5% si no está configurada o no se puede leer.
func (s *paymentHistoryService) currentFeePercent(ctx *gin.Context) float64 {
	if s.settingDao == nil {
		return daos.DefaultMarketplaceFeePercent
	}
	percent, setting, err := s.settingDao.Get(ctx, "marketplace_fee_percent", daos.DefaultMarketplaceFeePercent)
	if err != nil || setting == nil {
		return daos.DefaultMarketplaceFeePercent
	}
	return percent
}

// netOfFee es el monto menos la comisión, redondeada igual que al cobrar.
func netOfFee(amount, feePercent float64) float64 {
	return round2(amount - round2(amount*feePercent/100))
}
