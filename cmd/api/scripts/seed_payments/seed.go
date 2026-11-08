package main

import (
	"fmt"
	"time"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

func (s *seeder) run() error {
	fmt.Printf("Seed payments: team=%d group=%d months=%d\n", s.opts.teamID, s.opts.groupID, s.opts.months)

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

	team, err := daos.NewTeamDao(s.db).FindByID(nil, s.opts.teamID)
	if err != nil || team == nil {
		return fmt.Errorf("no se pudo leer el equipo %d: %v", s.opts.teamID, err)
	}
	ownerID := team.OwnerID

	// Migrar payments viejos con concept incorrecto.
	s.db.Model(&dbs.Payment{}).Where("concept = ?", "team_fee").Update("concept", string(constants.PaymentConceptTeamSubscription))

	if s.opts.dryRun || !s.opts.seed {
		fmt.Println("[dry-run] no se escribió nada. Pasá --seed para sembrar.")
		return nil
	}

	// Meses a cubrir: del mes actual hacia atrás.
	now := time.Now()
	months := make([]time.Time, 0, s.opts.months)
	for m := s.opts.months; m >= 1; m-- {
		months = append(months, now.AddDate(0, -m, 0))
	}

	created := 0
	for _, uid := range userIDs {
		// Base de installment_number para este usuario.
		var maxNum int
		s.db.Model(&dbs.Installment{}).Where("team_id = ? AND user_id = ?", s.opts.teamID, uid).Select("COALESCE(MAX(installment_number),0)").Scan(&maxNum)

		for i, mt := range months {
			num := maxNum + i + 1
			due := time.Date(mt.Year(), mt.Month(), 15, 0, 0, 0, 0, time.UTC)

			// Idempotencia: si ya existe, actualizar status y asegurar payment.
			var existing dbs.Installment
			err := s.db.Where("team_id = ? AND user_id = ? AND installment_number = ?", s.opts.teamID, uid, num).First(&existing).Error
			if err == nil {
				s.db.Model(&existing).Update("status", string(constants.InstallmentStatusPaid))
				var pay dbs.Payment
				if s.db.Where("installment_id = ?", existing.ID).First(&pay).Error != nil {
					p := dbs.Payment{
						UserID:        &uid,
						PaymentID:     fmt.Sprintf("seed-pay-%d", existing.ID),
						Concept:       string(constants.PaymentConceptTeamSubscription),
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

			inst := dbs.Installment{
				TeamID:            &s.opts.teamID,
				UserID:            uid,
				InstallmentNumber: num,
				Status:            string(constants.InstallmentStatusPaid),
				Amount:            5000.0,
				DueDate:           &due,
			}
			if err := s.db.Create(&inst).Error; err != nil {
				return fmt.Errorf("error creando cuota %d para user %d: %w", num, uid, err)
			}
			p := dbs.Payment{
				UserID:        &uid,
				PaymentID:     fmt.Sprintf("seed-pay-%d", inst.ID),
				Concept:       string(constants.PaymentConceptTeamSubscription),
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