package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
)

// Scope por default del seed (tarea 7.1). Los dos ids vienen por flag para poder
// sembrar otro equipo sin recompilar —el objetivo de diseño del seed—, y estas
// constantes son el default: el equipo y el grupo de desarrollo local.
const (
	seedTeamID  int64 = 29
	seedGroupID int64 = 39
)

// seedSummaryTolerance es la tolerancia con la que el seed verifica los
// contadores: la misma que usa el endpoint por default (D5).
const seedSummaryTolerance = 0.05

type options struct {
	seed       bool
	cleanup    bool
	dryRun     bool
	yes        bool
	start      string
	force      bool
	bootstrap  bool
	dbOverride string
	teamID     int64
	groupID    int64
}

func parseOptions() options {
	var opts options
	// --stage lo lee config desde os.Args (IsLocalStage/IsProductionStage), pero
	// flag.Parse aborta con cualquier flag que no declared: sin esta entrada
	// vacía, `--stage=local` —que el seed necesita para no pegar contra
	// testing— sería un error de línea de comandos.
	flag.String("stage", "", "ignorado acá: config lo lee de os.Args (--stage=local es obligatorio)")
	flag.BoolVar(&opts.seed, "seed", false, "sembrar el dataset (sin el flag solo informa qué hay)")
	flag.BoolVar(&opts.cleanup, "cleanup", false, "borrar lo que sembró el seed (requiere --yes)")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "no escribir nada: solo informar qué se haría")
	flag.BoolVar(&opts.yes, "yes", false, "confirmación explícita, obligatoria para --cleanup")
	flag.StringVar(&opts.start, "start", "", "primer día de la ventana (YYYY-MM-DD, lunes). Default: las dos semanas completas anteriores")
	flag.BoolVar(&opts.force, "force", false, "sobrescribir días de calendario ya existentes en el stamp (StampRequest.Force)")
	flag.BoolVar(&opts.bootstrap, "bootstrap-scope", false, "crear equipo, grupo y corredores si el scope no existe (solo para bases vacías)")
	flag.StringVar(&opts.dbOverride, "db", "", "DSN alternativo para sembrar una base descartable (ej. la de tests)")
	flag.Int64Var(&opts.teamID, "team-id", seedTeamID, "equipo del scope a sembrar")
	flag.Int64Var(&opts.groupID, "group-id", seedGroupID, "grupo del scope a sembrar")
	flag.Parse()
	return opts
}

func main() {
	opts := parseOptions()

	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)
	customlogger.Info(nil, "seed dashboard: stage resuelto",
		customlogger.Tag("stage", config.ResolvedStage()),
		customlogger.Tag("team_id", fmt.Sprintf("%d", opts.teamID)),
		customlogger.Tag("group_id", fmt.Sprintf("%d", opts.groupID)))

	// Mismo guard que la app en arranque (design.md D14): con SEEDING_ENABLED
	// activo y sin --stage=local, AssertSeedingStage aborta el proceso. No se
	// reimplementa: el seed usa la función que ya protege a la app.
	config.AssertSeedingStage()
	if !config.SeedingEnabled() {
		fatal("--seed necesita SEEDING_ENABLED=true en el entorno: sin la bandera el stamp rechaza toda fecha pasada")
	}

	dbConfig := config.MyDB
	if opts.dbOverride != "" {
		parsed, err := parseSeedDSN(opts.dbOverride)
		if err != nil {
			fatal("--db inválido: %v", err)
		}
		dbConfig = parsed
		customlogger.Warn(nil, "seed dashboard: la base viene de --db, no del stage",
			customlogger.Tag("db_name", dbConfig.Name),
			customlogger.Tag("host", dbConfig.Host))
	}
	db, err := postgresdb.ConfigDB(dbConfig)
	if err != nil {
		fatal("no se pudo conectar a la base: %v", err)
	}

	if opts.cleanup && opts.seed {
		fatal("--seed y --cleanup son modos distintos: elegí uno")
	}
	if opts.cleanup {
		runCleanupMode(db, opts)
		return
	}
	runSeedMode(db, opts)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "seed_dashboard: "+format+"\n", args...)
	os.Exit(1)
}

// seeder tiene todo el estado de una corrida. Se arma una vez y se pasa a las
// etapas, para que el orden (catálogo, sesiones, planes, stamp, series) se lea
// de arriba abajo.
type seeder struct {
	db      *gorm.DB
	opts    options
	window  seedWindow
	teamID  int64
	groupID int64
	ownerID int64
	roster  []seedAthlete
	counts  map[string]int64
	// catalogIDs es clave del seed -> id de exercises. Lo necesita el stamp
	// para traducir las instancias de vuelta a las claves del catálogo.
	catalogIDs map[string]int64
	// warnings son las cosas que el seed no puede resolver solo (por ejemplo un
	// ejercicio del catálogo que ya existía con otros objetivos). No son errores,
	// pero el operador tiene que verlas: se imprimen antes y después de sembrar.
	warnings []string
}

// seedAthlete es un corredor del grupo con el arquetipo que le tocó.
type seedAthlete struct {
	Index     int
	UserID    int64
	Name      string
	Archetype archetype
}

func runSeedMode(db *gorm.DB, opts options) {
	window, err := resolveWindow(opts.start)
	if err != nil {
		fatal("%v", err)
	}
	s := &seeder{
		db:      db,
		opts:    opts,
		window:  window,
		teamID:  opts.teamID,
		groupID: opts.groupID,
		counts:  map[string]int64{},
	}
	if err := s.resolveScope(); err != nil {
		fatal("%v", err)
	}
	s.printPlan()

	switch {
	case opts.dryRun:
		fmt.Println("\n[dry-run] no se escribió nada. Sacá --dry-run y sumá --seed para sembrar.")
		return
	case !opts.seed:
		fmt.Println("\nFalta --seed: esto solo informó el scope. Pasá con --seed para sembrar.")
		return
	}
	if err := s.run(); err != nil {
		fatal("%v", err)
	}
	s.report()
}

// resolveWindow parsea --start o calcula la ventana por default.
func resolveWindow(start string) (seedWindow, error) {
	if start == "" {
		return defaultWindow(time.Now()), nil
	}
	parsed, err := time.Parse(dateLayout, start)
	if err != nil {
		return seedWindow{}, fmt.Errorf("--start debe tener formato YYYY-MM-DD: %q", start)
	}
	monday := startOfWeek(parsed)
	if !monday.Equal(parsed) {
		return seedWindow{}, fmt.Errorf("--start tiene que ser un lunes: %s es %s", start, monday.Weekday())
	}
	return seedWindow{Start: monday}, nil
}

// printPlan imprime el alcance antes de tocar nada: contra qué base, con qué
// scope, en qué ventana y con qué corredores. Es lo que hay que leer cuando el
// seed hizo algo raro.
func (s *seeder) printPlan() {
	fmt.Println("Seed del dashboard analítico")
	fmt.Printf("  equipo %d · grupo %d · entrenador %d\n", s.teamID, s.groupID, s.ownerID)
	fmt.Printf("  ventana: %s\n", s.window.String())
	fmt.Println("  corredores:")
	for _, athlete := range s.roster {
		marker := ""
		if isLegacyAthlete(athlete.Index) {
			marker = "  [sin runner_session]"
		}
		if athlete.Archetype.Key == archetypeEnProgreso {
			marker += "  [1 sola sesión: insufficient_data al filtrar por él]"
		}
		fmt.Printf("    #%d user=%d %-22s %-12s%s\n", athlete.Index, athlete.UserID, athlete.Name, athlete.Archetype.Label, marker)
	}
	for _, warning := range s.warnings {
		fmt.Printf("  aviso: %s\n", warning)
	}
}

func (s *seeder) count(key string, delta int64) { s.counts[key] += delta }

// report imprime el resumen de la corrida: qué se creó, qué se reusó y cómo
// quedaron los seis contadores de data_quality.
func (s *seeder) report() {
	fmt.Println("\nEscrituras por tabla:")
	for _, key := range sortedKeys(s.counts) {
		fmt.Printf("  %-28s %d\n", key, s.counts[key])
	}
	for _, warning := range s.warnings {
		fmt.Printf("  aviso: %s\n", warning)
	}
}

func sortedKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
