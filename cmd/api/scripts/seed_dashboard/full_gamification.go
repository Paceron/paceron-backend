package main

import (
	"errors"
	"fmt"
	"time"

	appconstants "simple-arq-golang/cmd/api/constants"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const fullStreakPlanKey = "full_streak_9_days"

type fullGamificationAchievementSpec struct {
	userKey, achievementID string
	dateOffset             int
}

func (s *fullSeeder) seedGamification() error {
	if err := s.requireFullGamificationPrerequisites(); err != nil {
		return err
	}

	calendar := fullCalendarService(s.db)
	if err := s.stampFullPlan(calendar, "group_a_secondary", fullStreakPlanKey, s.now.AddDate(0, 0, -16)); err != nil {
		return fmt.Errorf("estampar plan auxiliar de racha: %w", err)
	}

	runnerIDs := make(map[string]int64, 13)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for day := 0; day < 9; day++ {
			date := fullDate(s.now.AddDate(0, 0, -16+day))
			calendarKey := fullStreakPlanKey + "_" + date.Format("2006-01-02")
			instanceID := s.ids.sessionInstances[calendarKey]
			if instanceID <= 0 {
				var row dbs.GroupCalendarDay
				if err := tx.Where("group_id = ? AND date = ?", s.ids.groups["group_a_secondary"], date).Take(&row).Error; err != nil {
					return fmt.Errorf("resolver instancia del día de racha %s: %w", date.Format("2006-01-02"), err)
				}
				if row.SessionInstanceID == nil {
					return fmt.Errorf("el día de racha %s no tiene instancia", date.Format("2006-01-02"))
				}
				instanceID = *row.SessionInstanceID
				s.ids.calendarDays[calendarKey] = row.ID
				s.ids.sessionInstances[calendarKey] = instanceID
			}
			for _, userKey := range []string{"r4", "r5"} {
				if userKey == "r5" && day >= 4 {
					continue
				}
				start := fullActivityClock(date, 7, 0)
				end := fullActivityClock(date, 8, 0)
				expected := dbs.RunnerSession{
					SessionInstanceID: instanceID, AthleteUserID: s.ids.users[userKey],
					Status:    runnersession.RunnerSessionStatusFinished,
					StartDate: start, EndDate: &end, CreatedAt: s.now, UpdatedAt: s.now,
				}
				row, err := ensureFullGamificationRunner(tx, expected)
				if err != nil {
					return fmt.Errorf("runner_session de racha %s/%s: %w", userKey, date.Format("2006-01-02"), err)
				}
				runnerIDs[fmt.Sprintf("streak_%s_%s", userKey, date.Format("2006-01-02"))] = row.ID
			}
		}

		streakService := services.NewStreakService(daos.NewGamificationDao(tx))
		for _, userKey := range []string{"r4", "r5", "r9"} {
			userID := s.ids.users[userKey]
			strict, flexible, err := streakService.ComputeCurrentStreaks(nil, userID)
			if err != nil {
				return fmt.Errorf("calcular rachas de %s: %w", userKey, err)
			}
			if err := ensureFullGamificationStreakState(tx, userID, strict, flexible, s.now); err != nil {
				return fmt.Errorf("persistir estado de rachas de %s: %w", userKey, err)
			}
		}

		for _, spec := range fullGamificationAchievementSpecs() {
			if err := ensureFullGamificationAchievement(tx, s.ids.users[spec.userKey], spec.achievementID, fullDate(s.now.AddDate(0, 0, spec.dateOffset))); err != nil {
				return fmt.Errorf("achievement %s para %s: %w", spec.achievementID, spec.userKey, err)
			}
		}
		for _, spec := range []struct{ userKey, section string }{
			{"r4", constants.GamificationSectionStreaks},
			{"r4", constants.GamificationSectionAchievements},
		} {
			if err := ensureFullGamificationUnseen(tx, s.ids.users[spec.userKey], spec.section, s.now); err != nil {
				return fmt.Errorf("unseen %s para %s: %w", spec.section, spec.userKey, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sembrar gamificación del perfil full: %w", err)
	}
	mergeFullActivityIDs(s.ids.runnerSessions, runnerIDs)
	return nil
}

func (s *fullSeeder) requireFullGamificationPrerequisites() error {
	for _, key := range []string{"trainer_1", "r3", "r4", "r5", "r9"} {
		if s.ids.users[key] <= 0 {
			return fmt.Errorf("seedGamification requiere seedIdentity: falta %s", key)
		}
	}
	if s.ids.teams["team_a"] <= 0 {
		return fmt.Errorf("seedGamification requiere seedTeams: falta team_a")
	}
	if s.ids.groups["group_a_main"] <= 0 || s.ids.groups["group_a_secondary"] <= 0 {
		return fmt.Errorf("seedGamification requiere seedTeams: faltan los grupos A")
	}
	if s.ids.trainingPlans[fullStreakPlanKey] <= 0 || s.ids.sessions["full_interval"] <= 0 {
		return fmt.Errorf("seedGamification requiere seedCatalogCalendar: falta el catálogo auxiliar de racha")
	}
	if s.ids.sessionInstances[fullActivitySecondaryDayKey] <= 0 || s.ids.runnerSessions["finished_r4_minus7"] <= 0 {
		return fmt.Errorf("seedGamification requiere seedActivity: falta la sesión finished de R4 en -7")
	}

	var group dbs.Group
	if err := s.db.First(&group, s.ids.groups["group_a_secondary"]).Error; err != nil {
		return fmt.Errorf("seedGamification requiere el grupo secundario full: %w", err)
	}
	if group.TeamID != s.ids.teams["team_a"] || !containsFullSeedMarker(group.Description) {
		return fmt.Errorf("seedGamification rechaza el grupo secundario ajeno al perfil full")
	}
	var plan dbs.TrainingPlan
	if err := s.db.First(&plan, s.ids.trainingPlans[fullStreakPlanKey]).Error; err != nil {
		return fmt.Errorf("seedGamification requiere el plan full de racha: %w", err)
	}
	if plan.OwnerID != s.ids.users["trainer_1"] || !containsFullSeedMarker(pointerValue(plan.Description)) {
		return fmt.Errorf("seedGamification rechaza el plan auxiliar ajeno al perfil full")
	}
	var membershipCount int64
	for _, userKey := range []string{"r4", "r5"} {
		if err := s.db.Model(&dbs.GroupUser{}).
			Where("group_id = ? AND user_id = ? AND deleted_at IS NULL AND date_start::date <= ?::date AND (date_end IS NULL OR date_end::date > ?::date)",
				s.ids.groups["group_a_secondary"], s.ids.users[userKey], fullDate(s.now.AddDate(0, 0, -16)), fullDate(s.now.AddDate(0, 0, -16))).
			Count(&membershipCount).Error; err != nil {
			return fmt.Errorf("validar membresía de %s: %w", userKey, err)
		}
		if membershipCount != 1 {
			return fmt.Errorf("seedGamification requiere membresía activa de %s desde el primer día de racha", userKey)
		}
		membershipCount = 0
	}
	var futureMembership dbs.GroupUser
	if err := s.db.Where("group_id = ? AND user_id = ? AND deleted_at IS NULL", s.ids.groups["group_a_secondary"], s.ids.users["r9"]).Take(&futureMembership).Error; err != nil {
		return fmt.Errorf("seedGamification requiere el alta futura de R9 en el grupo secundario: %w", err)
	}
	if !fullDate(futureMembership.DateStart).After(fullDate(s.now.AddDate(0, 0, -8))) {
		return fmt.Errorf("seedGamification requiere que R9 ingrese después de los días auxiliares")
	}
	var activity dbs.RunnerSession
	if err := s.db.First(&activity, s.ids.runnerSessions["finished_r4_minus7"]).Error; err != nil {
		return fmt.Errorf("leer actividad de R4 en -7: %w", err)
	}
	if activity.AthleteUserID != s.ids.users["r4"] || activity.Status != runnersession.RunnerSessionStatusFinished || activity.SessionInstanceID != s.ids.sessionInstances[fullActivitySecondaryDayKey] {
		return fmt.Errorf("seedGamification rechaza la actividad previa de R4 en -7")
	}
	return nil
}

func fullGamificationAchievementSpecs() []fullGamificationAchievementSpec {
	return []fullGamificationAchievementSpec{
		{userKey: "r4", achievementID: "first_session", dateOffset: -16},
		{userKey: "r4", achievementID: "streak_3", dateOffset: -14},
		{userKey: "r4", achievementID: "sessions_5", dateOffset: -12},
		{userKey: "r4", achievementID: "streak_7", dateOffset: -10},
		{userKey: "r4", achievementID: "team_first", dateOffset: -7},
	}
}

func ensureFullGamificationRunner(tx *gorm.DB, expected dbs.RunnerSession) (*dbs.RunnerSession, error) {
	var existing dbs.RunnerSession
	err := tx.Where("session_instance_id = ? AND athlete_user_id = ?", expected.SessionInstanceID, expected.AthleteUserID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(&expected).Error; err != nil {
			return nil, err
		}
		return &expected, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.Status != expected.Status || !existing.StartDate.Equal(expected.StartDate) || !sameTimePointer(existing.EndDate, expected.EndDate) || existing.CompetitionID != nil {
		return nil, fmt.Errorf("existe una sesión de corredor distinta; se rechaza actividad ajena")
	}
	return &existing, nil
}

func ensureFullGamificationStreakState(tx *gorm.DB, userID int64, strict, flexible int, now time.Time) error {
	state := dbs.GamificationStreakState{
		UserID: userID, StrictCurrent: strict, FlexibleCurrent: flexible, UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"strict_current", "flexible_current", "updated_at"}),
	}).Create(&state).Error
}

func ensureFullGamificationAchievement(tx *gorm.DB, userID int64, achievementID string, unlockedAt time.Time) error {
	valid := false
	for _, row := range appconstants.AchievementCatalog {
		if row.ID == achievementID {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("el ID no pertenece al catálogo de logros")
	}
	expected := dbs.UserAchievementProgress{
		UserID: userID, AchievementID: achievementID, UnlockedAt: unlockedAt, CreatedAt: unlockedAt,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&expected).Error; err != nil {
		return err
	}
	var actual dbs.UserAchievementProgress
	if err := tx.Where("user_id = ? AND achievement_id = ?", userID, achievementID).Take(&actual).Error; err != nil {
		return err
	}
	if !actual.UnlockedAt.Equal(unlockedAt) {
		return fmt.Errorf("el logro existente tiene una fecha de desbloqueo distinta")
	}
	return nil
}

func ensureFullGamificationUnseen(tx *gorm.DB, userID int64, section string, createdAt time.Time) error {
	var existing []dbs.GamificationUnseen
	if err := tx.Where("user_id = ? AND section = ?", userID, section).Find(&existing).Error; err != nil {
		return err
	}
	if len(existing) > 1 {
		return fmt.Errorf("hay filas unseen duplicadas")
	}
	if len(existing) == 1 {
		if existing[0].TeamID != nil {
			return fmt.Errorf("la fila unseen existente tiene team_id inesperado")
		}
		return nil
	}
	row := dbs.GamificationUnseen{UserID: userID, Section: section, CreatedAt: createdAt}
	return tx.Create(&row).Error
}
