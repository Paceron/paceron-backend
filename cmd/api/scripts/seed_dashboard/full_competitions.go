package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgtype"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils/gametime"
	"simple-arq-golang/cmd/api/utils/routematch"
)

const fullCompetitionMetricFeedbackKey = "competition_finished_metric_tie"

type fullCompetitionSpec struct {
	key, typeName, creatorKey string
	participants              []string
	metric                    *string
	route                     *string
	startOffset, endOffset    int
	maxAttempts               *int64
	status, rejectionReason   string
	finished                  bool
}

type fullCompetitionAttemptSpec struct {
	key, athleteKey, status string
	number                  int
	startedAt, endedAt      time.Time
	duration                time.Duration
	compliance              string
}

func (s *fullSeeder) seedCompetitions() error {
	if err := s.requireFullCompetitionPrerequisites(); err != nil {
		return err
	}

	competitionIDs := make(map[string]int64, 8)
	instanceIDs := make(map[string]int64)
	attemptIDs := make(map[string]int64)
	feedbackIDs := make(map[string]int64)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		service := services.NewCompetitionService(daos.NewCompetitionDao(tx), nil, nil)
		for _, spec := range s.fullCompetitionSpecs() {
			row, created, err := s.ensureFullCompetition(tx, service, spec)
			if err != nil {
				return fmt.Errorf("competencia %s: %w", spec.key, err)
			}
			competitionIDs[spec.key] = row.ID
			if row.Type == competition.TypeRoute {
				if row.RouteSessionInstanceID == nil || *row.RouteSessionInstanceID <= 0 {
					return fmt.Errorf("competencia %s sin instancia compartida", spec.key)
				}
				if err := validateFullCompetitionSharedInstance(tx, row); err != nil {
					return fmt.Errorf("instancia compartida %s: %w", spec.key, err)
				}
				instanceIDs[spec.key+"_shared"] = *row.RouteSessionInstanceID
				for _, attempt := range fullCompetitionAttempts(spec, s.now) {
					instanceID, attemptID, feedbackID, err := s.ensureFullCompetitionAttempt(tx, row, spec, attempt, instanceIDs)
					if err != nil {
						return fmt.Errorf("intento %s: %w", attempt.key, err)
					}
					attemptIDs[attempt.key] = attemptID
					instanceIDs[attempt.key] = instanceID
					if feedbackID > 0 {
						feedbackIDs[attempt.key] = feedbackID
					}
				}
			}

			if spec.key == "finished_metric_tie" {
				ids, err := s.ensureFullCompetitionTieFeedback(tx)
				if err != nil {
					return fmt.Errorf("feedback de empate métrico: %w", err)
				}
				for key, id := range ids {
					feedbackIDs[key] = id
				}
			}
			if spec.rejectionReason != "" && created {
				reason := spec.rejectionReason
				if _, err := service.Reject(nil, s.ids.users["trainer_1"], row.ID, &reason); err != nil {
					return fmt.Errorf("rechazar competencia %s: %w", spec.key, err)
				}
			} else if spec.finished && created {
				if _, _, err := service.Close(nil, s.ids.users["trainer_1"], row.ID); err != nil {
					return fmt.Errorf("cerrar competencia %s: %w", spec.key, err)
				}
			}
			if err := validateFullCompetitionFinalState(tx, row.ID, spec); err != nil {
				return fmt.Errorf("validar competencia %s: %w", spec.key, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sembrar competencias del perfil full: %w", err)
	}
	mergeFullActivityIDs(s.ids.competitions, competitionIDs)
	mergeFullActivityIDs(s.ids.competitionInstances, instanceIDs)
	mergeFullActivityIDs(s.ids.competitionAttempts, attemptIDs)
	mergeFullActivityIDs(s.ids.workoutFeedback, feedbackIDs)
	return nil
}

func (s *fullSeeder) requireFullCompetitionPrerequisites() error {
	for _, key := range []string{"trainer_1", "r1", "r2", "r3", "r4", "r5", "r6", "r7"} {
		if s.ids.users[key] <= 0 {
			return fmt.Errorf("seedCompetitions requiere seedIdentity: falta %s", key)
		}
	}
	if s.ids.teams["team_a"] <= 0 || s.ids.groups["group_a_main"] <= 0 {
		return fmt.Errorf("seedCompetitions requiere seedTeams: falta Team A o su grupo principal")
	}
	if len(s.ids.workoutFeedback) == 0 || s.ids.sessionInstances["today_a_main"] <= 0 || s.ids.exercises["full_speed_distance"] <= 0 {
		return fmt.Errorf("seedCompetitions requiere seedActivity y seedCatalogCalendar")
	}
	var team dbs.Team
	if err := s.db.First(&team, s.ids.teams["team_a"]).Error; err != nil {
		return fmt.Errorf("seedCompetitions requiere Team A: %w", err)
	}
	if team.OwnerID != s.ids.users["trainer_1"] || !containsFullSeedMarker(team.Description) {
		return fmt.Errorf("seedCompetitions rechaza el Team A ajeno al perfil full")
	}
	var group dbs.Group
	if err := s.db.First(&group, s.ids.groups["group_a_main"]).Error; err != nil {
		return fmt.Errorf("seedCompetitions requiere el grupo principal de A: %w", err)
	}
	if group.TeamID != team.ID || !containsFullSeedMarker(group.Description) {
		return fmt.Errorf("seedCompetitions rechaza el grupo principal ajeno al perfil full")
	}
	var unseen int64
	if err := s.db.Model(&dbs.GamificationUnseen{}).
		Where("user_id = ? AND section IN ?", s.ids.users["r4"], []string{"streaks", "achievements"}).Count(&unseen).Error; err != nil {
		return fmt.Errorf("verificar seedGamification: %w", err)
	}
	if unseen == 0 {
		return fmt.Errorf("seedCompetitions requiere seedGamification")
	}
	return nil
}

func (s *fullSeeder) fullCompetitionSpecs() []fullCompetitionSpec {
	activeRoute := fullCompetitionRouteJSON(-64.1888)
	finishedRoute := fullCompetitionRouteJSON(-64.1800)
	emptyRoute := fullCompetitionRouteJSON(-64.1750)
	exhaustedRoute := fullCompetitionRouteJSON(-64.1700)
	maxAttempts := int64(3)
	reason := "La propuesta no corresponde al objetivo del equipo."
	return []fullCompetitionSpec{
		{key: "pending_metric", typeName: competition.TypeMetric, creatorKey: "r3", participants: []string{"r1", "r2"}, metric: stringPointer("duration"), startOffset: -1, endOffset: 14, status: competition.StatusPendingApproval},
		{key: "active_metric", typeName: competition.TypeMetric, creatorKey: "trainer_1", participants: []string{"r1", "r4"}, metric: stringPointer("distance"), startOffset: -21, endOffset: 7, status: competition.StatusActive},
		{key: "active_route", typeName: competition.TypeRoute, creatorKey: "trainer_1", participants: []string{"r1", "r2"}, route: &activeRoute, startOffset: -1, endOffset: 7, maxAttempts: &maxAttempts, status: competition.StatusActive},
		{key: "finished_metric_tie", typeName: competition.TypeMetric, creatorKey: "trainer_1", participants: []string{"r6", "r7"}, metric: stringPointer("distance"), startOffset: -21, endOffset: -3, status: competition.StatusFinished, finished: true},
		{key: "finished_route_winner", typeName: competition.TypeRoute, creatorKey: "trainer_1", participants: []string{"r6", "r7"}, route: &finishedRoute, startOffset: -1, endOffset: 7, status: competition.StatusFinished, finished: true},
		{key: "finished_route_empty", typeName: competition.TypeRoute, creatorKey: "trainer_1", participants: []string{"r4", "r5"}, route: &emptyRoute, startOffset: -1, endOffset: 7, status: competition.StatusFinished, finished: true},
		{key: "rejected_metric", typeName: competition.TypeMetric, creatorKey: "r2", participants: []string{"r1", "r3"}, metric: stringPointer("pace"), startOffset: -1, endOffset: 14, status: competition.StatusRejected, rejectionReason: reason},
		{key: "active_route_exhausted", typeName: competition.TypeRoute, creatorKey: "trainer_1", participants: []string{"r3", "r5"}, route: &exhaustedRoute, startOffset: -1, endOffset: 7, maxAttempts: &maxAttempts, status: competition.StatusActive},
	}
}

func fullCompetitionRouteJSON(startLng float64) string {
	return fmt.Sprintf(`{"type":"LineString","coordinates":[[%.4f,-31.4201],[%.4f,-31.4201]]}`, startLng, startLng+0.005)
}

func (s *fullSeeder) ensureFullCompetition(tx *gorm.DB, service services.CompetitionServiceInterface, spec fullCompetitionSpec) (*dbs.Competition, bool, error) {
	startAt := s.now.AddDate(0, 0, spec.startOffset)
	endAt := s.now.AddDate(0, 0, spec.endOffset)
	var candidates []dbs.Competition
	query := tx.Where("team_id = ? AND group_id IS NULL AND type = ? AND created_by_user_id = ? AND created_by_role = ?",
		s.ids.teams["team_a"], spec.typeName, s.ids.users[spec.creatorKey], fullCompetitionCreatorRole(spec.creatorKey))
	if spec.typeName == competition.TypeMetric {
		query = query.Where("metric = ? AND route_geojson IS NULL AND max_attempts IS NULL", *spec.metric)
	} else {
		query = query.Where("route_geojson = ? AND metric IS NULL AND acceptance_radius_meters = ? AND max_attempts IS NOT DISTINCT FROM ?", *spec.route, 50.0, spec.maxAttempts)
	}
	if err := query.Find(&candidates).Error; err != nil {
		return nil, false, fmt.Errorf("buscar clave estable: %w", err)
	}
	var existing dbs.Competition
	created := true
	for i := range candidates {
		matches, err := s.validateFullCompetitionParticipants(tx, candidates[i].ID, spec)
		if err != nil {
			return nil, false, err
		}
		if !matches {
			continue
		}
		if !created {
			return nil, false, fmt.Errorf("clave estable ambigua: hay más de una competencia con los participantes esperados")
		}
		existing = candidates[i]
		created = false
	}
	if created {
		participants := make([]int64, 0, len(spec.participants))
		for _, key := range spec.participants {
			participants = append(participants, s.ids.users[key])
		}
		maxAttempts := spec.maxAttempts
		request := competition.CreateCompetitionRequest{
			Type: spec.typeName, Metric: spec.metric, StartAt: startAt, EndAt: &endAt,
			MaxAttempts: maxAttempts, ParticipantIDs: participants,
		}
		if spec.typeName == competition.TypeRoute {
			radius := 50.0
			request.RouteGeoJSON = spec.route
			request.AcceptanceRadiusMeters = &radius
		}
		response, err := service.Create(nil, s.ids.users[spec.creatorKey], s.ids.teams["team_a"], request)
		if err != nil {
			return nil, false, fmt.Errorf("crear: %w", err)
		}
		if response.Status != fullCompetitionCreatedStatus(spec) {
			return nil, false, fmt.Errorf("estado al crear %s, esperado %s", response.Status, fullCompetitionCreatedStatus(spec))
		}
		if err := tx.First(&existing, response.ID).Error; err != nil {
			return nil, false, fmt.Errorf("leer competencia creada: %w", err)
		}
	} else if existing.Status != fullCompetitionCreatedStatus(spec) && existing.Status != spec.status {
		return nil, false, fmt.Errorf("estado existente %q, esperado %q", existing.Status, spec.status)
	}
	if err := s.validateFullCompetitionRow(tx, &existing, spec); err != nil {
		return nil, false, err
	}
	return &existing, created, nil
}

func fullCompetitionCreatorRole(key string) string {
	if key == "trainer_1" {
		return competition.CreatorRoleTrainer
	}
	return competition.CreatorRoleRunner
}

func fullCompetitionCreatedStatus(spec fullCompetitionSpec) string {
	if fullCompetitionCreatorRole(spec.creatorKey) == competition.CreatorRoleTrainer {
		return competition.StatusActive
	}
	return competition.StatusPendingApproval
}

func (s *fullSeeder) validateFullCompetitionRow(tx *gorm.DB, row *dbs.Competition, spec fullCompetitionSpec) error {
	if row.TeamID != s.ids.teams["team_a"] || row.CreatedByUserID != s.ids.users[spec.creatorKey] || row.CreatedByRole != fullCompetitionCreatorRole(spec.creatorKey) ||
		row.Type != spec.typeName || row.GroupID != nil || row.StartAt.IsZero() || row.EndAt == nil || !row.EndAt.After(row.StartAt) {
		return fmt.Errorf("la competencia existente no coincide con su clave full")
	}
	if row.Type == competition.TypeMetric {
		if !sameStringPointer(row.Metric, spec.metric) || row.RouteGeoJSON != nil || row.AcceptanceRadiusMeters != nil || row.MaxAttempts != nil {
			return fmt.Errorf("la competencia metric existente no coincide con el perfil full")
		}
	} else if row.RouteGeoJSON == nil || *row.RouteGeoJSON != *spec.route || row.AcceptanceRadiusMeters == nil || *row.AcceptanceRadiusMeters != 50 || !sameInt64Pointer(row.MaxAttempts, spec.maxAttempts) {
		return fmt.Errorf("la competencia route existente no coincide con el perfil full")
	}
	matches, err := s.validateFullCompetitionParticipants(tx, row.ID, spec)
	if err != nil {
		return err
	}
	if !matches {
		return fmt.Errorf("los participantes no coinciden con la clave full")
	}
	return nil
}

func (s *fullSeeder) validateFullCompetitionParticipants(tx *gorm.DB, competitionID int64, spec fullCompetitionSpec) (bool, error) {
	var actual []int64
	if err := tx.Model(&dbs.CompetitionParticipant{}).Where("competition_id = ?", competitionID).Order("user_id").Pluck("user_id", &actual).Error; err != nil {
		return false, err
	}
	want := make([]int64, 0, len(spec.participants))
	for _, key := range spec.participants {
		want = append(want, s.ids.users[key])
	}
	sortInt64s(want)
	return equalInt64Slices(actual, want), nil
}

func validateFullCompetitionFinalState(tx *gorm.DB, id int64, spec fullCompetitionSpec) error {
	var row dbs.Competition
	if err := tx.First(&row, id).Error; err != nil {
		return err
	}
	if row.Status != spec.status {
		return fmt.Errorf("estado final %q, esperado %q", row.Status, spec.status)
	}
	if spec.rejectionReason != "" && (row.RejectionReason == nil || *row.RejectionReason != spec.rejectionReason) {
		return fmt.Errorf("motivo de rechazo no coincide")
	}
	return nil
}

func fullCompetitionAttempts(spec fullCompetitionSpec, now time.Time) []fullCompetitionAttemptSpec {
	if spec.key == "active_route" {
		return []fullCompetitionAttemptSpec{
			fullCompetitionAttempt("active_route_r1_1", "r1", 1, runnersession.RunnerSessionStatusFinished, now, 20*time.Minute, "full"),
			fullCompetitionAttempt("active_route_r2_1", "r2", 1, runnersession.RunnerSessionStatusFinished, now, 30*time.Minute, "partial"),
			fullCompetitionAttempt("active_route_r2_2", "r2", 2, runnersession.RunnerSessionStatusWip, now, 0, ""),
		}
	}
	if spec.key == "finished_route_winner" {
		return []fullCompetitionAttemptSpec{
			fullCompetitionAttempt("finished_route_winner_r6_1", "r6", 1, runnersession.RunnerSessionStatusFinished, now, 5*time.Minute, "full"),
			fullCompetitionAttempt("finished_route_winner_r7_1", "r7", 1, runnersession.RunnerSessionStatusFinished, now, 7*time.Minute, "full"),
		}
	}
	if spec.key == "active_route_exhausted" {
		return []fullCompetitionAttemptSpec{
			fullCompetitionAttempt("active_route_exhausted_r3_1", "r3", 1, runnersession.RunnerSessionStatusFinished, now, 6*time.Minute, "full"),
			fullCompetitionAttempt("active_route_exhausted_r3_2", "r3", 2, runnersession.RunnerSessionStatusFinished, now, 8*time.Minute, "partial"),
			fullCompetitionAttempt("active_route_exhausted_r3_3", "r3", 3, runnersession.RunnerSessionStatusFinished, now, 9*time.Minute, "partial"),
		}
	}
	return nil
}

func fullCompetitionAttempt(key, athlete string, number int, status string, now time.Time, duration time.Duration, compliance string) fullCompetitionAttemptSpec {
	start := fullCompetitionClock(now.AddDate(0, 0, -2), 9, number)
	if status == runnersession.RunnerSessionStatusWip {
		start = fullCompetitionClock(now, 8, number)
		return fullCompetitionAttemptSpec{key: key, athleteKey: athlete, number: number, status: status, startedAt: start}
	}
	end := start.Add(duration)
	return fullCompetitionAttemptSpec{key: key, athleteKey: athlete, number: number, status: status, startedAt: start, endedAt: end, duration: duration, compliance: compliance}
}

func (s *fullSeeder) ensureFullCompetitionAttempt(tx *gorm.DB, row *dbs.Competition, comp fullCompetitionSpec, attempt fullCompetitionAttemptSpec, instanceIDs map[string]int64) (int64, int64, int64, error) {
	instanceID := instanceIDs[comp.key+"_shared"]
	if attempt.number > 1 {
		name := fmt.Sprintf("Competencia ruta %d (intento %d)", row.ID, attempt.number)
		instance, err := ensureFullCompetitionAttemptInstance(tx, name, s.now)
		if err != nil {
			return 0, 0, 0, err
		}
		instanceID = instance
		instanceIDs[attempt.key] = instanceID
	}
	var existing dbs.RunnerSession
	err := tx.Where("competition_id = ? AND athlete_user_id = ? AND session_instance_id = ?", row.ID, s.ids.users[attempt.athleteKey], instanceID).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var endAt *time.Time
		if attempt.status == runnersession.RunnerSessionStatusFinished {
			endAt = timePointer(attempt.endedAt)
		}
		competitionID := row.ID
		existing = dbs.RunnerSession{
			SessionInstanceID: instanceID, AthleteUserID: s.ids.users[attempt.athleteKey],
			Status: attempt.status, StartDate: attempt.startedAt, EndDate: endAt, CompetitionID: &competitionID,
			CreatedAt: s.now, UpdatedAt: s.now,
		}
		if err := tx.Create(&existing).Error; err != nil {
			return 0, 0, 0, err
		}
	} else if err != nil {
		return 0, 0, 0, err
	} else if existing.Status != attempt.status || !existing.StartDate.Equal(attempt.startedAt) || existing.CompetitionID == nil || *existing.CompetitionID != row.ID {
		return 0, 0, 0, fmt.Errorf("runner_session existente distinto")
	}
	if attempt.status == runnersession.RunnerSessionStatusFinished && (existing.EndDate == nil || !existing.EndDate.Equal(attempt.endedAt)) {
		return 0, 0, 0, fmt.Errorf("runner_session finished sin end_date estable")
	}
	if attempt.status == runnersession.RunnerSessionStatusWip && existing.EndDate != nil {
		return 0, 0, 0, fmt.Errorf("runner_session wip tiene end_date")
	}
	if attempt.status != runnersession.RunnerSessionStatusFinished {
		return instanceID, existing.ID, 0, nil
	}
	feedbackID, err := s.ensureFullCompetitionRouteEvidence(tx, row, comp, attempt, instanceID)
	if err != nil {
		return 0, 0, 0, err
	}
	return instanceID, existing.ID, feedbackID, nil
}

func validateFullCompetitionSharedInstance(tx *gorm.DB, comp *dbs.Competition) error {
	var instance dbs.SessionInstance
	if err := tx.First(&instance, *comp.RouteSessionInstanceID).Error; err != nil {
		return err
	}
	if instance.Name != fmt.Sprintf("Competencia ruta %d", comp.ID) || instance.SourceSessionID != nil {
		return fmt.Errorf("la instancia compartida no tiene la identidad esperada")
	}
	var links []dbs.SessionExerciseInstance
	if err := tx.Where("session_instance_id = ?", instance.ID).Find(&links).Error; err != nil {
		return err
	}
	if len(links) != 1 || links[0].Role != "main" || links[0].RepeatCount != 1 || links[0].RestMinutes != 0 {
		return fmt.Errorf("la instancia compartida no tiene el ejercicio sintético esperado")
	}
	var exercise dbs.ExerciseInstance
	if err := tx.First(&exercise, links[0].ExerciseInstanceID).Error; err != nil {
		return err
	}
	if exercise.Name != "Corrida" || exercise.Kind != "velocidad_distancia" || exercise.SourceExerciseID != nil {
		return fmt.Errorf("el ejercicio compartido no es sintético")
	}
	return nil
}

func ensureFullCompetitionAttemptInstance(tx *gorm.DB, name string, now time.Time) (int64, error) {
	var instances []dbs.SessionInstance
	if err := tx.Where("name = ?", name).Find(&instances).Error; err != nil {
		return 0, err
	}
	if len(instances) > 1 {
		return 0, fmt.Errorf("hay instancias duplicadas con clave %q", name)
	}
	var instance dbs.SessionInstance
	if len(instances) == 1 {
		instance = instances[0]
		if !containsFullSeedMarker(pointerValue(instance.Description)) {
			return 0, fmt.Errorf("la instancia %q existe sin marker full", name)
		}
	} else {
		description := fullSeedMarker + " Instancia de intento de competencia."
		instance = dbs.SessionInstance{Name: name, Description: &description, CreatedAt: now}
		if err := tx.Create(&instance).Error; err != nil {
			return 0, err
		}
	}
	var links []dbs.SessionExerciseInstance
	if err := tx.Where("session_instance_id = ?", instance.ID).Find(&links).Error; err != nil {
		return 0, err
	}
	if len(links) == 0 {
		description := fullSeedMarker + " Ejercicio sintético para intento de competencia."
		exercise := dbs.ExerciseInstance{Name: "Corrida " + fullSeedMarker, Description: &description, Kind: "velocidad_distancia", CreatedAt: now}
		if err := tx.Create(&exercise).Error; err != nil {
			return 0, err
		}
		link := dbs.SessionExerciseInstance{SessionInstanceID: instance.ID, ExerciseInstanceID: exercise.ID, Role: "main", RepeatCount: 1, RestMinutes: 0}
		if err := tx.Create(&link).Error; err != nil {
			return 0, err
		}
		links = append(links, link)
	}
	if len(links) != 1 || links[0].Role != "main" || links[0].RepeatCount != 1 || links[0].RestMinutes != 0 {
		return 0, fmt.Errorf("la instancia %q no tiene el link sintético esperado", name)
	}
	var exercise dbs.ExerciseInstance
	if err := tx.First(&exercise, links[0].ExerciseInstanceID).Error; err != nil {
		return 0, err
	}
	if exercise.Name != "Corrida "+fullSeedMarker || exercise.Kind != "velocidad_distancia" || !containsFullSeedMarker(pointerValue(exercise.Description)) || exercise.SourceExerciseID != nil {
		return 0, fmt.Errorf("el ejercicio de la instancia %q no tiene identidad full", name)
	}
	return instance.ID, nil
}

func (s *fullSeeder) ensureFullCompetitionRouteEvidence(tx *gorm.DB, row *dbs.Competition, comp fullCompetitionSpec, attempt fullCompetitionAttemptSpec, instanceID int64) (int64, error) {
	var exerciseID int64
	if attempt.number == 1 {
		if row.RouteSessionInstanceID == nil {
			return 0, fmt.Errorf("falta instancia compartida")
		}
		instanceID = *row.RouteSessionInstanceID
	}
	var exercise struct{ ID int64 }
	if err := tx.Table("session_exercise_instances").Select("exercise_instance_id AS id").Where("session_instance_id = ?", instanceID).Take(&exercise).Error; err != nil {
		return 0, err
	}
	exerciseID = exercise.ID
	start := attempt.startedAt
	end := attempt.endedAt
	durationMS := attempt.duration.Milliseconds()
	distance := 500.0
	status := "completed"
	teamID := s.ids.teams["team_a"]
	feedback := dbs.WorkoutFeedback{
		TeamID: &teamID, AssignedSessionID: instanceID, AssignedExerciseID: exerciseID,
		AthleteUserID: s.ids.users[attempt.athleteKey], FeedbackOwnerUserID: s.ids.users[attempt.athleteKey],
		ReportSource: "corredor", SessionDate: fullDate(start), SetNumber: 1,
		StartedAt: &start, EndedAt: &end, DurationMs: &durationMS, DistanceMeters: &distance,
		CompletionStatus: &status, Annotations: stringPointer(fullSeedMarker + " Evidencia sintética de intento de competencia."),
	}
	feedback.MediaURLs.Status = pgtype.Null
	feedbackID, err := ensureFullCompetitionFeedback(tx, feedback)
	if err != nil {
		return 0, err
	}
	points := fullCompetitionGPSPoints(*comp.route, attempt.compliance)
	if err := ensureFullCompetitionGPSPoints(tx, feedbackID, instanceID, exerciseID, points, s.now); err != nil {
		return 0, err
	}
	return feedbackID, nil
}

func (s *fullSeeder) ensureFullCompetitionTieFeedback(tx *gorm.DB) (map[string]int64, error) {
	sourceID := s.ids.workoutFeedback["completed_r1_minus14_set1"]
	var source dbs.WorkoutFeedback
	if err := tx.First(&source, sourceID).Error; err != nil {
		return nil, fmt.Errorf("leer feedback full de referencia: %w", err)
	}
	ids := make(map[string]int64, 2)
	for _, athlete := range []string{"r6", "r7"} {
		teamID := s.ids.teams["team_a"]
		distance := 5000.0
		status := "completed"
		feedback := dbs.WorkoutFeedback{
			TeamID: &teamID, AssignedSessionID: source.AssignedSessionID, AssignedExerciseID: source.AssignedExerciseID,
			AthleteUserID: s.ids.users[athlete], FeedbackOwnerUserID: s.ids.users[athlete], ReportSource: "corredor",
			SessionDate: fullDate(s.now.AddDate(0, 0, -14)), SetNumber: 7, DistanceMeters: &distance,
			CompletionStatus: &status, Annotations: stringPointer(fullSeedMarker + " Dato de empate métrico de competencia."),
		}
		feedback.MediaURLs.Status = pgtype.Null
		id, err := ensureFullCompetitionFeedback(tx, feedback)
		if err != nil {
			return nil, err
		}
		ids[fullCompetitionMetricFeedbackKey+"_"+athlete] = id
	}
	return ids, nil
}

func ensureFullCompetitionFeedback(tx *gorm.DB, expected dbs.WorkoutFeedback) (int64, error) {
	var rows []dbs.WorkoutFeedback
	err := tx.Unscoped().Where("assigned_session_id = ? AND assigned_exercise_id = ? AND athlete_user_id = ? AND feedback_owner_user_id = ? AND set_number = ?",
		expected.AssignedSessionID, expected.AssignedExerciseID, expected.AthleteUserID, expected.FeedbackOwnerUserID, expected.SetNumber).Find(&rows).Error
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		if err := tx.Create(&expected).Error; err != nil {
			return 0, err
		}
		return expected.ID, nil
	}
	if len(rows) != 1 || rows[0].DeletedAt != nil || !containsFullSeedMarker(pointerValue(rows[0].Annotations)) ||
		rows[0].AssignedSessionID != expected.AssignedSessionID || rows[0].AssignedExerciseID != expected.AssignedExerciseID ||
		rows[0].AthleteUserID != expected.AthleteUserID || rows[0].FeedbackOwnerUserID != expected.FeedbackOwnerUserID ||
		!sameInt64Pointer(rows[0].TeamID, expected.TeamID) || rows[0].ReportSource != expected.ReportSource ||
		!sameFloat64Pointer(rows[0].DistanceMeters, expected.DistanceMeters) || !sameInt64Pointer(rows[0].DurationMs, expected.DurationMs) ||
		!sameTimePointer(rows[0].StartedAt, expected.StartedAt) || !sameTimePointer(rows[0].EndedAt, expected.EndedAt) ||
		!sameStringPointer(rows[0].CompletionStatus, expected.CompletionStatus) || rows[0].SessionDate.Format("2006-01-02") != expected.SessionDate.Format("2006-01-02") {
		return 0, fmt.Errorf("existe feedback distinto en la clave de competencia")
	}
	return rows[0].ID, nil
}

func ensureFullCompetitionGPSPoints(tx *gorm.DB, feedbackID, sessionID, exerciseID int64, expected []routematch.Point, now time.Time) error {
	var actual []dbs.WorkoutFeedbackPoint
	if err := tx.Where("feedback_id = ?", feedbackID).Order(`"order"`).Find(&actual).Error; err != nil {
		return err
	}
	if len(actual) == 0 {
		for i, point := range expected {
			row := dbs.WorkoutFeedbackPoint{
				FeedbackID: feedbackID, SessionInstanceID: sessionID, ExerciseInstanceID: exerciseID,
				Order: i, Latitude: point.Lat, Longitude: point.Lng, RecordedAt: now.Add(time.Duration(i) * time.Second),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("cantidad de puntos GPS %d, esperada %d", len(actual), len(expected))
	}
	for i := range expected {
		if actual[i].Order != i || actual[i].SessionInstanceID != sessionID || actual[i].ExerciseInstanceID != exerciseID ||
			actual[i].Latitude != expected[i].Lat || actual[i].Longitude != expected[i].Lng || !actual[i].RecordedAt.Equal(now.Add(time.Duration(i)*time.Second)) {
			return fmt.Errorf("punto GPS %d distinto al intento full", i)
		}
	}
	return nil
}

func fullCompetitionGPSPoints(rawRoute, compliance string) []routematch.Point {
	route, _ := routematch.ParseLineString(rawRoute)
	samples := routematch.SampleRoute(route)
	if compliance == "partial" && len(samples) > 1 {
		samples = samples[:(len(samples)+1)/2]
	}
	points := make([]routematch.Point, len(samples))
	for i, sample := range samples {
		points[i] = routematch.Point{Lat: sample.Lat + 0.00003, Lng: sample.Lng}
	}
	return points
}

func fullCompetitionClock(value time.Time, hour, minute int) time.Time {
	year, month, day := fullDate(value).Date()
	return time.Date(year, month, day, hour, minute, 0, 0, gametime.Location)
}

func sortInt64s(values []int64) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func equalInt64Slices(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
