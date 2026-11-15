package main

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils/gametime"
)

type membershipView struct {
	Membership dbs.GroupUser
	Group      dbs.Group
	Team       dbs.Team
}

type scope struct {
	user          dbs.User
	primary       membershipView
	memberships   []membershipView
	teamID        int64
	groupID       int64
	instanceByDay map[int]int64
	conflicts     []string
}

// resolveScope resuelve la identidad y el grupo primario. Es la parte común
// entre el modo estricto y el merge: no aborta por datos preexistentes.
func resolveScope(db *gorm.DB, opts options) (*scope, error) {
	var users []dbs.User
	if err := db.Where("email = ?", opts.email).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("buscar usuario por email: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("no existe un usuario con email %s", opts.email)
	}
	if len(users) > 1 {
		return nil, fmt.Errorf("hay %d usuarios con email %s; se aborta", len(users), opts.email)
	}
	user := users[0]
	if user.Status != "active" {
		return nil, fmt.Errorf("el usuario %s está %s; se requiere active", user.Email, user.Status)
	}

	var rows []dbs.GroupUser
	if err := db.Table("group_users AS gu").
		Select("gu.*").
		Joins("JOIN groups g ON g.id = gu.group_id AND g.deleted_at IS NULL").
		Joins("JOIN teams t ON t.id = g.team_id AND t.deleted_at IS NULL").
		Where("gu.user_id = ? AND gu.deleted_at IS NULL AND (gu.date_end IS NULL OR gu.date_end > NOW())", user.ID).
		Order("gu.group_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("buscar membresías activas: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("el usuario %s no tiene membresías activas en grupos/equipos no borrados", user.Email)
	}

	memberships := make([]membershipView, 0, len(rows))
	for _, row := range rows {
		var group dbs.Group
		if err := db.First(&group, row.GroupID).Error; err != nil {
			return nil, fmt.Errorf("leer grupo %d: %w", row.GroupID, err)
		}
		var team dbs.Team
		if err := db.First(&team, group.TeamID).Error; err != nil {
			return nil, fmt.Errorf("leer equipo %d: %w", group.TeamID, err)
		}
		memberships = append(memberships, membershipView{Membership: row, Group: group, Team: team})
	}
	sort.Slice(memberships, func(i, j int) bool { return memberships[i].Group.ID < memberships[j].Group.ID })
	primary := memberships[0]

	return &scope{
		user:          user,
		primary:       primary,
		memberships:   memberships,
		teamID:        primary.Team.ID,
		groupID:       primary.Group.ID,
		instanceByDay: map[int]int64{},
	}, nil
}

func inspectScope(db *gorm.DB, opts options, plan fixturePlan) (*scope, error) {
	s, err := resolveScope(db, opts)
	if err != nil {
		return nil, err
	}

	limit := plan.Anchor.AddDate(0, 0, -12)
	membershipDay := utcDateInCordoba(s.primary.Membership.DateStart)
	if membershipDay.After(limit) {
		return nil, fmt.Errorf("la membresía del usuario en el grupo %d empieza el %s; para alcanzar best=14 tiene que empezar el %s o antes",
			s.primary.Group.ID, membershipDay.Format("2006-01-02"), limit.Format("2006-01-02"))
	}

	markers := make([]string, 0, len(plan.Days))
	for _, day := range plan.Days {
		markers = append(markers, day.Marker)
	}
	var fixtureInstances []dbs.SessionInstance
	if err := db.Where("description IN ?", markers).Find(&fixtureInstances).Error; err != nil {
		return nil, fmt.Errorf("buscar instancias fixture existentes: %w", err)
	}
	byMarker := make(map[string][]dbs.SessionInstance, len(fixtureInstances))
	for _, instance := range fixtureInstances {
		if instance.Description == nil {
			continue
		}
		byMarker[*instance.Description] = append(byMarker[*instance.Description], instance)
	}
	for _, day := range plan.Days {
		matches := byMarker[day.Marker]
		if len(matches) > 1 {
			s.conflicts = append(s.conflicts, fmt.Sprintf("hay %d session_instances con el marker %s para el día %s", len(matches), day.Marker, day.Date.Format("2006-01-02")))
			continue
		}
		if len(matches) == 1 {
			s.instanceByDay[day.Offset] = matches[0].ID
		}
	}

	if err := inspectPrimaryCalendar(db, s, plan); err != nil {
		return nil, err
	}
	if err := inspectOtherGroups(db, s, plan); err != nil {
		return nil, err
	}
	if err := inspectForeignActivity(db, s, plan); err != nil {
		return nil, err
	}
	if err := inspectLockedAchievements(db, s); err != nil {
		return nil, err
	}
	if err := inspectExistingBestStreak(db, s, plan); err != nil {
		return nil, err
	}
	return s, nil
}

func inspectPrimaryCalendar(db *gorm.DB, s *scope, plan fixturePlan) error {
	first := plan.Days[len(plan.Days)-1].Date
	last := plan.Days[0].Date
	var days []dbs.GroupCalendarDay
	if err := db.Where("group_id = ? AND date >= ? AND date <= ?", s.groupID, first, last).Find(&days).Error; err != nil {
		return fmt.Errorf("leer calendario existente del grupo %d: %w", s.groupID, err)
	}
	byDate := make(map[string]dbs.GroupCalendarDay, len(days))
	for _, day := range days {
		byDate[utcDateInCordoba(day.Date).Format("2006-01-02")] = day
	}
	for _, day := range plan.Days {
		existing, ok := byDate[day.Date.Format("2006-01-02")]
		if !ok {
			continue
		}
		if existing.Kind != "training" || existing.SessionInstanceID == nil {
			s.conflicts = append(s.conflicts, fmt.Sprintf("el día %s del grupo %d ya existe con kind=%q e instancia=%v; no se pisa", day.Date.Format("2006-01-02"), s.groupID, existing.Kind, existing.SessionInstanceID))
			continue
		}
		var instance dbs.SessionInstance
		if err := db.First(&instance, *existing.SessionInstanceID).Error; err != nil {
			s.conflicts = append(s.conflicts, fmt.Sprintf("el día %s apunta a una instancia inexistente: %v", day.Date.Format("2006-01-02"), err))
			continue
		}
		if !isFixtureMarker(instance.Description) || *instance.Description != day.Marker {
			s.conflicts = append(s.conflicts, fmt.Sprintf("el día %s del grupo %d apunta a una instancia ajena al fixture", day.Date.Format("2006-01-02"), s.groupID))
		}
	}
	return nil
}

func inspectOtherGroups(db *gorm.DB, s *scope, plan fixturePlan) error {
	first := plan.Days[len(plan.Days)-1].Date
	last := plan.Days[0].Date
	for _, membership := range s.memberships {
		if membership.Group.ID == s.groupID {
			continue
		}
		var days []dbs.GroupCalendarDay
		if err := db.Where("group_id = ? AND date >= ? AND date <= ? AND kind = ?",
			membership.Group.ID, first, last, "training").Find(&days).Error; err != nil {
			return fmt.Errorf("leer días de training del grupo %d: %w", membership.Group.ID, err)
		}
		for _, day := range days {
			if day.SessionInstanceID == nil {
				s.conflicts = append(s.conflicts, fmt.Sprintf("el grupo activo %d tiene un día training sin instancia el %s", membership.Group.ID, utcDateInCordoba(day.Date).Format("2006-01-02")))
				continue
			}
			var count int64
			if err := db.Model(&dbs.RunnerSession{}).
				Where("session_instance_id = ? AND athlete_user_id = ? AND status = ?", *day.SessionInstanceID, s.user.ID, runnersession.RunnerSessionStatusFinished).
				Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				s.conflicts = append(s.conflicts, fmt.Sprintf("el grupo activo %d tiene un día training no completado el %s: afectaría la racha", membership.Group.ID, utcDateInCordoba(day.Date).Format("2006-01-02")))
			}
		}
	}
	return nil
}

func inspectForeignActivity(db *gorm.DB, s *scope, plan fixturePlan) error {
	fixtureIDs := make([]int64, 0, len(s.instanceByDay))
	for _, id := range s.instanceByDay {
		fixtureIDs = append(fixtureIDs, id)
	}

	finishedQuery := db.Model(&dbs.RunnerSession{}).Where("athlete_user_id = ? AND status = ?", s.user.ID, runnersession.RunnerSessionStatusFinished)
	if len(fixtureIDs) > 0 {
		finishedQuery = finishedQuery.Where("session_instance_id NOT IN ?", fixtureIDs)
	}
	var foreignFinished int64
	if err := finishedQuery.Count(&foreignFinished).Error; err != nil {
		return fmt.Errorf("contar sesiones finished ajenas: %w", err)
	}
	if foreignFinished > 0 {
		s.conflicts = append(s.conflicts, fmt.Sprintf("el usuario ya tiene %d sesiones finished fuera del fixture; los totales exactos no se pueden garantizar", foreignFinished))
	}

	distanceQuery := db.Table("workout_feedback").Where("athlete_user_id = ? AND deleted_at IS NULL AND distance_meters IS NOT NULL", s.user.ID)
	if len(fixtureIDs) > 0 {
		distanceQuery = distanceQuery.Where("assigned_session_id NOT IN ?", fixtureIDs)
	}
	var foreignDistance struct{ Total float64 }
	if err := distanceQuery.Select("COALESCE(SUM(distance_meters), 0) AS total").Scan(&foreignDistance).Error; err != nil {
		return fmt.Errorf("sumar distancia ajena: %w", err)
	}
	if foreignDistance.Total > 0 {
		s.conflicts = append(s.conflicts, fmt.Sprintf("el usuario ya tiene %.0fm de feedback fuera del fixture; el total de 65km no se puede garantizar", foreignDistance.Total))
	}

	gpsQuery := db.Table("workout_feedback_points AS p").
		Joins("JOIN workout_feedback wf ON wf.id = p.feedback_id").
		Where("wf.athlete_user_id = ?", s.user.ID)
	if len(fixtureIDs) > 0 {
		gpsQuery = gpsQuery.Where("p.session_instance_id NOT IN ?", fixtureIDs)
	}
	var foreignGPS int64
	if err := gpsQuery.Distinct("p.session_instance_id").Count(&foreignGPS).Error; err != nil {
		return fmt.Errorf("contar GPS ajeno: %w", err)
	}
	if foreignGPS > 0 {
		s.conflicts = append(s.conflicts, fmt.Sprintf("el usuario ya tiene %d sesiones GPS fuera del fixture", foreignGPS))
	}

	attendanceQuery := db.Table("attendances AS att").
		Joins("JOIN group_calendar_days gcd ON gcd.session_instance_id = att.training_session_id").
		Where("att.user_id = ?", s.user.ID)
	if len(fixtureIDs) > 0 {
		attendanceQuery = attendanceQuery.Where("att.training_session_id NOT IN ?", fixtureIDs)
	}
	var foreignAttendance int64
	if err := attendanceQuery.Distinct("gcd.date").Count(&foreignAttendance).Error; err != nil {
		return fmt.Errorf("contar asistencias ajenas: %w", err)
	}
	if foreignAttendance > 0 {
		s.conflicts = append(s.conflicts, fmt.Sprintf("el usuario ya tiene %d días de asistencia fuera del fixture", foreignAttendance))
	}
	return nil
}

func inspectLockedAchievements(db *gorm.DB, s *scope) error {
	var rows []dbs.UserAchievementProgress
	if err := db.Where("user_id = ?", s.user.ID).Find(&rows).Error; err != nil {
		return fmt.Errorf("leer logros existentes: %w", err)
	}
	allowed := make(map[string]struct{}, len(expectedUnlockedAchievements))
	for _, id := range expectedUnlockedAchievements {
		allowed[id] = struct{}{}
	}
	for _, row := range rows {
		if _, ok := allowed[row.AchievementID]; ok {
			continue
		}
		s.conflicts = append(s.conflicts, fmt.Sprintf("el logro %s ya está desbloqueado y no se puede revocar: el fixture quedaría con menos de 8 bloqueados", row.AchievementID))
	}
	return nil
}

func inspectExistingBestStreak(db *gorm.DB, s *scope, plan fixturePlan) error {
	snapshot, err := services.NewStreakService(daos.NewGamificationDao(db)).ComputeSnapshot(nil, s.user.ID)
	if err != nil {
		return fmt.Errorf("calcular racha existente: %w", err)
	}
	if snapshot.StrictBest > plan.ExpectedStreakBest {
		s.conflicts = append(s.conflicts, fmt.Sprintf("el usuario ya tiene best estricta=%d (>%d); el fixture no puede dejarlo en 14 sin revocar historia", snapshot.StrictBest, plan.ExpectedStreakBest))
	}
	if snapshot.FlexibleBest > plan.ExpectedStreakBest {
		s.conflicts = append(s.conflicts, fmt.Sprintf("el usuario ya tiene best flexible=%d (>%d); el fixture no puede dejarlo en 14 sin revocar historia", snapshot.FlexibleBest, plan.ExpectedStreakBest))
	}
	return nil
}

// utcDateInCordoba replica la convención de streak_service.utcDateToDay: las
// columnas date se escanean como medianoche UTC (el DSN fija TimeZone=UTC), y
// el día lógico del histórico es esa fecha en TZ Córdoba.
func utcDateInCordoba(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, gametime.Location)
}

func printScopeConflicts(s *scope) error {
	if len(s.conflicts) == 0 {
		fmt.Println("  preflight: sin conflictos")
		return nil
	}
	fmt.Println("  preflight: conflictos detectados (--apply abortaría):")
	for _, conflict := range s.conflicts {
		fmt.Printf("    - %s\n", conflict)
	}
	return nil
}

var errPreflight = errors.New("preflight rechazado")

func (s *scope) requireNoConflicts() error {
	if len(s.conflicts) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", errPreflight, s.conflicts[0])
}
