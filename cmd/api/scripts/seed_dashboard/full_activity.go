package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgtype"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const fullActivitySecondaryDayKey = "activity_a_secondary_minus7"

type fullActivityRunnerSpec struct {
	key, sessionKey, athleteKey, status string
	start                               time.Time
	end                                 *time.Time
}

type fullActivityFeedbackSpec struct {
	key, sessionKey, exerciseKey, athleteKey, ownerKey, status, reportSource string
	date                                                                     time.Time
	setNumber                                                                int
	startedAt, endedAt                                                       *time.Time
	durationMs, activeDurationMs                                             *int64
	distanceMeters, elevationGainMeters                                      *float64
	rpe, avgHeartRate, maxHeartRate, cadence                                 *int16
	deletedAt                                                                *time.Time
	withPoints                                                               bool
}

type fullActivityAttendanceSpec struct {
	key, sessionKey, groupKey, userKey, source string
	registeredByKey                            string
}

func (s *fullSeeder) seedActivity() error {
	if err := s.requireFullActivityPrerequisites(); err != nil {
		return err
	}

	runnerIDs := make(map[string]int64, 4)
	feedbackIDs := make(map[string]int64, 5)
	attendanceIDs := make(map[string]int64, 6)
	calendarIDs := make(map[string]int64, 1)
	sessionInstanceIDs := make(map[string]int64, 1)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		dayID, sessionInstanceID, err := s.ensureFullActivitySecondaryDay(tx)
		if err != nil {
			return err
		}
		calendarIDs[fullActivitySecondaryDayKey] = dayID
		sessionInstanceIDs[fullActivitySecondaryDayKey] = sessionInstanceID

		for _, spec := range s.fullActivityRunnerSpecs() {
			instanceID := sessionInstanceIDForActivity(spec.sessionKey, s.ids.sessionInstances, sessionInstanceIDs)
			row, err := ensureFullActivityRunner(tx, dbs.RunnerSession{
				SessionInstanceID: instanceID,
				AthleteUserID:     s.ids.users[spec.athleteKey],
				Status:            spec.status,
				StartDate:         spec.start,
				EndDate:           spec.end,
				CreatedAt:         s.now,
				UpdatedAt:         s.now,
			})
			if err != nil {
				return fmt.Errorf("runner_session %s: %w", spec.key, err)
			}
			runnerIDs[spec.key] = row.ID
		}

		for _, spec := range s.fullActivityFeedbackSpecs() {
			sessionID := sessionInstanceIDForActivity(spec.sessionKey, s.ids.sessionInstances, sessionInstanceIDs)
			exerciseID, err := fullActivityExerciseInstanceID(tx, sessionID, s.ids.exercises[spec.exerciseKey])
			if err != nil {
				return fmt.Errorf("resolver ejercicio %s para %s: %w", spec.exerciseKey, spec.key, err)
			}
			teamID := s.ids.teams["team_a"]
			feedback := dbs.WorkoutFeedback{
				TeamID: &teamID, AssignedSessionID: sessionID, AssignedExerciseID: exerciseID,
				AthleteUserID: s.ids.users[spec.athleteKey], FeedbackOwnerUserID: s.ids.users[spec.ownerKey],
				ReportSource: spec.reportSource, SessionDate: fullDate(spec.date), SetNumber: spec.setNumber,
				StartedAt: spec.startedAt, EndedAt: spec.endedAt, DurationMs: spec.durationMs,
				ActiveDurationMs: spec.activeDurationMs, DistanceMeters: spec.distanceMeters,
				RPE: spec.rpe, AvgHeartRate: spec.avgHeartRate, MaxHeartRate: spec.maxHeartRate,
				ElevationGainMeters: spec.elevationGainMeters, Cadence: spec.cadence,
				CompletionStatus: stringPointer(spec.status),
				Annotations:      stringPointer(fullSeedMarker + " Actividad del perfil full."),
				DeletedAt:        spec.deletedAt,
			}
			feedback.MediaURLs.Status = pgtype.Null
			row, err := ensureFullActivityFeedback(tx, feedback)
			if err != nil {
				return fmt.Errorf("workout_feedback %s: %w", spec.key, err)
			}
			feedbackIDs[spec.key] = row.ID
			if err := ensureFullActivityPoints(tx, row.ID, sessionID, exerciseID, spec, s.now); err != nil {
				return fmt.Errorf("puntos GPS %s: %w", spec.key, err)
			}
		}

		for _, spec := range s.fullActivityAttendanceSpecs() {
			teamID := s.ids.teams["team_a"]
			instanceID := sessionInstanceIDForActivity(spec.sessionKey, s.ids.sessionInstances, sessionInstanceIDs)
			userID := s.ids.users[spec.userKey]
			if err := validateFullActivityAttendanceContext(tx, s.ids.groups[spec.groupKey], teamID, instanceID, userID, spec.sessionKey, s.now); err != nil {
				return fmt.Errorf("attendance %s: %w", spec.key, err)
			}
			var actorID *int64
			if spec.registeredByKey != "" {
				actorID = int64Pointer(s.ids.users[spec.registeredByKey])
			}
			row, err := ensureFullActivityAttendance(tx, dbs.Attendance{
				TeamID: teamID, TrainingSessionID: instanceID, UserID: userID,
				Source: spec.source, RegisteredByUserID: actorID, CreatedAt: s.now, UpdatedAt: s.now,
			})
			if err != nil {
				return fmt.Errorf("attendance %s: %w", spec.key, err)
			}
			attendanceIDs[spec.key] = row.ID
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sembrar actividad del perfil full: %w", err)
	}

	mergeFullActivityIDs(s.ids.runnerSessions, runnerIDs)
	mergeFullActivityIDs(s.ids.workoutFeedback, feedbackIDs)
	mergeFullActivityIDs(s.ids.attendances, attendanceIDs)
	mergeFullActivityIDs(s.ids.calendarDays, calendarIDs)
	mergeFullActivityIDs(s.ids.sessionInstances, sessionInstanceIDs)
	return nil
}

func (s *fullSeeder) requireFullActivityPrerequisites() error {
	for _, key := range []string{"trainer_1", "trainer_2", "r1", "r2", "r3", "r4", "r5"} {
		if s.ids.users[key] <= 0 {
			return fmt.Errorf("seedActivity requiere seedIdentity: falta %s", key)
		}
	}
	for _, key := range []string{"team_a"} {
		if s.ids.teams[key] <= 0 {
			return fmt.Errorf("seedActivity requiere seedTeams: falta %s", key)
		}
	}
	for _, key := range []string{"group_a_main", "group_a_secondary"} {
		if s.ids.groups[key] <= 0 {
			return fmt.Errorf("seedActivity requiere seedTeams: falta %s", key)
		}
	}
	for _, key := range []string{"full_speed_distance", "full_endurance"} {
		if s.ids.exercises[key] <= 0 {
			return fmt.Errorf("seedActivity requiere seedCatalogCalendar: falta ejercicio %s", key)
		}
		var exercise dbs.Exercise
		if err := s.db.First(&exercise, s.ids.exercises[key]).Error; err != nil {
			return fmt.Errorf("seedActivity requiere seedCatalogCalendar: ejercicio %s no existe: %w", key, err)
		}
		if exercise.OwnerID != s.ids.users["trainer_1"] || !containsFullSeedMarker(pointerValue(exercise.Description)) {
			return fmt.Errorf("seedActivity rechaza ejercicio %s ajeno al perfil full", key)
		}
	}
	minus14Key := fullActivityStampedDayKey(s.now.AddDate(0, 0, -14))
	for _, key := range []string{minus14Key, "today_a_main"} {
		if s.ids.calendarDays[key] <= 0 || s.ids.sessionInstances[key] <= 0 {
			return fmt.Errorf("seedActivity requiere seedCatalogCalendar: faltan calendario/instancia %s", key)
		}
	}

	var team dbs.Team
	if err := s.db.First(&team, s.ids.teams["team_a"]).Error; err != nil {
		return fmt.Errorf("seedActivity requiere seedTeams: equipo team_a no existe: %w", err)
	}
	if team.OwnerID != s.ids.users["trainer_1"] || !containsFullSeedMarker(team.Description) {
		return fmt.Errorf("seedActivity rechaza equipo team_a ajeno al perfil full")
	}
	for _, key := range []string{"group_a_main", "group_a_secondary"} {
		var group dbs.Group
		if err := s.db.First(&group, s.ids.groups[key]).Error; err != nil {
			return fmt.Errorf("seedActivity requiere seedTeams: grupo %s no existe: %w", key, err)
		}
		if group.TeamID != s.ids.teams["team_a"] || !containsFullSeedMarker(group.Description) {
			return fmt.Errorf("seedActivity rechaza grupo %s ajeno al perfil full", key)
		}
	}
	for _, key := range []string{"trainer_1", "trainer_2", "r1", "r2", "r3", "r4", "r5"} {
		var user dbs.User
		if err := s.db.First(&user, s.ids.users[key]).Error; err != nil {
			return fmt.Errorf("seedActivity requiere seedIdentity: usuario %s no existe: %w", key, err)
		}
		if user.Email == "" || !stringsHasFullSeedEmail(user.Email) {
			return fmt.Errorf("seedActivity rechaza usuario %s ajeno al perfil full", key)
		}
	}
	for _, key := range []string{minus14Key, "today_a_main"} {
		var day dbs.GroupCalendarDay
		if err := s.db.First(&day, s.ids.calendarDays[key]).Error; err != nil {
			return fmt.Errorf("seedActivity requiere seedCatalogCalendar: día %s no existe: %w", key, err)
		}
		if day.SessionInstanceID == nil || *day.SessionInstanceID != s.ids.sessionInstances[key] {
			return fmt.Errorf("seedActivity rechaza día %s sin su instancia full", key)
		}
		expectedGroup := s.ids.groups["group_a_main"]
		if day.GroupID != expectedGroup || day.Kind != "training" {
			return fmt.Errorf("seedActivity rechaza día %s fuera del grupo principal full", key)
		}
		wantDate := fullDate(s.now)
		if key == minus14Key {
			wantDate = fullDate(s.now.AddDate(0, 0, -14))
		}
		if day.Date.Format("2006-01-02") != wantDate.Format("2006-01-02") {
			return fmt.Errorf("seedActivity rechaza día %s con fecha inesperada", key)
		}
		var instance dbs.SessionInstance
		if err := s.db.First(&instance, s.ids.sessionInstances[key]).Error; err != nil {
			return fmt.Errorf("seedActivity requiere seedCatalogCalendar: instancia %s no existe: %w", key, err)
		}
		if instance.SourceSessionID == nil || !containsFullSeedMarker(pointerValue(instance.Description)) {
			return fmt.Errorf("seedActivity rechaza instancia %s ajena al catálogo full", key)
		}
		var source dbs.Session
		if err := s.db.First(&source, *instance.SourceSessionID).Error; err != nil {
			return fmt.Errorf("seedActivity requiere seedCatalogCalendar: sesión de %s no existe: %w", key, err)
		}
		if source.OwnerID != s.ids.users["trainer_1"] || !containsFullSeedMarker(pointerValue(source.Description)) {
			return fmt.Errorf("seedActivity rechaza el catálogo de la instancia %s", key)
		}
	}
	return nil
}

func (s *fullSeeder) fullActivityRunnerSpecs() []fullActivityRunnerSpec {
	minus14 := fullDate(s.now.AddDate(0, 0, -14))
	minus7 := fullDate(s.now.AddDate(0, 0, -7))
	today := fullDate(s.now)
	return []fullActivityRunnerSpec{
		{key: "finished_r1_minus14", sessionKey: fullActivityStampedDayKey(minus14), athleteKey: "r1", status: runnersession.RunnerSessionStatusFinished, start: fullActivityClock(minus14, 8, 0), end: timePointer(fullActivityClock(minus14, 9, 0))},
		{key: "finished_r4_minus7", sessionKey: fullActivitySecondaryDayKey, athleteKey: "r4", status: runnersession.RunnerSessionStatusFinished, start: fullActivityClock(minus7, 8, 0), end: timePointer(fullActivityClock(minus7, 9, 0))},
		{key: "interrupted_r3_minus14", sessionKey: fullActivityStampedDayKey(minus14), athleteKey: "r3", status: runnersession.RunnerSessionStatusInterrupted, start: fullActivityClock(minus14, 8, 15), end: timePointer(fullActivityClock(minus14, 8, 25))},
		{key: "wip_r1_today", sessionKey: "today_a_main", athleteKey: "r1", status: runnersession.RunnerSessionStatusWip, start: fullActivityClock(today, 8, 0)},
	}
}

func (s *fullSeeder) fullActivityFeedbackSpecs() []fullActivityFeedbackSpec {
	minus14 := fullDate(s.now.AddDate(0, 0, -14))
	minus7 := fullDate(s.now.AddDate(0, 0, -7))
	startMinus14 := fullActivityClock(minus14, 8, 0)
	startMinus7 := fullActivityClock(minus7, 8, 0)
	completedOne := fullActivityCompletedFeedback("completed_r1_minus14_set1", fullActivityStampedDayKey(minus14), "full_speed_distance", "r1", "r1", comparison.SetStatusCompleted, minus14, startMinus14, 1, 15*time.Minute, 3000, true, false)
	skipped := fullActivityFeedbackSpec{
		key: "skipped_r1_minus14_set2", sessionKey: fullActivityStampedDayKey(minus14), exerciseKey: "full_speed_distance",
		athleteKey: "r1", ownerKey: "r1", status: comparison.SetStatusSkipped, reportSource: "corredor",
		date: minus14, setNumber: 2,
	}
	unregistered := fullActivityFeedbackSpec{
		key: "unregistered_r4_minus7_set1", sessionKey: fullActivitySecondaryDayKey, exerciseKey: "full_endurance",
		athleteKey: "r4", ownerKey: "trainer_1", status: comparison.SetStatusUnregistered, reportSource: "entrenador",
		date: minus7, setNumber: 1,
	}
	completedTwo := fullActivityCompletedFeedback("completed_r4_minus7_set2", fullActivitySecondaryDayKey, "full_endurance", "r4", "r4", comparison.SetStatusCompleted, minus7, startMinus7, 2, 20*time.Minute, 4000, true, false)
	deleted := fullActivityCompletedFeedback("deleted_r3_minus14_set3", fullActivityStampedDayKey(minus14), "full_speed_distance", "r3", "r3", comparison.SetStatusCompleted, minus14, fullActivityClock(minus14, 8, 30), 3, 12*time.Minute, 2400, false, true)
	return []fullActivityFeedbackSpec{completedOne, skipped, unregistered, completedTwo, deleted}
}

func (s *fullSeeder) fullActivityAttendanceSpecs() []fullActivityAttendanceSpec {
	return []fullActivityAttendanceSpec{
		{key: "r1_minus14_qr", sessionKey: fullActivityStampedDayKey(s.now.AddDate(0, 0, -14)), groupKey: "group_a_main", userKey: "r1", source: string(constants.AttendanceSourceQR)},
		{key: "r2_minus14_manual", sessionKey: fullActivityStampedDayKey(s.now.AddDate(0, 0, -14)), groupKey: "group_a_main", userKey: "r2", source: string(constants.AttendanceSourceManual), registeredByKey: "trainer_1"},
		{key: "r4_minus7_qr", sessionKey: fullActivitySecondaryDayKey, groupKey: "group_a_secondary", userKey: "r4", source: string(constants.AttendanceSourceQR)},
		{key: "r5_minus7_manual", sessionKey: fullActivitySecondaryDayKey, groupKey: "group_a_secondary", userKey: "r5", source: string(constants.AttendanceSourceManual), registeredByKey: "trainer_1"},
		{key: "r1_today_qr", sessionKey: "today_a_main", groupKey: "group_a_main", userKey: "r1", source: string(constants.AttendanceSourceQR)},
		{key: "r2_today_manual", sessionKey: "today_a_main", groupKey: "group_a_main", userKey: "r2", source: string(constants.AttendanceSourceManual), registeredByKey: "trainer_1"},
	}
}

func (s *fullSeeder) ensureFullActivitySecondaryDay(tx *gorm.DB) (int64, int64, error) {
	date := fullDate(s.now.AddDate(0, 0, -7))
	groupID := s.ids.groups["group_a_secondary"]
	sourceSessionID := s.ids.sessions["full_interval"]
	var sourceSession dbs.Session
	if sourceSessionID <= 0 {
		return 0, 0, fmt.Errorf("seedActivity requiere la sesión full_interval marcada")
	}
	if err := tx.First(&sourceSession, sourceSessionID).Error; err != nil {
		return 0, 0, fmt.Errorf("leer sesión full_interval: %w", err)
	}
	if sourceSession.OwnerID != s.ids.users["trainer_1"] || !containsFullSeedMarker(pointerValue(sourceSession.Description)) {
		return 0, 0, fmt.Errorf("seedActivity requiere la sesión full_interval marcada")
	}
	var existing dbs.GroupCalendarDay
	err := tx.Where("group_id = ? AND date = ?", groupID, date).Take(&existing).Error
	if err == nil {
		if existing.Kind != "training" || !existing.IsPresencial || existing.SessionInstanceID == nil {
			return 0, 0, fmt.Errorf("la fecha -7 del grupo secundario contiene actividad ajena")
		}
		var instance dbs.SessionInstance
		if err := tx.First(&instance, *existing.SessionInstanceID).Error; err != nil {
			return 0, 0, err
		}
		if instance.SourceSessionID == nil || *instance.SourceSessionID != sourceSessionID || !containsFullSeedMarker(pointerValue(instance.Description)) {
			return 0, 0, fmt.Errorf("la fecha -7 del grupo secundario apunta a una instancia ajena")
		}
		return existing.ID, instance.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, fmt.Errorf("buscar fecha -7 del grupo secundario: %w", err)
	}
	instance, err := instantiateFullSession(tx, sourceSessionID, s.ids.users["trainer_1"], s.now)
	if err != nil {
		return 0, 0, err
	}
	from := time.Date(0, time.January, 1, 8, 0, 0, 0, time.UTC)
	to := time.Date(0, time.January, 1, 9, 30, 0, 0, time.UTC)
	day := dbs.GroupCalendarDay{
		GroupID: groupID, Date: date, Kind: "training", SessionInstanceID: &instance.ID,
		IsPresencial: true, PresencialTimeFrom: &from, PresencialTimeTo: &to,
	}
	if err := tx.Create(&day).Error; err != nil {
		return 0, 0, fmt.Errorf("crear día presencial full de -7: %w", err)
	}
	return day.ID, instance.ID, nil
}

func ensureFullActivityRunner(tx *gorm.DB, expected dbs.RunnerSession) (*dbs.RunnerSession, error) {
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
		return nil, fmt.Errorf("existe un runner_session distinto en la instancia full; se rechaza actividad ajena")
	}
	return &existing, nil
}

func ensureFullActivityFeedback(tx *gorm.DB, expected dbs.WorkoutFeedback) (*dbs.WorkoutFeedback, error) {
	var rows []dbs.WorkoutFeedback
	err := tx.Unscoped().Where(
		"assigned_session_id = ? AND assigned_exercise_id = ? AND athlete_user_id = ? AND feedback_owner_user_id = ? AND set_number = ?",
		expected.AssignedSessionID, expected.AssignedExerciseID, expected.AthleteUserID, expected.FeedbackOwnerUserID, expected.SetNumber,
	).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		if err := tx.Create(&expected).Error; err != nil {
			return nil, err
		}
		return &expected, nil
	}
	if len(rows) != 1 || !sameFullActivityFeedback(rows[0], expected) {
		return nil, fmt.Errorf("existe un feedback distinto o sin marker en la clave full; se rechaza actividad ajena")
	}
	return &rows[0], nil
}

func ensureFullActivityPoints(tx *gorm.DB, feedbackID, sessionID, exerciseID int64, spec fullActivityFeedbackSpec, now time.Time) error {
	var expected []dbs.WorkoutFeedbackPoint
	if spec.withPoints {
		start := *spec.startedAt
		for index, delta := range []float64{0, 0.00012, 0.00027} {
			expected = append(expected, dbs.WorkoutFeedbackPoint{
				FeedbackID: feedbackID, SessionInstanceID: sessionID, ExerciseInstanceID: exerciseID,
				Order: index, Latitude: -31.4201 + delta, Longitude: -64.1888,
				RecordedAt: start.Add(time.Duration(index) * 10 * time.Second), CreatedAt: now,
			})
		}
		if _, err := daos.NewWorkoutFeedbackDao(tx).BulkCreatePointsRecompute(nil, feedbackID, expected); err != nil {
			return err
		}
	}
	var actual []dbs.WorkoutFeedbackPoint
	if err := tx.Where("feedback_id = ?", feedbackID).Order(`"order"`).Find(&actual).Error; err != nil {
		return err
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("cantidad de puntos %d, esperada %d", len(actual), len(expected))
	}
	for index := range expected {
		want, got := expected[index], actual[index]
		if got.Order != want.Order || got.SessionInstanceID != want.SessionInstanceID || got.ExerciseInstanceID != want.ExerciseInstanceID || got.Latitude != want.Latitude || got.Longitude != want.Longitude || !got.RecordedAt.Equal(want.RecordedAt) {
			return fmt.Errorf("el punto %d no coincide con la serie full", index)
		}
	}
	return nil
}

func ensureFullActivityAttendance(tx *gorm.DB, expected dbs.Attendance) (*dbs.Attendance, error) {
	var existing dbs.Attendance
	err := tx.Where("team_id = ? AND training_session_id = ? AND user_id = ?", expected.TeamID, expected.TrainingSessionID, expected.UserID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := tx.Create(&expected).Error; err != nil {
			return nil, err
		}
		return &expected, nil
	}
	if err != nil {
		return nil, err
	}
	if existing.Source != expected.Source || !sameInt64Pointer(existing.RegisteredByUserID, expected.RegisteredByUserID) {
		return nil, fmt.Errorf("existe una asistencia distinta en la instancia full; se rechaza actividad ajena")
	}
	return &existing, nil
}

func validateFullActivityAttendanceContext(tx *gorm.DB, groupID, teamID, sessionInstanceID, userID int64, sessionKey string, now time.Time) error {
	var expectedDate time.Time
	switch sessionKey {
	case "today_a_main":
		expectedDate = fullDate(now)
	case fullActivitySecondaryDayKey:
		expectedDate = fullDate(now.AddDate(0, 0, -7))
	default:
		expectedDate = fullDate(now.AddDate(0, 0, -14))
	}
	var day dbs.GroupCalendarDay
	if err := tx.Where("group_id = ? AND session_instance_id = ?", groupID, sessionInstanceID).Take(&day).Error; err != nil {
		return fmt.Errorf("no hay día de calendario para el grupo y la instancia full: %w", err)
	}
	if day.Kind != "training" || !day.IsPresencial || day.Date.Format("2006-01-02") != expectedDate.Format("2006-01-02") {
		return fmt.Errorf("la instancia full no es presencial en la fecha requerida")
	}
	var group dbs.Group
	if err := tx.First(&group, groupID).Error; err != nil {
		return err
	}
	if group.TeamID != teamID {
		return fmt.Errorf("el grupo de la asistencia no pertenece al equipo full")
	}
	var membershipCount int64
	if err := tx.Model(&dbs.GroupUser{}).
		Where("group_id = ? AND user_id = ? AND deleted_at IS NULL AND date_start::date <= ?::date AND (date_end IS NULL OR date_end::date >= ?::date)", groupID, userID, expectedDate, expectedDate).
		Count(&membershipCount).Error; err != nil {
		return err
	}
	if membershipCount != 1 {
		return fmt.Errorf("el usuario no era miembro activo del grupo en la fecha de asistencia")
	}
	return nil
}

func fullActivityExerciseInstanceID(tx *gorm.DB, sessionInstanceID, catalogExerciseID int64) (int64, error) {
	var row struct{ ID int64 }
	err := tx.Table("session_exercise_instances AS sei").
		Select("ei.id").
		Joins("JOIN exercise_instances ei ON ei.id = sei.exercise_instance_id").
		Where("sei.session_instance_id = ? AND ei.source_exercise_id = ?", sessionInstanceID, catalogExerciseID).
		Order("sei.id").Take(&row).Error
	if err != nil {
		return 0, err
	}
	return row.ID, nil
}

func fullActivityStampedDayKey(date time.Time) string {
	return "full_21_days_" + fullDate(date).Format("2006-01-02")
}

func sessionInstanceIDForActivity(key string, existing, staged map[string]int64) int64 {
	if id := staged[key]; id > 0 {
		return id
	}
	return existing[key]
}

func fullActivityCompletedFeedback(key, sessionKey, exerciseKey, athleteKey, ownerKey, status string, date, start time.Time, setNumber int, duration time.Duration, distance float64, withPoints, deleted bool) fullActivityFeedbackSpec {
	end := start.Add(duration)
	durationMS := duration.Milliseconds()
	activeDurationMS := durationMS - int64(time.Minute/time.Millisecond)
	distanceMeters := distance
	rpe, avgHeartRate, maxHeartRate, cadence := int16(7), int16(148), int16(171), int16(172)
	elevationGain := 24.0
	spec := fullActivityFeedbackSpec{
		key: key, sessionKey: sessionKey, exerciseKey: exerciseKey, athleteKey: athleteKey,
		ownerKey: ownerKey, status: status, reportSource: "corredor", date: date, setNumber: setNumber,
		startedAt: &start, endedAt: &end, durationMs: &durationMS, activeDurationMs: &activeDurationMS,
		distanceMeters: &distanceMeters, elevationGainMeters: &elevationGain, rpe: &rpe,
		avgHeartRate: &avgHeartRate, maxHeartRate: &maxHeartRate, cadence: &cadence, withPoints: withPoints,
	}
	if deleted {
		deletedAt := fullActivityClock(date, 12, 0)
		spec.deletedAt = &deletedAt
	}
	return spec
}

func sameFullActivityFeedback(got, want dbs.WorkoutFeedback) bool {
	return sameInt64Pointer(got.TeamID, want.TeamID) &&
		got.AssignedSessionID == want.AssignedSessionID && got.AssignedExerciseID == want.AssignedExerciseID &&
		got.AthleteUserID == want.AthleteUserID && got.FeedbackOwnerUserID == want.FeedbackOwnerUserID &&
		got.ReportSource == want.ReportSource && got.SessionDate.Format("2006-01-02") == want.SessionDate.Format("2006-01-02") &&
		got.SetNumber == want.SetNumber && sameTimePointer(got.StartedAt, want.StartedAt) && sameTimePointer(got.EndedAt, want.EndedAt) &&
		sameInt64Pointer(got.DurationMs, want.DurationMs) && sameInt64Pointer(got.ActiveDurationMs, want.ActiveDurationMs) &&
		sameFloat64Pointer(got.DistanceMeters, want.DistanceMeters) && sameFloat64Pointer(got.ElevationGainMeters, want.ElevationGainMeters) &&
		sameInt16Pointer(got.RPE, want.RPE) && sameInt16Pointer(got.AvgHeartRate, want.AvgHeartRate) &&
		sameInt16Pointer(got.MaxHeartRate, want.MaxHeartRate) && sameInt16Pointer(got.Cadence, want.Cadence) &&
		sameStringPointer(got.CompletionStatus, want.CompletionStatus) &&
		containsFullSeedMarker(pointerValue(got.Annotations)) && sameTimePointer(got.DeletedAt, want.DeletedAt)
}

func fullActivityClock(date time.Time, hour, minute int) time.Time {
	year, month, day := fullDate(date).Date()
	return time.Date(year, month, day, hour, minute, 0, 0, gametime.Location)
}

func mergeFullActivityIDs(target, source map[string]int64) {
	for key, id := range source {
		target[key] = id
	}
}

func sameTimePointer(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func sameInt64Pointer(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameFloat64Pointer(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameInt16Pointer(left, right *int16) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameStringPointer(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func timePointer(value time.Time) *time.Time { return &value }

func int64Pointer(value int64) *int64 { return &value }

func stringPointer(value string) *string { return &value }

func stringsHasFullSeedEmail(email string) bool {
	return len(email) >= len("@seed-full.local") && email[len(email)-len("@seed-full.local"):] == "@seed-full.local"
}
