package main

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// cleanupScope es lo único que el cleanup puede tocar. Todo lo demás de la base
// es ajeno por definición (tarea 8.2).
type cleanupScope struct {
	TeamID  int64
	GroupID int64
	OwnerID int64
	From    time.Time
	To      time.Time
}

// cleanupStep es un DELETE con su WHERE. El mismo WHERE sirve para el conteo
// previo y para el borrado, así que el número que se imprime antes de borrar es
// exactamente el número de filas que se borran (tarea 8.3).
type cleanupStep struct {
	Table string
	Where string
	Args  []interface{}
}

// cleanupResult son las filas afectadas por tabla.
type cleanupResult map[string]int64

// runCleanupMode es la entrada de --cleanup: cuenta, muestra y recién ahí borra,
// y solo con --yes explícito (tarea 8.3).
func runCleanupMode(db *gorm.DB, opts options) {
	scope, err := resolveCleanupScope(db, opts)
	if err != nil {
		fatal("%v", err)
	}
	steps, err := buildCleanupSteps(db, scope)
	if err != nil {
		fatal("%v", err)
	}
	counts, err := countCleanupSteps(db, steps)
	if err != nil {
		fatal("%v", err)
	}

	fmt.Printf("Limpieza del seed · equipo %d · grupo %d · %s..%s\n",
		scope.TeamID, scope.GroupID, scope.From.Format(dateLayout), scope.To.Format(dateLayout))
	total := int64(0)
	for _, table := range cleanupOrder {
		total += counts[table]
	}
	if total == 0 {
		fmt.Println("  no hay nada del seed en esa ventana: nada que borrar")
		return
	}
	for _, table := range cleanupOrder {
		fmt.Printf("  %-28s %d\n", table, counts[table])
	}
	fmt.Println("  group_users NO se borra: el seed solo ajusta date_start (ver RUNBOOK.md).")

	if opts.dryRun {
		fmt.Println("\n[dry-run] no se borró nada.")
		return
	}
	if !opts.yes {
		fmt.Println("\nFalta --yes: no se borra nada. Volvé con --cleanup --yes cuando quieras confirmar.")
		return
	}

	affected, err := executeCleanup(db, steps)
	if err != nil {
		fatal("la limpieza falló y se revirtió toda: %v", err)
	}
	fmt.Println("\nBorrado:")
	for _, table := range cleanupOrder {
		fmt.Printf("  %-28s %d\n", table, affected[table])
	}
	fmt.Println("\nSi querés volver el estado anterior, fijá date_start de los corredores a los valores que")
	fmt.Println("imprimió el seed (RUNBOOK.md los lista) o corré make demo-restore.")
}

// resolveCleanupScope arma el scope y calcula la ventana: sin --start son las dos
// semanas completas anteriores, la misma que siembra el seed.
func resolveCleanupScope(db *gorm.DB, opts options) (cleanupScope, error) {
	teamID, groupID := opts.teamID, opts.groupID
	team, err := daos.NewTeamDao(db).FindByID(nil, teamID)
	if err != nil {
		return cleanupScope{}, fmt.Errorf("error al leer el equipo %d: %w", teamID, err)
	}
	if team == nil {
		return cleanupScope{}, fmt.Errorf("el equipo %d no existe", teamID)
	}
	group, err := daos.NewGroupDao(db).FindByIDAndTeamID(nil, groupID, teamID)
	if err != nil {
		return cleanupScope{}, fmt.Errorf("error al leer el grupo %d: %w", groupID, err)
	}
	if group == nil {
		return cleanupScope{}, fmt.Errorf("el grupo %d no pertenece al equipo %d", groupID, teamID)
	}
	window, err := resolveWindow(opts.start)
	if err != nil {
		return cleanupScope{}, err
	}
	return cleanupScope{
		TeamID:  teamID,
		GroupID: groupID,
		OwnerID: team.OwnerID,
		From:    window.from(),
		To:      window.to(),
	}, nil
}

// cleanupOrder es el orden de borrado: inverso de FK. runner_session va PRIMERO y
// es obligatorio —el seed crea esas filas en finished y borrarlas al final dejaría
// filas apuntando a session_instances inexistentes.
var cleanupOrder = []string{
	"runner_session",
	"workout_feedback_points",
	"workout_feedback",
	"session_exercise_instances",
	"exercise_instances",
	"session_instances",
	"group_calendar_days",
	"plan_days",
	"training_plans",
	"session_exercises",
	"sessions",
	"exercises",
}

// buildCleanupSteps arma los WHERE de cada tabla en el orden de borrado.
//
// Cada paso está acotado por (grupo/equipo + ventana temporal + marcador del
// seed), y los pasos que borran instancias o catálogo agregan una condición de
// exclusividad: si una sesión instancia o un ejercicio del catálogo está
// referenciado por algo que el seed no creó, no se borra. Preferir dejar filas a
// borrar catálogo de alguien.
func buildCleanupSteps(db *gorm.DB, scope cleanupScope) ([]cleanupStep, error) {
	planIDs, err := seedPlanIDs(db, scope)
	if err != nil {
		return nil, err
	}
	if len(planIDs) == 0 {
		return nil, nil
	}
	_, instanceIDs, err := seedCalendarArtifacts(db, scope, planIDs)
	if err != nil {
		return nil, err
	}
	// Si no hay días propios, el catálogo se limpia igual: puede que el seed se
	// haya interrumpido entre el catálogo y el stamp.
	exerciseInstanceIDs, err := seedExerciseInstanceIDs(db, instanceIDs)
	if err != nil {
		return nil, err
	}
	sessionIDs, err := seedSessionIDs(db, planIDs)
	if err != nil {
		return nil, err
	}
	exerciseIDs, err := seedExerciseIDs(db, scope, sessionIDs)
	if err != nil {
		return nil, err
	}

	steps := make([]cleanupStep, 0, len(cleanupOrder))
	add := func(table, where string, args ...interface{}) {
		steps = append(steps, cleanupStep{Table: table, Where: where, Args: args})
	}
	if len(instanceIDs) > 0 {
		add("runner_session", "session_instance_id IN ?", instanceIDs)
		add("workout_feedback_points", "session_instance_id IN ?", instanceIDs)
		add("workout_feedback", "assigned_session_id IN ? AND session_date BETWEEN ? AND ?",
			instanceIDs, scope.From, scope.To)
		add("session_exercise_instances", "session_instance_id IN ?", instanceIDs)
	}
	if len(exerciseInstanceIDs) > 0 {
		add("exercise_instances", "id IN ?", exerciseInstanceIDs)
	}
	if len(instanceIDs) > 0 {
		add("session_instances", "id IN ?", instanceIDs)
	}
	add("group_calendar_days", "group_id = ? AND source_plan_id IN ? AND date BETWEEN ? AND ?",
		scope.GroupID, planIDs, scope.From, scope.To)
	if len(planIDs) > 0 {
		add("plan_days", "plan_id IN ?", planIDs)
		add("training_plans", "id IN ? AND owner_id = ?", planIDs, scope.OwnerID)
	}
	if len(sessionIDs) > 0 {
		add("session_exercises", "session_id IN ?", sessionIDs)
		add("sessions", "id IN ? AND owner_id = ? AND description LIKE ?", sessionIDs, scope.OwnerID, "%"+seedMarker+"%")
	}
	if len(exerciseIDs) > 0 {
		add("exercises", "id IN ? AND owner_id = ? AND description LIKE ?",
			exerciseIDs, scope.OwnerID, "%"+seedMarker+"%")
	}
	return steps, nil
}

// seedPlanIDs son los planes que creó el seed. El marcador va en `description`
// porque el nombre podría coincidir con un plan propio del entrenador.
func seedPlanIDs(db *gorm.DB, scope cleanupScope) ([]int64, error) {
	var ids []int64
	if err := db.Model(&dbs.TrainingPlan{}).
		Where("owner_id = ? AND description LIKE ?", scope.OwnerID, "%"+seedMarker+"%").
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("error al leer los planes del seed: %w", err)
	}
	return ids, nil
}

// seedCalendarArtifacts son los días de calendario y las instancias que el seed
// dejó en la ventana. Solo se consideran los días con source_plan_id del seed:
// un día propio del entrenador que caiga en la ventana no se toca.
func seedCalendarArtifacts(db *gorm.DB, scope cleanupScope, planIDs []int64) (dayIDs, instanceIDs []int64, err error) {
	type row struct {
		ID                int64
		SessionInstanceID *int64
	}
	var rows []row
	if err := db.Model(&dbs.GroupCalendarDay{}).
		Select("id", "session_instance_id").
		Where("group_id = ? AND source_plan_id IN ? AND date BETWEEN ? AND ?",
			scope.GroupID, planIDs, scope.From, scope.To).
		Find(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("error al leer los días del seed: %w", err)
	}
	for _, row := range rows {
		dayIDs = append(dayIDs, row.ID)
		if row.SessionInstanceID != nil {
			instanceIDs = append(instanceIDs, *row.SessionInstanceID)
		}
	}
	return dayIDs, uniqueIDs(instanceIDs), nil
}

// seedExerciseInstanceIDs son las instancias de ejercicio de las sesiones del
// seed, y solo las que no están referenciadas por otra sesión instancia (con un
// stamp --forzado, la instancia vieja puede quedar referenciada por otro día).
func seedExerciseInstanceIDs(db *gorm.DB, instanceIDs []int64) ([]int64, error) {
	if len(instanceIDs) == 0 {
		return nil, nil
	}
	type row struct{ ExerciseInstanceID int64 }
	var rows []row
	if err := db.Model(&dbs.SessionExerciseInstance{}).
		Select("exercise_instance_id").
		Where("session_instance_id IN ?", instanceIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("error al leer las instancias de ejercicio del seed: %w", err)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ExerciseInstanceID)
	}
	exclusive, err := exclusiveIDs(db, ids,
		`SELECT sei.exercise_instance_id AS id FROM session_exercise_instances sei
		 WHERE sei.exercise_instance_id IN ? AND sei.session_instance_id NOT IN ?`, ids, instanceIDs)
	if err != nil {
		return nil, err
	}
	return exclusive, nil
}

// seedSessionIDs son las sesiones de catálogo de las que el seed estampó.
func seedSessionIDs(db *gorm.DB, planIDs []int64) ([]int64, error) {
	type row struct{ SessionID *int64 }
	var rows []row
	if err := db.Model(&dbs.PlanDay{}).
		Select("session_id").
		Where("plan_id IN ? AND session_id IS NOT NULL", planIDs).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("error al leer los días de plan del seed: %w", err)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, *row.SessionID)
	}
	return uniqueIDs(ids), nil
}

// seedExerciseIDs son los ejercicios de catálogo del seed, y solo los que ninguna
// sesión fuera del seed usa.
func seedExerciseIDs(db *gorm.DB, scope cleanupScope, sessionIDs []int64) ([]int64, error) {
	names := make([]string, 0, len(catalog))
	for _, item := range catalog {
		names = append(names, item.Name)
	}
	var ids []int64
	if err := db.Model(&dbs.Exercise{}).
		Where("owner_id = ? AND description LIKE ? AND name IN ?",
			scope.OwnerID, "%"+seedMarker+"%", names).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("error al leer los ejercicios del seed: %w", err)
	}
	if len(ids) == 0 || len(sessionIDs) == 0 {
		return ids, nil
	}
	return exclusiveIDs(db, ids,
		`SELECT se.exercise_id AS id FROM session_exercises se
		 WHERE se.exercise_id IN ? AND se.session_id NOT IN ?`, ids, sessionIDs)
}

// exclusiveIDs saca los ids que alguna fila ajena al seed todavía referencia, para
// borrar solo lo que es exclusivamente suyo. referencedQuery tiene que devolver
// una columna `id`.
func exclusiveIDs(db *gorm.DB, ids []int64, referencedQuery string, args ...interface{}) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	type idRow struct{ ID int64 }
	var referenced []idRow
	if err := db.Raw(referencedQuery, args...).Scan(&referenced).Error; err != nil {
		return nil, fmt.Errorf("error al verificar referencias: %w", err)
	}
	shared := make(map[int64]struct{}, len(referenced))
	for _, row := range referenced {
		shared[row.ID] = struct{}{}
	}
	exclusive := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := shared[id]; !ok {
			exclusive = append(exclusive, id)
		}
	}
	return exclusive, nil
}

// countCleanupSteps cuenta lo que se va a borrar, sin borrar.
func countCleanupSteps(db *gorm.DB, steps []cleanupStep) (cleanupResult, error) {
	counts := cleanupResult{}
	for _, step := range steps {
		var total int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", step.Table, step.Where)
		if err := db.Raw(query, step.Args...).Scan(&total).Error; err != nil {
			return nil, fmt.Errorf("error contando %s: %w", step.Table, err)
		}
		counts[step.Table] = total
	}
	return counts, nil
}

// executeCleanup borra todo en una transacción: o se va todo, o no se borra nada.
// Un cleanup a medias deja el scope en un estado que ni el seed ni la app saben
// explicar.
func executeCleanup(db *gorm.DB, steps []cleanupStep) (cleanupResult, error) {
	affected := cleanupResult{}
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, step := range steps {
			res := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s", step.Table, step.Where), step.Args...)
			if res.Error != nil {
				return fmt.Errorf("borrando %s: %w", step.Table, res.Error)
			}
			affected[step.Table] = res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return affected, nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	unique := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
