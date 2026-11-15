package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
)

// chunkSize acota los IN (...) para no pasarse del límite de parámetros de
// Postgres cuando un rango de fechas arrastra miles de filas.
const chunkSize = 500

// scope es el pedido ya validado: qué team, desde/hasta cuándo y qué días.
type scope struct {
	teamID int64
	from   time.Time
	to     *time.Time
	kinds  []string
	groups []int64 // vacío = todos los grupos activos del team
}

type groupRow struct {
	ID   int64
	Name string
}

type feedbackRow struct {
	ID            int64
	AthleteUserID int64
	SessionDate   time.Time
}

type attendanceRow struct {
	ID                int64
	UserID            int64
	TrainingSessionID int64
}

type runnerSessionRow struct {
	ID                int64
	AthleteUserID     int64
	SessionInstanceID int64
}

type sessionLinkRow struct {
	SessionInstanceID  int64 `gorm:"column:session_instance_id"`
	ExerciseInstanceID int64 `gorm:"column:exercise_instance_id"`
}

type gamificationCounts struct {
	Achievements int64
	StreakStates int64
	Unseen       int64
	Stamps       int64
}

// instanceCleanup separa las instancias que se pueden borrar de las que hay
// que conservar porque algo fuera del rango las sigue referenciando.
type instanceCleanup struct {
	Deletable   []int64
	ExerciseIDs []int64
	Kept        map[int64]string
}

type plan struct {
	scope           scope
	groups          []groupRow
	groupIDs        []int64
	days            []dbs.GroupCalendarDay
	dayIDs          []int64
	daysByKind      map[string]int
	instanceIDs     []int64
	feedback        []feedbackRow
	feedbackIDs     []int64
	points          int64
	attendances     []attendanceRow
	runnerSessions  []runnerSessionRow
	athletes        []int64
	athleteFeedback map[int64]int
	gamification    gamificationCounts
	cleanup         instanceCleanup
}

// buildPlan es 100% read-only: junta todo lo que borraría y lo que conservaría,
// para que el dry-run sea la confirmación real antes de --apply.
func buildPlan(db *gorm.DB, sc scope) (*plan, error) {
	p := &plan{scope: sc, daysByKind: map[string]int{}, athleteFeedback: map[int64]int{}}
	if err := loadGroups(db, p); err != nil {
		return nil, err
	}
	if err := loadDays(db, p); err != nil {
		return nil, err
	}
	if err := loadFeedback(db, p); err != nil {
		return nil, err
	}
	if err := loadPointsCount(db, p); err != nil {
		return nil, err
	}
	if err := loadAttendances(db, p); err != nil {
		return nil, err
	}
	if err := loadRunnerSessions(db, p); err != nil {
		return nil, err
	}
	p.collectAthletes()
	if err := loadGamificationCounts(db, p); err != nil {
		return nil, err
	}
	// Los días, el feedback y las asistencias todavía existen: se excluyen de la
	// búsqueda de referencias o ninguna instancia sería borrable en el plan.
	cleanup, err := classifyInstances(db, p.instanceIDs, pendingDeletes{
		DayIDs:           p.dayIDs,
		FeedbackIDs:      p.feedbackIDs,
		AttendanceIDs:    attendanceIDs(p.attendances),
		RunnerSessionIDs: runnerSessionIDs(p.runnerSessions),
	})
	if err != nil {
		return nil, err
	}
	p.cleanup = cleanup
	return p, nil
}

func loadGroups(db *gorm.DB, p *plan) error {
	q := db.Model(&dbs.Group{}).
		Select("id, name").
		Where("team_id = ? AND deleted_at IS NULL", p.scope.teamID).
		Order("id")
	if len(p.scope.groups) > 0 {
		q = q.Where("id IN ?", p.scope.groups)
	}
	if err := q.Scan(&p.groups).Error; err != nil {
		return fmt.Errorf("leer grupos del team %d: %w", p.scope.teamID, err)
	}
	if len(p.groups) == 0 {
		return fmt.Errorf("el team %d no tiene grupos activos que matcheen el pedido", p.scope.teamID)
	}
	if len(p.scope.groups) > 0 {
		found := make(map[int64]struct{}, len(p.groups))
		for _, g := range p.groups {
			found[g.ID] = struct{}{}
		}
		missing := make([]string, 0, len(p.scope.groups))
		for _, id := range p.scope.groups {
			if _, ok := found[id]; !ok {
				missing = append(missing, strconv.FormatInt(id, 10))
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("grupos %s no son del team %d (o están dados de baja)", strings.Join(missing, ", "), p.scope.teamID)
		}
	}
	for _, g := range p.groups {
		p.groupIDs = append(p.groupIDs, g.ID)
	}
	return nil
}

func loadDays(db *gorm.DB, p *plan) error {
	q := db.Model(&dbs.GroupCalendarDay{}).
		Where("group_id IN ? AND date >= ?::date AND kind IN ?",
			p.groupIDs, p.scope.from.Format("2006-01-02"), p.scope.kinds)
	if p.scope.to != nil {
		q = q.Where("date <= ?::date", p.scope.to.Format("2006-01-02"))
	}
	if err := q.Order("date, group_id").Find(&p.days).Error; err != nil {
		return fmt.Errorf("leer días de calendario: %w", err)
	}
	seen := map[int64]struct{}{}
	for _, d := range p.days {
		p.dayIDs = append(p.dayIDs, d.ID)
		p.daysByKind[d.Kind]++
		if d.SessionInstanceID == nil {
			continue
		}
		if _, dup := seen[*d.SessionInstanceID]; dup {
			continue
		}
		seen[*d.SessionInstanceID] = struct{}{}
		p.instanceIDs = append(p.instanceIDs, *d.SessionInstanceID)
	}
	return nil
}

func loadFeedback(db *gorm.DB, p *plan) error {
	q := db.Model(&dbs.WorkoutFeedback{}).
		Select("id, athlete_user_id, session_date").
		Where("team_id = ? AND session_date >= ?::date", p.scope.teamID, p.scope.from.Format("2006-01-02"))
	if p.scope.to != nil {
		q = q.Where("session_date <= ?::date", p.scope.to.Format("2006-01-02"))
	}
	if err := q.Order("session_date, id").Scan(&p.feedback).Error; err != nil {
		return fmt.Errorf("leer feedback del team %d: %w", p.scope.teamID, err)
	}
	for _, f := range p.feedback {
		p.feedbackIDs = append(p.feedbackIDs, f.ID)
	}
	return nil
}

func loadPointsCount(db *gorm.DB, p *plan) error {
	for _, chunk := range chunkIDs(p.feedbackIDs) {
		var n int64
		if err := db.Model(&dbs.WorkoutFeedbackPoint{}).
			Where("feedback_id IN ?", chunk).
			Count(&n).Error; err != nil {
			return fmt.Errorf("contar puntos GPS: %w", err)
		}
		p.points += n
	}
	return nil
}

func loadAttendances(db *gorm.DB, p *plan) error {
	for _, chunk := range chunkIDs(p.instanceIDs) {
		var rows []attendanceRow
		if err := db.Model(&dbs.Attendance{}).
			Select("id, user_id, training_session_id").
			Where("team_id = ? AND training_session_id IN ?", p.scope.teamID, chunk).
			Scan(&rows).Error; err != nil {
			return fmt.Errorf("leer asistencias: %w", err)
		}
		p.attendances = append(p.attendances, rows...)
	}
	return nil
}

func loadRunnerSessions(db *gorm.DB, p *plan) error {
	for _, chunk := range chunkIDs(p.instanceIDs) {
		var rows []runnerSessionRow
		if err := db.Model(&dbs.RunnerSession{}).
			Select("id, athlete_user_id, session_instance_id").
			Where("session_instance_id IN ?", chunk).
			Scan(&rows).Error; err != nil {
			return fmt.Errorf("leer runner_session: %w", err)
		}
		p.runnerSessions = append(p.runnerSessions, rows...)
	}
	return nil
}

// collectAthletes arma la lista de atletas afectados: los que tienen feedback,
// runner_session o asistencia en el alcance. Es a quienes se les limpia la
// gamificación derivada.
func (p *plan) collectAthletes() {
	seen := map[int64]struct{}{}
	add := func(id int64) {
		if id <= 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		p.athletes = append(p.athletes, id)
	}
	for _, f := range p.feedback {
		add(f.AthleteUserID)
		p.athleteFeedback[f.AthleteUserID]++
	}
	for _, a := range p.attendances {
		add(a.UserID)
	}
	for _, rs := range p.runnerSessions {
		add(rs.AthleteUserID)
	}
	sort.Slice(p.athletes, func(i, j int) bool { return p.athletes[i] < p.athletes[j] })
}

func loadGamificationCounts(db *gorm.DB, p *plan) error {
	if len(p.athletes) == 0 {
		return nil
	}
	until := untilExclusive(p.scope.to)
	var err error
	if p.gamification.Achievements, err = countByAthletes(db, &dbs.UserAchievementProgress{}, p.athletes, "unlocked_at", p.scope.from, until); err != nil {
		return fmt.Errorf("contar logros: %w", err)
	}
	if p.gamification.StreakStates, err = countByAthletes(db, &dbs.GamificationStreakState{}, p.athletes, "", p.scope.from, nil); err != nil {
		return fmt.Errorf("contar rachas: %w", err)
	}
	if p.gamification.Unseen, err = countByAthletes(db, &dbs.GamificationUnseen{}, p.athletes, "created_at", p.scope.from, until); err != nil {
		return fmt.Errorf("contar unseen: %w", err)
	}
	if p.gamification.Stamps, err = countByAthletes(db, &dbs.GamificationStamp{}, p.athletes, "granted_at", p.scope.from, until); err != nil {
		return fmt.Errorf("contar estampas: %w", err)
	}
	return nil
}

// pendingDeletes son las filas que este mismo script ya tiene en el plan. Al
// clasificar instancias hay que excluirlas, porque en el dry-run todavía están
// en la DB y si no se ven como "referencias de otro rango" que en realidad se
// van a borrar en el mismo --apply.
type pendingDeletes struct {
	DayIDs           []int64
	FeedbackIDs      []int64
	AttendanceIDs    []int64
	RunnerSessionIDs []int64
}

// classifyInstances decide, instancia por instancia, si se puede borrar sin
// tocar datos de otro rango. Es el mismo criterio que deleteSupersededInstance
// (D10) pero en lote: feedback activo sobre la sesión o sobre cualquiera de sus
// ejercicios la protege. Se agregan protecciones propias del script más
// conservadoras que la API: otro día de calendario, una competencia de ruta
// anclada, o cualquier asistencia/runner_session que no se vaya a borrar.
func classifyInstances(db *gorm.DB, instanceIDs []int64, pending pendingDeletes) (instanceCleanup, error) {
	out := instanceCleanup{Kept: map[int64]string{}}
	if len(instanceIDs) == 0 {
		return out, nil
	}
	for _, chunk := range chunkIDs(instanceIDs) {
		var links []sessionLinkRow
		if err := db.Raw(`SELECT session_instance_id, exercise_instance_id
			FROM session_exercise_instances
			WHERE session_instance_id IN ?`, chunk).Scan(&links).Error; err != nil {
			return out, fmt.Errorf("leer vínculos de ejercicios: %w", err)
		}
		exercisesBySession := make(map[int64][]int64, len(chunk))
		allExercises := make([]int64, 0, len(links))
		for _, l := range links {
			exercisesBySession[l.SessionInstanceID] = append(exercisesBySession[l.SessionInstanceID], l.ExerciseInstanceID)
			allExercises = append(allExercises, l.ExerciseInstanceID)
		}

		var dayRefs []int64
		q := db.Model(&dbs.GroupCalendarDay{}).Where("session_instance_id IN ?", chunk)
		if len(pending.DayIDs) > 0 {
			q = q.Where("id NOT IN ?", pending.DayIDs)
		}
		if err := q.Distinct().Pluck("session_instance_id", &dayRefs).Error; err != nil {
			return out, fmt.Errorf("leer días que referencian instancias: %w", err)
		}

		var attendanceRefs []int64
		attendanceQuery := db.Model(&dbs.Attendance{}).Where("training_session_id IN ?", chunk)
		if len(pending.AttendanceIDs) > 0 {
			attendanceQuery = attendanceQuery.Where("id NOT IN ?", pending.AttendanceIDs)
		}
		if err := attendanceQuery.Distinct().Pluck("training_session_id", &attendanceRefs).Error; err != nil {
			return out, fmt.Errorf("leer asistencias que referencian instancias: %w", err)
		}

		var runnerRefs []int64
		runnerQuery := db.Model(&dbs.RunnerSession{}).Where("session_instance_id IN ?", chunk)
		if len(pending.RunnerSessionIDs) > 0 {
			runnerQuery = runnerQuery.Where("id NOT IN ?", pending.RunnerSessionIDs)
		}
		if err := runnerQuery.Distinct().Pluck("session_instance_id", &runnerRefs).Error; err != nil {
			return out, fmt.Errorf("leer runner_session que referencian instancias: %w", err)
		}

		var competitionRefs []int64
		if err := db.Model(&dbs.Competition{}).
			Where("route_session_instance_id IN ?", chunk).
			Distinct().
			Pluck("route_session_instance_id", &competitionRefs).Error; err != nil {
			return out, fmt.Errorf("leer competencias que referencian instancias: %w", err)
		}

		var sessionFeedback []int64
		sessionFeedbackQuery := db.Model(&dbs.WorkoutFeedback{}).
			Where("assigned_session_id IN ? AND deleted_at IS NULL", chunk)
		if len(pending.FeedbackIDs) > 0 {
			sessionFeedbackQuery = sessionFeedbackQuery.Where("id NOT IN ?", pending.FeedbackIDs)
		}
		if err := sessionFeedbackQuery.
			Distinct().
			Pluck("assigned_session_id", &sessionFeedback).Error; err != nil {
			return out, fmt.Errorf("leer feedback por sesión: %w", err)
		}

		exerciseFeedback := map[int64]struct{}{}
		for _, exerciseChunk := range chunkIDs(allExercises) {
			var ids []int64
			exerciseFeedbackQuery := db.Model(&dbs.WorkoutFeedback{}).
				Where("assigned_exercise_id IN ? AND deleted_at IS NULL", exerciseChunk)
			if len(pending.FeedbackIDs) > 0 {
				exerciseFeedbackQuery = exerciseFeedbackQuery.Where("id NOT IN ?", pending.FeedbackIDs)
			}
			if err := exerciseFeedbackQuery.
				Distinct().
				Pluck("assigned_exercise_id", &ids).Error; err != nil {
				return out, fmt.Errorf("leer feedback por ejercicio: %w", err)
			}
			for _, id := range ids {
				exerciseFeedback[id] = struct{}{}
			}
		}

		daySet := int64Set(dayRefs)
		competitionSet := int64Set(competitionRefs)
		sessionFeedbackSet := int64Set(sessionFeedback)
		attendanceSet := int64Set(attendanceRefs)
		runnerSet := int64Set(runnerRefs)

		for _, id := range chunk {
			reason := ""
			switch {
			case hasKey(daySet, id):
				reason = "otro día de calendario la referencia"
			case hasKey(competitionSet, id):
				reason = "una competencia de ruta la referencia"
			case hasKey(sessionFeedbackSet, id):
				reason = "tiene feedback fuera del rango"
			case hasKey(attendanceSet, id):
				reason = "tiene asistencias fuera del alcance"
			case hasKey(runnerSet, id):
				reason = "tiene runner_session fuera del alcance"
			default:
				for _, exerciseID := range exercisesBySession[id] {
					if hasKey(exerciseFeedback, exerciseID) {
						reason = "un ejercicio tiene feedback fuera del rango"
						break
					}
				}
			}
			if reason != "" {
				out.Kept[id] = reason
				continue
			}
			out.Deletable = append(out.Deletable, id)
			out.ExerciseIDs = append(out.ExerciseIDs, exercisesBySession[id]...)
		}
	}
	sort.Slice(out.Deletable, func(i, j int) bool { return out.Deletable[i] < out.Deletable[j] })
	return out, nil
}

func printPlan(p *plan) {
	window := "desde " + p.scope.from.Format("2006-01-02") + " (sin tope)"
	if p.scope.to != nil {
		window = fmt.Sprintf("del %s al %s (inclusive)", p.scope.from.Format("2006-01-02"), p.scope.to.Format("2006-01-02"))
	}
	fmt.Printf("Alcance: team %d · %s · kinds: %s\n", p.scope.teamID, window, strings.Join(p.scope.kinds, ", "))
	fmt.Printf("Grupos: %s\n", formatGroups(p.groups))

	fmt.Printf("\nDías de calendario a borrar: %d %s\n", len(p.days), formatKindCounts(p.daysByKind))
	fmt.Printf("Instancias: %d → se borran %d, se conservan %d\n",
		len(p.instanceIDs), len(p.cleanup.Deletable), len(p.cleanup.Kept))
	if len(p.cleanup.Kept) > 0 {
		ids := make([]int64, 0, len(p.cleanup.Kept))
		for id := range p.cleanup.Kept {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			fmt.Printf("  instancia %d conservada: %s\n", id, p.cleanup.Kept[id])
		}
	}

	fmt.Printf("\nFeedback a borrar: %d filas · puntos GPS: %d · atletas afectados: %d\n",
		len(p.feedback), p.points, len(p.athletes))
	if len(p.athleteFeedback) > 0 {
		fmt.Printf("  feedback por atleta: %s\n", formatAthleteCounts(p.athleteFeedback, p.athletes))
	}
	fmt.Printf("Runner sessions: %d · asistencias: %d\n", len(p.runnerSessions), len(p.attendances))

	fmt.Printf("\nGamificación a limpiar (atletas afectados): logros %d · estampas %d · unseen %d · rachas %d (sin filtro de fecha)\n",
		p.gamification.Achievements, p.gamification.Stamps, p.gamification.Unseen, p.gamification.StreakStates)

	if len(p.days) == 0 && len(p.feedback) == 0 && len(p.runnerSessions) == 0 && len(p.attendances) == 0 {
		fmt.Println("\nNo hay nada que borrar en el alcance pedido.")
	}
}

func formatGroups(groups []groupRow) string {
	if len(groups) == 0 {
		return "(ninguno)"
	}
	const maxListed = 5
	parts := make([]string, 0, maxListed)
	for i, g := range groups {
		if i >= maxListed {
			break
		}
		parts = append(parts, fmt.Sprintf("%d %q", g.ID, g.Name))
	}
	out := strings.Join(parts, ", ")
	if len(groups) > maxListed {
		out += fmt.Sprintf(" y %d más", len(groups)-maxListed)
	}
	return out
}

func formatKindCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	kinds := make([]string, 0, len(counts))
	for kind := range counts {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	parts := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		parts = append(parts, fmt.Sprintf("%s=%d", kind, counts[kind]))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func formatAthleteCounts(counts map[int64]int, athletes []int64) string {
	const maxListed = 10
	parts := make([]string, 0, maxListed)
	for i, id := range athletes {
		if i >= maxListed {
			break
		}
		if n := counts[id]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d:%d", id, n))
		}
	}
	out := strings.Join(parts, " ")
	if len(counts) > maxListed {
		out += fmt.Sprintf(" … (+%d atletas)", len(counts)-maxListed)
	}
	return out
}

func parseIDs(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("id inválido %q", part)
		}
		out = append(out, id)
	}
	return out, nil
}

// untilExclusive convierte el --to inclusivo (YYYY-MM-DD) en el límite
// exclusivo que necesitan las columnas timestamptz.
func untilExclusive(to *time.Time) *time.Time {
	if to == nil {
		return nil
	}
	next := to.AddDate(0, 0, 1)
	return &next
}

func chunkIDs(ids []int64) [][]int64 {
	if len(ids) == 0 {
		return nil
	}
	out := make([][]int64, 0, (len(ids)+chunkSize-1)/chunkSize)
	for start := 0; start < len(ids); start += chunkSize {
		end := start + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[start:end])
	}
	return out
}

func int64Set(ids []int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

func hasKey(set map[int64]struct{}, id int64) bool {
	_, ok := set[id]
	return ok
}
