package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils/gametime"
)

func (s *fullSeeder) seedFullCalendar() error {
	calendarService := fullCalendarService(s.db)
	if err := s.stampFullPlan(calendarService, "group_a_main", "full_21_days", s.now.AddDate(0, 0, -21)); err != nil {
		return fmt.Errorf("estampar calendario histórico full: %w", err)
	}
	if err := s.stampFullPlan(calendarService, "group_a_main", "full_14_days", s.now.AddDate(0, 0, 1)); err != nil {
		return fmt.Errorf("estampar calendario futuro full: %w", err)
	}

	today := fullDate(s.now)
	from, to, err := fullCalendarClockRange(s.now)
	if err != nil {
		return err
	}
	openedAt := s.now.Add(-20 * time.Minute)
	closedAt := s.now.Add(-5 * time.Minute)
	location := trainingplan.Location{Lat: -31.4201, Lng: -64.1888, Label: strPtr("Parque del equipo")}
	fixtures := []fullTodayCalendarSpec{
		{key: "today_a_main", groupKey: "group_a_main", ownerKey: "trainer_1", sessionKey: "full_interval", presencial: true, from: from, to: to, location: &location, openedAt: &openedAt},
		{key: "today_a_secondary", groupKey: "group_a_secondary", ownerKey: "trainer_1", sessionKey: "full_interval", presencial: true, from: from, to: to, location: &location},
		{key: "today_f_closed", groupKey: "group_f", ownerKey: "trainer_1", sessionKey: "full_interval", presencial: true, from: from, to: to, location: &location, openedAt: &openedAt, closedAt: &closedAt},
		{key: "today_b_async", groupKey: "group_b", ownerKey: "trainer_2", sessionKey: "full_b_async"},
	}
	for _, spec := range fixtures {
		groupID := s.ids.groups[spec.groupKey]
		sessionID := s.ids.sessions[spec.sessionKey]
		day, instanceID, err := s.ensureFullTodayDay(today, groupID, sessionID, s.ids.users[spec.ownerKey], spec)
		if err != nil {
			return fmt.Errorf("insertar día full %s: %w", spec.key, err)
		}
		s.ids.calendarDays[spec.key] = day.ID
		s.ids.sessionInstances[spec.key] = instanceID
	}

	if err := s.cancelFullFutureTraining(calendarService, today.AddDate(0, 0, 12)); err != nil {
		return err
	}
	if err := s.ensureFullSameTeamTraining(calendarService, today.AddDate(0, 0, 1), time.Date(0, time.January, 1, 18, 0, 0, 0, time.UTC), time.Date(0, time.January, 1, 20, 0, 0, 0, time.UTC), &location); err != nil {
		return err
	}
	return nil
}

func fullCalendarService(db *gorm.DB) services.CalendarServiceInterface {
	return services.NewCalendarService(
		daos.NewGroupCalendarDayDao(db), daos.NewGroupDao(db), daos.NewTeamDao(db),
		daos.NewGroupUserDao(db), daos.NewTeamUserDao(db), daos.NewTrainingPlanDao(db),
		daos.NewPlanDayDao(db), daos.NewSessionDao(db), db,
	)
}

type fullTodayCalendarSpec struct {
	key, groupKey, ownerKey, sessionKey string
	presencial                          bool
	from, to                            *time.Time
	location                            *trainingplan.Location
	openedAt, closedAt                  *time.Time
}

func (s *fullSeeder) stampFullPlan(service services.CalendarServiceInterface, groupKey, planKey string, start time.Time) error {
	groupID := s.ids.groups[groupKey]
	planID := s.ids.trainingPlans[planKey]
	var plan dbs.TrainingPlan
	if err := s.db.First(&plan, planID).Error; err != nil {
		return fmt.Errorf("buscar plan %s: %w", planKey, err)
	}
	var planDays []dbs.PlanDay
	if err := s.db.Where("plan_id = ?", planID).Order("sequence_no").Find(&planDays).Error; err != nil {
		return fmt.Errorf("leer días del plan %s: %w", planKey, err)
	}
	if len(planDays) == 0 {
		return fmt.Errorf("el plan %s no tiene días", planKey)
	}
	start = fullDate(start)
	end := start.AddDate(0, 0, len(planDays)-1)
	rows, err := daos.NewGroupCalendarDayDao(s.db).FindByGroupAndRange(nil, groupID, start, end)
	if err != nil {
		return fmt.Errorf("leer calendario existente para %s: %w", groupKey, err)
	}
	excludeDates := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.SourcePlanID == nil || *row.SourcePlanID != planID {
			return fmt.Errorf("la fecha %s del grupo %s ya tiene contenido fuera del plan full %s", row.Date.Format("2006-01-02"), groupKey, planKey)
		}
		excludeDates = append(excludeDates, row.Date.Format("2006-01-02"))
	}
	response, err := service.Stamp(nil, groupID, s.ids.users["trainer_1"], calendar.StampRequest{
		PlanID: planID, StartDate: start.Format("2006-01-02"), ExcludeDates: excludeDates,
	})
	if err != nil {
		return fmt.Errorf("calendar stamp %s: %w", planKey, err)
	}
	for _, day := range response.Days {
		key := fmt.Sprintf("%s_%s", planKey, day.Date)
		s.ids.calendarDays[key] = day.ID
		if day.SessionInstance != nil {
			s.ids.sessionInstances[key] = day.SessionInstance.ID
		}
	}
	rows, err = daos.NewGroupCalendarDayDao(s.db).FindByGroupAndRange(nil, groupID, start, end)
	if err != nil {
		return fmt.Errorf("recargar días del plan %s: %w", planKey, err)
	}
	for _, day := range rows {
		key := fmt.Sprintf("%s_%s", planKey, day.Date.Format("2006-01-02"))
		s.ids.calendarDays[key] = day.ID
		if day.SessionInstanceID != nil {
			s.ids.sessionInstances[key] = *day.SessionInstanceID
		}
	}
	return nil
}

func (s *fullSeeder) ensureFullTodayDay(date time.Time, groupID, sessionID, ownerID int64, spec fullTodayCalendarSpec) (dbs.GroupCalendarDay, int64, error) {
	var day dbs.GroupCalendarDay
	var instanceID int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var existing dbs.GroupCalendarDay
		lookupErr := tx.Where("group_id = ? AND date = ?", groupID, date).Take(&existing).Error
		found := lookupErr == nil
		if found {
			if existing.SessionInstanceID == nil {
				return fmt.Errorf("la fecha de hoy ya contiene datos ajenos en el grupo reservado %d", groupID)
			}
			var instance dbs.SessionInstance
			if err := tx.First(&instance, *existing.SessionInstanceID).Error; err != nil {
				return err
			}
			if instance.SourceSessionID == nil || *instance.SourceSessionID != sessionID {
				return fmt.Errorf("la instancia de hoy no corresponde a la sesión full %d", sessionID)
			}
			instanceID = instance.ID
		} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		} else {
			instance, err := instantiateFullSession(tx, sessionID, ownerID, s.now)
			if err != nil {
				return err
			}
			instanceID = instance.ID
		}
		row := dbs.GroupCalendarDay{
			GroupID: groupID, Date: date, Kind: string(constants.GroupCalendarDayKindTraining),
			SessionInstanceID: &instanceID, IsPresencial: spec.presencial,
			PresencialTimeFrom: spec.from, PresencialTimeTo: spec.to,
			PresencialOpenedAt: spec.openedAt, PresencialClosedAt: spec.closedAt,
		}
		if spec.location != nil {
			encoded, err := json.Marshal(spec.location)
			if err != nil {
				return err
			}
			location := string(encoded)
			row.PresencialLocation = &location
		}
		if !found {
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			day = row
			return nil
		}
		if err := tx.Model(&dbs.GroupCalendarDay{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"kind": row.Kind, "session_instance_id": instanceID, "is_presencial": row.IsPresencial,
			"presencial_time_from": row.PresencialTimeFrom, "presencial_time_to": row.PresencialTimeTo,
			"presencial_location": row.PresencialLocation, "presencial_opened_at": row.PresencialOpenedAt,
			"presencial_closed_at": row.PresencialClosedAt, "source_plan_id": nil, "updated_at": s.now,
		}).Error; err != nil {
			return err
		}
		row.ID = existing.ID
		day = row
		return nil
	})
	return day, instanceID, err
}

func instantiateFullSession(tx *gorm.DB, sessionID, ownerID int64, now time.Time) (*dbs.SessionInstance, error) {
	var source dbs.Session
	if err := tx.First(&source, sessionID).Error; err != nil {
		return nil, fmt.Errorf("leer sesión full %d: %w", sessionID, err)
	}
	if source.OwnerID != ownerID || !containsFullSeedMarker(pointerValue(source.Description)) {
		return nil, fmt.Errorf("la sesión %d no pertenece al catálogo full del owner %d", sessionID, ownerID)
	}
	var links []dbs.SessionExercise
	if err := tx.Where("session_id = ?", sessionID).Order("id").Find(&links).Error; err != nil {
		return nil, fmt.Errorf("leer bloques de sesión full %d: %w", sessionID, err)
	}
	if len(links) == 0 {
		return nil, fmt.Errorf("la sesión full %d no tiene ejercicios", sessionID)
	}
	instance := dbs.SessionInstance{Name: source.Name, Description: source.Description, SourceSessionID: &sessionID, CreatedAt: now}
	if err := tx.Create(&instance).Error; err != nil {
		return nil, fmt.Errorf("crear instancia de sesión full: %w", err)
	}
	for _, link := range links {
		var exercise dbs.Exercise
		if err := tx.First(&exercise, link.ExerciseID).Error; err != nil {
			return nil, fmt.Errorf("leer ejercicio full %d: %w", link.ExerciseID, err)
		}
		if exercise.OwnerID != ownerID || !containsFullSeedMarker(pointerValue(exercise.Description)) {
			return nil, fmt.Errorf("el ejercicio %d no pertenece al catálogo full del owner %d", exercise.ID, ownerID)
		}
		sourceExerciseID := exercise.ID
		exerciseInstance := dbs.ExerciseInstance{
			Name: exercise.Name, Description: exercise.Description, Kind: exercise.Kind,
			Intensity: exercise.Intensity, Minutes: exercise.Minutes, DistanceM: exercise.DistanceM,
			SpeedKph: exercise.SpeedKph, MuscleGroup: exercise.MuscleGroup, VideoURL: exercise.VideoURL,
			SourceExerciseID: &sourceExerciseID, CreatedAt: now,
		}
		if err := tx.Create(&exerciseInstance).Error; err != nil {
			return nil, fmt.Errorf("crear ejercicio instancia full: %w", err)
		}
		instanceLink := dbs.SessionExerciseInstance{
			SessionInstanceID: instance.ID, ExerciseInstanceID: exerciseInstance.ID,
			Role: link.Role, RepeatCount: link.RepeatCount, RestMinutes: link.RestMinutes,
		}
		if err := tx.Create(&instanceLink).Error; err != nil {
			return nil, fmt.Errorf("crear vínculo de ejercicio instancia full: %w", err)
		}
	}
	return &instance, nil
}

func (s *fullSeeder) cancelFullFutureTraining(service services.CalendarServiceInterface, date time.Time) error {
	date = fullDate(date)
	var existing dbs.GroupCalendarDay
	if err := s.db.Where("group_id = ? AND date = ?", s.ids.groups["group_a_main"], date).Take(&existing).Error; err != nil {
		return fmt.Errorf("leer día futuro a cancelar: %w", err)
	}
	if existing.SourcePlanID == nil || *existing.SourcePlanID != s.ids.trainingPlans["full_14_days"] {
		return fmt.Errorf("el día futuro %s no pertenece al plan full de 14 días", date.Format("2006-01-02"))
	}
	if existing.Kind != string(constants.GroupCalendarDayKindCancelled) {
		reason := "Reprogramado por el entrenador."
		if _, err := service.UpsertDay(nil, existing.GroupID, s.ids.users["trainer_1"], date, calendar.CalendarDayRequest{
			Kind: string(constants.GroupCalendarDayKindCancelled), CancelledReason: &reason,
		}); err != nil {
			return fmt.Errorf("cancelar día futuro full: %w", err)
		}
	}
	return s.db.Model(&dbs.GroupCalendarDay{}).Where("id = ?", existing.ID).Update("source_plan_id", s.ids.trainingPlans["full_14_days"]).Error
}

func (s *fullSeeder) ensureFullSameTeamTraining(service services.CalendarServiceInterface, date time.Time, timeFrom, timeTo time.Time, location *trainingplan.Location) error {
	date = fullDate(date)
	groupID := s.ids.groups["group_a_secondary"]
	var existing dbs.GroupCalendarDay
	err := s.db.Where("group_id = ? AND date = ?", groupID, date).Take(&existing).Error
	if err == nil {
		if existing.SourcePlanID == nil || *existing.SourcePlanID != s.ids.trainingPlans["full_14_days"] || existing.SessionInstanceID == nil {
			return fmt.Errorf("el día secundario %s ya contiene datos ajenos", date.Format("2006-01-02"))
		}
		var instance dbs.SessionInstance
		if err := s.db.First(&instance, *existing.SessionInstanceID).Error; err != nil || instance.SourceSessionID == nil || *instance.SourceSessionID != s.ids.sessions["full_interval"] {
			return fmt.Errorf("la instancia del grupo A secundario no es del catálogo full")
		}
		key := "full_same_team_" + date.Format("2006-01-02")
		s.ids.calendarDays[key] = existing.ID
		s.ids.sessionInstances[key] = *existing.SessionInstanceID
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("buscar día del grupo A secundario: %w", err)
	}
	from, to := timeFrom.Format("15:04"), timeTo.Format("15:04")
	isPresencial := true
	sessionID := s.ids.sessions["full_interval"]
	result, err := service.UpsertDay(nil, groupID, s.ids.users["trainer_1"], date, calendar.CalendarDayRequest{
		Kind: string(constants.GroupCalendarDayKindTraining), SessionID: &sessionID, IsPresencial: &isPresencial,
		PresencialTimeFrom: &from, PresencialTimeTo: &to, PresencialLocation: location,
	})
	if err != nil {
		return fmt.Errorf("crear presencial del grupo A secundario: %w", err)
	}
	if len(result.SameTeamWarnings) == 0 {
		return fmt.Errorf("se esperaba warning presencial same-team para el grupo A secundario")
	}
	if err := s.db.Model(&dbs.GroupCalendarDay{}).
		Where("group_id = ? AND date = ?", groupID, date).
		Update("source_plan_id", s.ids.trainingPlans["full_14_days"]).Error; err != nil {
		return err
	}
	if err := s.db.Where("group_id = ? AND date = ?", groupID, date).Take(&existing).Error; err != nil {
		return err
	}
	s.ids.calendarDays["full_same_team_"+date.Format("2006-01-02")] = existing.ID
	if existing.SessionInstanceID != nil {
		s.ids.sessionInstances["full_same_team_"+date.Format("2006-01-02")] = *existing.SessionInstanceID
	}
	return nil
}

func fullDate(value time.Time) time.Time {
	year, month, day := value.In(gametime.Location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, gametime.Location)
}

func fullClock(value time.Time) time.Time {
	local := value.In(gametime.Location)
	return time.Date(0, time.January, 1, local.Hour(), local.Minute(), 0, 0, time.UTC)
}

func fullCalendarClockRange(now time.Time) (*time.Time, *time.Time, error) {
	from, to := fullClock(now.Add(-30*time.Minute)), fullClock(now.Add(2*time.Hour))
	if !from.Before(to) {
		return nil, nil, fmt.Errorf("la ventana presencial full cruza la medianoche y no puede representarse como HH:MM")
	}
	return &from, &to, nil
}
