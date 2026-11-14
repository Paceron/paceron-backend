package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// Copia competencias de un equipo a otro conservando tipo, métrica, ventana y
// mapa (route_geojson), pero con participantes del equipo destino. Para las
// competencias metric solo elige corredores con feedback dentro de la ventana,
// así el progreso que calcula el backend no queda vacío.
//
// Uso:
//
//	go run ./cmd/api/scripts/copy_competitions --stage=local --apply
//	go run ./cmd/api/scripts/copy_competitions --stage=local --source-ids=2,3,4 --apply
//
// Sin --apply solo muestra qué copiaría.
type options struct {
	apply      bool
	force      bool
	sourceTeam int64
	targetTeam int64
	groupID    int64
	creatorID  int64
	sourceIDs  string
}

// Valores del destino que usan los helpers de copia. Se inicializan una vez en
// main; el script es de una sola corrida, no un paquete reutilizable.
var (
	targetTeamID  int64
	targetGroupID int64
	creatorID     int64
)

func parseOptions() options {
	opts := options{}
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.String("stage", "", "ignorado acá: config lo lee de os.Args")
	fs.BoolVar(&opts.apply, "apply", false, "escribir en la base (sin esto es dry-run)")
	fs.BoolVar(&opts.force, "force", false, "copiar aunque el team destino ya tenga competencias")
	fs.Int64Var(&opts.sourceTeam, "source-team", 25, "equipo de origen")
	fs.Int64Var(&opts.targetTeam, "target-team", 29, "equipo de destino")
	fs.Int64Var(&opts.groupID, "group-id", 39, "grupo del equipo destino")
	fs.Int64Var(&opts.creatorID, "creator-id", 3, "usuario que figura como creador")
	fs.StringVar(&opts.sourceIDs, "source-ids", "2,3,4", "ids de competencias de origen, separados por coma")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal("%v", err)
	}
	return opts
}

func main() {
	opts := parseOptions()
	targetTeamID = opts.targetTeam
	targetGroupID = opts.groupID
	creatorID = opts.creatorID
	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)
	config.AssertSeedingStage()
	if !config.SeedingEnabled() {
		fatal("necesita SEEDING_ENABLED=true")
	}

	db, err := postgresdb.ConfigDB(config.MyDB)
	if err != nil {
		fatal("no se pudo conectar: %v", err)
	}

	sourceIDs, err := parseIDs(opts.sourceIDs)
	if err != nil {
		fatal("%v", err)
	}
	if len(sourceIDs) == 0 {
		fatal("--source-ids no puede quedar vacío")
	}

	runners, err := targetRunners(db, opts.targetTeam, opts.groupID)
	if err != nil {
		fatal("%v", err)
	}
	if len(runners) < 2 {
		fatal("el grupo %d del equipo %d tiene %d corredores; se necesitan al menos 2", opts.groupID, opts.targetTeam, len(runners))
	}

	sources, err := sourceCompetitions(db, opts.sourceTeam, sourceIDs)
	if err != nil {
		fatal("%v", err)
	}
	if len(sources) != len(sourceIDs) {
		fatal("se pidieron %d competencias y se encontraron %d en el equipo %d", len(sourceIDs), len(sources), opts.sourceTeam)
	}

	var existing int64
	if err := db.Model(&dbs.Competition{}).Where("team_id = ?", opts.targetTeam).Count(&existing).Error; err != nil {
		fatal("no se pudo verificar el destino: %v", err)
	}
	if existing != 0 && !opts.force {
		fatal("el team %d ya tiene %d competencias; usá --force si querés copiar igual", opts.targetTeam, existing)
	}

	fmt.Printf("Origen: team %d, competencias %v\n", opts.sourceTeam, sourceIDs)
	fmt.Printf("Destino: team %d, grupo %d, corredores %v\n\n", opts.targetTeam, opts.groupID, runners)

	plan, err := buildPlan(db, sources, runners, opts)
	if err != nil {
		fatal("%v", err)
	}
	printPlan(plan)

	if !opts.apply {
		fmt.Println("\nDry-run: no se escribió nada. Pasá --apply para copiar.")
		return
	}

	created, err := applyPlan(db, plan)
	if err != nil {
		fatal("%v", err)
	}
	fmt.Printf("\nCopiadas %d competencias al team %d:\n", len(created), opts.targetTeam)
	for _, c := range created {
		fmt.Printf("  id=%d type=%s metric=%s group=%d participantes=%v\n", c.ID, c.Type, deref(c.Metric), opts.groupID, c.Participants)
	}
}

type plannedCompetition struct {
	source       dbs.Competition
	participants []int64
}

func buildPlan(db *gorm.DB, sources []dbs.Competition, runners []int64, opts options) ([]plannedCompetition, error) {
	plan := make([]plannedCompetition, 0, len(sources))
	for i, source := range sources {
		participants, err := chooseParticipants(db, source, runners, i, opts.targetTeam)
		if err != nil {
			return nil, fmt.Errorf("competencia origen %d: %w", source.ID, err)
		}
		plan = append(plan, plannedCompetition{source: source, participants: participants})
	}
	return plan, nil
}

// chooseParticipants rota la lista para que no todos los participantes sean
// siempre los mismos. En metric exige feedback dentro de la ventana; en route
// alcanza con que sean corredores del grupo.
func chooseParticipants(db *gorm.DB, source dbs.Competition, runners []int64, index int, targetTeam int64) ([]int64, error) {
	if source.Type == "route" {
		return rotate(runners, (index*2)+1, 3), nil
	}

	eligible := make([]int64, 0, len(runners))
	for _, uid := range runners {
		ok, err := hasFeedbackInWindow(db, targetTeam, uid, source)
		if err != nil {
			return nil, err
		}
		if ok {
			eligible = append(eligible, uid)
		}
	}
	if len(eligible) < 2 {
		return nil, fmt.Errorf("solo %d corredores tienen feedback en la ventana %s..%s", len(eligible), source.StartAt.Format("2006-01-02"), endDate(source).Format("2006-01-02"))
	}
	return rotate(eligible, index*2, 5), nil
}

func hasFeedbackInWindow(db *gorm.DB, teamID, userID int64, source dbs.Competition) (bool, error) {
	query := `SELECT count(*) FROM workout_feedback
		WHERE team_id = ? AND athlete_user_id = ? AND deleted_at IS NULL
		  AND session_date >= ? AND session_date <= ?`
	args := []interface{}{teamID, userID, gametime.DateOnly(source.StartAt), gametime.DateOnly(endDate(source))}
	switch deref(source.Metric) {
	case "duration":
		query += " AND duration_ms > 0"
	case "peak_speed":
		query += " AND peak_speed_kph > 0"
	case "pace":
		query += " AND distance_meters > 0 AND duration_ms > 0"
	default:
		query += " AND distance_meters > 0"
	}
	var count int64
	if err := db.Raw(query, args...).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func rotate(ids []int64, offset, count int) []int64 {
	if len(ids) <= count {
		out := append([]int64(nil), ids...)
		return out
	}
	out := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, ids[(offset+i)%len(ids)])
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func applyPlan(db *gorm.DB, plan []plannedCompetition) ([]createdCompetition, error) {
	created := make([]createdCompetition, 0, len(plan))
	err := db.Transaction(func(tx *gorm.DB) error {
		for _, item := range plan {
			c, err := copyCompetition(tx, item)
			if err != nil {
				return err
			}
			created = append(created, c)
		}
		return nil
	})
	return created, err
}

type createdCompetition struct {
	ID           int64
	Type         string
	Metric       *string
	Participants []int64
}

func copyCompetition(tx *gorm.DB, item plannedCompetition) (createdCompetition, error) {
	source := item.source
	groupID := targetGroupID
	comp := dbs.Competition{
		TeamID:                 targetTeamID,
		GroupID:                &groupID,
		Type:                   source.Type,
		Metric:                 source.Metric,
		StartAt:                source.StartAt,
		EndAt:                  source.EndAt,
		RouteGeoJSON:           source.RouteGeoJSON,
		AcceptanceRadiusMeters: source.AcceptanceRadiusMeters,
		MaxAttempts:            source.MaxAttempts,
		CreatedByUserID:        creatorID,
		CreatedByRole:          "trainer",
		Status:                 "active",
	}
	if err := tx.Create(&comp).Error; err != nil {
		return createdCompetition{}, fmt.Errorf("crear competencia copia: %w", err)
	}

	if comp.Type == "route" {
		instance, err := createSyntheticRouteInstance(tx, fmt.Sprintf("Competencia ruta %d", comp.ID))
		if err != nil {
			return createdCompetition{}, err
		}
		if err := tx.Model(&dbs.Competition{}).Where("id = ?", comp.ID).Update("route_session_instance_id", instance.ID).Error; err != nil {
			return createdCompetition{}, fmt.Errorf("guardar instancia de ruta: %w", err)
		}
	}

	for _, userID := range item.participants {
		if err := tx.Create(&dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: userID}).Error; err != nil {
			return createdCompetition{}, fmt.Errorf("crear participante %d: %w", userID, err)
		}
	}
	return createdCompetition{ID: comp.ID, Type: comp.Type, Metric: comp.Metric, Participants: item.participants}, nil
}

func createSyntheticRouteInstance(tx *gorm.DB, name string) (*dbs.SessionInstance, error) {
	instance := &dbs.SessionInstance{
		Name:        name,
		Description: strPtr("Instancia sintética de la competencia de ruta"),
	}
	if err := tx.Create(instance).Error; err != nil {
		return nil, fmt.Errorf("crear instancia de sesión: %w", err)
	}
	exercise := &dbs.ExerciseInstance{
		Name: "Corrida",
		Kind: string(constants.ExerciseKindVelocidadDistancia),
	}
	if err := tx.Create(exercise).Error; err != nil {
		return nil, fmt.Errorf("crear ejercicio sintético: %w", err)
	}
	link := &dbs.SessionExerciseInstance{
		SessionInstanceID:  instance.ID,
		ExerciseInstanceID: exercise.ID,
		Role:               string(constants.SessionExerciseRoleMain),
		RepeatCount:        1,
		RestMinutes:        0,
	}
	if err := tx.Create(link).Error; err != nil {
		return nil, fmt.Errorf("crear link de ejercicio: %w", err)
	}
	return instance, nil
}

func targetRunners(db *gorm.DB, teamID, groupID int64) ([]int64, error) {
	var ids []int64
	err := db.Raw(`SELECT DISTINCT tu.user_id
		FROM team_users tu
		JOIN group_users gu ON gu.user_id = tu.user_id AND gu.group_id = ? AND gu.deleted_at IS NULL
		WHERE tu.team_id = ? AND tu.role_in_team = 'corredor' AND tu.deleted_at IS NULL
		ORDER BY tu.user_id`, groupID, teamID).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("leer corredores destino: %w", err)
	}
	return ids, nil
}

func sourceCompetitions(db *gorm.DB, teamID int64, ids []int64) ([]dbs.Competition, error) {
	var comps []dbs.Competition
	if err := db.Where("team_id = ? AND id IN ?", teamID, ids).Order("id ASC").Find(&comps).Error; err != nil {
		return nil, fmt.Errorf("leer competencias origen: %w", err)
	}
	return comps, nil
}

func printPlan(plan []plannedCompetition) {
	for _, item := range plan {
		s := item.source
		fmt.Printf("origen id=%d type=%s metric=%s ventana=%s..%s\n", s.ID, s.Type, deref(s.Metric),
			s.StartAt.Format("2006-01-02"), endDate(s).Format("2006-01-02"))
		fmt.Printf("  participantes: %v (grupo %d)\n", item.participants, targetGroupID)
		if s.Type == "route" {
			fmt.Printf("  ruta: %d chars, radio=%.0f m, max_attempts=%s\n", len(deref(s.RouteGeoJSON)), derefFloat(s.AcceptanceRadiusMeters), derefInt(s.MaxAttempts))
		}
	}
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

func endDate(comp dbs.Competition) time.Time {
	if comp.EndAt != nil {
		return *comp.EndAt
	}
	return time.Now()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(v *int64) string {
	if v == nil {
		return "ilimitados"
	}
	return fmt.Sprint(*v)
}

func derefFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func strPtr(s string) *string { return &s }

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "copy_competitions: "+format+"\n", args...)
	os.Exit(1)
}
