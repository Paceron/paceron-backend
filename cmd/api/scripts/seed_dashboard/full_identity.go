package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const fullSeedPassword = "Paceron.123"

type fullIdentityUser struct {
	key, name, surname, email, dni, status string
	roleTiers                              []string
	allowInvitations                       bool
}

type fullIdentitySubscription struct {
	key, userKey, tierKey, status string
	start                         time.Time
	paidInstallments              int
	installments                  []dbs.Installment
}

func (s *fullSeeder) seedIdentity() error {
	userIDs := make(map[string]int64)
	roleIDs := make(map[string]int64)
	tierIDs := make(map[string]int64)
	tierAmounts := make(map[string]float64)
	subscriptionIDs := make(map[string]int64)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(fullSeedPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashear contraseña del perfil full: %w", err)
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		roleSpecs := []struct{ key, name string }{
			{"entrenador", "entrenador"}, {"corredor", "corredor"},
		}
		for _, spec := range roleSpecs {
			role, err := fullIdentityRole(tx, spec.name, s.now)
			if err != nil {
				return err
			}
			roleIDs[spec.key] = role.ID
		}

		permissionNames := []string{
			"consultar_entrenamientos", "registrar_entrenamientos", "ver_progreso",
			"crear_equipos", "administrar_grupos", "asignar_entrenamientos",
		}
		permissionIDs := make(map[string]int64, len(permissionNames))
		for _, name := range permissionNames {
			permission, err := fullIdentityPermission(tx, name, s.now)
			if err != nil {
				return err
			}
			permissionIDs[name] = permission.ID
		}

		tierPermissions := map[string][]string{
			"corredor_base":      {"consultar_entrenamientos"},
			"corredor_medium":    {"consultar_entrenamientos", "registrar_entrenamientos"},
			"corredor_premium":   {"consultar_entrenamientos", "registrar_entrenamientos", "ver_progreso"},
			"entrenador_base":    {"crear_equipos"},
			"entrenador_medium":  {"crear_equipos", "administrar_grupos"},
			"entrenador_premium": {"crear_equipos", "administrar_grupos", "asignar_entrenamientos"},
		}
		for _, roleKey := range []string{"corredor", "entrenador"} {
			for hierarchy, name := range []string{"base", "medium", "premium"} {
				key := roleKey + "_" + name
				tier, err := fullIdentityTier(tx, roleIDs[roleKey], roleKey, name, hierarchy+1, s.now)
				if err != nil {
					return err
				}
				tierIDs[key] = tier.ID
				tierAmounts[key] = tier.TierAmount
				for _, permissionName := range tierPermissions[key] {
					if err := ensureFullTierPermission(tx, tier.ID, permissionIDs[permissionName], s.now); err != nil {
						return fmt.Errorf("asociar permiso %s al tier %s: %w", permissionName, key, err)
					}
				}
			}
		}

		users := fullIdentityUsers()
		for _, user := range users {
			id, err := ensureFullIdentityUser(tx, user, passwordHash, s.now)
			if err != nil {
				return err
			}
			userIDs[user.key] = id
			for _, tierKey := range user.roleTiers {
				roleKey := "corredor"
				if len(tierKey) >= len("entrenador") && tierKey[:len("entrenador")] == "entrenador" {
					roleKey = "entrenador"
				}
				if err := ensureFullUserRole(tx, id, roleIDs[roleKey], tierIDs[tierKey], s.now); err != nil {
					return fmt.Errorf("asignar rol %s a usuario %s: %w", roleKey, user.key, err)
				}
			}
		}

		if err := ensureFullMarketplaceFee(tx, s.now); err != nil {
			return err
		}

		for _, subscription := range fullIdentitySubscriptions(s.now, tierAmounts) {
			userID := userIDs[subscription.userKey]
			tierID := tierIDs[subscription.tierKey]
			amount := tierAmounts[subscription.tierKey]
			roleKey := "corredor"
			roleID := roleIDs[roleKey]
			sub, err := ensureFullSubscription(tx, userID, roleID, tierID, amount, subscription, s.now)
			if err != nil {
				return fmt.Errorf("sembrar suscripción %s: %w", subscription.key, err)
			}
			subscriptionIDs[subscription.key] = sub.ID
			for i := range subscription.installments {
				installment := subscription.installments[i]
				installment.Amount = amount
				installment.SubscriptionID = &sub.ID
				installment.UserID = userID
				if err := ensureFullInstallment(tx, installment, s.now); err != nil {
					return fmt.Errorf("sembrar cuota %s/%d: %w", subscription.key, installment.InstallmentNumber, err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	for key, id := range userIDs {
		s.ids.users[key] = id
	}
	for key, id := range roleIDs {
		s.ids.roles[key] = id
	}
	for key, id := range tierIDs {
		s.ids.tiers[key] = id
	}
	for key, id := range subscriptionIDs {
		s.ids.subscriptions[key] = id
	}
	return nil
}

func fullIdentityUsers() []fullIdentityUser {
	users := []fullIdentityUser{
		{key: "trainer_1", name: "Lucía", surname: "Entrenadora", email: "trainer1.seed-full@seed-full.local", dni: "990200001", status: "active", roleTiers: []string{"entrenador_base", "corredor_base"}, allowInvitations: true},
		{key: "trainer_2", name: "Mateo", surname: "Entrenador", email: "trainer2.seed-full@seed-full.local", dni: "990200002", status: "active", roleTiers: []string{"entrenador_base", "corredor_base"}, allowInvitations: true},
	}
	for i := 1; i <= 12; i++ {
		tierKey := "corredor_base"
		if i == 1 || i == 2 {
			tierKey = "corredor_premium"
		} else if i == 3 {
			tierKey = "corredor_medium"
		}
		users = append(users, fullIdentityUser{
			key: fmt.Sprintf("r%d", i), name: fmt.Sprintf("Corredor %d", i), surname: "Perfil Full",
			email: fmt.Sprintf("runner%d.seed-full@seed-full.local", i), dni: fmt.Sprintf("990200%03d", i+2),
			status: "active", roleTiers: []string{tierKey}, allowInvitations: true,
		})
	}
	return append(users,
		fullIdentityUser{key: "blocked", name: "Corredor Bloqueado", surname: "Perfil Full", email: "blocked.seed-full@seed-full.local", dni: "990200015", status: string(constants.UserStatusBlocked), roleTiers: []string{"corredor_base"}, allowInvitations: true},
		fullIdentityUser{key: "inactive", name: "Corredor Inactivo", surname: "Perfil Full", email: "inactive.seed-full@seed-full.local", dni: "990200016", status: string(constants.UserStatusInactive), roleTiers: []string{"corredor_base"}, allowInvitations: true},
		fullIdentityUser{key: "no_role", name: "Sin Rol", surname: "Perfil Full", email: "no-role.seed-full@seed-full.local", dni: "990200017", status: "active", allowInvitations: true},
		fullIdentityUser{key: "invitations_off", name: "Invitaciones Desactivadas", surname: "Perfil Full", email: "invitations-off.seed-full@seed-full.local", dni: "990200018", status: "active", roleTiers: []string{"corredor_base"}, allowInvitations: false},
	)
}

func fullIdentitySubscriptions(now time.Time, tierAmounts map[string]float64) []fullIdentitySubscription {
	r1Amount := tierAmounts["corredor_premium"]
	r2Amount := tierAmounts["corredor_premium"]
	r3Amount := tierAmounts["corredor_medium"]
	startR1 := fullEndOfCordobaDay(now).AddDate(0, -2, 0)
	startR2 := fullStartForThirdInstallmentDue(now.AddDate(0, 0, -32))
	startR3 := now.AddDate(0, -2, 0)
	return []fullIdentitySubscription{
		{
			key: "r1_premium_active", userKey: "r1", tierKey: "corredor_premium", status: "active",
			start: startR1, paidInstallments: 2, installments: fullInstallmentSequence(startR1, r1Amount, 3, 2),
		},
		{
			key: "r2_premium_debt", userKey: "r2", tierKey: "corredor_premium", status: "active",
			start: startR2, paidInstallments: 2, installments: fullInstallmentSequence(startR2, r2Amount, 3, 2),
		},
		{
			key: "r3_medium_active", userKey: "r3", tierKey: "corredor_medium", status: "active",
			start: startR3, paidInstallments: 1,
			installments: fullInstallmentSequence(startR3, r3Amount, 1, 1),
		},
		{
			key: "r3_premium_pending", userKey: "r3", tierKey: "corredor_premium", status: string(constants.SubscriptionStatusFirstPaymentPending),
			start: now, paidInstallments: 0,
			installments: fullInstallmentSequence(now, r1Amount, 1, 0),
		},
	}
}

func fullEndOfCordobaDay(now time.Time) time.Time {
	local := now.In(gametime.Location)
	return time.Date(local.Year(), local.Month(), local.Day(), 23, 59, 59, 999_999_000, gametime.Location)
}

func fullStartForThirdInstallmentDue(target time.Time) time.Time {
	if target.Day() > 28 {
		// Día 28 existe en todos los meses; evita el rollover de AddDate al invertir el ciclo.
		target = target.AddDate(0, 0, 28-target.Day())
	}
	return target.AddDate(0, -2, 0)
}

func fullInstallmentSequence(startDate time.Time, amount float64, count, paidThrough int) []dbs.Installment {
	first := services.FirstInstallment(nil, nil, 0, amount)
	if paidThrough >= 1 {
		first.Status = string(constants.InstallmentStatusPaid)
	}
	sequence := []dbs.Installment{*first}
	paid := first
	cycle := services.CycleContext{StartDate: startDate, InitAmount: amount}
	for number := 2; number <= count; number++ {
		next := services.BuildNextInstallment(cycle, paid, nil, nil, 0)
		if number <= paidThrough {
			next.Status = string(constants.InstallmentStatusPaid)
			paid = next
		}
		sequence = append(sequence, *next)
	}
	return sequence
}

func fullIdentityRole(tx *gorm.DB, name string, now time.Time) (*dbs.Role, error) {
	var role dbs.Role
	err := tx.Where("name = ?", name).Take(&role).Error
	if err == nil {
		if role.DeletedAt != nil {
			return nil, fmt.Errorf("rol canónico %q está eliminado", name)
		}
		return &role, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("buscar rol canónico %q: %w", name, err)
	}
	role = dbs.Role{Name: name, Description: "Rol del perfil full", CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&role).Error; err != nil {
		return nil, fmt.Errorf("crear rol canónico %q: %w", name, err)
	}
	return &role, nil
}

func fullIdentityPermission(tx *gorm.DB, name string, now time.Time) (*dbs.Permission, error) {
	var permission dbs.Permission
	err := tx.Where("name = ?", name).Take(&permission).Error
	if err == nil {
		if permission.DeletedAt != nil {
			return nil, fmt.Errorf("permiso canónico %q está eliminado", name)
		}
		return &permission, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("buscar permiso canónico %q: %w", name, err)
	}
	permission = dbs.Permission{Name: name, Description: "Permiso del perfil full", CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&permission).Error; err != nil {
		return nil, fmt.Errorf("crear permiso canónico %q: %w", name, err)
	}
	return &permission, nil
}

func fullIdentityTier(tx *gorm.DB, roleID int64, roleName, name string, hierarchy int, now time.Time) (*dbs.Tier, error) {
	var tier dbs.Tier
	err := tx.Where("role_id = ? AND name = ?", roleID, name).Take(&tier).Error
	if err == nil {
		if tier.DeletedAt != nil {
			return nil, fmt.Errorf("tier canónico %s/%s está eliminado", roleName, name)
		}
		paymentRequired := hierarchy > 1
		if tier.RoleName != roleName || tier.Hierarchy != hierarchy || tier.PaymentRequired != paymentRequired ||
			(paymentRequired && tier.TierAmount <= 0) || (!paymentRequired && tier.TierAmount != 0) {
			return nil, fmt.Errorf("tier canónico %s/%s tiene configuración incompatible; no se sobrescribe", roleName, name)
		}
		return &tier, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("buscar tier canónico %s/%s: %w", roleName, name, err)
	}
	tier = dbs.Tier{
		Name: name, Description: fmt.Sprintf("Tier %s del perfil full", name), RoleID: roleID,
		RoleName: roleName, PaymentRequired: hierarchy > 1, TierAmount: fullTierAmount(hierarchy),
		Hierarchy: hierarchy, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&tier).Error; err != nil {
		return nil, fmt.Errorf("crear tier canónico %s/%s: %w", roleName, name, err)
	}
	return &tier, nil
}

func fullTierAmount(hierarchy int) float64 {
	switch hierarchy {
	case 2:
		return 900
	case 3:
		return 1500
	default:
		return 0
	}
}

func ensureFullTierPermission(tx *gorm.DB, tierID, permissionID int64, now time.Time) error {
	var count int64
	if err := tx.Model(&dbs.TierPermission{}).
		Where("tier_id = ? AND permission_id = ? AND deleted_at IS NULL", tierID, permissionID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.Create(&dbs.TierPermission{
		TierID: tierID, PermissionID: permissionID, AsignationDate: now, CreatedAt: now, UpdatedAt: now,
	}).Error
}

func ensureFullIdentityUser(tx *gorm.DB, spec fullIdentityUser, passwordHash []byte, now time.Time) (int64, error) {
	var user dbs.User
	err := tx.Where("email = ?", spec.email).Take(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = dbs.User{
			Name: spec.name, Surname: spec.surname, Email: spec.email, DNI: spec.dni,
			BirthDate: now.AddDate(-30, 0, 0), Password: string(passwordHash), Status: spec.status,
			AllowTeamInvitations: spec.allowInvitations, DefaultTheme: "dark", CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&user).Error; err != nil {
			return 0, fmt.Errorf("crear usuario sintético %s: %w", spec.key, err)
		}
		if !spec.allowInvitations {
			if err := tx.Model(&dbs.User{}).Where("id = ?", user.ID).UpdateColumn("allow_team_invitations", false).Error; err != nil {
				return 0, fmt.Errorf("desactivar invitaciones del usuario sintético %s: %w", spec.key, err)
			}
		}
		return user.ID, nil
	}
	if err != nil {
		return 0, fmt.Errorf("buscar usuario sintético %s: %w", spec.key, err)
	}
	if user.DNI != spec.dni {
		return 0, fmt.Errorf("email reservado %s ya pertenece a un usuario ajeno; se aborta", spec.email)
	}

	password := user.Password
	if bcrypt.CompareHashAndPassword([]byte(password), []byte(fullSeedPassword)) != nil {
		password = string(passwordHash)
	}
	updates := map[string]any{
		"name": spec.name, "surname": spec.surname, "birth_date": now.AddDate(-30, 0, 0),
		"password": password, "status": spec.status, "allow_team_invitations": spec.allowInvitations,
		"default_theme": "dark", "updated_at": now,
	}
	if err := tx.Model(&dbs.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		return 0, fmt.Errorf("actualizar usuario sintético %s: %w", spec.key, err)
	}
	return user.ID, nil
}

func ensureFullUserRole(tx *gorm.DB, userID, roleID, tierID int64, now time.Time) error {
	var assignment dbs.UserRole
	err := tx.Where("user_id = ? AND role_id = ?", userID, roleID).Order("id").Take(&assignment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&dbs.UserRole{
			UserID: userID, RoleID: roleID, TierID: tierID, AssignmentDate: now,
			DateSince: timePtr(now.AddDate(-1, 0, 0)), Status: "active", CreatedAt: now, UpdatedAt: now,
		}).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&dbs.UserRole{}).Where("id = ?", assignment.ID).Updates(map[string]any{
		"tier_id": tierID, "assignment_date": now, "date_since": now.AddDate(-1, 0, 0),
		"date_to": nil, "status": "active", "deleted_at": nil, "updated_at": now,
	}).Error
}

func ensureFullSubscription(tx *gorm.DB, userID, roleID, tierID int64, tierAmount float64, spec fullIdentitySubscription, now time.Time) (*dbs.UserRoleTierSubscription, error) {
	var subscription dbs.UserRoleTierSubscription
	err := tx.Where("user_id = ? AND role_id = ? AND tier_id = ? AND status = ?", userID, roleID, tierID, spec.status).Order("id").Take(&subscription).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		subscription = dbs.UserRoleTierSubscription{
			UserID: userID, RoleID: roleID, TierID: tierID, Status: spec.status,
			InitAmount: tierAmount, PaidInstallments: spec.paidInstallments,
			StartDate: spec.start, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&subscription).Error; err != nil {
			return nil, err
		}
		return &subscription, nil
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Model(&dbs.UserRoleTierSubscription{}).Where("id = ?", subscription.ID).Updates(map[string]any{
		"init_amount": tierAmount, "paid_installments": spec.paidInstallments,
		"start_date": spec.start, "ended_date": nil, "updated_at": now,
	}).Error; err != nil {
		return nil, err
	}
	subscription.TierID = tierID
	subscription.InitAmount = tierAmount
	subscription.PaidInstallments = spec.paidInstallments
	subscription.StartDate = spec.start
	subscription.EndedDate = nil
	return &subscription, nil
}

func ensureFullInstallment(tx *gorm.DB, spec dbs.Installment, now time.Time) error {
	var existing dbs.Installment
	err := tx.Where("subscription_id = ? AND installment_number = ?", spec.SubscriptionID, spec.InstallmentNumber).Take(&existing).Error
	values := map[string]any{
		"team_id": nil, "user_id": spec.UserID, "status": spec.Status,
		"amount":   spec.Amount,
		"due_date": spec.DueDate, "blocked_date": spec.BlockedDate, "updated_at": now,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		spec.CreatedAt, spec.UpdatedAt = now, now
		return tx.Create(&spec).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&dbs.Installment{}).Where("id = ?", existing.ID).Updates(values).Error
}

func ensureFullMarketplaceFee(tx *gorm.DB, now time.Time) error {
	var setting dbs.PlatformSetting
	err := tx.Where("key = ?", "marketplace_fee_percent").Take(&setting).Error
	if err == nil {
		var value any
		if err := json.Unmarshal([]byte(setting.Value), &value); err != nil {
			return fmt.Errorf("marketplace_fee_percent existente no es JSON numérico: %w", err)
		}
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("marketplace_fee_percent existente no es JSON numérico")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("buscar marketplace_fee_percent: %w", err)
	}
	return tx.Create(&dbs.PlatformSetting{
		Key: "marketplace_fee_percent", Value: "5", CreatedAt: now, UpdatedAt: now,
	}).Error
}

func timePtr(value time.Time) *time.Time {
	return &value
}
