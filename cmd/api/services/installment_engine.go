package services

import (
	"time"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// --- Motor de cuotas (compartido tier/equipo) ---
// El ciclo mensual y la deuda son las MISMAS reglas para suscripciones de tier
// (change cambio-tier-suscripciones) y de equipo (change suscripcion-teams-split),
// cambia solo el contexto del padre (subscription_id vs team_id). Acá viven las
// dos piezas compartidas: calcular la cuota N+1 y detectar deuda.

// CycleContext parametriza el ciclo mensual: arranque del ciclo y monto de la
// cuota (init_amount). Para tier es la sub (StartDate/InitAmount); para equipos,
// la membresía (team_user AssignmentDate/InitAmount).
type CycleContext struct {
	StartDate  time.Time
	InitAmount float64
}

// BuildNextInstallment arma la cuota N+1 del ciclo (D6/D5):
// amount = InitAmount; due_date = StartDate + 1 mes si recién se pagó la #1, si
// no due_date de la cuota pagada + 1 mes; blocked_date = due_date + 7 días de
// gracia. El padre se parametriza: exactamente uno de subscriptionID/teamID va
// seteado (arco exclusivo de installments).
func BuildNextInstallment(cc CycleContext, paid *dbs.Installment, subscriptionID, teamID *int64, userID int64) *dbs.Installment {
	base := cc.StartDate
	if paid.InstallmentNumber > 1 && paid.DueDate != nil {
		base = *paid.DueDate
	}
	dueDate := base.AddDate(0, 1, 0)
	blockedDate := dueDate.AddDate(0, 0, 7)

	return &dbs.Installment{
		SubscriptionID:    subscriptionID,
		TeamID:            teamID,
		UserID:            userID,
		InstallmentNumber: paid.InstallmentNumber + 1,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            cc.InitAmount,
		DueDate:           &dueDate,
		BlockedDate:       &blockedDate,
	}
}

// NextTierInstallment genera la cuota N+1 de una suscripción de tier.
func NextTierInstallment(sub *dbs.UserRoleTierSubscription, paid *dbs.Installment) *dbs.Installment {
	return BuildNextInstallment(
		CycleContext{StartDate: sub.StartDate, InitAmount: sub.InitAmount},
		paid,
		&sub.ID,
		nil,
		sub.UserID,
	)
}

// NextTeamInstallment genera la cuota N+1 de la membresía de un corredor a un
// equipo (change suscripcion-teams-split).
func NextTeamInstallment(tu *dbs.TeamUser, paid *dbs.Installment) *dbs.Installment {
	return BuildNextInstallment(
		CycleContext{StartDate: tu.AssignmentDate, InitAmount: tu.InitAmount},
		paid,
		nil,
		&tu.TeamID,
		tu.UserID,
	)
}

// HasPendingDebt indica si alguna cuota pendiente de la lista ya vence: cuota
// pending con blocked_date (o due_date) anterior a now. La cuota #1 (sin
// due_date/blocked_date) nunca cuenta como deuda. Compartido por tier y equipos
// (D5); para equipos además bloquea la salida (D4) y el uso de métodos de equipo.
func HasPendingDebt(pending []dbs.Installment) bool {
	now := time.Now()
	for _, ins := range pending {
		if (ins.BlockedDate != nil && ins.BlockedDate.Before(now)) ||
			(ins.DueDate != nil && ins.DueDate.Before(now)) {
			return true
		}
	}
	return false
}

// FirstInstallment arma la cuota #1 de un padre (subscription_id o team_id):
// pending, sin due_date/blocked_date (nunca genera deuda). La cantidad de campos
// de ciclo se setea con BuildNextInstallment a partir de la #2.
func FirstInstallment(subscriptionID, teamID *int64, userID int64, amount float64) *dbs.Installment {
	return &dbs.Installment{
		SubscriptionID:    subscriptionID,
		TeamID:            teamID,
		UserID:            userID,
		InstallmentNumber: 1,
		Status:            string(constants.InstallmentStatusPending),
		Amount:            amount,
	}
}