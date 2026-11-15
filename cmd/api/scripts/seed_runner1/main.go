package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const (
	defaultManifestPath      = ".superpowers/sdd/tasks/runner1-data-manifest.json"
	defaultMergeManifestPath = ".superpowers/sdd/tasks/runner1-data-manifest-merge.json"
)

type options struct {
	email       string
	anchor      string
	dryRun      bool
	apply       bool
	cleanup     bool
	merge       bool
	yes         bool
	target      string
	dbOverride  string
	manifest    string
	manifestSet bool
}

func parseOptions() options {
	opts, err := parseOptionsArgs(flag.CommandLine, os.Args[1:])
	if err != nil {
		fatal("%v", err)
	}
	return opts
}

func parseOptionsArgs(fs *flag.FlagSet, args []string) (options, error) {
	var opts options
	fs.String("stage", "", "ignorado acá: config lo lee de os.Args (--stage=local o --stage=production)")
	fs.StringVar(&opts.email, "email", defaultEmail, "email del corredor a sembrar")
	fs.StringVar(&opts.anchor, "anchor", "", "día ancla YYYY-MM-DD (default: hoy en America/Argentina/Cordoba)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "no escribir nada (modo por default)")
	fs.BoolVar(&opts.apply, "apply", false, "aplicar el fixture en una transacción")
	fs.BoolVar(&opts.cleanup, "cleanup", false, "borrar exactamente lo que creó el manifest")
	fs.BoolVar(&opts.merge, "merge", false, "top-up no destructivo: agrega solo días libres y conserva lo existente")
	fs.BoolVar(&opts.yes, "yes", false, "confirmación explícita obligatoria para testing remoto")
	fs.StringVar(&opts.target, "target", "local", "destino de escritura: local (default) o testing (requiere SEEDING_ALLOW_REMOTE)")
	fs.StringVar(&opts.dbOverride, "db", "", "DSN alternativo para una base descartable")
	fs.StringVar(&opts.manifest, "manifest", defaultManifestPath, "ruta del manifest JSON de IDs creados")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "manifest" {
			opts.manifestSet = true
		}
	})
	if !opts.manifestSet && opts.merge {
		opts.manifest = defaultMergeManifestPath
	}
	if opts.target != "local" && opts.target != "testing" {
		return options{}, fmt.Errorf("--target inválido %q: usá local o testing", opts.target)
	}
	if opts.apply && opts.cleanup {
		return options{}, fmt.Errorf("--apply y --cleanup son modos distintos: elegí uno")
	}
	if opts.dryRun && (opts.apply || opts.cleanup) {
		return options{}, fmt.Errorf("--dry-run no se combina con --apply ni --cleanup")
	}
	if !opts.apply && !opts.cleanup {
		opts.dryRun = true
	}
	if strings.TrimSpace(opts.email) == "" {
		return options{}, fmt.Errorf("--email no puede estar vacío")
	}
	if strings.TrimSpace(opts.manifest) == "" {
		return options{}, fmt.Errorf("--manifest no puede estar vacío")
	}
	return opts, nil
}

func main() {
	opts := parseOptions()
	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)

	anchor, err := resolveAnchor(opts.anchor)
	if err != nil {
		fatal("%v", err)
	}

	tgt, err := resolveTarget(opts)
	if err != nil {
		fatal("%v", err)
	}
	if tgt.remote {
		if !opts.yes {
			fatal("--target=testing requiere --yes explícito además de SEEDING_ALLOW_REMOTE=true")
		}
		fmt.Println("Destino: testing (remoto)")
	} else {
		fmt.Println("Destino: local")
	}

	var db *gorm.DB
	if opts.dryRun {
		// El dry-run tiene que ser estrictamente read-only: ConfigDB corre
		// AutoMigrate y un backfill, así que acá abrimos sin inicializar schema.
		db, err = openReadOnly(tgt.db)
	} else {
		db, err = postgresdb.ConfigDB(tgt.db)
	}
	if err != nil {
		fatal("no se pudo conectar a la base %s/%s: %v", tgt.db.Host, tgt.db.Name, err)
	}

	plan := buildFixturePlan(anchor)

	switch {
	case opts.cleanup:
		if tgt.remote && !opts.yes {
			fatal("--cleanup contra testing requiere --yes explícito")
		}
		if err := runCleanup(db, opts); err != nil {
			fatal("%v", err)
		}
	case opts.apply:
		if tgt.remote && !opts.yes {
			fatal("--apply contra testing requiere --yes explícito")
		}
		if opts.merge {
			if err := runMergeApply(db, opts, anchor); err != nil {
				fatal("%v", err)
			}
		} else if err := runApply(db, opts, plan); err != nil {
			fatal("%v", err)
		}
	default:
		if opts.merge {
			if err := runMergeDryRun(db, opts, anchor); err != nil {
				fatal("%v", err)
			}
		} else if err := runDryRun(db, opts, plan); err != nil {
			fatal("%v", err)
		}
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "seed_runner1: "+format+"\n", args...)
	os.Exit(1)
}

func resolveAnchor(raw string) (time.Time, error) {
	today := gametime.Today()
	if strings.TrimSpace(raw) == "" {
		return today, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, gametime.Location)
	if err != nil {
		return time.Time{}, fmt.Errorf("--anchor debe tener formato YYYY-MM-DD: %q", raw)
	}
	parsed = dateOnly(parsed)
	if !parsed.Equal(today) {
		return time.Time{}, fmt.Errorf("--anchor=%s no es hoy (%s): la semántica de rachas de la API usa el reloj real; para un ancla pasada usá un cambio de datos dedicado", parsed.Format("2006-01-02"), today.Format("2006-01-02"))
	}
	return parsed, nil
}

type target struct {
	remote bool
	db     config.DB
}

func resolveTarget(opts options) (target, error) {
	if config.IsProductionStage() {
		return target{}, fmt.Errorf("producción está prohibida para este comando")
	}
	switch opts.target {
	case "testing":
		return resolveTestingTarget(opts)
	default:
		return resolveLocalTarget(opts)
	}
}

func resolveLocalTarget(opts options) (target, error) {
	if opts.dbOverride != "" {
		db, err := parseDSN(opts.dbOverride)
		if err != nil {
			return target{}, fmt.Errorf("--db inválido: %w", err)
		}
		fmt.Printf("Aviso: la base viene de --db (%s/%s), no del stage.\n", db.Host, db.Name)
		return target{db: db}, nil
	}
	if config.IsLocalStage() {
		if config.MyDB.Host == "" || config.MyDB.Name == "" {
			return target{}, fmt.Errorf("--stage=local no resolvió DATABASE_URL; revisá .env.local o pasá --db")
		}
		return target{db: config.MyDB}, nil
	}
	if err := godotenv.Overload(".env.local"); err != nil {
		return target{}, fmt.Errorf("no se pudo leer .env.local (%v): usá --stage=local, --db o --target=testing", err)
	}
	raw := os.Getenv("DATABASE_URL")
	if raw == "" {
		return target{}, fmt.Errorf(".env.local no define DATABASE_URL: usá --stage=local, --db o --target=testing")
	}
	db, err := parseDSN(raw)
	if err != nil {
		return target{}, fmt.Errorf("DATABASE_URL inválida: %w", err)
	}
	if !isLocalHost(db.Host) {
		return target{}, fmt.Errorf("DATABASE_URL apunta a %q, no a localhost: se rechaza el target local", db.Host)
	}
	return target{db: db}, nil
}

func resolveTestingTarget(opts options) (target, error) {
	if !envBool("SEEDING_ALLOW_REMOTE") {
		return target{}, fmt.Errorf("--target=testing requiere SEEDING_ALLOW_REMOTE=true")
	}
	raw := os.Getenv("SUPABASE_TESTING_DATABASE_URL")
	if raw == "" {
		return target{}, fmt.Errorf("SUPABASE_TESTING_DATABASE_URL no está seteada")
	}
	if production := os.Getenv("SUPABASE_PRODUCTION_DATABASE_URL"); production != "" && production == raw {
		return target{}, fmt.Errorf("la URL de testing coincide con la de producción: se rechaza")
	}
	db, err := parseDSN(raw)
	if err != nil {
		return target{}, fmt.Errorf("SUPABASE_TESTING_DATABASE_URL inválida: %w", err)
	}
	return target{remote: true, db: db}, nil
}

func parseDSN(raw string) (config.DB, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return config.DB{}, fmt.Errorf("no parece una URL de Postgres: %w", err)
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if parsed.Hostname() == "" || name == "" {
		return config.DB{}, fmt.Errorf("faltan host o nombre de base")
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	password, _ := parsed.User.Password()
	return config.LoadDBConfigDB(config.DB{
		Host:     parsed.Hostname(),
		Port:     port,
		Username: parsed.User.Username(),
		Password: password,
		Name:     name,
	}), nil
}

func isLocalHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// openReadOnly abre la conexión sin AutoMigrate ni backfills. Es el único
// camino que usa --dry-run, para que revisar el plan no escriba schema ni datos.
func openReadOnly(dbConfig config.DB) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(postgresdb.ConnString(dbConfig)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(2)
	sqlDB.SetMaxIdleConns(1)
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func runDryRun(db *gorm.DB, opts options, plan fixturePlan) error {
	scope, err := inspectScope(db, opts, plan)
	if err != nil {
		return err
	}
	fmt.Printf("Plan para %s (user_id=%d) · grupo %d · equipo %d · ancla %s\n",
		scope.user.Email, scope.user.ID, scope.groupID, scope.teamID, plan.Anchor.Format("2006-01-02"))
	fmt.Printf("  días=%d · finished=%d · early=%d · distancia=%.0fm · GPS=%d · asistencias=%d · racha=%d/%d (best %d)\n",
		len(plan.Days), plan.ExpectedFinishedSessions, plan.ExpectedEarlySessions,
		plan.ExpectedDistanceMeters, plan.ExpectedGPSDays, plan.ExpectedAttendanceDays,
		plan.ExpectedStreakCurrent, plan.ExpectedStreakBest, plan.ExpectedStreakBest)
	fmt.Printf("  logros esperados: %d desbloqueados, %d bloqueados\n",
		len(expectedUnlockedAchievements), len(expectedLockedAchievements))
	fmt.Println("  timeline:")
	for _, day := range plan.Days {
		details := []string{day.DayStatus, day.RunnerStatus}
		if day.HasFeedback {
			details = append(details, fmt.Sprintf("%.0fm", day.DistanceMeters))
		}
		if day.HasGPS {
			details = append(details, "GPS")
		}
		if day.HasAttendance {
			details = append(details, "asistencia")
		}
		fmt.Printf("    D%-2d %s  %s\n", day.Offset, day.Date.Format("2006-01-02"), strings.Join(details, " · "))
	}
	if err := printScopeConflicts(scope); err != nil {
		return err
	}
	fmt.Println("\n[dry-run] no se escribió nada. Pasá --apply para aplicar en una transacción.")
	return nil
}
