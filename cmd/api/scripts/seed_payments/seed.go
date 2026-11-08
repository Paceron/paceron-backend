package main

import (
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

func (s *seeder) run() error {
	fmt.Printf("Seed payments: team=%d group=%d count=%d\n", s.opts.teamID, s.opts.groupID, s.opts.count)

	members, err := daos.NewGroupUserDao(s.db).FindByGroupID(nil, s.opts.groupID)
	if err != nil {
		return fmt.Errorf("error leyendo miembros del grupo: %w", err)
	}
	var userIDs []int64
	for _, m := range members {
		if m.DeletedAt == nil {
			userIDs = append(userIDs, m.UserID)
		}
	}
	fmt.Printf("Corredores del grupo: %d\n", len(userIDs))

	if s.opts.dryRun || !s.opts.seed {
		fmt.Println("[dry-run] no se escribió nada. Pasá --seed para sembrar.")
		return nil
	}

	// Owner del equipo (vendedor de las cuotas).
	team, err := daos.NewTeamDao(s.db).FindByID(nil, s.opts.teamID)
	if err != nil || team == nil {
		return fmt.Errorf("no se pudo leer el equipo %d: %v", s.opts.teamID, err)
	}
	ownerID := team.OwnerID

	created := 0
	for _, uid := range userIDs {
		for n := 1; n <= s.opts.count; n++ {
			// Idempotencia: si ya existe, saltar.
			var existing dbs.Installment
			err := s.db.Where("team_id = ? AND user_id = ? AND installment_number = ?", s.opts.teamID, uid, n).First(&existing).Error
			if err == nil {
				s.db.Model(&existing).Update("status", string(constants.InstallmentStatusPaid))
				// Asegurar Payment (pago recibido por el entrenador).
				var pay dbs.Payment
				if s.db.Where("installment_id = ?", existing.ID).First(&pay).Error != nil {
					p := dbs.Payment{
						UserID:        &uid,
						PaymentID:     fmt.Sprintf("seed-pay-%d", existing.ID),
						Concept:       "team_fee",
						Description:   "Cuota del equipo (seed)",
						Amount:        5000.0,
						CurrencyID:    "ARS",
						Status:        "approved",
						SellerUserID:  &ownerID,
						InstallmentID: &existing.ID,
					}
					s.db.Create(&p)
				}
				continue
			}
			status := constants.InstallmentStatusPaid
			var dueDate *time.Time
			d := time.Now().AddDate(0, 0, -10*(n-1))
			dueDate = &d
			inst := dbs.Installment{
				TeamID:            &s.opts.teamID,
				UserID:            uid,
				InstallmentNumber: n,
				Status:            string(status),
				Amount:            5000.0,
				DueDate:           dueDate,
			}
			if err := s.db.Create(&inst).Error; err != nil {
				return fmt.Errorf("error creando cuota %d para user %d: %w", n, uid, err)
			}
			// Payment recibido por el entrenador (para que aparezca en /profile/payments).
			p := dbs.Payment{
				UserID:        &uid,
				PaymentID:     fmt.Sprintf("seed-pay-%d", inst.ID),
				Concept:       "team_fee",
				Description:   "Cuota del equipo (seed)",
				Amount:        5000.0,
				CurrencyID:    "ARS",
				Status:        "approved",
				SellerUserID:  &ownerID,
				InstallmentID: &inst.ID,
			}
			s.db.Create(&p)
			created++
		}
	}
	fmt.Printf("Cuotas creadas: %d\n", created)
	return nil
}