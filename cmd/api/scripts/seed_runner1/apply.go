package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils/gametime"
)

type applyResult struct {
	manifest *manifest
	summary  verificationSummary
}

type verificationSummary struct {
	FinishedSessions int64
	EarlySessions    int64
	DistanceMeters   float64
	MaxSingleMeters  float64
	GPSDays          int64
	AttendanceDays   int64
	StreakCurrent    int
	StreakBest       int
	Achievements     int
	Stamps           int64
}

var stampFixtures = []struct {
	SourceID   int64
	Name       string
	DaysBefore int
}{
	{SourceID: 990000001, Name: "Competencia de prueba · 5K", DaysBefore: 60},
	{SourceID: 990000002, Name: "Competencia de prueba · 10K", DaysBefore: 45},
	{SourceID: 990000003, Name: "Competencia de prueba · 21K", DaysBefore: 30},
}

func runApply(db *gorm.DB, opts options, plan fixturePlan) error {
	previous, err := loadManifest(opts.manifest)
	if err != nil {
		return err
	}
	anchor := plan.Anchor.Format("2006-01-02")
	if previous != nil {
		if err := requireManifestMode(previous, manifestModeStrict); err != nil {
			return err
		}
		if previous.Email != opts.email || previous.Anchor != anchor {
			return fmt.Errorf("el manifest %s pertenece a %s/%s; limpiá con --cleanup antes de sembrar otro ancla", opts.manifest, previous.Email, previous.Anchor)
		}
	}

	var result *applyResult
	err = db.Transaction(func(tx *gorm.DB) error {
		scope, err := inspectScope(tx, opts, plan)
		if err != nil {
			return err
		}
		if err := scope.requireNoConflicts(); err != nil {
			return err
		}
		result, err = applyFixtures(tx, scope, plan, opts)
		return err
	})
	if err != nil {
		return err
	}

	merged := mergeManifest(previous, result.manifest)
	if err := writeManifest(opts.manifest, merged); err != nil {
		return fmt.Errorf("los datos se aplicaron pero el manifest no se pudo escribir (%v); no hay rollback automático: usá --cleanup con el manifest previo o restaurá el backup", err)
	}

	fmt.Println("\nAplicado en una transacción.")
	fmt.Printf("  finished=%d · early=%d · distancia=%.0fm · max_single=%.0fm · GPS=%d · asistencias=%d\n",
		result.summary.FinishedSessions, result.summary.EarlySessions,
		result.summary.DistanceMeters, result.summary.MaxSingleMeters,
		result.summary.GPSDays, result.summary.AttendanceDays)
	fmt.Printf("  racha: current=%d · best=%d · logros=%d (bloqueados=%d) · estampas=%d\n",
		result.summary.StreakCurrent, result.summary.StreakBest,
		result.summary.Achievements, len(expectedLockedAchievements), result.summary.Stamps)
	fmt.Printf("  manifest: %s\n", opts.manifest)
	return nil
}

func applyFixtures(tx *gorm.DB, s *scope, plan fixturePlan, opts options) (*applyResult, error) {
	now := time.Now().In(gametime.Location)
	m := &manifest{
		Mode:      manifestModeStrict,
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

	for _, day := range plan.Days {
		instance, _, err := ensureSessionInstance(tx, day, now)
		if err != nil {
			return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
		}
		m.SessionInstanceIDs = append(m.SessionInstanceIDs, instance.ID)

		exercise, _, err := ensureExerciseInstance(tx, day, now)
		if err != nil {
			return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
		}
		m.ExerciseInstanceIDs = append(m.ExerciseInstanceIDs, exercise.ID)

		link, _, err := ensureSessionExerciseLink(tx, instance.ID, exercise.ID)
		if err != nil {
			return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
		}
		m.SessionExerciseInstanceIDs = append(m.SessionExerciseInstanceIDs, link.ID)

		calendarDay, _, err := ensureCalendarDay(tx, s, day, instance.ID, now)
		if err != nil {
			return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
		}
		m.CalendarDayIDs = append(m.CalendarDayIDs, calendarDay.ID)

		if day.RunnerStatus != "" {
			runner, _, err := ensureRunnerSession(tx, s.user.ID, instance.ID, day, now)
			if err != nil {
				return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			m.RunnerSessionIDs = append(m.RunnerSessionIDs, runner.ID)
		}

		if day.HasAttendance {
			attendance, _, err := ensureAttendance(tx, s, instance.ID, day, now)
			if err != nil {
				return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			m.AttendanceIDs = append(m.AttendanceIDs, attendance.ID)
		}

		if day.HasFeedback {
			feedback, _, err := ensureFeedback(tx, s, instance.ID, exercise.ID, day, now)
			if err != nil {
				return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			m.WorkoutFeedbackIDs = append(m.WorkoutFeedbackIDs, feedback.ID)

			pointIDs, err := ensurePoints(tx, feedback, instance.ID, exercise.ID, day, now)
			if err != nil {
				return nil, fmt.Errorf("día %s: %w", day.Date.Format("2006-01-02"), err)
			}
			m.WorkoutFeedbackPointIDs = append(m.WorkoutFeedbackPointIDs, pointIDs...)
		}
	}

	streakService := services.NewStreakService(daos.NewGamificationDao(tx))
	snapshot, err := streakService.ComputeSnapshot(nil, s.user.ID)
	if err != nil {
		return nil, fmt.Errorf("calcular racha después del fixture: %w", err)
	}
	if snapshot.StrictCurrent != plan.ExpectedStreakCurrent || snapshot.FlexibleCurrent != plan.ExpectedStreakCurrent {
		return nil, fmt.Errorf("racha actual inesperada: strict=%d flexible=%d (esperado %d)", snapshot.StrictCurrent, snapshot.FlexibleCurrent, plan.ExpectedStreakCurrent)
	}
	if snapshot.StrictBest != plan.ExpectedStreakBest || snapshot.FlexibleBest != plan.ExpectedStreakBest {
		return nil, fmt.Errorf("mejor racha inesperada: strict=%d flexible=%d (esperado %d)", snapshot.StrictBest, snapshot.FlexibleBest, plan.ExpectedStreakBest)
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
			return nil, fmt.Errorf("evaluar logros (%s): %w", trigger, err)
		}
	}

	afterAchievements, err := listAchievementIDs(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	m.NewAchievementIDs = diffStrings(beforeAchievements, afterAchievements)
	if err := verifyAchievementSet(tx, s.user.ID); err != nil {
		return nil, err
	}

	if _, _, err := ensureUnseen(tx, s.user.ID, constants.GamificationSectionStreaks, nil, now); err != nil {
		return nil, err
	}
	teamID := s.teamID
	if _, _, err := ensureUnseen(tx, s.user.ID, constants.GamificationSectionCompetitions, &teamID, now); err != nil {
		return nil, err
	}

	afterUnseen, err := listUnseenIDs(tx, s.user.ID)
	if err != nil {
		return nil, err
	}
	m.NewUnseenIDs = diffInt64s(beforeUnseen, afterUnseen)

	for index, spec := range stampFixtures {
		stamp, created, err := ensureStamp(tx, s.user.ID, spec.SourceID, stampSourceName(index+1, spec.Name),
			plan.Anchor.AddDate(0, 0, -spec.DaysBefore), now)
		if err != nil {
			return nil, fmt.Errorf("estampa %d: %w", spec.SourceID, err)
		}
		if created {
			m.NewStampIDs = append(m.NewStampIDs, stamp.ID)
		}
	}

	if err := upsertStreakState(tx, s.user.ID, snapshot.StrictCurrent, snapshot.FlexibleCurrent, now); err != nil {
		return nil, err
	}

	summary, err := verifyFixture(tx, s, plan)
	if err != nil {
		return nil, err
	}
	return &applyResult{manifest: m, summary: summary}, nil
}

func listAchievementIDs(tx *gorm.DB, userID int64) ([]string, error) {
	var ids []string
	if err := tx.Model(&dbs.UserAchievementProgress{}).
		Where("user_id = ?", userID).Pluck("achievement_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("leer logros existentes: %w", err)
	}
	return ids, nil
}

func listUnseenIDs(tx *gorm.DB, userID int64) ([]int64, error) {
	var ids []int64
	if err := tx.Model(&dbs.GamificationUnseen{}).
		Where("user_id = ?", userID).Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("leer unseen existentes: %w", err)
	}
	return ids, nil
}

func loadPriorStreakState(tx *gorm.DB, userID int64) (*streakStateSnapshot, error) {
	var state dbs.GamificationStreakState
	err := tx.Where("user_id = ?", userID).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("leer estado de racha previo: %w", err)
	}
	return &streakStateSnapshot{
		StrictCurrent: state.StrictCurrent, FlexibleCurrent: state.FlexibleCurrent, UpdatedAt: state.UpdatedAt,
	}, nil
}

func upsertStreakState(tx *gorm.DB, userID int64, strict, flexible int, now time.Time) error {
	state := dbs.GamificationStreakState{
		UserID: userID, StrictCurrent: strict, FlexibleCurrent: flexible, UpdatedAt: now,
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"strict_current", "flexible_current", "updated_at"}),
	}).Create(&state).Error
}

func ensureSessionInstance(tx *gorm.DB, day fixtureDay, now time.Time) (*dbs.SessionInstance, bool, error) {
	var rows []dbs.SessionInstance
	if err := tx.Where("description = ?", day.Marker).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d session_instances con el marker %s", len(rows), day.Marker)
	}
	if len(rows) == 1 {
		return &rows[0], false, nil
	}
	description := day.Marker
	row := dbs.SessionInstance{
		Name:        "Sesión runner1 " + day.Date.Format("2006-01-02"),
		Description: &description,
		CreatedAt:   now,
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensureExerciseInstance(tx *gorm.DB, day fixtureDay, now time.Time) (*dbs.ExerciseInstance, bool, error) {
	var rows []dbs.ExerciseInstance
	if err := tx.Where("description = ?", day.Marker).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d exercise_instances con el marker %s", len(rows), day.Marker)
	}
	if len(rows) == 1 {
		return &rows[0], false, nil
	}
	description := day.Marker
	kind := string(constants.ExerciseKindRitmoContinuo)
	minutes := 30
	distance := 4000
	if day.DistanceMeters > 0 {
		distance = int(day.DistanceMeters)
	}
	row := dbs.ExerciseInstance{
		Name:        "Entrenamiento runner1",
		Description: &description,
		Kind:        kind,
		Minutes:     &minutes,
		DistanceM:   &distance,
		CreatedAt:   now,
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensureSessionExerciseLink(tx *gorm.DB, sessionInstanceID, exerciseInstanceID int64) (*dbs.SessionExerciseInstance, bool, error) {
	var rows []dbs.SessionExerciseInstance
	if err := tx.Where("session_instance_id = ? AND exercise_instance_id = ?", sessionInstanceID, exerciseInstanceID).
		Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d vínculos para la instancia %d", len(rows), sessionInstanceID)
	}
	if len(rows) == 1 {
		if rows[0].Role != string(constants.SessionExerciseRoleMain) {
			return nil, false, fmt.Errorf("el vínculo existente tiene rol %q", rows[0].Role)
		}
		return &rows[0], false, nil
	}
	row := dbs.SessionExerciseInstance{
		SessionInstanceID:  sessionInstanceID,
		ExerciseInstanceID: exerciseInstanceID,
		Role:               string(constants.SessionExerciseRoleMain),
		RepeatCount:        1,
		RestMinutes:        0,
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensureCalendarDay(tx *gorm.DB, s *scope, day fixtureDay, instanceID int64, now time.Time) (*dbs.GroupCalendarDay, bool, error) {
	var rows []dbs.GroupCalendarDay
	if err := tx.Where("group_id = ? AND date = ?", s.groupID, day.Date).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d días para la fecha", len(rows))
	}
	if len(rows) == 1 {
		row := rows[0]
		if row.Kind != "training" || row.SessionInstanceID == nil || *row.SessionInstanceID != instanceID {
			return nil, false, fmt.Errorf("el día existente no es training o no apunta a la instancia fixture")
		}
		return &row, false, nil
	}
	row := dbs.GroupCalendarDay{
		GroupID: s.groupID, Date: day.Date, Kind: "training",
		SessionInstanceID: &instanceID, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensureRunnerSession(tx *gorm.DB, userID, instanceID int64, day fixtureDay, now time.Time) (*dbs.RunnerSession, bool, error) {
	var rows []dbs.RunnerSession
	if err := tx.Where("session_instance_id = ? AND athlete_user_id = ?", instanceID, userID).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d runner_session para la instancia %d", len(rows), instanceID)
	}
	if len(rows) == 1 {
		row := rows[0]
		if row.Status != day.RunnerStatus {
			return nil, false, fmt.Errorf("existe una runner_session %q y se esperaba %q; no se convierte actividad ajena", row.Status, day.RunnerStatus)
		}
		if !row.StartDate.Equal(day.Start) {
			return nil, false, fmt.Errorf("la runner_session existente tiene start_date distinto")
		}
		if day.RunnerStatus == runnersession.RunnerSessionStatusFinished {
			if row.EndDate == nil || day.End == nil || !row.EndDate.Equal(*day.End) {
				return nil, false, fmt.Errorf("la runner_session existente tiene end_date distinto")
			}
		} else if row.EndDate != nil {
			return nil, false, fmt.Errorf("una runner_session %q no debería tener end_date", row.Status)
		}
		return &row, false, nil
	}
	row := dbs.RunnerSession{
		SessionInstanceID: instanceID, AthleteUserID: userID,
		Status: day.RunnerStatus, StartDate: day.Start, EndDate: day.End,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensureAttendance(tx *gorm.DB, s *scope, instanceID int64, day fixtureDay, now time.Time) (*dbs.Attendance, bool, error) {
	return ensureAttendanceForTeam(tx, s.teamID, s.user.ID, instanceID, day, now)
}

func ensureAttendanceForTeam(tx *gorm.DB, teamID, userID, instanceID int64, day fixtureDay, now time.Time) (*dbs.Attendance, bool, error) {
	var rows []dbs.Attendance
	if err := tx.Where("team_id = ? AND training_session_id = ? AND user_id = ?", teamID, instanceID, userID).
		Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d asistencias para la instancia %d", len(rows), instanceID)
	}
	if len(rows) == 1 {
		row := rows[0]
		if row.Source != string(constants.AttendanceSourceManual) || row.RegisteredByUserID != nil {
			return nil, false, fmt.Errorf("la asistencia existente tiene procedencia distinta")
		}
		return &row, false, nil
	}
	return createAttendance(tx, teamID, userID, instanceID, day)
}

// ensureAttendanceForTeamMerge preserva cualquier asistencia preexistente sin
// validar su procedencia: en merge nunca se aborta ni se pisa data ajena.
func ensureAttendanceForTeamMerge(tx *gorm.DB, teamID, userID, instanceID int64, day fixtureDay) (*dbs.Attendance, bool, error) {
	var rows []dbs.Attendance
	if err := tx.Where("team_id = ? AND training_session_id = ? AND user_id = ?", teamID, instanceID, userID).
		Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 0 {
		return &rows[0], false, nil
	}
	return createAttendance(tx, teamID, userID, instanceID, day)
}

func createAttendance(tx *gorm.DB, teamID, userID, instanceID int64, day fixtureDay) (*dbs.Attendance, bool, error) {
	year, month, date := day.Date.Date()
	createdAt := time.Date(year, month, date, 19, 0, 0, 0, gametime.Location)
	row := dbs.Attendance{
		TeamID: teamID, TrainingSessionID: instanceID, UserID: userID,
		Source: string(constants.AttendanceSourceManual), CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensureFeedback(tx *gorm.DB, s *scope, instanceID, exerciseID int64, day fixtureDay, now time.Time) (*dbs.WorkoutFeedback, bool, error) {
	var rows []dbs.WorkoutFeedback
	if err := tx.Where(`assigned_session_id = ? AND assigned_exercise_id = ? AND athlete_user_id = ? AND feedback_owner_user_id = ? AND set_number = ? AND deleted_at IS NULL`,
		instanceID, exerciseID, s.user.ID, s.user.ID, 1).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d feedbacks activos para el set", len(rows))
	}
	if len(rows) == 1 {
		row := rows[0]
		if row.Annotations == nil || !strings.HasPrefix(*row.Annotations, day.Marker) ||
			row.DistanceMeters == nil || *row.DistanceMeters != day.DistanceMeters {
			return nil, false, fmt.Errorf("el feedback existente no coincide con el fixture")
		}
		return &row, false, nil
	}

	teamID := s.teamID
	duration := day.End.Sub(day.Start)
	durationMS := duration.Milliseconds()
	activeMS := durationMS
	distance := day.DistanceMeters
	completion := string("completed")
	annotations := day.Marker + " actividad del fixture runner1."
	rpe, avgHR, maxHR, cadence := int16(7), int16(148), int16(171), int16(172)
	elevation := 24.0
	row := dbs.WorkoutFeedback{
		TeamID: &teamID, AssignedSessionID: instanceID, AssignedExerciseID: exerciseID,
		AthleteUserID: s.user.ID, FeedbackOwnerUserID: s.user.ID,
		ReportSource: "corredor", SessionDate: day.Date, SetNumber: 1,
		StartedAt: &day.Start, EndedAt: day.End, DurationMs: &durationMS, ActiveDurationMs: &activeMS,
		DistanceMeters: &distance, RPE: &rpe, AvgHeartRate: &avgHR, MaxHeartRate: &maxHR,
		ElevationGainMeters: &elevation, Cadence: &cadence,
		CompletionStatus: &completion, Annotations: &annotations,
		CreatedAt: now, UpdatedAt: now,
	}
	row.MediaURLs.Status = pgtype.Null
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensurePoints(tx *gorm.DB, feedback *dbs.WorkoutFeedback, instanceID, exerciseID int64, day fixtureDay, now time.Time) ([]int64, error) {
	if !day.HasGPS {
		return nil, nil
	}
	points := make([]dbs.WorkoutFeedbackPoint, 0, 3)
	for index, delta := range []float64{0, 0.00012, 0.00027} {
		points = append(points, dbs.WorkoutFeedbackPoint{
			FeedbackID: feedback.ID, SessionInstanceID: instanceID, ExerciseInstanceID: exerciseID,
			Order: index, Latitude: -31.4201 + delta, Longitude: -64.1888,
			RecordedAt: day.Start.Add(time.Duration(index) * 10 * time.Second), CreatedAt: now,
		})
	}
	if _, err := daos.NewWorkoutFeedbackDao(tx).BulkCreatePointsRecompute(nil, feedback.ID, points); err != nil {
		return nil, err
	}
	var rows []dbs.WorkoutFeedbackPoint
	if err := tx.Where("feedback_id = ?", feedback.ID).Order(`"order"`).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(points) {
		return nil, fmt.Errorf("se esperaban %d puntos GPS y hay %d", len(points), len(rows))
	}
	ids := make([]int64, 0, len(rows))
	for index, row := range rows {
		want := points[index]
		if row.Order != want.Order || row.SessionInstanceID != want.SessionInstanceID ||
			row.ExerciseInstanceID != want.ExerciseInstanceID ||
			row.Latitude != want.Latitude || row.Longitude != want.Longitude ||
			!row.RecordedAt.Equal(want.RecordedAt) {
			return nil, fmt.Errorf("el punto GPS %d no coincide con el fixture", index)
		}
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func ensureUnseen(tx *gorm.DB, userID int64, section string, teamID *int64, now time.Time) (*dbs.GamificationUnseen, bool, error) {
	query := tx.Where("user_id = ? AND section = ?", userID, section)
	if teamID == nil {
		query = query.Where("team_id IS NULL")
	} else {
		query = query.Where("team_id = ?", *teamID)
	}
	var rows []dbs.GamificationUnseen
	if err := query.Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d filas unseen para %s", len(rows), section)
	}
	if len(rows) == 1 {
		return &rows[0], false, nil
	}
	row := dbs.GamificationUnseen{UserID: userID, Section: section, TeamID: teamID, CreatedAt: now}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

// ensureUnseenMerge no aborta si ya hay filas (incluso duplicadas): solo
// garantiza que exista al menos una para la sección.
func ensureUnseenMerge(tx *gorm.DB, userID int64, section string, teamID *int64, now time.Time) (*dbs.GamificationUnseen, bool, error) {
	query := tx.Where("user_id = ? AND section = ?", userID, section)
	if teamID == nil {
		query = query.Where("team_id IS NULL")
	} else {
		query = query.Where("team_id = ?", *teamID)
	}
	var rows []dbs.GamificationUnseen
	if err := query.Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 0 {
		return &rows[0], false, nil
	}
	row := dbs.GamificationUnseen{UserID: userID, Section: section, TeamID: teamID, CreatedAt: now}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func ensureStamp(tx *gorm.DB, userID, sourceID int64, sourceName string, grantedAt, now time.Time) (*dbs.GamificationStamp, bool, error) {
	var rows []dbs.GamificationStamp
	if err := tx.Where("user_id = ? AND type = ? AND source_id = ?", userID, "competition", sourceID).
		Find(&rows).Error; err != nil {
		return nil, false, err
	}
	if len(rows) > 1 {
		return nil, false, fmt.Errorf("hay %d estampas para source_id %d", len(rows), sourceID)
	}
	if len(rows) == 1 {
		row := rows[0]
		if row.SourceName == "" || !strings.HasPrefix(row.SourceName, runTag) {
			return nil, false, fmt.Errorf("la estampa existente no tiene el marker del fixture")
		}
		return &row, false, nil
	}
	row := dbs.GamificationStamp{
		UserID: userID, Type: "competition", SourceID: sourceID, SourceName: sourceName,
		GrantedAt: grantedAt, Count: 1, CreatedAt: now,
	}
	if err := tx.Create(&row).Error; err != nil {
		return nil, false, err
	}
	return &row, true, nil
}

func verifyAchievementSet(tx *gorm.DB, userID int64) error {
	ids, err := listAchievementIDs(tx, userID)
	if err != nil {
		return err
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	if len(set) != len(expectedUnlockedAchievements) {
		return fmt.Errorf("se esperaban %d logros desbloqueados y hay %d", len(expectedUnlockedAchievements), len(set))
	}
	for _, id := range expectedUnlockedAchievements {
		if _, ok := set[id]; !ok {
			return fmt.Errorf("falta el logro desbloqueado %s", id)
		}
	}
	for _, id := range expectedLockedAchievements {
		if _, ok := set[id]; ok {
			return fmt.Errorf("el logro %s debería quedar bloqueado y se desbloqueó", id)
		}
	}
	return nil
}

func verifyFixture(tx *gorm.DB, s *scope, plan fixturePlan) (verificationSummary, error) {
	achievementDao := daos.NewAchievementDao(tx)
	stats, err := achievementDao.CountFinishedSessionStats(nil, s.user.ID)
	if err != nil {
		return verificationSummary{}, err
	}
	if stats.Finished != int64(plan.ExpectedFinishedSessions) {
		return verificationSummary{}, fmt.Errorf("finished=%d, esperado %d", stats.Finished, plan.ExpectedFinishedSessions)
	}
	if stats.Early != int64(plan.ExpectedEarlySessions) {
		return verificationSummary{}, fmt.Errorf("early=%d, esperado %d", stats.Early, plan.ExpectedEarlySessions)
	}
	distances, err := achievementDao.SumAndMaxDistance(nil, s.user.ID)
	if err != nil {
		return verificationSummary{}, err
	}
	if distances.Total != plan.ExpectedDistanceMeters {
		return verificationSummary{}, fmt.Errorf("distancia=%.0fm, esperado %.0fm", distances.Total, plan.ExpectedDistanceMeters)
	}
	if distances.MaxSingle != plan.ExpectedMaxSingleMeters {
		return verificationSummary{}, fmt.Errorf("max por sesión=%.0fm, esperado %.0fm", distances.MaxSingle, plan.ExpectedMaxSingleMeters)
	}
	gps, err := achievementDao.CountGPSFinishedSessions(nil, s.user.ID)
	if err != nil {
		return verificationSummary{}, err
	}
	if gps != int64(plan.ExpectedGPSDays) {
		return verificationSummary{}, fmt.Errorf("GPS=%d, esperado %d", gps, plan.ExpectedGPSDays)
	}
	attendance, err := achievementDao.CountAttendanceDays(nil, s.user.ID)
	if err != nil {
		return verificationSummary{}, err
	}
	if attendance != int64(plan.ExpectedAttendanceDays) {
		return verificationSummary{}, fmt.Errorf("asistencias=%d, esperado %d", attendance, plan.ExpectedAttendanceDays)
	}
	snapshot, err := services.NewStreakService(daos.NewGamificationDao(tx)).ComputeSnapshot(nil, s.user.ID)
	if err != nil {
		return verificationSummary{}, err
	}
	if snapshot.StrictCurrent != plan.ExpectedStreakCurrent || snapshot.FlexibleCurrent != plan.ExpectedStreakCurrent ||
		snapshot.StrictBest != plan.ExpectedStreakBest || snapshot.FlexibleBest != plan.ExpectedStreakBest {
		return verificationSummary{}, fmt.Errorf("racha inesperada: current=%d/%d best=%d/%d", snapshot.StrictCurrent, snapshot.FlexibleCurrent, snapshot.StrictBest, snapshot.FlexibleBest)
	}
	if err := verifyAchievementSet(tx, s.user.ID); err != nil {
		return verificationSummary{}, err
	}
	for _, section := range []string{constants.GamificationSectionStreaks, constants.GamificationSectionAchievements} {
		ok, err := daos.NewGamificationDao(tx).HasUnseenSection(nil, s.user.ID, section)
		if err != nil {
			return verificationSummary{}, err
		}
		if !ok {
			return verificationSummary{}, fmt.Errorf("falta unseen de %s", section)
		}
	}
	var stamps int64
	if err := tx.Model(&dbs.GamificationStamp{}).
		Where("user_id = ? AND type = ? AND source_name LIKE ?", s.user.ID, "competition", runTag+"%").
		Count(&stamps).Error; err != nil {
		return verificationSummary{}, err
	}
	if stamps != int64(len(stampFixtures)) {
		return verificationSummary{}, fmt.Errorf("estampas=%d, esperadas %d", stamps, len(stampFixtures))
	}
	achievements, err := listAchievementIDs(tx, s.user.ID)
	if err != nil {
		return verificationSummary{}, err
	}
	return verificationSummary{
		FinishedSessions: stats.Finished, EarlySessions: stats.Early,
		DistanceMeters: distances.Total, MaxSingleMeters: distances.MaxSingle,
		GPSDays: gps, AttendanceDays: attendance,
		StreakCurrent: snapshot.StrictCurrent, StreakBest: snapshot.StrictBest,
		Achievements: len(achievements), Stamps: stamps,
	}, nil
}
