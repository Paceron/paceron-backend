package main

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils/gametime"
)

var errFullVerifyRollback = errors.New("rollback del login de verificación")

func (s *fullSeeder) verifyFull() error {
	if err := s.hydrateFullIDs(); err != nil {
		return err
	}
	if len(s.ids.users) == 0 {
		if err := ensureFullScopeEmpty(s.db); err != nil {
			return err
		}
		fmt.Println("Perfil full vacío: no hay filas atribuibles al seed.")
		return nil
	}
	if err := s.guardScope(); err != nil {
		return err
	}
	if err := s.verifyFullUsers(); err != nil {
		return err
	}
	if err := s.verifyFullCalendar(); err != nil {
		return err
	}
	if err := s.verifyFullGamification(); err != nil {
		return err
	}
	if err := s.verifyFullCompetitions(); err != nil {
		return err
	}
	if err := s.verifyFullPayments(); err != nil {
		return err
	}
	fmt.Println("Perfil full coherente: identidad, calendario, gamificación, competencias y pagos.")
	return nil
}

func (s *fullSeeder) hydrateFullIDs() error {
	users, err := fullOwnedUserIDs(s.db)
	if err != nil {
		return err
	}
	s.ids.users = users
	if len(users) == 0 {
		return nil
	}
	for _, spec := range fullTeamSpecs(s.scope, map[string]int64{
		"trainer_1": users["trainer_1"], "trainer_2": users["trainer_2"],
	}) {
		var team dbs.Team
		if err := s.db.Where("id = ? AND description LIKE ?", spec.id, "%"+fullSeedMarker+"%").Take(&team).Error; err != nil {
			return fmt.Errorf("verificar equipo reservado %d: %w", spec.id, err)
		}
		s.ids.teams["team_"+strings.ToLower(spec.key)] = team.ID
	}
	for _, spec := range fullGroupSpecs(s.scope) {
		var group dbs.Group
		if err := s.db.Where("id = ? AND team_id = ? AND description LIKE ?", spec.id, spec.teamID, "%"+fullSeedMarker+"%").Take(&group).Error; err != nil {
			return fmt.Errorf("verificar grupo reservado %d: %w", spec.id, err)
		}
		s.ids.groups[spec.key] = group.ID
	}
	if err := s.hydrateFullCatalogIDs(); err != nil {
		return err
	}
	if err := s.resolveFullSeedClock(); err != nil {
		return err
	}
	if err := hydrateFullCompetitionIDs(s); err != nil {
		return err
	}
	return nil
}

func (s *fullSeeder) hydrateFullCatalogIDs() error {
	ownerIDs := map[string]int64{"trainer_1": s.ids.users["trainer_1"], "trainer_2": s.ids.users["trainer_2"]}
	for _, spec := range fullExerciseSpecs() {
		id, err := markedCatalogID(s.db, "exercises", ownerIDs["trainer_1"], spec.name)
		if err != nil {
			return err
		}
		s.ids.exercises[spec.key] = id
	}
	for _, spec := range fullSecondaryExercises() {
		id, err := markedCatalogID(s.db, "exercises", ownerIDs["trainer_2"], spec.name)
		if err != nil {
			return err
		}
		s.ids.exercises[spec.key] = id
	}
	for _, spec := range fullSessionSpecs() {
		id, err := markedCatalogID(s.db, "sessions", ownerIDs["trainer_1"], spec.name)
		if err != nil {
			return err
		}
		s.ids.sessions[spec.key] = id
	}
	id, err := markedCatalogID(s.db, "sessions", ownerIDs["trainer_2"], "Semilla Full · Rodaje asincrónico B")
	if err != nil {
		return err
	}
	s.ids.sessions["full_b_async"] = id
	for _, spec := range fullTrainingPlanSpecs() {
		id, err := markedCatalogID(s.db, "training_plans", ownerIDs["trainer_1"], spec.name)
		if err != nil {
			return err
		}
		s.ids.trainingPlans[spec.key] = id
	}
	return nil
}

func markedCatalogID(db *gorm.DB, table string, ownerID int64, name string) (int64, error) {
	query := fmt.Sprintf("SELECT id FROM %s WHERE owner_id = ? AND name = ? AND description LIKE ?", table)
	var ids []int64
	if err := db.Raw(query, ownerID, name, "%"+fullSeedMarker+"%").Scan(&ids).Error; err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, fmt.Errorf("catálogo full %q en %s: se esperaba una fila marcada y hay %d", name, table, len(ids))
	}
	return ids[0], nil
}

func (s *fullSeeder) resolveFullSeedClock() error {
	return s.resolveFullClockForPlan(s.ids.trainingPlans["full_21_days"])
}

func (s *fullSeeder) resolveExistingFullClock() error {
	var trainer dbs.User
	err := s.db.Where("email = ?", "trainer1.seed-full@seed-full.local").Take(&trainer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("resolver entrenador para el ancla full: %w", err)
	}
	planName := "Semilla Full · Plan de 21 días"
	var plans []dbs.TrainingPlan
	if err := s.db.Where("owner_id = ? AND name = ?", trainer.ID, planName).Find(&plans).Error; err != nil {
		return fmt.Errorf("resolver plan ancla full: %w", err)
	}
	if len(plans) == 0 {
		return nil
	}
	if len(plans) != 1 || !containsFullSeedMarker(pointerValue(plans[0].Description)) {
		return fmt.Errorf("el plan ancla full no tiene identidad única y marcada")
	}
	return s.resolveFullClockForPlan(plans[0].ID)
}

func (s *fullSeeder) resolveFullClockForPlan(planID int64) error {
	var dates []time.Time
	if err := s.db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND source_plan_id = ?", s.scope.groupAMainID, planID).Order("date").Pluck("date", &dates).Error; err != nil {
		return fmt.Errorf("resolver ancla del calendario full: %w", err)
	}
	if len(dates) == 0 {
		return nil
	}
	anchor := fullDBDate(dates[0])
	var day dbs.GroupCalendarDay
	if err := s.db.Where("group_id = ? AND date = ?", s.scope.groupAMainID, anchor.AddDate(0, 0, 21)).Take(&day).Error; err == nil && day.PresencialOpenedAt != nil {
		clock := day.PresencialOpenedAt.In(gametime.Location).Add(20 * time.Minute)
		year, month, date := anchor.AddDate(0, 0, 21).Date()
		s.now = time.Date(year, month, date, clock.Hour(), clock.Minute(), clock.Second(), clock.Nanosecond(), gametime.Location)
		return nil
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("resolver hora ancla del calendario full: %w", err)
	}
	year, month, date := anchor.AddDate(0, 0, 21).Date()
	clock := s.now.In(gametime.Location)
	s.now = time.Date(year, month, date, clock.Hour(), clock.Minute(), clock.Second(), clock.Nanosecond(), gametime.Location)
	return nil
}

func fullOwnedUserIDs(db *gorm.DB) (map[string]int64, error) {
	ids := make(map[string]int64)
	for _, spec := range fullIdentityUsers() {
		var rows []dbs.User
		if err := db.Where("email = ?", spec.email).Find(&rows).Error; err != nil {
			return nil, fmt.Errorf("buscar identidad full %s: %w", spec.key, err)
		}
		if len(rows) > 1 {
			return nil, fmt.Errorf("identidad full %s ambigua", spec.key)
		}
		if len(rows) == 1 {
			if rows[0].DNI != spec.dni || !stringsHasFullSeedEmail(rows[0].Email) {
				return nil, fmt.Errorf("identidad reservada %s no coincide con la identidad full", spec.email)
			}
			ids[spec.key] = rows[0].ID
		}
	}
	if len(ids) != 0 && len(ids) != len(fullIdentityUsers()) {
		return nil, fmt.Errorf("identidad full incompleta: hay %d de %d usuarios; se aborta sin adoptar el scope", len(ids), len(fullIdentityUsers()))
	}
	var reservedUsers []dbs.User
	if err := db.Where("email LIKE ?", "%@seed-full.local").Find(&reservedUsers).Error; err != nil {
		return nil, fmt.Errorf("listar identidades reservadas full: %w", err)
	}
	if len(reservedUsers) != len(ids) {
		return nil, fmt.Errorf("hay identidades @seed-full.local fuera del conjunto conocido; se aborta")
	}
	return ids, nil
}

func (s *fullSeeder) verifyFullUsers() error {
	var verifyErr error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		service := services.NewAuthService(daos.NewAuthDao(tx), daos.NewUserRoleDao(tx), daos.NewRoleDao(tx), daos.NewRefreshTokenDao(tx), nil)
		for _, key := range []string{"trainer_1", "trainer_2", "r1", "r4", "r6", "r10"} {
			var user dbs.User
			if err := tx.First(&user, s.ids.users[key]).Error; err != nil {
				verifyErr = fmt.Errorf("login de %s: usuario ausente: %w", key, err)
				return errFullVerifyRollback
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(fullSeedPassword)); err != nil {
				verifyErr = fmt.Errorf("login de %s: contraseña inválida", key)
				return errFullVerifyRollback
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			if _, err := service.Login(ctx, user.Email, fullSeedPassword); err != nil {
				verifyErr = fmt.Errorf("login de %s: %w", key, err)
				return errFullVerifyRollback
			}
		}
		return errFullVerifyRollback
	})
	if errors.Is(err, errFullVerifyRollback) {
		return verifyErr
	}
	return err
}

func (s *fullSeeder) verifyFullCalendar() error {
	date := fullDate(s.now)
	var day dbs.GroupCalendarDay
	if err := s.db.Where("group_id = ? AND date = ?", s.scope.groupAMainID, date).Take(&day).Error; err != nil {
		return fmt.Errorf("calendario de hoy del grupo principal: %w", err)
	}
	if day.Kind != "training" || !day.IsPresencial || day.PresencialOpenedAt == nil || day.PresencialClosedAt != nil || day.SessionInstanceID == nil {
		return fmt.Errorf("calendario de hoy %s no tiene entrenamiento presencial abierto (row=%d kind=%s presencial=%t opened=%t closed=%t instance=%t)", date.Format(dateLayout), day.ID, day.Kind, day.IsPresencial, day.PresencialOpenedAt != nil, day.PresencialClosedAt != nil, day.SessionInstanceID != nil)
	}
	var instance dbs.SessionInstance
	if err := s.db.First(&instance, *day.SessionInstanceID).Error; err != nil {
		return fmt.Errorf("instancia del calendario de hoy: %w", err)
	}
	if instance.SourceSessionID == nil || *instance.SourceSessionID != s.ids.sessions["full_interval"] || !containsFullSeedMarker(pointerValue(instance.Description)) {
		return fmt.Errorf("la sesión del calendario de hoy no es la instancia full esperada")
	}
	var cancelled, future int64
	if err := s.db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ? AND kind = 'cancelled' AND source_plan_id = ?", s.scope.groupAMainID, date.AddDate(0, 0, 12), s.ids.trainingPlans["full_14_days"]).Count(&cancelled).Error; err != nil {
		return err
	}
	if err := s.db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND date = ? AND kind = 'training' AND is_presencial = true", s.scope.groupAMainID, date.AddDate(0, 0, 1)).Count(&future).Error; err != nil {
		return err
	}
	if cancelled != 1 || future != 1 {
		return fmt.Errorf("banners del calendario inconsistentes: cancelado=%d presencial_futuro=%d", cancelled, future)
	}
	return nil
}

func (s *fullSeeder) verifyFullGamification() error {
	streakService := services.NewStreakService(daos.NewGamificationDao(s.db))
	streaks, err := streakService.GetStreaks(nil, s.ids.users["r4"], 365)
	if err != nil {
		return fmt.Errorf("racha de R4: %w", err)
	}
	if streaks.Strict.Best != 10 || streaks.Flexible.Best != 10 {
		return fmt.Errorf("racha de R4 inconsistente: mejor estricta=%d flexible=%d", streaks.Strict.Best, streaks.Flexible.Best)
	}
	var cachedState dbs.GamificationStreakState
	if err := s.db.Where("user_id = ?", s.ids.users["r4"]).Take(&cachedState).Error; err != nil {
		return fmt.Errorf("estado de racha de R4: %w", err)
	}
	if cachedState.StrictCurrent != 10 || cachedState.FlexibleCurrent != 10 {
		return fmt.Errorf("estado de racha cacheado de R4 inconsistente: estricta=%d flexible=%d", cachedState.StrictCurrent, cachedState.FlexibleCurrent)
	}
	var achievements int64
	if err := s.db.Model(&dbs.UserAchievementProgress{}).Where("user_id = ? AND achievement_id IN ?", s.ids.users["r4"], []string{"first_session", "streak_3", "sessions_5", "streak_7", "team_first"}).Count(&achievements).Error; err != nil {
		return err
	}
	if achievements != 5 {
		return fmt.Errorf("logros full: se esperaban 5 y hay %d", achievements)
	}
	var unseen int64
	if err := s.db.Model(&dbs.GamificationUnseen{}).Where("user_id = ? AND section IN ? AND team_id IS NULL", s.ids.users["r4"], []string{string(constants.GamificationSectionStreaks), string(constants.GamificationSectionAchievements)}).Count(&unseen).Error; err != nil {
		return err
	}
	if unseen != 2 {
		return fmt.Errorf("unseen de gamificación full: se esperaban 2 y hay %d", unseen)
	}
	achievementsService := services.NewAchievementsService(daos.NewAchievementDao(s.db), daos.NewGamificationDao(s.db), streakService, nil, nil)
	achievementResponse, err := achievementsService.GetUserAchievements(nil, s.ids.users["r4"])
	if err != nil {
		return fmt.Errorf("logros de R4: %w", err)
	}
	if !achievementResponse.HasUnseen {
		return fmt.Errorf("R4 no tiene logros unseen")
	}
	unlocked := make(map[string]bool, 5)
	for _, achievement := range achievementResponse.Data {
		if achievement.UnlockedAt != nil {
			unlocked[achievement.AchievementID] = true
		}
	}
	for _, id := range []string{"first_session", "streak_3", "sessions_5", "streak_7", "team_first"} {
		if !unlocked[id] {
			return fmt.Errorf("logro %s de R4 no está desbloqueado", id)
		}
	}
	var stampCount int64
	if err := s.db.Model(&dbs.GamificationStamp{}).Where("user_id IN ? AND type = 'competition'", fullUserIDList(s.ids.users)).Count(&stampCount).Error; err != nil {
		return err
	}
	if stampCount != 3 {
		return fmt.Errorf("estampas de competencia full: se esperaban 3 y hay %d", stampCount)
	}
	stamps, err := achievementsService.GetUserStamps(nil, s.ids.users["r6"])
	if err != nil {
		return fmt.Errorf("estampas de R6: %w", err)
	}
	if len(stamps.Data) != 2 {
		return fmt.Errorf("estampas de R6: se esperaban 2 y hay %d", len(stamps.Data))
	}
	leaderboard := services.NewLeaderboardService(daos.NewLeaderboardDao(s.db))
	for _, metric := range []string{"distance", "duration", "pace"} {
		response, err := leaderboard.GetLeaderboard(nil, s.ids.users["r4"], s.scope.teamAID, metric, "all", nil, nil)
		if err != nil {
			return fmt.Errorf("leaderboard %s: %w", metric, err)
		}
		if len(response.Rows) == 0 || response.Record == nil {
			return fmt.Errorf("leaderboard %s vacío", metric)
		}
	}
	return nil
}

func (s *fullSeeder) verifyFullCompetitions() error {
	counts := map[string]int64{"pending_approval": 1, "active": 3, "finished": 3, "rejected": 1}
	for status, expected := range counts {
		var actual int64
		if err := s.db.Model(&dbs.Competition{}).Where("team_id = ? AND group_id IS NULL AND status = ?", s.scope.teamAID, status).Count(&actual).Error; err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("competencias %s: esperadas %d, obtenidas %d", status, expected, actual)
		}
	}
	for _, spec := range s.fullCompetitionSpecs() {
		var row dbs.Competition
		if err := s.db.First(&row, s.ids.competitions[spec.key]).Error; err != nil {
			return fmt.Errorf("competencia %s: %w", spec.key, err)
		}
		if err := s.validateFullCompetitionRow(s.db, &row, spec); err != nil {
			return fmt.Errorf("identidad de competencia %s: %w", spec.key, err)
		}
		if err := validateFullCompetitionFinalState(s.db, row.ID, spec); err != nil {
			return fmt.Errorf("estado de competencia %s: %w", spec.key, err)
		}
	}
	return nil
}

func (s *fullSeeder) verifyFullPayments() error {
	history := services.NewPaymentHistoryService(
		daos.NewPaymentHistoryDao(s.db), daos.NewPlatformSettingDao(s.db),
	)
	response, err := history.ListHistory(nil, s.ids.users["r1"], string(constants.PaymentHistoryTypeSubscription), "", 1)
	if err != nil {
		return fmt.Errorf("historial de pagos de R1: %w", err)
	}
	if len(response.Payments) != 3 {
		return fmt.Errorf("historial de pagos de R1: se esperaban 3 y hay %d", len(response.Payments))
	}
	r3History, err := history.ListHistory(nil, s.ids.users["r3"], string(constants.PaymentHistoryTypeSubscription), "", 1)
	if err != nil {
		return fmt.Errorf("historial de pagos de R3: %w", err)
	}
	if len(r3History.Payments) != 2 {
		return fmt.Errorf("historial de pagos de R3: se esperaban 2 y hay %d", len(r3History.Payments))
	}
	received, err := history.ListReceived(nil, s.ids.users["trainer_1"], int64Pointer(s.ids.teams["team_f"]), "approved", 1)
	if err != nil {
		return fmt.Errorf("historial de cobros de Team F: %w", err)
	}
	if len(received.Payments) != 1 {
		return fmt.Errorf("historial de cobros de Team F: se esperaba 1 y hay %d", len(received.Payments))
	}
	for _, userKey := range []string{"r1", "r2", "r3", "r10"} {
		var count int64
		if err := s.db.Model(&dbs.Payment{}).Where("external_reference LIKE ? AND user_id = ?", "seed_full:payment:%", s.ids.users[userKey]).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("historial de pagos de %s vacío", userKey)
		}
	}
	return nil
}

func hydrateFullCompetitionIDs(s *fullSeeder) error {
	for _, spec := range s.fullCompetitionSpecs() {
		var candidates []dbs.Competition
		query := s.db.Where("team_id = ? AND group_id IS NULL AND type = ? AND created_by_user_id = ? AND created_by_role = ?", s.scope.teamAID, spec.typeName, s.ids.users[spec.creatorKey], fullCompetitionCreatorRole(spec.creatorKey))
		if spec.typeName == "metric" {
			query = query.Where("metric = ? AND route_geojson IS NULL AND max_attempts IS NULL", *spec.metric)
		} else {
			query = query.Where("route_geojson = ? AND metric IS NULL AND acceptance_radius_meters = ? AND max_attempts IS NOT DISTINCT FROM ?", *spec.route, 50.0, spec.maxAttempts)
		}
		if err := query.Find(&candidates).Error; err != nil {
			return err
		}
		for i := range candidates {
			matches, err := s.validateFullCompetitionParticipants(s.db, candidates[i].ID, spec)
			if err != nil {
				return err
			}
			if matches {
				if current := s.ids.competitions[spec.key]; current != 0 && current != candidates[i].ID {
					return fmt.Errorf("competencia full %s ambigua", spec.key)
				}
				s.ids.competitions[spec.key] = candidates[i].ID
			}
		}
		if s.ids.competitions[spec.key] == 0 {
			return fmt.Errorf("falta la competencia full %s", spec.key)
		}
	}
	return nil
}

func fullUserIDList(users map[string]int64) []int64 {
	ids := make([]int64, 0, len(users))
	for _, id := range users {
		ids = append(ids, id)
	}
	return ids
}

func fullDBDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, gametime.Location)
}

func ensureFullScopeEmpty(db *gorm.DB) error {
	checks := []struct {
		table, where string
		args         []any
	}{
		{"teams", "id BETWEEN ? AND ? AND description LIKE ?", []any{fullTeamAID, fullTeamFID, "%" + fullSeedMarker + "%"}},
		{"groups", "id BETWEEN ? AND ? AND description LIKE ?", []any{fullGroupAMainID, fullGroupFID, "%" + fullSeedMarker + "%"}},
		{"users", "email LIKE ?", []any{"%@seed-full.local"}},
		{"exercises", "description LIKE ? AND owner_id IN (SELECT id FROM users WHERE email LIKE ?)", []any{"%" + fullSeedMarker + "%", "%@seed-full.local"}},
		{"sessions", "description LIKE ? AND owner_id IN (SELECT id FROM users WHERE email LIKE ?)", []any{"%" + fullSeedMarker + "%", "%@seed-full.local"}},
		{"training_plans", "description LIKE ? AND owner_id IN (SELECT id FROM users WHERE email LIKE ?)", []any{"%" + fullSeedMarker + "%", "%@seed-full.local"}},
		{"group_calendar_days", "source_plan_id IN (SELECT id FROM training_plans WHERE description LIKE ?)", []any{"%" + fullSeedMarker + "%"}},
		{"session_instances", "description LIKE ? OR source_session_id IN (SELECT id FROM sessions WHERE description LIKE ?)", []any{"%" + fullSeedMarker + "%", "%" + fullSeedMarker + "%"}},
		{"exercise_instances", "description LIKE ?", []any{"%" + fullSeedMarker + "%"}},
		{"competition", "team_id IN ?", []any{[]int64{fullTeamAID, fullTeamBID}}},
		{"payments", "external_reference LIKE ?", []any{"seed_full:payment:%"}},
		{"session_messages", "body LIKE ?", []any{"%[seed_full:message:%"}},
		{"push_tokens", "token LIKE ?", []any{"ExponentPushToken[seed-full-%"}},
		{"seller_connections", "mp_user_id = ? AND public_key = ?", []any{"seed-full-mp-user-trainer-1", "TEST-seed-full-trainer-1"}},
	}
	for _, check := range checks {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", check.table, check.where)
		if err := db.Raw(query, check.args...).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("verify vacío encontró %d filas atribuibles a full en %s", count, check.table)
		}
	}
	return nil
}
