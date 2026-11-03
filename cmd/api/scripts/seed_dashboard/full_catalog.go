package main

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/services"
)

type fullExerciseSpec struct {
	key, name, kind    string
	minutes, distanceM *int
	muscleGroup        *string
}

type fullSessionExerciseSpec struct {
	exerciseKey, role string
	repeat, rest      int
}

type fullSessionSpec struct {
	key, name string
	exercises []fullSessionExerciseSpec
}

type fullPlanDaySpec struct {
	kind, sessionKey, otherName string
	presencial                  bool
	timeFrom, timeTo            string
}

type fullTrainingPlanSpec struct {
	key, name   string
	days        int
	assignments map[int]fullPlanDaySpec
}

func (s *fullSeeder) seedFullCatalog() error {
	ownerID := s.ids.users["trainer_1"]
	specs := fullExerciseSpecs()
	s.ids.exercises = make(map[string]int64, len(specs))
	for _, spec := range specs {
		row, err := ensureFullExercise(s.db, ownerID, spec, s.now)
		if err != nil {
			return err
		}
		s.ids.exercises[spec.key] = row.ID
	}

	sessionService := services.NewSessionService(
		daos.NewSessionDao(s.db), daos.NewSessionExerciseDao(s.db), daos.NewExerciseDao(s.db),
	)
	s.ids.sessions = make(map[string]int64)
	for _, spec := range fullSessionSpecs() {
		req := fullSessionRequest(ownerID, spec, s.ids.exercises)
		rows, err := daos.NewSessionDao(s.db).FindByOwner(nil, ownerID)
		if err != nil {
			return fmt.Errorf("leer sesiones full: %w", err)
		}
		var existing *dbs.Session
		for i := range rows {
			if rows[i].Name == spec.name {
				existing = &rows[i]
				break
			}
		}
		var resultID int64
		if existing == nil {
			created, err := sessionService.Create(nil, ownerID, req)
			if err != nil {
				return fmt.Errorf("crear sesión full %q: %w", spec.key, err)
			}
			resultID = created.ID
		} else {
			if !containsFullSeedMarker(pointerValue(existing.Description)) {
				return fmt.Errorf("sesión full %q ya existe sin marker %s", spec.key, fullSeedMarker)
			}
			updated, err := sessionService.Update(nil, existing.ID, ownerID, req)
			if err != nil {
				return fmt.Errorf("actualizar sesión full %q: %w", spec.key, err)
			}
			resultID = updated.ID
		}
		s.ids.sessions[spec.key] = resultID
	}

	secondaryOwner := s.ids.users["trainer_2"]
	secondaryExerciseIDs := make(map[string]int64, 3)
	for _, spec := range fullSecondaryExercises() {
		row, err := ensureFullExercise(s.db, secondaryOwner, spec, s.now)
		if err != nil {
			return err
		}
		secondaryExerciseIDs[spec.key] = row.ID
		s.ids.exercises[spec.key] = row.ID
	}
	secondarySpec := fullSessionSpec{
		key: "full_b_async", name: "Semilla Full · Rodaje asincrónico B",
		exercises: []fullSessionExerciseSpec{
			{"b_async_warmup", string(constants.SessionExerciseRoleWarmup), 1, 0},
			{"b_async_main", string(constants.SessionExerciseRoleMain), 1, 0},
			{"b_async_cooldown", string(constants.SessionExerciseRoleCooldown), 1, 0},
		},
	}
	secondaryReq := fullSessionRequest(secondaryOwner, secondarySpec, secondaryExerciseIDs)
	secondaryRows, err := daos.NewSessionDao(s.db).FindByOwner(nil, secondaryOwner)
	if err != nil {
		return fmt.Errorf("leer sesión asincrónica del equipo B: %w", err)
	}
	var secondaryExisting *dbs.Session
	for i := range secondaryRows {
		if secondaryRows[i].Name == secondarySpec.name {
			secondaryExisting = &secondaryRows[i]
			break
		}
	}
	if secondaryExisting == nil {
		created, err := sessionService.Create(nil, secondaryOwner, secondaryReq)
		if err != nil {
			return fmt.Errorf("crear sesión asincrónica del equipo B: %w", err)
		}
		s.ids.sessions[secondarySpec.key] = created.ID
	} else {
		if !containsFullSeedMarker(pointerValue(secondaryExisting.Description)) {
			return fmt.Errorf("sesión asincrónica del equipo B ya existe sin marker %s", fullSeedMarker)
		}
		updated, err := sessionService.Update(nil, secondaryExisting.ID, secondaryOwner, secondaryReq)
		if err != nil {
			return fmt.Errorf("actualizar sesión asincrónica del equipo B: %w", err)
		}
		s.ids.sessions[secondarySpec.key] = updated.ID
	}

	planService := services.NewTrainingPlanService(
		daos.NewTrainingPlanDao(s.db), daos.NewPlanDayDao(s.db), daos.NewSessionDao(s.db),
		daos.NewGroupCalendarDayDao(s.db),
	)
	planDao := daos.NewTrainingPlanDao(s.db)
	s.ids.trainingPlans = make(map[string]int64)
	for _, spec := range fullTrainingPlanSpecs() {
		req := fullTrainingPlanRequest(ownerID, spec, s.ids.sessions)
		rows, err := planDao.FindByOwner(nil, ownerID)
		if err != nil {
			return fmt.Errorf("leer planes full: %w", err)
		}
		var existing *dbs.TrainingPlan
		for i := range rows {
			if rows[i].Name == spec.name {
				existing = &rows[i]
				break
			}
		}
		var resultID int64
		if existing == nil {
			created, err := planService.Create(nil, ownerID, req)
			if err != nil {
				return fmt.Errorf("crear plan full %q: %w", spec.key, err)
			}
			resultID = created.ID
		} else {
			if !containsFullSeedMarker(pointerValue(existing.Description)) {
				return fmt.Errorf("plan full %q ya existe sin marker %s", spec.key, fullSeedMarker)
			}
			name, description := req.Name, pointerValue(req.Description)
			updated, err := planService.Update(nil, existing.ID, ownerID, trainingplan.TrainingPlanUpdateRequest{
				Name: &name, Description: &description, Days: &req.Days,
			})
			if err != nil {
				return fmt.Errorf("actualizar plan full %q: %w", spec.key, err)
			}
			resultID = updated.ID
		}
		s.ids.trainingPlans[spec.key] = resultID
	}
	return nil
}

func fullExerciseSpecs() []fullExerciseSpec {
	return []fullExerciseSpec{
		{key: "full_warmup", name: "Semilla Full · Activación", kind: string(constants.ExerciseKindLiviano), minutes: intPtr(12), distanceM: intPtr(1500)},
		{key: "full_mobility", name: "Semilla Full · Movilidad", kind: string(constants.ExerciseKindElongacion), minutes: intPtr(10), muscleGroup: strPtr(string(constants.MuscleGroupGemelos))},
		{key: "full_endurance", name: "Semilla Full · Fondo", kind: string(constants.ExerciseKindRitmoContinuo), minutes: intPtr(45), distanceM: intPtr(8000)},
		{key: "full_speed_time", name: "Semilla Full · Velocidad por tiempo", kind: string(constants.ExerciseKindVelocidadTiempo), minutes: intPtr(1), distanceM: intPtr(400)},
		{key: "full_speed_distance", name: "Semilla Full · Velocidad por distancia", kind: string(constants.ExerciseKindVelocidadDistancia), minutes: intPtr(5), distanceM: intPtr(1000)},
		{key: "full_cooldown", name: "Semilla Full · Vuelta a la calma", kind: string(constants.ExerciseKindLiviano), minutes: intPtr(8), distanceM: intPtr(800)},
	}
}

func fullSecondaryExercises() []fullExerciseSpec {
	return []fullExerciseSpec{
		{key: "b_async_warmup", name: "Semilla Full · B Activación", kind: string(constants.ExerciseKindLiviano), minutes: intPtr(8), distanceM: intPtr(800)},
		{key: "b_async_main", name: "Semilla Full · B Rodaje", kind: string(constants.ExerciseKindRitmoContinuo), minutes: intPtr(35), distanceM: intPtr(6000)},
		{key: "b_async_cooldown", name: "Semilla Full · B Recuperación", kind: string(constants.ExerciseKindLiviano), minutes: intPtr(7), distanceM: intPtr(700)},
	}
}

func fullSessionSpecs() []fullSessionSpec {
	return []fullSessionSpec{
		{
			key: "full_interval", name: "Semilla Full · Intervalos",
			exercises: []fullSessionExerciseSpec{
				{"full_warmup", string(constants.SessionExerciseRoleWarmup), 1, 0},
				{"full_mobility", string(constants.SessionExerciseRoleWarmup), 1, 0},
				{"full_endurance", string(constants.SessionExerciseRoleMain), 3, 2},
				{"full_speed_time", string(constants.SessionExerciseRoleMain), 4, 1},
				{"full_cooldown", string(constants.SessionExerciseRoleCooldown), 1, 0},
			},
		},
		{
			key: "full_distance", name: "Semilla Full · Series de distancia",
			exercises: []fullSessionExerciseSpec{
				{"full_warmup", string(constants.SessionExerciseRoleWarmup), 1, 0},
				{"full_speed_distance", string(constants.SessionExerciseRoleMain), 6, 2},
				{"full_endurance", string(constants.SessionExerciseRoleMain), 1, 0},
				{"full_cooldown", string(constants.SessionExerciseRoleCooldown), 1, 0},
			},
		},
	}
}

func fullSessionRequest(ownerID int64, spec fullSessionSpec, exerciseIDs map[string]int64) session.SessionRequest {
	req := session.SessionRequest{
		OwnerID: ownerID, Name: spec.name,
		Description: strPtr(fullSeedMarker + " Plantilla de sesión del perfil full."),
	}
	for _, item := range spec.exercises {
		req.Exercises = append(req.Exercises, session.SessionExerciseRequest{
			ExerciseID: exerciseIDs[item.exerciseKey], Role: item.role,
			RepeatCount: intPtr(item.repeat), RestMinutes: intPtr(item.rest),
		})
	}
	return req
}

func fullTrainingPlanSpecs() []fullTrainingPlanSpec {
	return []fullTrainingPlanSpec{
		{key: "full_7_days", name: "Semilla Full · Plan de 7 días", days: 7, assignments: map[int]fullPlanDaySpec{
			2: {kind: "training", sessionKey: "full_interval", presencial: true, timeFrom: "18:00", timeTo: "20:00"},
			5: {kind: "training", sessionKey: "full_distance"},
		}},
		{key: "full_14_days", name: "Semilla Full · Plan de 14 días", days: 14, assignments: map[int]fullPlanDaySpec{
			1:  {kind: "training", sessionKey: "full_interval", presencial: true, timeFrom: "18:00", timeTo: "20:00"},
			4:  {kind: "training", sessionKey: "full_distance"},
			7:  {kind: "other", otherName: "Movilidad guiada"},
			9:  {kind: "training", sessionKey: "full_distance", presencial: true, timeFrom: "08:00", timeTo: "09:30"},
			12: {kind: "training", sessionKey: "full_interval"},
		}},
		{key: "full_21_days", name: "Semilla Full · Plan de 21 días", days: 21, assignments: map[int]fullPlanDaySpec{
			2:  {kind: "training", sessionKey: "full_interval", presencial: true, timeFrom: "18:00", timeTo: "20:00"},
			5:  {kind: "training", sessionKey: "full_distance"},
			8:  {kind: "training", sessionKey: "full_distance", presencial: true, timeFrom: "08:00", timeTo: "09:30"},
			14: {kind: "training", sessionKey: "full_interval"},
			17: {kind: "other", otherName: "Evaluación funcional"},
			20: {kind: "training", sessionKey: "full_distance", presencial: true, timeFrom: "18:00", timeTo: "20:00"},
		}},
	}
}

func fullTrainingPlanRequest(ownerID int64, spec fullTrainingPlanSpec, sessionIDs map[string]int64) trainingplan.TrainingPlanRequest {
	location := trainingplan.Location{Lat: -31.4201, Lng: -64.1888, Label: strPtr("Parque del equipo")}
	req := trainingplan.TrainingPlanRequest{
		OwnerID: ownerID, Name: spec.name,
		Description: strPtr(fullSeedMarker + " Plan secuencial del perfil full."),
		Days:        make([]trainingplan.PlanDayRequest, 0, spec.days),
	}
	for sequence := 1; sequence <= spec.days; sequence++ {
		day := trainingplan.PlanDayRequest{SequenceNo: sequence, Kind: string(constants.PlanDayKindRest)}
		if assigned, ok := spec.assignments[sequence]; ok {
			day.Kind = assigned.kind
			switch assigned.kind {
			case string(constants.PlanDayKindTraining):
				sessionID := sessionIDs[assigned.sessionKey]
				day.SessionID = &sessionID
				day.DefaultPresencial = boolPtr(assigned.presencial)
				if assigned.presencial {
					day.DefaultTimeFrom = strPtr(assigned.timeFrom)
					day.DefaultTimeTo = strPtr(assigned.timeTo)
					day.DefaultLocation = &location
				}
			case string(constants.PlanDayKindOther):
				day.OtherName = strPtr(assigned.otherName)
			}
		}
		req.Days = append(req.Days, day)
	}
	return req
}

func ensureFullExercise(db *gorm.DB, ownerID int64, spec fullExerciseSpec, now time.Time) (*dbs.Exercise, error) {
	var existing dbs.Exercise
	err := db.Where("owner_id = ? AND name = ?", ownerID, spec.name).Take(&existing).Error
	description := fullSeedMarker + " Ejercicio del perfil full."
	values := map[string]any{
		"owner_id": ownerID, "name": spec.name, "description": description,
		"kind": spec.kind, "minutes": spec.minutes, "distance_m": spec.distanceM,
		"intensity": nil, "speed_kph": nil, "muscle_group": spec.muscleGroup,
		"video_url": nil, "deleted_at": nil, "updated_at": now,
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row := &dbs.Exercise{
			OwnerID: ownerID, Name: spec.name, Description: &description, Kind: spec.kind,
			Minutes: spec.minutes, DistanceM: spec.distanceM, MuscleGroup: spec.muscleGroup,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(row).Error; err != nil {
			return nil, fmt.Errorf("crear ejercicio full %q: %w", spec.key, err)
		}
		return row, nil
	}
	if err != nil {
		return nil, fmt.Errorf("buscar ejercicio full %q: %w", spec.key, err)
	}
	if !containsFullSeedMarker(pointerValue(existing.Description)) {
		return nil, fmt.Errorf("ejercicio full %q ya existe sin marker %s", spec.key, fullSeedMarker)
	}
	if err := db.Model(&existing).Updates(values).Error; err != nil {
		return nil, fmt.Errorf("actualizar ejercicio full %q: %w", spec.key, err)
	}
	existing.Kind, existing.Minutes, existing.DistanceM = spec.kind, spec.minutes, spec.distanceM
	existing.MuscleGroup, existing.Description, existing.DeletedAt = spec.muscleGroup, &description, nil
	existing.Intensity, existing.SpeedKph, existing.VideoURL = nil, nil, nil
	return &existing, nil
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
