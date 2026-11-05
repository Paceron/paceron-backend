package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/crypto"
	"simple-arq-golang/cmd/api/utils/gametime"
)

type fullCleanupState struct {
	users, teams, groups, plans, sessions, exercises       []int64
	calendarDays, calendarInstances                        []int64
	competitionIDs, competitionInstances                   []int64
	runnerIDs, activityFeedbackIDs, competitionFeedbackIDs []int64
	attendanceIDs, messageIDs, messageRecipientIDs         []int64
	exerciseInstances, subscriptionIDs, installmentIDs     []int64
	paymentIDs, teamInstallmentIDs                         []int64
	sellerConnectionIDs                                    []int64
	fullUserIDs                                            map[string]int64
	fullTeamIDs                                            map[string]int64
	fullGroupIDs                                           map[string]int64
	planByName                                             map[string]int64
	calendarInstanceByDate                                 map[string]int64
	anchor                                                 time.Time
}

func (s *fullSeeder) cleanupFull() error {
	if s.opts.dryRun {
		var state fullCleanupState
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			var err error
			state, err = collectFullCleanupState(tx)
			return err
		}); err != nil {
			return err
		}
		fmt.Printf("[dry-run] cleanup full: %d usuarios, %d equipos, %d filas de calendario, %d competencias\n", len(state.users), len(state.teams), len(state.calendarDays), len(state.competitionIDs))
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		state, err := collectFullCleanupState(tx)
		if err != nil {
			return err
		}
		steps := fullCleanupSteps(state)
		for _, step := range steps {
			result := tx.Exec("DELETE FROM "+step.Table+" WHERE "+step.Where, step.Args...)
			if result.Error != nil {
				return fmt.Errorf("cleanup full borrando %s: %w", step.Table, result.Error)
			}
			if result.RowsAffected > 0 {
				fmt.Printf("  %-32s %d\n", step.Table, result.RowsAffected)
			}
		}
		return nil
	})
}

func collectFullCleanupState(db *gorm.DB) (fullCleanupState, error) {
	state := fullCleanupState{
		fullUserIDs: make(map[string]int64), fullTeamIDs: make(map[string]int64),
		fullGroupIDs: make(map[string]int64), planByName: make(map[string]int64),
		calendarInstanceByDate: make(map[string]int64),
	}
	users, err := fullOwnedUserIDs(db)
	if err != nil {
		return state, err
	}
	state.fullUserIDs = users
	state.users = fullUserIDList(users)
	if len(users) == 0 {
		if err := ensureFullScopeEmpty(db); err != nil {
			return state, fmt.Errorf("no hay usuarios full y quedan artefactos sin propietario atribuible: %w", err)
		}
	}
	teamSpecs := []struct {
		key string
		id  int64
	}{
		{"team_a", fullTeamAID}, {"team_b", fullTeamBID}, {"team_c", fullTeamCID},
		{"team_d", fullTeamDID}, {"team_e", fullTeamEID}, {"team_f", fullTeamFID},
	}
	for _, spec := range teamSpecs {
		var row dbs.Team
		err := db.Select("id", "owner_id", "description").Where("id = ?", spec.id).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return state, err
		}
		if !containsFullSeedMarker(row.Description) {
			return state, fmt.Errorf("equipo reservado %d sin marker full; cleanup abortado", spec.id)
		}
		if len(users) > 0 && row.OwnerID != users["trainer_1"] && row.OwnerID != users["trainer_2"] {
			return state, fmt.Errorf("equipo reservado %d tiene owner ajeno; cleanup abortado", spec.id)
		}
		state.fullTeamIDs[spec.key] = row.ID
		state.teams = append(state.teams, row.ID)
	}
	groupSpecs := []struct {
		key      string
		id, team int64
	}{
		{"group_a_main", fullGroupAMainID, fullTeamAID}, {"group_a_secondary", fullGroupASecondaryID, fullTeamAID},
		{"group_b", fullGroupBID, fullTeamBID}, {"group_c", fullGroupCID, fullTeamCID},
		{"group_d", fullGroupDID, fullTeamDID}, {"group_e", fullGroupEID, fullTeamEID}, {"group_f", fullGroupFID, fullTeamFID},
	}
	for _, spec := range groupSpecs {
		var row dbs.Group
		err := db.Select("id", "team_id", "description").Where("id = ?", spec.id).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return state, err
		}
		if !containsFullSeedMarker(row.Description) || row.TeamID != spec.team {
			return state, fmt.Errorf("grupo reservado %d no coincide con el scope full; cleanup abortado", spec.id)
		}
		state.fullGroupIDs[spec.key] = row.ID
		state.groups = append(state.groups, row.ID)
	}
	if len(users) > 0 {
		for _, table := range []string{"training_plans", "sessions", "exercises"} {
			var ids []int64
			query := fmt.Sprintf("SELECT id FROM %s WHERE owner_id IN ? AND description LIKE ?", table)
			if err := db.Raw(query, state.users, "%"+fullSeedMarker+"%").Scan(&ids).Error; err != nil {
				return state, fmt.Errorf("listar catálogo full en %s: %w", table, err)
			}
			switch table {
			case "training_plans":
				state.plans = ids
			case "sessions":
				state.sessions = ids
			case "exercises":
				state.exercises = ids
			}
		}
		for _, spec := range fullTrainingPlanSpecs() {
			var rows []dbs.TrainingPlan
			if err := db.Where("owner_id = ? AND name = ? AND description LIKE ?", users["trainer_1"], spec.name, "%"+fullSeedMarker+"%").Find(&rows).Error; err != nil {
				return state, err
			}
			if len(rows) > 1 {
				return state, fmt.Errorf("plan full %q ambiguo; cleanup abortado", spec.key)
			}
			if len(rows) == 1 {
				state.planByName[spec.key] = rows[0].ID
			}
		}
		if err := validateFullCatalogReferences(db, &state); err != nil {
			return state, err
		}
		if err := collectFullCalendarRows(db, &state); err != nil {
			return state, err
		}
		if err := collectFullCompetitionRows(db, &state); err != nil {
			return state, err
		}
		if err := collectFullActivityRows(db, &state); err != nil {
			return state, err
		}
		if err := collectFullMessages(db, &state); err != nil {
			return state, err
		}
		if err := collectFullPayments(db, &state); err != nil {
			return state, err
		}
		if err := collectFullSellerConnection(db, &state); err != nil {
			return state, err
		}
		if err := ensureNoUnownedFullReferences(db, &state); err != nil {
			return state, err
		}
	}
	return state, nil
}

func ensureNoUnownedFullReferences(db *gorm.DB, state *fullCleanupState) error {
	count := func(query *gorm.DB, label string) error {
		var total int64
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		if total > 0 {
			return fmt.Errorf("hay %d filas full sin selector de cleanup verificable en %s", total, label)
		}
		return nil
	}
	checkOwnedIDs := func(query *gorm.DB, table string, ids []int64) error {
		if len(ids) > 0 {
			query = query.Where("id NOT IN ?", ids)
		}
		return count(query, table)
	}
	if err := checkOwnedIDs(db.Model(&dbs.RunnerSession{}).Where("athlete_user_id IN ?", state.users), "runner_session", state.runnerIDs); err != nil {
		return err
	}
	if err := checkOwnedIDs(db.Unscoped().Model(&dbs.WorkoutFeedback{}).Where("(athlete_user_id IN ? OR feedback_owner_user_id IN ?)", state.users, state.users), "workout_feedback", uniqueIDs(append(append([]int64{}, state.activityFeedbackIDs...), state.competitionFeedbackIDs...))); err != nil {
		return err
	}
	if err := checkOwnedIDs(db.Model(&dbs.Attendance{}).Where("(user_id IN ? OR registered_by_user_id IN ?)", state.users, state.users), "attendances", state.attendanceIDs); err != nil {
		return err
	}
	if err := checkOwnedIDs(db.Model(&dbs.SessionMessage{}).Where("sender_user_id IN ?", state.users), "session_messages", state.messageIDs); err != nil {
		return err
	}
	if err := count(db.Model(&dbs.PushToken{}).Where("user_id IN ? AND token NOT LIKE ?", state.users, "ExponentPushToken[seed-full-%"), "push_tokens"); err != nil {
		return err
	}
	if err := checkOwnedIDs(db.Model(&dbs.Payment{}).Where("(user_id IN ? OR seller_user_id IN ?)", state.users, state.users), "payments", state.paymentIDs); err != nil {
		return err
	}
	if err := checkOwnedIDs(db.Model(&dbs.Installment{}).Where("user_id IN ?", state.users), "installments", state.installmentIDs); err != nil {
		return err
	}
	teamUsers := db.Model(&dbs.TeamUser{}).Where("user_id IN ?", state.users)
	invitations := db.Model(&dbs.Invitation{}).Where("(inviter_id IN ? OR invitee_id IN ?)", state.users, state.users)
	joinRequests := db.Model(&dbs.JoinRequest{}).Where("runner_id IN ?", state.users)
	if len(state.teams) > 0 {
		teamUsers = teamUsers.Where("team_id NOT IN ?", state.teams)
		invitations = invitations.Where("team_id NOT IN ?", state.teams)
		joinRequests = joinRequests.Where("team_id NOT IN ?", state.teams)
	}
	if err := count(teamUsers, "team_users fuera de equipos reservados"); err != nil {
		return err
	}
	if err := count(invitations, "invitations fuera de equipos reservados"); err != nil {
		return err
	}
	if err := count(joinRequests, "join_requests fuera de equipos reservados"); err != nil {
		return err
	}
	groupUsers := db.Model(&dbs.GroupUser{}).Where("user_id IN ?", state.users)
	if len(state.groups) > 0 {
		groupUsers = groupUsers.Where("group_id NOT IN ?", state.groups)
	}
	if err := count(groupUsers, "group_users fuera de grupos reservados"); err != nil {
		return err
	}
	participants := db.Model(&dbs.CompetitionParticipant{}).Where("user_id IN ?", state.users)
	winners := db.Model(&dbs.CompetitionWinner{}).Where("user_id IN ?", state.users)
	stamps := db.Model(&dbs.GamificationStamp{}).Where("user_id IN ?", state.users)
	if len(state.competitionIDs) > 0 {
		participants = participants.Where("competition_id NOT IN ?", state.competitionIDs)
		winners = winners.Where("competition_id NOT IN ?", state.competitionIDs)
		stamps = stamps.Where("type <> 'competition' OR source_id NOT IN ?", state.competitionIDs)
	}
	if err := count(participants, "competition_participant ajeno"); err != nil {
		return err
	}
	if err := count(winners, "competition_winner ajeno"); err != nil {
		return err
	}
	if err := count(stamps, "gamification_stamp ajena"); err != nil {
		return err
	}
	knownAchievementWhere := "user_id = ? AND achievement_id IN ?"
	knownAchievementArgs := []any{state.fullUserIDs["r4"], []string{"first_session", "streak_3", "sessions_5", "streak_7", "team_first"}}
	if err := count(db.Model(&dbs.UserAchievementProgress{}).Where("user_id IN ? AND NOT ("+knownAchievementWhere+")", append([]any{state.users}, knownAchievementArgs...)...), "user_achievement_progress ajeno"); err != nil {
		return err
	}
	knownUnseen := "(user_id = ? AND section IN ? AND team_id IS NULL) OR (team_id = ? AND section = 'competitions')"
	knownUnseenArgs := []any{state.fullUserIDs["r4"], []string{"streaks", "achievements"}, state.fullTeamIDs["team_a"]}
	if err := count(db.Model(&dbs.GamificationUnseen{}).Where("user_id IN ? AND NOT ("+knownUnseen+")", append([]any{state.users}, knownUnseenArgs...)...), "gamification_unseen ajena"); err != nil {
		return err
	}
	if len(state.installmentIDs) > 0 {
		unknownPaymentLinks := db.Model(&dbs.Payment{}).Where("installment_id IN ?", state.installmentIDs)
		if len(state.paymentIDs) > 0 {
			unknownPaymentLinks = unknownPaymentLinks.Where("id NOT IN ?", state.paymentIDs)
		}
		if err := count(unknownPaymentLinks, "payment enlazado a ledger full"); err != nil {
			return err
		}
	}
	return nil
}

func validateFullCatalogReferences(db *gorm.DB, state *fullCleanupState) error {
	if len(state.sessions) > 0 {
		plans := db.Model(&dbs.PlanDay{}).Where("session_id IN ?", state.sessions)
		if len(state.plans) > 0 {
			plans = plans.Where("plan_id NOT IN ?", state.plans)
		}
		var count int64
		if err := plans.Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("sesiones full referenciadas por plan_days ajenos; cleanup abortado")
		}
	}
	if len(state.exercises) > 0 {
		sessions := db.Model(&dbs.SessionExercise{}).Where("exercise_id IN ?", state.exercises)
		if len(state.sessions) > 0 {
			sessions = sessions.Where("session_id NOT IN ?", state.sessions)
		}
		var count int64
		if err := sessions.Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("ejercicios full referenciados por sesiones ajenas; cleanup abortado")
		}
	}
	return nil
}

func collectFullCalendarRows(db *gorm.DB, state *fullCleanupState) error {
	if id := state.planByName["full_21_days"]; id > 0 {
		var dates []time.Time
		if err := db.Model(&dbs.GroupCalendarDay{}).Where("group_id = ? AND source_plan_id = ?", fullGroupAMainID, id).Order("date").Pluck("date", &dates).Error; err != nil {
			return err
		}
		if len(dates) > 0 {
			state.anchor = fullDBDate(dates[0])
		}
	}
	groupIDs := fullGroupIDList(state.fullGroupIDs)
	if len(groupIDs) == 0 {
		return nil
	}
	addPlanRange := func(planKey, groupKey string, start, days int) error {
		planID := state.planByName[planKey]
		groupID := state.fullGroupIDs[groupKey]
		if planID <= 0 || groupID <= 0 || state.anchor.IsZero() {
			return nil
		}
		from := state.anchor.AddDate(0, 0, start)
		to := from.AddDate(0, 0, days-1)
		var rows []dbs.GroupCalendarDay
		if err := db.Where("group_id = ? AND source_plan_id = ? AND date BETWEEN ? AND ?", groupID, planID, from, to).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			state.calendarDays = append(state.calendarDays, row.ID)
			if row.SessionInstanceID != nil {
				state.calendarInstanceByDate[fullCalendarKey(groupID, row.Date)] = *row.SessionInstanceID
				state.calendarInstances = append(state.calendarInstances, *row.SessionInstanceID)
			}
		}
		return nil
	}
	if !state.anchor.IsZero() {
		if err := addPlanRange("full_21_days", "group_a_main", 0, 21); err != nil {
			return err
		}
		if err := addPlanRange("full_14_days", "group_a_main", 22, 14); err != nil {
			return err
		}
		if err := addPlanRange(fullStreakPlanKey, "group_a_secondary", 5, 9); err != nil {
			return err
		}
		if err := addPlanRange("full_14_days", "group_a_secondary", 22, 1); err != nil {
			return err
		}
		if err := collectFullExtraCalendarDays(db, state); err != nil {
			return err
		}
	}
	state.calendarDays = uniqueIDs(state.calendarDays)
	state.calendarInstances = uniqueIDs(state.calendarInstances)
	reservedDays := db.Model(&dbs.GroupCalendarDay{}).Where("group_id IN ?", groupIDs)
	if len(state.calendarDays) > 0 {
		reservedDays = reservedDays.Where("id NOT IN ?", state.calendarDays)
	}
	var unselectedReservedDays int64
	if err := reservedDays.Count(&unselectedReservedDays).Error; err != nil {
		return err
	}
	if unselectedReservedDays > 0 {
		return fmt.Errorf("hay días no atribuibles dentro de grupos reservados; cleanup abortado")
	}
	if len(state.calendarInstances) == 0 && (len(state.plans) > 0 || len(state.sessions) > 0) {
		return nil
	}
	if len(state.calendarInstances) > 0 {
		var foreignDays int64
		if err := db.Model(&dbs.GroupCalendarDay{}).
			Where("session_instance_id IN ? AND id NOT IN ?", state.calendarInstances, state.calendarDays).Count(&foreignDays).Error; err != nil {
			return err
		}
		if foreignDays > 0 {
			return fmt.Errorf("instancias full referenciadas por días fuera del scope reservado; cleanup abortado")
		}
	}
	if len(state.plans) > 0 {
		var unselected int64
		query := db.Model(&dbs.GroupCalendarDay{}).Where("source_plan_id IN ?", state.plans)
		if len(state.calendarDays) > 0 {
			query = query.Where("id NOT IN ?", state.calendarDays)
		}
		if err := query.Count(&unselected).Error; err != nil {
			return err
		}
		if unselected > 0 {
			return fmt.Errorf("hay días que referencian planes full fuera de sus fechas/grupos ancla; cleanup abortado")
		}
	}
	return nil
}

func collectFullExtraCalendarDays(db *gorm.DB, state *fullCleanupState) error {
	if state.anchor.IsZero() {
		return nil
	}
	var sessionRows []dbs.Session
	if err := db.Where("id IN ?", state.sessions).Find(&sessionRows).Error; err != nil {
		return err
	}
	fullIntervalID, asyncID := int64(0), int64(0)
	for _, row := range sessionRows {
		if row.Name == "Semilla Full · Intervalos" {
			fullIntervalID = row.ID
		}
		if row.Name == "Semilla Full · Rodaje asincrónico B" {
			asyncID = row.ID
		}
	}
	if fullIntervalID == 0 {
		return nil
	}
	checks := []struct {
		groupID   int64
		date      time.Time
		predicate string
		args      []any
	}{
		{fullGroupASecondaryID, state.anchor.AddDate(0, 0, 14), "source_plan_id IS NULL", nil},
		{fullGroupAMainID, state.anchor.AddDate(0, 0, 21), "presencial_opened_at IS NOT NULL AND presencial_closed_at IS NULL", nil},
		{fullGroupASecondaryID, state.anchor.AddDate(0, 0, 21), "true", nil},
		{fullGroupFID, state.anchor.AddDate(0, 0, 21), "presencial_closed_at IS NOT NULL", nil},
		{fullGroupBID, state.anchor.AddDate(0, 0, 21), "true", nil},
	}
	for _, check := range checks {
		var day dbs.GroupCalendarDay
		err := db.Where("group_id = ? AND date = ?", check.groupID, check.date).Take(&day).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if day.SessionInstanceID == nil {
			continue
		}
		var instance dbs.SessionInstance
		if err := db.First(&instance, *day.SessionInstanceID).Error; err != nil {
			return err
		}
		if instance.SourceSessionID == nil || (*instance.SourceSessionID != fullIntervalID && (*instance.SourceSessionID != asyncID || asyncID == 0)) || !containsFullSeedMarker(pointerValue(instance.Description)) {
			continue
		}
		if check.predicate != "true" {
			var count int64
			if err := db.Model(&dbs.GroupCalendarDay{}).Where("id = ? AND "+check.predicate, day.ID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return fmt.Errorf("día full %d no conserva la identidad esperada; cleanup abortado", day.ID)
			}
		}
		state.calendarDays = append(state.calendarDays, day.ID)
		state.calendarInstances = append(state.calendarInstances, *day.SessionInstanceID)
		state.calendarInstanceByDate[fullCalendarKey(day.GroupID, day.Date)] = *day.SessionInstanceID
	}
	state.calendarDays = uniqueIDs(state.calendarDays)
	state.calendarInstances = uniqueIDs(state.calendarInstances)
	return nil
}

func collectFullCompetitionRows(db *gorm.DB, state *fullCleanupState) error {
	teamID := state.fullTeamIDs["team_a"]
	if teamID == 0 || len(state.fullUserIDs) == 0 {
		return nil
	}
	s := newFullSeeder(db, options{})
	s.now = fullCleanupSeedTime(db, state)
	s.ids.users = state.fullUserIDs
	s.ids.teams["team_a"] = teamID
	for _, spec := range s.fullCompetitionSpecs() {
		query := db.Where("team_id = ? AND group_id IS NULL AND type = ? AND created_by_user_id = ? AND created_by_role = ?", teamID, spec.typeName, state.fullUserIDs[spec.creatorKey], fullCompetitionCreatorRole(spec.creatorKey))
		if spec.typeName == "metric" {
			query = query.Where("metric = ? AND route_geojson IS NULL AND max_attempts IS NULL", *spec.metric)
		} else {
			query = query.Where("route_geojson = ? AND metric IS NULL AND acceptance_radius_meters = ? AND max_attempts IS NOT DISTINCT FROM ?", *spec.route, 50.0, spec.maxAttempts)
		}
		var rows []dbs.Competition
		if err := query.Find(&rows).Error; err != nil {
			return err
		}
		matched := false
		for i := range rows {
			matches, err := s.validateFullCompetitionParticipants(db, rows[i].ID, spec)
			if err != nil {
				return err
			}
			if !matches {
				knownOther := false
				for _, other := range s.fullCompetitionSpecs() {
					if other.key == spec.key {
						continue
					}
					otherMatches, err := s.validateFullCompetitionParticipants(db, rows[i].ID, other)
					if err != nil {
						return err
					}
					if otherMatches {
						knownOther = true
						break
					}
				}
				if knownOther {
					continue
				}
				return fmt.Errorf("competencia con identidad estable %s tiene participantes distintos; cleanup abortado", spec.key)
			}
			if matched {
				return fmt.Errorf("competencia full %s ambigua; cleanup abortado", spec.key)
			}
			matched = true
			state.competitionIDs = append(state.competitionIDs, rows[i].ID)
			if rows[i].RouteSessionInstanceID != nil {
				if err := validateFullCompetitionSharedInstance(db, &rows[i]); err != nil {
					return fmt.Errorf("instancia de competencia %s sin identidad full: %w", spec.key, err)
				}
				state.competitionInstances = append(state.competitionInstances, *rows[i].RouteSessionInstanceID)
			}
			if spec.key == "finished_metric_tie" {
				var instances []dbs.SessionInstance
				name := fmt.Sprintf("Competencia métrica %d", rows[i].ID)
				if err := db.Where("name = ?", name).Find(&instances).Error; err != nil {
					return err
				}
				if len(instances) != 1 || !containsFullSeedMarker(pointerValue(instances[0].Description)) || instances[0].SourceSessionID != nil {
					return fmt.Errorf("instancia métrica sintética %q ambigua o sin marker", name)
				}
				state.competitionInstances = append(state.competitionInstances, instances[0].ID)
			}
			for _, attempt := range fullCompetitionAttempts(spec, s.now) {
				if attempt.number <= 1 {
					continue
				}
				name := fmt.Sprintf("Competencia ruta %d (intento %d)", rows[i].ID, attempt.number)
				var instances []dbs.SessionInstance
				if err := db.Where("name = ?", name).Find(&instances).Error; err != nil {
					return err
				}
				if len(instances) != 1 || !containsFullSeedMarker(pointerValue(instances[0].Description)) || instances[0].SourceSessionID != nil {
					return fmt.Errorf("instancia de intento sintética %q ambigua o sin marker", name)
				}
				state.competitionInstances = append(state.competitionInstances, instances[0].ID)
			}
		}
	}
	state.competitionIDs = uniqueIDs(state.competitionIDs)
	state.competitionInstances = uniqueIDs(state.competitionInstances)
	return nil
}

func collectFullActivityRows(db *gorm.DB, state *fullCleanupState) error {
	if len(state.calendarInstances) == 0 && len(state.competitionIDs) == 0 {
		return nil
	}
	allInstances := uniqueIDs(append(append([]int64(nil), state.calendarInstances...), state.competitionInstances...))
	s := newFullSeeder(db, options{})
	s.ids.users = state.fullUserIDs
	s.now = fullCleanupSeedTime(db, state)
	for _, key := range []string{"team_a"} {
		s.ids.teams[key] = state.fullTeamIDs[key]
	}
	for _, key := range []string{"group_a_main", "group_a_secondary"} {
		s.ids.groups[key] = state.fullGroupIDs[key]
	}
	for dateKey, id := range state.calendarInstanceByDate {
		groupID, date, err := parseFullCalendarKey(dateKey)
		if err != nil {
			return err
		}
		if groupID == fullGroupAMainID && date.Equal(state.anchor.AddDate(0, 0, 7)) {
			s.ids.sessionInstances[fullActivityStampedDayKey(s.now.AddDate(0, 0, -14))] = id
		}
		if groupID == fullGroupAMainID && date.Equal(state.anchor.AddDate(0, 0, 21)) {
			s.ids.sessionInstances["today_a_main"] = id
		}
		if groupID == fullGroupASecondaryID && date.Equal(state.anchor.AddDate(0, 0, 14)) {
			s.ids.sessionInstances[fullActivitySecondaryDayKey] = id
		}
	}
	if !state.anchor.IsZero() {
		if err := collectExactFullRunners(db, state, s); err != nil {
			return err
		}
		if err := collectExactFullAttendance(db, state, allInstances, s); err != nil {
			return err
		}
		if err := collectExactFullFeedback(db, state, s); err != nil {
			return err
		}
	}
	if len(state.competitionIDs) > 0 {
		if err := collectExactFullCompetitionAttempts(db, state, s); err != nil {
			return err
		}
		var linkedDays, linkedMessages int64
		if err := db.Model(&dbs.GroupCalendarDay{}).Where("session_instance_id IN ?", state.competitionInstances).Count(&linkedDays).Error; err != nil {
			return err
		}
		if err := db.Model(&dbs.SessionMessage{}).Where("session_instance_id IN ?", state.competitionInstances).Count(&linkedMessages).Error; err != nil {
			return err
		}
		if linkedDays > 0 || linkedMessages > 0 {
			return fmt.Errorf("instancia sintética de competencia enlazada a calendario o mensajes; cleanup abortado")
		}
		var feedback []dbs.WorkoutFeedback
		if err := db.Unscoped().Where("assigned_session_id IN ?", state.competitionInstances).Find(&feedback).Error; err != nil {
			return err
		}
		for _, row := range feedback {
			if !strings.Contains(pointerValue(row.Annotations), fullCompetitionEvidenceMarker) || !containsID(state.users, row.AthleteUserID) {
				return fmt.Errorf("feedback de competencia sin identidad sintética clara; cleanup abortado")
			}
			state.competitionFeedbackIDs = append(state.competitionFeedbackIDs, row.ID)
		}
	}
	allOwnedInstances := uniqueIDs(append(append([]int64(nil), state.calendarInstances...), state.competitionInstances...))
	if len(allOwnedInstances) > 0 {
		var linked []int64
		if err := db.Model(&dbs.SessionExerciseInstance{}).Where("session_instance_id IN ?", allOwnedInstances).Pluck("exercise_instance_id", &linked).Error; err != nil {
			return err
		}
		linked = uniqueIDs(linked)
		exclusive, err := exclusiveIDs(db, linked,
			`SELECT exercise_instance_id AS id FROM session_exercise_instances WHERE exercise_instance_id IN ? AND session_instance_id NOT IN ?`, linked, allOwnedInstances)
		if err != nil {
			return err
		}
		state.exerciseInstances = exclusive
	}
	state.runnerIDs = uniqueIDs(state.runnerIDs)
	state.activityFeedbackIDs = uniqueIDs(state.activityFeedbackIDs)
	state.competitionFeedbackIDs = uniqueIDs(state.competitionFeedbackIDs)
	state.attendanceIDs = uniqueIDs(state.attendanceIDs)
	return nil
}

func collectExactFullRunners(db *gorm.DB, state *fullCleanupState, s *fullSeeder) error {
	expected := make(map[string]struct{})
	add := func(instanceID, athleteID int64, status string, start time.Time, end *time.Time) {
		if instanceID <= 0 || athleteID <= 0 {
			return
		}
		expected[fullRunnerKey(instanceID, athleteID, status, start, end)] = struct{}{}
	}
	for _, spec := range s.fullActivityRunnerSpecs() {
		instanceID := sessionInstanceIDForActivity(spec.sessionKey, s.ids.sessionInstances, nil)
		add(instanceID, state.fullUserIDs[spec.athleteKey], spec.status, spec.start, spec.end)
	}
	for day := 0; day < 9; day++ {
		date := fullDate(s.now.AddDate(0, 0, -16+day))
		instanceID := state.calendarInstanceByDate[fullCalendarKey(fullGroupASecondaryID, date)]
		for _, userKey := range []string{"r4", "r5"} {
			if userKey == "r5" && day >= 4 {
				continue
			}
			start := fullActivityClock(date, 7, 0)
			end := fullActivityClock(date, 8, 0)
			add(instanceID, state.fullUserIDs[userKey], "finished", start, &end)
		}
	}
	var rows []dbs.RunnerSession
	if err := db.Where("session_instance_id IN ? AND competition_id IS NULL", state.calendarInstances).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		key := fullRunnerKey(row.SessionInstanceID, row.AthleteUserID, row.Status, row.StartDate, row.EndDate)
		if _, ok := expected[key]; !ok {
			return fmt.Errorf("runner_session no atribuible al ledger full; cleanup abortado")
		}
		state.runnerIDs = append(state.runnerIDs, row.ID)
	}
	return nil
}

func collectExactFullCompetitionAttempts(db *gorm.DB, state *fullCleanupState, s *fullSeeder) error {
	expected := make(map[int64]map[string]struct{})
	for _, spec := range s.fullCompetitionSpecs() {
		competitionID := stateCompetitionID(db, state, s, spec)
		if competitionID == 0 {
			continue
		}
		for _, attempt := range fullCompetitionAttempts(spec, s.now) {
			end := (*time.Time)(nil)
			if attempt.status == "finished" {
				end = &attempt.endedAt
			}
			if expected[competitionID] == nil {
				expected[competitionID] = make(map[string]struct{})
			}
			expected[competitionID][fullRunnerKey(0, state.fullUserIDs[attempt.athleteKey], attempt.status, attempt.startedAt, end)] = struct{}{}
		}
	}
	var rows []dbs.RunnerSession
	if err := db.Where("competition_id IN ?", state.competitionIDs).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if row.CompetitionID == nil {
			return fmt.Errorf("intento sin competition_id; cleanup abortado")
		}
		key := fullRunnerKey(0, row.AthleteUserID, row.Status, row.StartDate, row.EndDate)
		if _, ok := expected[*row.CompetitionID][key]; !ok {
			return fmt.Errorf("intento de competencia con identidad inesperada; cleanup abortado")
		}
		state.runnerIDs = append(state.runnerIDs, row.ID)
	}
	return nil
}

func stateCompetitionID(db *gorm.DB, state *fullCleanupState, s *fullSeeder, spec fullCompetitionSpec) int64 {
	var row dbs.Competition
	query := db.Where("team_id = ? AND group_id IS NULL AND type = ? AND created_by_user_id = ? AND created_by_role = ?", state.fullTeamIDs["team_a"], spec.typeName, state.fullUserIDs[spec.creatorKey], fullCompetitionCreatorRole(spec.creatorKey))
	if spec.typeName == "metric" {
		query = query.Where("metric = ? AND route_geojson IS NULL AND max_attempts IS NULL", *spec.metric)
	} else {
		query = query.Where("route_geojson = ? AND metric IS NULL AND acceptance_radius_meters = ? AND max_attempts IS NOT DISTINCT FROM ?", *spec.route, 50.0, spec.maxAttempts)
	}
	if query.Take(&row).Error != nil {
		return 0
	}
	return row.ID
}

func collectExactFullAttendance(db *gorm.DB, state *fullCleanupState, instances []int64, s *fullSeeder) error {
	expected := make(map[string]struct{})
	for _, spec := range s.fullActivityAttendanceSpecs() {
		instance := sessionInstanceIDForActivity(spec.sessionKey, s.ids.sessionInstances, nil)
		userID := state.fullUserIDs[spec.userKey]
		registeredBy := int64(0)
		if spec.registeredByKey != "" {
			registeredBy = state.fullUserIDs[spec.registeredByKey]
		}
		expected[fmt.Sprintf("%d/%d/%s/%d", instance, userID, spec.source, registeredBy)] = struct{}{}
	}
	var rows []dbs.Attendance
	if err := db.Where("team_id = ? AND training_session_id IN ?", state.fullTeamIDs["team_a"], instances).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		registeredBy := int64(0)
		if row.RegisteredByUserID != nil {
			registeredBy = *row.RegisteredByUserID
		}
		key := fmt.Sprintf("%d/%d/%s/%d", row.TrainingSessionID, row.UserID, row.Source, registeredBy)
		if _, ok := expected[key]; !ok {
			return fmt.Errorf("attendance fuera de las seis tuplas full; cleanup abortado")
		}
		state.attendanceIDs = append(state.attendanceIDs, row.ID)
	}
	return nil
}

func collectExactFullFeedback(db *gorm.DB, state *fullCleanupState, s *fullSeeder) error {
	if len(state.calendarInstances) == 0 {
		return nil
	}
	var rows []dbs.WorkoutFeedback
	if err := db.Unscoped().Where("assigned_session_id IN ?", state.calendarInstances).Find(&rows).Error; err != nil {
		return err
	}
	expected := make(map[string]struct{})
	for _, spec := range s.fullActivityFeedbackSpecs() {
		instance := sessionInstanceIDForActivity(spec.sessionKey, s.ids.sessionInstances, nil)
		key := fmt.Sprintf("%d/%d/%d/%d/%s", instance, state.fullUserIDs[spec.athleteKey], state.fullUserIDs[spec.ownerKey], spec.setNumber, fullSeedMarker+" Actividad del perfil full.")
		expected[key] = struct{}{}
	}
	for _, row := range rows {
		if row.TeamID == nil || *row.TeamID != state.fullTeamIDs["team_a"] || pointerValue(row.Annotations) != fullSeedMarker+" Actividad del perfil full." {
			return fmt.Errorf("feedback en una instancia full sin marker de actividad exacto; cleanup abortado")
		}
		key := fmt.Sprintf("%d/%d/%d/%d/%s", row.AssignedSessionID, row.AthleteUserID, row.FeedbackOwnerUserID, row.SetNumber, pointerValue(row.Annotations))
		if _, ok := expected[key]; !ok {
			return fmt.Errorf("feedback de actividad con tupla inesperada; cleanup abortado")
		}
		state.activityFeedbackIDs = append(state.activityFeedbackIDs, row.ID)
	}
	return nil
}

func collectFullMessages(db *gorm.DB, state *fullCleanupState) error {
	if len(state.calendarInstances) == 0 {
		return nil
	}
	var rows []dbs.SessionMessage
	if err := db.Where("session_instance_id IN ?", state.calendarInstances).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if !strings.Contains(row.Body, "[seed_full:message:") || !containsID(state.users, row.SenderUserID) {
			return fmt.Errorf("mensaje en instancia full sin marcador atribuible; cleanup abortado")
		}
		state.messageIDs = append(state.messageIDs, row.ID)
	}
	if len(state.messageIDs) > 0 {
		if err := db.Model(&dbs.SessionMessageRecipient{}).Where("message_id IN ?", state.messageIDs).Pluck("message_id", &state.messageRecipientIDs).Error; err != nil {
			return err
		}
	}
	return nil
}

func collectFullPayments(db *gorm.DB, state *fullCleanupState) error {
	if len(state.users) == 0 {
		return nil
	}
	var payments []dbs.Payment
	if err := db.Where("external_reference LIKE ?", "seed_full:payment:%").Find(&payments).Error; err != nil {
		return err
	}
	for _, row := range payments {
		if row.ExternalRef == nil || !strings.HasPrefix(*row.ExternalRef, "seed_full:payment:") || row.UserID == nil || !containsID(state.users, *row.UserID) || !containsFullSeedMarker(row.Description) {
			return fmt.Errorf("pago con prefijo seed_full sin identidad completa; cleanup abortado")
		}
		state.paymentIDs = append(state.paymentIDs, row.ID)
	}
	var subscriptions []dbs.UserRoleTierSubscription
	if err := db.Where("user_id IN ?", state.users).Find(&subscriptions).Error; err != nil {
		return err
	}
	for _, row := range subscriptions {
		state.subscriptionIDs = append(state.subscriptionIDs, row.ID)
	}
	if len(state.subscriptionIDs) > 0 {
		var subscriptionInstallmentIDs []int64
		if err := db.Model(&dbs.Installment{}).Where("subscription_id IN ?", state.subscriptionIDs).Pluck("id", &subscriptionInstallmentIDs).Error; err != nil {
			return err
		}
		state.installmentIDs = append(state.installmentIDs, subscriptionInstallmentIDs...)
	}
	if len(state.paymentIDs) > 0 {
		var paymentInstallmentIDs []int64
		if err := db.Model(&dbs.Installment{}).Where("internal_payment_id IN ?", state.paymentIDs).Pluck("id", &paymentInstallmentIDs).Error; err != nil {
			return err
		}
		state.installmentIDs = append(state.installmentIDs, paymentInstallmentIDs...)
	}
	var teamInstallments []dbs.Installment
	if len(state.teams) > 0 {
		if err := db.Where("team_id IN ? AND user_id IN ?", state.teams, state.users).Find(&teamInstallments).Error; err != nil {
			return err
		}
		for _, row := range teamInstallments {
			state.teamInstallmentIDs = append(state.teamInstallmentIDs, row.ID)
		}
	}
	state.installmentIDs = uniqueIDs(append(state.installmentIDs, state.teamInstallmentIDs...))
	return nil
}

func collectFullSellerConnection(db *gorm.DB, state *fullCleanupState) error {
	userID := state.fullUserIDs["trainer_1"]
	if userID == 0 {
		return nil
	}
	var rows []dbs.SellerConnection
	trainerIDs := []int64{userID, state.fullUserIDs["trainer_2"]}
	if err := db.Where("user_id IN ?", trainerIDs).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	if config.TokenEncryptionKey == "" {
		return fmt.Errorf("seller connection de entrenadores full no se puede identificar sin TOKEN_ENCRYPTION_KEY; cleanup abortado")
	}
	encryptor := crypto.NewAESGCMEncryptor(config.TokenEncryptionKey)
	for _, row := range rows {
		if row.UserID != userID || row.MPUserID != "seed-full-mp-user-trainer-1" || row.PublicKey != "TEST-seed-full-trainer-1" {
			return fmt.Errorf("seller connection de entrenador full sin identidad sintética completa; cleanup abortado para preservarla")
		}
		access, accessErr := encryptor.Decrypt(row.AccessToken)
		refresh, refreshErr := encryptor.Decrypt(row.RefreshToken)
		if accessErr == nil && refreshErr == nil && access == fullSyntheticSellerAccessToken && refresh == fullSyntheticSellerRefreshToken {
			state.sellerConnectionIDs = append(state.sellerConnectionIDs, row.ID)
			continue
		}
		return fmt.Errorf("seller connection de trainer_1 no coincide con los tokens sintéticos; cleanup abortado para preservarla")
	}
	return nil
}

func fullCleanupSteps(state fullCleanupState) []cleanupStep {
	steps := make([]cleanupStep, 0, 40)
	add := func(table, where string, args ...any) {
		steps = append(steps, cleanupStep{Table: table, Where: where, Args: args})
	}
	addIDs := func(table, column string, ids []int64) {
		if len(ids) > 0 {
			add(table, column+" IN ?", ids)
		}
	}
	addIDs("runner_session", "id", state.runnerIDs)
	addIDs("workout_feedback_points", "feedback_id", uniqueIDs(append(append([]int64{}, state.activityFeedbackIDs...), state.competitionFeedbackIDs...)))
	addIDs("workout_feedback", "id", uniqueIDs(append(append([]int64{}, state.activityFeedbackIDs...), state.competitionFeedbackIDs...)))
	addIDs("attendances", "id", state.attendanceIDs)
	addIDs("session_message_recipients", "message_id", uniqueIDs(state.messageRecipientIDs))
	addIDs("session_messages", "id", state.messageIDs)
	addIDs("competition_winner", "competition_id", state.competitionIDs)
	addIDs("competition_participant", "competition_id", state.competitionIDs)
	if len(state.competitionIDs) > 0 && len(state.users) > 0 {
		add("gamification_stamp", "type = 'competition' AND source_id IN ? AND user_id IN ?", state.competitionIDs, state.users)
	}
	addIDs("competition", "id", state.competitionIDs)
	if len(state.fullUserIDs) > 0 {
		add("gamification_unseen", "(user_id = ? AND section IN ? AND team_id IS NULL) OR (team_id = ? AND section = 'competitions' AND user_id IN ?)", state.fullUserIDs["r4"], []string{"streaks", "achievements"}, state.fullTeamIDs["team_a"], state.users)
		add("gamification_streak_state", "user_id IN ?", state.users)
		add("user_achievement_progress", "user_id = ? AND achievement_id IN ?", state.fullUserIDs["r4"], []string{"first_session", "streak_3", "sessions_5", "streak_7", "team_first"})
	}
	if len(state.users) > 0 {
		add("push_tokens", "user_id IN ? AND token LIKE ?", state.users, "ExponentPushToken[seed-full-%")
	}
	addIDs("payments", "id", state.paymentIDs)
	addIDs("installments", "id", state.installmentIDs)
	addIDs("user_role_tier_subscriptions", "id", state.subscriptionIDs)
	if len(state.teams) > 0 && len(state.users) > 0 {
		add("invitations", "team_id IN ? AND (inviter_id IN ? OR invitee_id IN ?)", state.teams, state.users, state.users)
		add("join_requests", "team_id IN ? AND runner_id IN ?", state.teams, state.users)
		add("team_users", "team_id IN ? AND user_id IN ?", state.teams, state.users)
	}
	if len(state.groups) > 0 && len(state.users) > 0 {
		add("group_users", "group_id IN ? AND user_id IN ?", state.groups, state.users)
	}
	addIDs("session_exercise_instances", "session_instance_id", uniqueIDs(append(append([]int64{}, state.calendarInstances...), state.competitionInstances...)))
	addIDs("exercise_instances", "id", state.exerciseInstances)
	addIDs("session_instances", "id", uniqueIDs(append(append([]int64{}, state.calendarInstances...), state.competitionInstances...)))
	addIDs("group_calendar_days", "id", state.calendarDays)
	addIDs("plan_days", "plan_id", state.plans)
	addIDs("training_plans", "id", state.plans)
	addIDs("session_exercises", "session_id", state.sessions)
	addIDs("sessions", "id", state.sessions)
	addIDs("exercises", "id", state.exercises)
	if len(state.fullUserIDs) > 0 {
		addIDs("seller_connections", "id", state.sellerConnectionIDs)
		add("user_roles", "user_id IN ?", state.users)
		add("refresh_tokens", "user_id IN ?", state.users)
		add("password_reset_tokens", "user_id IN ?", state.users)
	}
	addIDs("groups", "id", state.groups)
	addIDs("teams", "id", state.teams)
	if len(state.users) > 0 {
		add("users", "id IN ? AND email LIKE ?", state.users, "%@seed-full.local")
	}
	return steps
}

func fullGroupIDList(groups map[string]int64) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, id := range groups {
		ids = append(ids, id)
	}
	return uniqueIDs(ids)
}

func fullCleanupSeedTime(db *gorm.DB, state *fullCleanupState) time.Time {
	now := gametime.Now().In(gametime.Location)
	if state.anchor.IsZero() {
		return now
	}
	date := state.anchor.AddDate(0, 0, 21)
	var day dbs.GroupCalendarDay
	if err := db.Where("group_id = ? AND date = ?", fullGroupAMainID, date).Take(&day).Error; err == nil && day.PresencialOpenedAt != nil {
		clock := day.PresencialOpenedAt.In(gametime.Location).Add(20 * time.Minute)
		return time.Date(date.Year(), date.Month(), date.Day(), clock.Hour(), clock.Minute(), clock.Second(), clock.Nanosecond(), gametime.Location)
	}
	return time.Date(date.Year(), date.Month(), date.Day(), now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), gametime.Location)
}

func fullCalendarKey(groupID int64, date time.Time) string {
	return fmt.Sprintf("%d/%s", groupID, fullDBDate(date).Format(dateLayout))
}

func parseFullCalendarKey(key string) (int64, time.Time, error) {
	var groupID int64
	var dateText string
	if _, err := fmt.Sscanf(key, "%d/%s", &groupID, &dateText); err != nil {
		return 0, time.Time{}, err
	}
	date, err := time.ParseInLocation(dateLayout, dateText, gametime.Location)
	return groupID, date, err
}

// fullRunnerKey identifica un runner_session por instante, no por offset: los
// valores sembrados viven en Cordoba y los leídos de Postgres pueden venir en UTC
// o Local. timestamptz guarda microsegundos, así que se redondea a esa precisión.
func fullRunnerKey(instanceID, athleteID int64, status string, start time.Time, end *time.Time) string {
	const layout = "2006-01-02T15:04:05.000000Z"
	endKey := "nil"
	if end != nil {
		endKey = end.UTC().Round(time.Microsecond).Format(layout)
	}
	return fmt.Sprintf("%d/%d/%s/%s/%s", instanceID, athleteID, status, start.UTC().Round(time.Microsecond).Format(layout), endKey)
}

func containsID(ids []int64, wanted int64) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}
