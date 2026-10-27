package main

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

// run encadena las etapas del seed en el orden en que se dependen: nada se
// escribe antes de que exista lo que lo referencia.
func (s *seeder) run() error {
	if err := s.ensureCatalog(); err != nil {
		return err
	}
	sessionIDs, err := s.ensureSessions()
	if err != nil {
		return err
	}
	planIDs, err := s.ensurePlans(sessionIDs)
	if err != nil {
		return err
	}
	days, err := s.stamp(planIDs)
	if err != nil {
		return err
	}
	if err := s.ensureMembershipWindow(); err != nil {
		return err
	}
	specs := s.buildSeriesPlan(days)
	if err := s.writeFeedback(days, specs); err != nil {
		return err
	}
	if err := s.writeRunnerSessions(days); err != nil {
		return err
	}
	if err := s.markLegacyInstances(days); err != nil {
		return err
	}
	return s.verify(days, specs)
}

// resolveScope valida el scope del seed y arma el roster de corredores.
//
// El guard es explícito (tarea 7.10): si el grupo no es del equipo, se aborta. Es
// el error más caro posible — sembrar el catálogo y las series de un equipo para
// leerlos desde otro—, así que no se deja pasar por un default.
func (s *seeder) resolveScope() error {
	teamDao := daos.NewTeamDao(s.db)
	groupDao := daos.NewGroupDao(s.db)

	team, err := teamDao.FindByID(nil, s.teamID)
	if err != nil {
		return fmt.Errorf("error al leer el equipo %d: %w", s.teamID, err)
	}
	if team == nil {
		if !s.opts.bootstrap {
			return fmt.Errorf("el equipo %d no existe. Pasá --bootstrap-scope si querés que el seed lo cree (es para bases vacías)", s.teamID)
		}
		if err := s.bootstrapScope(); err != nil {
			return err
		}
		if team, err = teamDao.FindByID(nil, s.teamID); err != nil || team == nil {
			return fmt.Errorf("no se pudo resolver el equipo %d después del bootstrap: %v", s.teamID, err)
		}
	}
	group, err := groupDao.FindByIDAndTeamID(nil, s.groupID, s.teamID)
	if err != nil {
		return fmt.Errorf("error al leer el grupo %d: %w", s.groupID, err)
	}
	if group == nil {
		return fmt.Errorf("el grupo %d no pertenece al equipo %d: el seed aborta en vez de sembrar un scope ajeno", s.groupID, s.teamID)
	}
	s.ownerID = team.OwnerID
	if err := s.buildRoster(); err != nil {
		return err
	}
	// Con --bootstrap-scope, un grupo chico se completa con corredores sintéticos:
	// sin los cinco arquetipos el seed no puede producir el dataset que dice
	// producir, y es mejor inventarlos que sembrar la mitad.
	if s.opts.bootstrap && len(s.roster) < minAthletes {
		if err := s.bootstrapScope(); err != nil {
			return err
		}
		if err := s.buildRoster(); err != nil {
			return err
		}
	}
	if len(s.roster) < minAthletes {
		s.warnings = append(s.warnings, fmt.Sprintf(
			"el grupo %d tiene %d corredores y el seed necesita %d para tener los cinco arquetipos; con menos, el caso de insufficient_data no se produce",
			s.groupID, len(s.roster), minAthletes))
	}
	return nil
}

// buildRoster son los corredores del grupo ordenados por id, con el arquetipo que
// les toca por posición. El orden por id es lo que hace reproducible el dataset:
// el mismo grupo genera siempre el mismo reparto velocista/fondista.
func (s *seeder) buildRoster() error {
	members, err := daos.NewGroupUserDao(s.db).FindByGroupID(nil, s.groupID)
	if err != nil {
		return fmt.Errorf("error al leer los miembros del grupo %d: %w", s.groupID, err)
	}
	active := make([]dbs.GroupUser, 0, len(members))
	for _, member := range members {
		if member.DeletedAt == nil {
			active = append(active, member)
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].UserID < active[j].UserID })

	userDao := daos.NewUserDao(s.db)
	for i, member := range active {
		name := fmt.Sprintf("user %d", member.UserID)
		if user, err := userDao.GetByID(nil, member.UserID); err == nil && user != nil {
			name = user.Name
		}
		s.roster = append(s.roster, seedAthlete{
			Index:     i,
			UserID:    member.UserID,
			Name:      name,
			Archetype: archetypes[i%archetypeCount],
		})
	}
	return nil
}

// ---- catálogo ----

// ensureCatalog crea (o reutiliza) los 12 ejercicios del entrenador y deja el
// mapa clave -> id en s.catalogIDs.
//
// La idempotencia va por (dueño, nombre normalizado): reusar
// dashboard.NormalizeExerciseName es lo que hace que "Distancia 4 km" y
// "distancia 4 KM." sean el mismo ejercicio para el seed y para el fallback del
// dashboard.
func (s *seeder) ensureCatalog() error {
	exerciseDao := daos.NewExerciseDao(s.db)
	existing, err := exerciseDao.FindByOwner(nil, s.ownerID)
	if err != nil {
		return fmt.Errorf("error al leer el catálogo del entrenador: %w", err)
	}
	byName := make(map[string]dbs.Exercise, len(existing))
	for _, row := range existing {
		byName[dashboard.NormalizeExerciseName(row.Name)] = row
	}

	s.catalogIDs = make(map[string]int64, len(catalog))
	created := 0
	for _, item := range catalog {
		normalized := dashboard.NormalizeExerciseName(item.Name)
		if found, ok := byName[normalized]; ok {
			s.catalogIDs[item.Key] = found.ID
			if found.Kind != item.Kind {
				s.warnings = append(s.warnings, fmt.Sprintf(
					"el ejercicio %q ya existía con kind %q y el seed usa %q: el dashboard mezcla objetivos distintos",
					found.Name, found.Kind, item.Kind))
			}
			continue
		}
		row := &dbs.Exercise{
			OwnerID:     s.ownerID,
			Name:        item.Name,
			Description: seedDescription(fmt.Sprintf("ejercicio %q", item.Name)),
			Kind:        item.Kind,
			Minutes:     item.Minutes,
			DistanceM:   item.DistanceM,
			MuscleGroup: item.MuscleGroup,
		}
		if err := exerciseDao.Create(nil, row); err != nil {
			return fmt.Errorf("error creando el ejercicio %q: %w", item.Name, err)
		}
		s.catalogIDs[item.Key] = row.ID
		created++
	}
	s.count("exercises_creados", int64(created))
	s.count("exercises_reutilizados", int64(len(catalog)-created))
	return nil
}

// ---- sesiones y planes ----

func (s *seeder) ensureSessions() (map[string]int64, error) {
	sessionDao := daos.NewSessionDao(s.db)
	sessionService := services.NewSessionService(sessionDao, daos.NewSessionExerciseDao(s.db), daos.NewExerciseDao(s.db))

	existing, err := sessionDao.FindByOwner(nil, s.ownerID)
	if err != nil {
		return nil, fmt.Errorf("error al leer las sesiones del entrenador: %w", err)
	}
	byName := make(map[string]int64, len(existing))
	for _, row := range existing {
		byName[row.Name] = row.ID
	}

	ids := make(map[string]int64, len(sessions))
	created := 0
	for _, template := range sessions {
		if id, ok := byName[template.Name]; ok {
			ids[template.Key] = id
			continue
		}
		createdSession, err := sessionService.Create(nil, s.ownerID, sessionRequestFor(template, s.catalogIDs, s.ownerID))
		if err != nil {
			return nil, fmt.Errorf("error creando la sesión %q: %w", template.Name, err)
		}
		ids[template.Key] = createdSession.ID
		created++
	}
	s.count("sessions_creadas", int64(created))
	s.count("sessions_reutilizadas", int64(len(sessions)-created))
	return ids, nil
}

func sessionRequestFor(template sessionTemplate, exerciseIDs map[string]int64, ownerID int64) session.SessionRequest {
	req := session.SessionRequest{
		OwnerID:     ownerID,
		Name:        template.Name,
		Description: seedDescription(fmt.Sprintf("sesión %q", template.Name)),
	}
	for _, block := range template.Blocks {
		req.Exercises = append(req.Exercises, session.SessionExerciseRequest{
			ExerciseID:  exerciseIDs[block.ExerciseKey],
			Role:        block.Role,
			RepeatCount: intPtr(block.Repeat),
			RestMinutes: intPtr(block.RestMinutes),
		})
	}
	return req
}

func (s *seeder) ensurePlans(sessionIDs map[string]int64) (map[string]int64, error) {
	planDao := daos.NewTrainingPlanDao(s.db)
	planService := services.NewTrainingPlanService(planDao, daos.NewPlanDayDao(s.db), daos.NewSessionDao(s.db), daos.NewGroupCalendarDayDao(s.db))

	existing, err := planDao.FindByOwner(nil, s.ownerID)
	if err != nil {
		return nil, fmt.Errorf("error al leer los planes del entrenador: %w", err)
	}
	byName := make(map[string]int64, len(existing))
	for _, row := range existing {
		byName[row.Name] = row.ID
	}

	ids := make(map[string]int64, len(plans))
	created := 0
	for _, template := range plans {
		if id, ok := byName[template.Name]; ok {
			ids[template.Key] = id
			continue
		}
		createdPlan, err := planService.Create(nil, s.ownerID, trainingplan.TrainingPlanRequest{
			OwnerID:     s.ownerID,
			Name:        template.Name,
			Description: seedDescription(fmt.Sprintf("plan %q", template.Name)),
			Days:        template.planDayRequest(sessionIDs),
		})
		if err != nil {
			return nil, fmt.Errorf("error creando el plan %q: %w", template.Name, err)
		}
		ids[template.Key] = createdPlan.ID
		created++
	}
	s.count("plans_creados", int64(created))
	s.count("plans_reutilizados", int64(len(plans)-created))
	return ids, nil
}

// ensureMembershipWindow ajusta la membresía para que cubra la ventana sembrada.
//
// Va DESPUÉS del stamp a propósito: group_users.date_start es NOT NULL y la
// ventana de membresía es lo que acota la query del dashboard (tarea 6.2). Fijarlo
// antes del stamp sería adivinar la ventana; correrlo después deja claro que el
// ajuste es una consecuencia de las fechas ya elegidas.
//
// Solo se mueve hacia atrás (para que la sesión del primer día entre) y la fila
// nunca se borra: revertir la membresía a mano es parte del runbook (§8).
func (s *seeder) ensureMembershipWindow() error {
	from := s.window.from()
	moved := 0
	for _, athlete := range s.roster {
		var member dbs.GroupUser
		err := s.db.Where("group_id = ? AND user_id = ? AND deleted_at IS NULL", s.groupID, athlete.UserID).First(&member).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			created := dbs.GroupUser{GroupID: s.groupID, UserID: athlete.UserID, DateStart: from}
			if err := s.db.Create(&created).Error; err != nil {
				return fmt.Errorf("error creando la membresía del corredor %d: %w", athlete.UserID, err)
			}
			s.count("membresias_creadas", 1)
			continue
		}
		if err != nil {
			return fmt.Errorf("error al leer la membresía del corredor %d: %w", athlete.UserID, err)
		}
		if member.DateStart.After(from) {
			customlogger.Warn(nil, "seed dashboard: membresía movida para cubrir la ventana",
				customlogger.Tag("group_id", fmt.Sprintf("%d", s.groupID)),
				customlogger.Tag("user_id", fmt.Sprintf("%d", athlete.UserID)),
				customlogger.Tag("date_start_previo", member.DateStart.Format(dateLayout)),
				customlogger.Tag("date_start_nuevo", from.Format(dateLayout)))
			if err := s.db.Model(&dbs.GroupUser{}).Where("id = ?", member.ID).
				Update("date_start", from).Error; err != nil {
				return fmt.Errorf("error ajustando la membresía del corredor %d: %w", athlete.UserID, err)
			}
			moved++
		}
	}
	s.count("membresias_ajustadas", int64(moved))
	return nil
}

// ---- stamp ----

// seededDay es un día de calendario del seed con su SessionInstance ya
// instanciada y sus bloques resueltos.
type seededDay struct {
	DayIndex        int
	Date            time.Time
	CalendarDayID   int64
	SessionInstance int64
	PresencialFrom  *time.Time
	Blocks          []instanceBlock
}

// instanceBlock es un bloque de la sesión instanciada, con el objetivo CONGELADO
// de la ExerciseInstance (design.md D3) y no el vigente del catálogo.
type instanceBlock struct {
	ExerciseKey      string
	ExerciseInstance int64
	Role             string
	Repeat           int
	Kind             string
	Minutes          *int
	DistanceM        *int
	Name             string
}

// stamp aplica los dos planes por el mecanismo de stamp del calendario (tarea
// 7.5). Insertar los días a mano saltaría la instanciación —que es lo que copia
// el catálogo a exercise_instances— y el dashboard no tendría nada que leer.
//
// La idempotencia NO viene de Force sino de exclude_dates: las fechas objetivo
// que ya tienen un día de ESTE plan se listan en exclude_dates, que es
// exactamente "no las toques". Así una segunda corrida no crea una segunda
// SessionInstance ni borra la instancia anterior con el feedback encima.
func (s *seeder) stamp(planIDs map[string]int64) ([]seededDay, error) {
	calendarService := services.NewCalendarService(
		daos.NewGroupCalendarDayDao(s.db), daos.NewGroupDao(s.db), daos.NewTeamDao(s.db),
		daos.NewGroupUserDao(s.db), daos.NewTeamUserDao(s.db), daos.NewTrainingPlanDao(s.db),
		daos.NewPlanDayDao(s.db), daos.NewSessionDao(s.db), s.db)

	existingDays, err := s.existingCalendarDays()
	if err != nil {
		return nil, err
	}

	stamped := 0
	for weekIndex, template := range plans {
		planID := planIDs[template.Key]
		exclude := s.window.excludeDatesForWeek(weekIndex)
		for _, target := range s.weekTargets(weekIndex) {
			day, found := existingDays[target.Format(dateLayout)]
			if !found {
				continue
			}
			if day.SourcePlanID != nil && *day.SourcePlanID == planID {
				exclude = append(exclude, target.Format(dateLayout))
				continue
			}
			// Un día ajeno en una fecha objetivo es el error más caro del seed
			// (se pisa una sesión real), así que aborta nombrando la fecha. Con
			// --force el operador lo asume explícitamente: el stamp reemplaza el
			// día por uno nuevo y el feedback que tenía deja de estar visible en
			// el dashboard (la instancia vieja queda protegida del borrado).
			if !s.opts.force {
				return nil, fmt.Errorf(
					"la fecha %s ya tiene un día de calendario que no salió de este seed (plan %v). "+
						"Elegí otra ventana con --start, o pasá --force si querés que el stamp lo sobrescriba",
					target.Format(dateLayout), day.SourcePlanID)
			}
			customlogger.Warn(nil, "seed dashboard: --force pisa un día ajeno del calendario",
				customlogger.Tag("fecha", target.Format(dateLayout)),
				customlogger.Tag("plan_ajeno", fmt.Sprintf("%v", day.SourcePlanID)))
		}

		response, err := calendarService.Stamp(nil, s.groupID, s.ownerID, calendar.StampRequest{
			PlanID:       planID,
			StartDate:    s.window.weekStart(weekIndex).Format(dateLayout),
			Force:        s.opts.force,
			ExcludeDates: exclude,
		})
		if err != nil {
			switch {
			case errors.Is(err, services.ErrCalendarStampConflict):
				return nil, fmt.Errorf("el stamp del plan %q chocó con días ya existentes: %w", template.Name, err)
			case errors.Is(err, services.ErrCalendarDayClosed):
				return nil, fmt.Errorf("el stamp del plan %q quiso escribir una fecha cerrada: %w", template.Name, err)
			case errors.Is(err, services.ErrCalendarPresencialCollision):
				return nil, fmt.Errorf("el stamp del plan %q chocó con una sesión presencial de otro equipo: %w", template.Name, err)
			}
			return nil, fmt.Errorf("error al estampar el plan %q: %w", template.Name, err)
		}
		stamped += len(response.Days)
		customlogger.Info(nil, "seed dashboard: plan estamparado",
			customlogger.Tag("plan", template.Name),
			customlogger.Tag("dias_creados", fmt.Sprintf("%d", len(response.Days))),
			customlogger.Tag("dias_reutilizados", fmt.Sprintf("%d", len(s.weekTargets(weekIndex))-len(response.Days))))
	}
	s.count("dias_estampados", int64(stamped))

	return s.loadSeededDays(planIDs)
}

// weekTargets son las dos fechas de una semana que se estamparan.
func (s *seeder) weekTargets(weekIndex int) []time.Time {
	start := s.window.weekStart(weekIndex)
	return []time.Time{
		start.AddDate(0, 0, planDayTuesday-1),
		start.AddDate(0, 0, planDayThursday-1),
	}
}

func (s *seeder) existingCalendarDays() (map[string]dbs.GroupCalendarDay, error) {
	rows, err := daos.NewGroupCalendarDayDao(s.db).FindByGroupAndRange(nil, s.groupID, s.window.from(), s.window.to())
	if err != nil {
		return nil, fmt.Errorf("error al leer el calendario del grupo: %w", err)
	}
	byDate := make(map[string]dbs.GroupCalendarDay, len(rows))
	for _, row := range rows {
		byDate[row.Date.Format(dateLayout)] = row
	}
	return byDate, nil
}

// instanceRow es una fila de la lectura de instancias del seed.
type instanceRow struct {
	Date             time.Time
	CalendarDayID    int64
	SessionInstance  *int64
	PresencialFrom   *time.Time
	ExerciseInstance int64
	Role             string
	RepeatCount      int
	InstanceName     string
	Kind             string
	Minutes          *int
	DistanceM        *int
	SourceExerciseID *int64
}

// loadSeededDays arma los días con sus bloques leyendo las instancias que
// quedaron en la base (recién creadas o reutilizadas de una corrida anterior).
// El filtro por source_plan_id es lo que mantiene el resultado en exactamente los
// 4 días del seed aunque el grupo tenga otros días cargados en la ventana.
func (s *seeder) loadSeededDays(planIDs map[string]int64) ([]seededDay, error) {
	ids := make([]int64, 0, len(planIDs))
	for _, id := range planIDs {
		ids = append(ids, id)
	}
	var rows []instanceRow
	if err := s.db.Raw(`
		SELECT gcd.date                  AS date,
		       gcd.id                    AS calendar_day_id,
		       gcd.session_instance_id   AS session_instance,
		       gcd.presencial_time_from  AS presencial_from,
		       sei.exercise_instance_id  AS exercise_instance,
		       sei.role                  AS role,
		       sei.repeat_count          AS repeat_count,
		       ei.name                   AS instance_name,
		       ei.kind                   AS kind,
		       ei.minutes                AS minutes,
		       ei.distance_m             AS distance_m,
		       ei.source_exercise_id     AS source_exercise_id
		FROM group_calendar_days gcd
		JOIN session_exercise_instances sei ON sei.session_instance_id = gcd.session_instance_id
		JOIN exercise_instances ei          ON ei.id = sei.exercise_instance_id
		WHERE gcd.group_id = ? AND gcd.date BETWEEN ? AND ? AND gcd.source_plan_id IN ?
		ORDER BY gcd.date, sei.id`, s.groupID, s.window.from(), s.window.to(), ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("error al leer las instancias sembradas: %w", err)
	}

	keyByID := make(map[int64]string, len(s.catalogIDs))
	for key, id := range s.catalogIDs {
		keyByID[id] = key
	}
	keyByName := make(map[string]string, len(catalog))
	for _, item := range catalog {
		keyByName[dashboard.NormalizeExerciseName(item.Name)] = item.Key
	}

	byDate := make(map[string]*seededDay, seedSessionsCount)
	order := make([]string, 0, seedSessionsCount)
	for _, row := range rows {
		if row.SessionInstance == nil {
			continue
		}
		dateKey := row.Date.Format(dateLayout)
		day, seen := byDate[dateKey]
		if !seen {
			day = &seededDay{
				Date:            row.Date,
				CalendarDayID:   row.CalendarDayID,
				SessionInstance: *row.SessionInstance,
				PresencialFrom:  row.PresencialFrom,
			}
			byDate[dateKey] = day
			order = append(order, dateKey)
		}
		key, ok := resolveCatalogKey(row.SourceExerciseID, row.InstanceName, keyByID, keyByName)
		if !ok {
			// Un bloque cuyo ejercicio no es del catálogo del seed (una sesión
			// vieja del grupo, por ejemplo): no se le genera feedback.
			continue
		}
		day.Blocks = append(day.Blocks, instanceBlock{
			ExerciseKey:      key,
			ExerciseInstance: row.ExerciseInstance,
			Role:             row.Role,
			Repeat:           row.RepeatCount,
			Kind:             row.Kind,
			Minutes:          row.Minutes,
			DistanceM:        row.DistanceM,
			Name:             row.InstanceName,
		})
	}

	days := make([]seededDay, 0, len(order))
	for _, dateKey := range order {
		days = append(days, *byDate[dateKey])
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date.Before(days[j].Date) })
	for i := range days {
		days[i].DayIndex = i
	}
	return days, validateDays(days)
}

// validateDays es el chequeo de cordura del stamp: si no quedaron las 4 sesiones,
// o si un día quedó sin bloque principal, mejor fallar acá que dejar un dataset
// donde el dashboard no muestra nada sin decir por qué.
func validateDays(days []seededDay) error {
	if len(days) != seedSessionsCount {
		return fmt.Errorf("quedaron %d días de calendario del seed y se esperaban %d: revisá la ventana con --start",
			len(days), seedSessionsCount)
	}
	for _, day := range days {
		mains := 0
		for _, block := range day.Blocks {
			if block.Role == string(constants.SessionExerciseRoleMain) {
				mains++
			}
		}
		if mains == 0 {
			return fmt.Errorf("el día %s quedó sin bloque principal: el dashboard no contaría ninguna serie",
				day.Date.Format(dateLayout))
		}
	}
	return nil
}

// resolveCatalogKey traduce una ExerciseInstance a la clave del seed. Primero por
// source_exercise_id; si está en NULL (una instancia anterior a la migración), por
// nombre normalizado — el mismo mecanismo que el fallback del dashboard, así que el
// seed sigue funcionando después de una corrida que dejó filas legacy.
func resolveCatalogKey(sourceExerciseID *int64, instanceName string, byID map[int64]string, byName map[string]string) (string, bool) {
	if sourceExerciseID != nil {
		if key, ok := byID[*sourceExerciseID]; ok {
			return key, true
		}
	}
	key, ok := byName[dashboard.NormalizeExerciseName(instanceName)]
	return key, ok
}
