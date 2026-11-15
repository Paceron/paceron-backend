package main

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const (
	mergeMarkerPrefix = "[runner1_merge:"
	// mergeCandidateDays cubre anchor-8 .. anchor (9 fechas). En testing deja
	// libres 11-07, 11-09..11-13 y 11-15; 11-08 y 11-14 están ocupados.
	mergeCandidateDays = 9
	// mergeBreakOffset es 11-08 para anchor 11-15. Ese día es el corte
	// intencional: no se completa ninguna sesión faltante ahí.
	mergeBreakOffset = 7
)

type mergeExistingDay struct {
	GroupID      int64
	TeamID       int64
	Kind         string
	InstanceID   *int64
	RunnerStatus string
}

type mergeCompletion struct {
	GroupID    int64
	TeamID     int64
	InstanceID int64
}

type mergeDayPlan struct {
	Offset            int
	Date              time.Time
	PrimaryOccupied   bool
	PrimaryKind       string
	PrimaryInstanceID *int64
	PrimaryStatus     string
	CreateDay         bool
	Day               fixtureDay
	CompletionDay     fixtureDay
	Completions       []mergeCompletion
	SkipReason        string
}

func (d mergeDayPlan) hasActions() bool { return d.CreateDay || len(d.Completions) > 0 }

type mergePlan struct {
	Anchor  time.Time
	Days    []mergeDayPlan
	Actions []mergeDayPlan
	Skipped []mergeDayPlan
}

type mergePlanCounts struct {
	CreatedDays           int
	CompletedExistingDays int
	RunnerSessions        int
	Attendances           int
	Feedbacks             int
	GPSPoints             int
	Skipped               int
}

type mergeSummary struct {
	mergePlanCounts
	CreatedRows      int
	FinishedSessions int64
	EarlySessions    int64
	DistanceMeters   float64
	GPSDays          int64
	AttendanceDays   int64
	StreakCurrent    int
	StreakBest       int
	Achievements     int
	AchievementsNew  []string
	UnseenNew        int
}

type mergeApplyResult struct {
	manifest *manifest
	summary  mergeSummary
}

func mergeMarkerFor(date time.Time) string {
	return fmt.Sprintf("%s%s]", mergeMarkerPrefix, dateOnly(date).Format("2006-01-02"))
}

func mergeFixtureDay(anchor, date time.Time) fixtureDay {
	date = dateOnly(date)
	offset := int(dateOnly(anchor).Sub(date).Hours() / 24)
	start := time.Date(date.Year(), date.Month(), date.Day(), 6, 45, 0, 0, gametime.Location)
	end := start.Add(30 * time.Minute)
	return fixtureDay{
		Offset: offset, Date: date, Marker: mergeMarkerFor(date),
		DayStatus: "completed", RunnerStatus: runnersession.RunnerSessionStatusFinished,
		Start: start, End: &end, HasFeedback: true, DistanceMeters: 4000,
		HasGPS: true, HasAttendance: true,
	}
}

// buildMergePlan es puro: recibe los días ya existentes de todos los grupos
// activos y decide qué piezas faltan sin tocar nada preexistente.
func buildMergePlan(anchor time.Time, primaryGroupID int64, existing map[string][]mergeExistingDay) mergePlan {
	anchor = dateOnly(anchor)
	breakDate := anchor.AddDate(0, 0, -mergeBreakOffset).Format("2006-01-02")
	plan := mergePlan{Anchor: anchor}
	for offset := mergeCandidateDays - 1; offset >= 0; offset-- {
		date := anchor.AddDate(0, 0, -offset)
		dateStr := date.Format("2006-01-02")
		item := mergeDayPlan{Offset: offset, Date: date, CompletionDay: mergeFixtureDay(anchor, date)}

		days := existing[dateStr]
		var primary *mergeExistingDay
		for i := range days {
			if days[i].GroupID == primaryGroupID {
				primary = &days[i]
				break
			}
		}
		if primary != nil {
			item.PrimaryOccupied = true
			item.PrimaryKind = primary.Kind
			item.PrimaryInstanceID = primary.InstanceID
			item.PrimaryStatus = primary.RunnerStatus
		}

		// El break sólo aplica si el día ya está ocupado en el grupo primario.
		// Si está libre (base limpia), se puede crear como cualquier otro día.
		if dateStr == breakDate && primary != nil {
			item.SkipReason = "break intencional (11-08): día ocupado, no se completa"
		} else {
			if primary == nil {
				item.CreateDay = true
				item.Day = mergeFixtureDay(anchor, date)
			}
			for _, day := range days {
				if day.Kind != "training" || day.InstanceID == nil || day.RunnerStatus != "" {
					continue
				}
				item.Completions = append(item.Completions, mergeCompletion{
					GroupID: day.GroupID, TeamID: day.TeamID, InstanceID: *day.InstanceID,
				})
			}
			if !item.hasActions() && item.SkipReason == "" {
				switch {
				case primary != nil && primary.Kind != "training":
					item.SkipReason = "ocupado kind=" + primary.Kind
				case primary != nil && primary.InstanceID == nil:
					item.SkipReason = "training sin instancia"
				case primary != nil && primary.RunnerStatus != "":
					item.SkipReason = "runner_session " + primary.RunnerStatus + ": no se toca"
				default:
					item.SkipReason = "sin piezas faltantes"
				}
			}
		}

		plan.Days = append(plan.Days, item)
		if item.hasActions() {
			plan.Actions = append(plan.Actions, item)
		} else {
			plan.Skipped = append(plan.Skipped, item)
		}
	}
	return plan
}

func countMergePlan(plan mergePlan) mergePlanCounts {
	counts := mergePlanCounts{Skipped: len(plan.Skipped)}
	for _, item := range plan.Actions {
		if item.CreateDay {
			counts.CreatedDays++
			counts.RunnerSessions++
			counts.Attendances++
			counts.Feedbacks++
			counts.GPSPoints += 3
		}
		if len(item.Completions) > 0 {
			counts.CompletedExistingDays++
			counts.RunnerSessions += len(item.Completions)
			counts.Attendances += len(item.Completions)
		}
	}
	return counts
}

func loadMergeExisting(db *gorm.DB, s *scope, anchor time.Time) (map[string][]mergeExistingDay, error) {
	anchor = dateOnly(anchor)
	first := anchor.AddDate(0, 0, -(mergeCandidateDays - 1))
	teamByGroup := make(map[int64]int64, len(s.memberships))
	groupIDs := make([]int64, 0, len(s.memberships))
	for _, membership := range s.memberships {
		teamByGroup[membership.Group.ID] = membership.Team.ID
		groupIDs = append(groupIDs, membership.Group.ID)
	}
	var days []dbs.GroupCalendarDay
	if err := db.Where("group_id IN ? AND date >= ? AND date <= ?", groupIDs, first, anchor).Find(&days).Error; err != nil {
		return nil, fmt.Errorf("leer días existentes de los grupos activos: %w", err)
	}

	instanceIDs := make([]int64, 0, len(days))
	for _, day := range days {
		if day.SessionInstanceID != nil {
			instanceIDs = append(instanceIDs, *day.SessionInstanceID)
		}
	}
	statusByInstance := make(map[int64]string, len(instanceIDs))
	if len(instanceIDs) > 0 {
		var sessions []dbs.RunnerSession
		if err := db.Where("athlete_user_id = ? AND session_instance_id IN ?", s.user.ID, instanceIDs).Find(&sessions).Error; err != nil {
			return nil, fmt.Errorf("leer runner sessions existentes: %w", err)
		}
		for _, session := range sessions {
			statusByInstance[session.SessionInstanceID] = session.Status
		}
	}

	existing := make(map[string][]mergeExistingDay, len(days))
	for _, day := range days {
		status := ""
		if day.SessionInstanceID != nil {
			status = statusByInstance[*day.SessionInstanceID]
		}
		existing[utcDateInCordoba(day.Date).Format("2006-01-02")] = append(existing[utcDateInCordoba(day.Date).Format("2006-01-02")], mergeExistingDay{
			GroupID: day.GroupID, TeamID: teamByGroup[day.GroupID], Kind: day.Kind,
			InstanceID: day.SessionInstanceID, RunnerStatus: status,
		})
	}
	return existing, nil
}

func runMergeDryRun(db *gorm.DB, opts options, anchor time.Time) error {
	scope, err := resolveScope(db, opts)
	if err != nil {
		return err
	}
	existing, err := loadMergeExisting(db, scope, anchor)
	if err != nil {
		return err
	}
	plan := buildMergePlan(anchor, scope.groupID, existing)
	counts := countMergePlan(plan)
	summary, err := existingMergeSummary(db, scope.user.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Merge (top-up) para %s (user_id=%d) · grupo %d · equipo %d · ancla %s\n",
		scope.user.Email, scope.user.ID, scope.groupID, scope.teamID, plan.Anchor.Format("2006-01-02"))
	fmt.Printf("  existente: finished=%d · early=%d · distancia=%.0fm · GPS=%d · asistencias=%d · logros=%d · racha=%d/%d\n",
		summary.FinishedSessions, summary.EarlySessions, summary.DistanceMeters,
		summary.GPSDays, summary.AttendanceDays, summary.Achievements,
		summary.StreakCurrent, summary.StreakBest)
	fmt.Println("  plan (anchor-8 .. anchor):")
	for _, item := range plan.Days {
		date := item.Date.Format("2006-01-02")
		if !item.hasActions() {
			fmt.Printf("    %s  SKIP (%s)\n", date, item.SkipReason)
			continue
		}
		if item.CreateDay {
			fmt.Printf("    %s  ADD día completo (session_instance/calendar/runner_session/attendance/feedback 4000m/GPS 3)\n", date)
		}
		for _, completion := range item.Completions {
			fmt.Printf("    %s  ADD runner_session/attendance sobre instancia existente group=%d team=%d instance=%d\n",
				date, completion.GroupID, completion.TeamID, completion.InstanceID)
		}
	}
	fmt.Printf("  resumen: días nuevos=%d · días existentes completados=%d · runner_sessions=%d · asistencias=%d · feedback=%d · puntos GPS=%d · salteados=%d\n",
		counts.CreatedDays, counts.CompletedExistingDays, counts.RunnerSessions,
		counts.Attendances, counts.Feedbacks, counts.GPSPoints, counts.Skipped)
	if previous, err := loadManifest(opts.manifest); err != nil {
		return err
	} else if previous != nil {
		if previous.Mode != manifestModeMerge {
			fmt.Printf("  aviso: el manifest %s es modo %q; --merge --apply abortará\n", opts.manifest, previous.Mode)
		} else {
			fmt.Printf("  manifest previo merge: %s (se reutiliza para idempotencia)\n", opts.manifest)
		}
	}
	fmt.Println("  nota: la API de rachas calcula hasta ayer; 11-15 queda pending hasta mañana.")
	fmt.Println("\n[merge dry-run] no se escribió nada. Pasá --merge --apply para aplicar en una transacción.")
	return nil
}

func runMergeApply(db *gorm.DB, opts options, anchor time.Time) error {
	previous, err := loadManifest(opts.manifest)
	if err != nil {
		return err
	}
	anchorStr := dateOnly(anchor).Format("2006-01-02")
	if previous != nil {
		if err := requireManifestMode(previous, manifestModeMerge); err != nil {
			return err
		}
		if previous.Email != opts.email || previous.Anchor != anchorStr {
			return fmt.Errorf("el manifest merge %s pertenece a %s/%s; limpiá con --merge --cleanup o usá otro --manifest", opts.manifest, previous.Email, previous.Anchor)
		}
	}

	var result *mergeApplyResult
	err = db.Transaction(func(tx *gorm.DB) error {
		scope, err := resolveScope(tx, opts)
		if err != nil {
			return err
		}
		existing, err := loadMergeExisting(tx, scope, anchor)
		if err != nil {
			return err
		}
		plan := buildMergePlan(anchor, scope.groupID, existing)
		result, err = applyMergeFixtures(tx, scope, plan, opts)
		return err
	})
	if err != nil {
		return err
	}

	merged := mergeManifest(previous, result.manifest)
	if err := writeManifest(opts.manifest, merged); err != nil {
		return fmt.Errorf("los datos se aplicaron pero el manifest merge no se pudo escribir (%v); no hay rollback automático", err)
	}

	fmt.Println("\nMerge aplicado en una transacción.")
	fmt.Printf("  días nuevos=%d · días existentes completados=%d · filas creadas≈%d\n",
		result.summary.CreatedDays, result.summary.CompletedExistingDays, result.summary.CreatedRows)
	fmt.Printf("  runner_sessions=%d · asistencias=%d · feedback=%d · puntos GPS=%d · salteados=%d\n",
		result.summary.RunnerSessions, result.summary.Attendances,
		result.summary.Feedbacks, result.summary.GPSPoints, result.summary.Skipped)
	fmt.Printf("  actual: finished=%d · early=%d · distancia=%.0fm · GPS=%d · asistencias=%d\n",
		result.summary.FinishedSessions, result.summary.EarlySessions,
		result.summary.DistanceMeters, result.summary.GPSDays, result.summary.AttendanceDays)
	fmt.Printf("  racha actual=%d · best=%d · logros=%d (nuevos=%d) · unseen nuevos=%d\n",
		result.summary.StreakCurrent, result.summary.StreakBest,
		result.summary.Achievements, len(result.summary.AchievementsNew), result.summary.UnseenNew)
	if len(result.summary.AchievementsNew) > 0 {
		fmt.Printf("  logros nuevos: %v\n", result.summary.AchievementsNew)
	}
	fmt.Printf("  manifest: %s\n", opts.manifest)
	return nil
}

func applyMergeFixtures(tx *gorm.DB, s *scope, plan mergePlan, opts options) (*mergeApplyResult, error) {
	now := time.Now().In(gametime.Location)
	m := &manifest{
		Mode:      manifestModeMerge,
		Email:     opts.email,
		UserID:    s.user.ID,
		Anchor:    plan.Anchor.Format("2006-01-02"),
		CreatedAt: now.UTC(),
	}

	beforeAchievements, err := listAchievementIDs(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	beforeUnseen, err := listUnseenIDs(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	priorState, err := loadPriorStreakState(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	m.PriorStreakState = priorState

	createdRows := 0
	for _, item := range plan.Actions {
		day := item.Day
		if item.CreateDay {
			instance, created, err := ensureSessionInstance(tx, day, now)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			if created {
				createdRows++
			}
			m.SessionInstanceIDs = append(m.SessionInstanceIDs, instance.ID)

			exercise, created, err := ensureExerciseInstance(tx, day, now)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			if created {
				createdRows++
			}
			m.ExerciseInstanceIDs = append(m.ExerciseInstanceIDs, exercise.ID)

			link, created, err := ensureSessionExerciseLink(tx, instance.ID, exercise.ID)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			if created {
				createdRows++
			}
			m.SessionExerciseInstanceIDs = append(m.SessionExerciseInstanceIDs, link.ID)

			calendarDay, created, err := ensureCalendarDay(tx, s, day, instance.ID, now)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			if created {
				createdRows++
			}
			m.CalendarDayIDs = append(m.CalendarDayIDs, calendarDay.ID)

			runner, created, err := ensureRunnerSession(tx, s.user.ID, instance.ID, day, now)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			if created {
				createdRows++
			}
			m.RunnerSessionIDs = append(m.RunnerSessionIDs, runner.ID)

			attendance, created, err := ensureAttendanceForTeamMerge(tx, s.teamID, s.user.ID, instance.ID, day)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			if created {
				createdRows++
			}
			m.AttendanceIDs = append(m.AttendanceIDs, attendance.ID)

			feedback, created, err := ensureFeedback(tx, s, instance.ID, exercise.ID, day, now)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			if created {
				createdRows++
			}
			m.WorkoutFeedbackIDs = append(m.WorkoutFeedbackIDs, feedback.ID)

			pointIDs, err := ensurePoints(tx, feedback, instance.ID, exercise.ID, day, now)
			if err != nil {
				return nil, fmt.Errorf("merge día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			createdRows += len(pointIDs)
			m.WorkoutFeedbackPointIDs = append(m.WorkoutFeedbackPointIDs, pointIDs...)
		}

		completionDay := item.CompletionDay
		for _, completion := range item.Completions {
			runner, created, err := ensureRunnerSession(tx, s.user.ID, completion.InstanceID, completionDay, now)
			if err != nil {
				return nil, fmt.Errorf("merge día %s grupo %d: %w", item.Date.Format("2006-01-02"), completion.GroupID, err)
			}
			if created {
				createdRows++
			}
			m.RunnerSessionIDs = append(m.RunnerSessionIDs, runner.ID)

			attendance, created, err := ensureAttendanceForTeamMerge(tx, completion.TeamID, s.user.ID, completion.InstanceID, completionDay)
			if err != nil {
				return nil, fmt.Errorf("merge día %s grupo %d: %w", item.Date.Format("2006-01-02"), completion.GroupID, err)
			}
			if created {
				createdRows++
			}
			m.AttendanceIDs = append(m.AttendanceIDs, attendance.ID)
		}
	}

	streakService := services.NewStreakService(daos.NewGamificationDao(tx))
	snapshot, err := streakService.ComputeSnapshot(nil, s.user.ID)
	if err != nil {
		return nil, fmt.Errorf("recalcular racha después del merge: %w", err)
	}

	achievements := services.NewAchievementsService(
		daos.NewAchievementDao(tx), daos.NewGamificationDao(tx), streakService, nil, nil,
	)
	for _, trigger := range []services.AchievementTrigger{
		services.AchievementTriggerRunnerSession,
		services.AchievementTriggerFeedback,
		services.AchievementTriggerGPSPoints,
		services.AchievementTriggerAttendance,
	} {
		if err := achievements.EvaluateEvent(nil, s.user.ID, trigger); err != nil {
			return nil, fmt.Errorf("merge: evaluar logros (%s): %w", trigger, err)
		}
	}

	afterAchievements, err := listAchievementIDs(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	m.NewAchievementIDs = diffStrings(beforeAchievements, afterAchievements)

	if _, _, err := ensureUnseenMerge(tx, s.user.ID, constants.GamificationSectionStreaks, nil, now); err != nil {
		return nil, err
	}
	afterUnseen, err := listUnseenIDs(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	m.NewUnseenIDs = diffInt64s(beforeUnseen, afterUnseen)

	if err := upsertStreakState(tx, s.user.ID, snapshot.StrictCurrent, snapshot.FlexibleCurrent, now); err != nil {
		return nil, err
	}

	summary, err := mergeActualSummary(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	summary.mergePlanCounts = countMergePlan(plan)
	summary.CreatedRows = createdRows
	summary.AchievementsNew = m.NewAchievementIDs
	summary.UnseenNew = len(m.NewUnseenIDs)
	summary.StreakCurrent = snapshot.StrictCurrent
	summary.StreakBest = snapshot.StrictBest
	return &mergeApplyResult{manifest: m, summary: summary}, nil
}

func existingMergeSummary(db *gorm.DB, userID int64) (mergeSummary, error) {
	summary, err := mergeActualSummary(db, userID)
	if err != nil {
		return mergeSummary{}, err
	}
	snapshot, err := services.NewStreakService(daos.NewGamificationDao(db)).ComputeSnapshot(nil, userID)
	if err != nil {
		return mergeSummary{}, err
	}
	summary.StreakCurrent = snapshot.StrictCurrent
	summary.StreakBest = snapshot.StrictBest
	return summary, nil
}

func mergeActualSummary(db *gorm.DB, userID int64) (mergeSummary, error) {
	achievementDao := daos.NewAchievementDao(db)
	stats, err := achievementDao.CountFinishedSessionStats(nil, userID)
	if err != nil {
		return mergeSummary{}, err
	}
	distances, err := achievementDao.SumAndMaxDistance(nil, userID)
	if err != nil {
		return mergeSummary{}, err
	}
	gps, err := achievementDao.CountGPSFinishedSessions(nil, userID)
	if err != nil {
		return mergeSummary{}, err
	}
	attendance, err := achievementDao.CountAttendanceDays(nil, userID)
	if err != nil {
		return mergeSummary{}, err
	}
	achievements, err := listAchievementIDs(db, userID)
	if err != nil {
		return mergeSummary{}, err
	}
	return mergeSummary{
		FinishedSessions: stats.Finished, EarlySessions: stats.Early,
		DistanceMeters: distances.Total, GPSDays: gps, AttendanceDays: attendance,
		Achievements: len(achievements),
	}, nil
}
