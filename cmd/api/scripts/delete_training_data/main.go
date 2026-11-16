// Comando delete_training_data: borra el historial de entrenamiento de un
// equipo a partir de una fecha, en un solo rango y de forma irreversible.
//
// Qué borra, para el team destino y desde --from (incluido) en adelante:
//
//   - workout_feedback de ese team (hard delete, incluidos los ya soft-deleted)
//     junto con sus puntos GPS en workout_feedback_points.
//   - attendances del team cuya sesión (training_session_id) es un día del
//     calendario que entra en el rango.
//   - runner_session (estado wip/finished por atleta) de esas sesiones.
//   - group_calendar_days del team en el rango (todos los kinds por default) y
//     las session_instances/exercise_instances que queden huérfanas.
//   - la gamificación derivada de esos atletas: logros desbloqueados, estampas,
//     avisos "unseen" y el cache de rachas, con el mismo criterio de fecha
//     (streak_state no tiene fecha: se borra entero para esos atletas).
//
// Las instancias NO se borran si algo fuera del rango las sigue referenciando
// (feedback activo, otro día de calendario, o una competencia de ruta): se
// conservan y el dry-run dice por qué.
//
// Uso:
//
//	go run ./cmd/api/scripts/delete_training_data --stage=local --team=29 --from=2026-11-15
//	go run ./cmd/api/scripts/delete_training_data --stage=local --team=29 --from=2026-11-15 --apply
//
// Sin --apply es dry-run y no escribe nada (abre la conexión sin AutoMigrate,
// igual que seed_runner1). Con --apply todo corre en una transacción.
//
// Guardas: producción está prohibida siempre; el target local exige
// --stage=local; el target testing exige SEEDING_ALLOW_REMOTE=true y --yes.
package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
	"simple-arq-golang/cmd/api/utils/gametime"
)

type options struct {
	team   int64
	from   string
	to     string
	groups string
	kinds  string
	apply  bool
	yes    bool
	target string
	db     string
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
	fs.Int64Var(&opts.team, "team", 0, "id del equipo destino (obligatorio)")
	fs.StringVar(&opts.from, "from", "", "primer día a borrar, YYYY-MM-DD (obligatorio, incluido)")
	fs.StringVar(&opts.to, "to", "", "último día a borrar, YYYY-MM-DD (opcional, incluido; sin esto no hay tope)")
	fs.StringVar(&opts.groups, "groups", "", "ids de grupos separados por coma (default: todos los del team)")
	fs.StringVar(&opts.kinds, "kinds", strings.Join(constants.GetValidGroupCalendarDayKinds(), ","),
		"kinds de días de calendario a borrar, separados por coma")
	fs.BoolVar(&opts.apply, "apply", false, "borrar de verdad (sin esto es dry-run)")
	fs.BoolVar(&opts.yes, "yes", false, "confirmación explícita obligatoria para --target=testing")
	fs.StringVar(&opts.target, "target", "local", "destino de escritura: local (default) o testing")
	fs.StringVar(&opts.db, "db", "", "DSN alternativo para una base descartable")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if opts.team <= 0 {
		return options{}, fmt.Errorf("--team es obligatorio y tiene que ser > 0")
	}
	if strings.TrimSpace(opts.from) == "" {
		return options{}, fmt.Errorf("--from es obligatorio (YYYY-MM-DD): el script no elige fecha por su cuenta")
	}
	if opts.target != "local" && opts.target != "testing" {
		return options{}, fmt.Errorf("--target inválido %q: usá local o testing", opts.target)
	}
	if opts.yes && opts.target != "testing" {
		return options{}, fmt.Errorf("--yes solo tiene sentido con --target=testing")
	}
	return opts, nil
}

func main() {
	opts := parseOptions()
	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)

	sc, err := newScope(opts)
	if err != nil {
		fatal("%v", err)
	}

	tgt, err := resolveTarget(opts)
	if err != nil {
		fatal("%v", err)
	}
	if tgt.remote {
		fmt.Println("Destino: testing (remoto)")
	} else {
		fmt.Printf("Destino: %s/%s\n", tgt.db.Host, tgt.db.Name)
	}

	// El dry-run tiene que ser estrictamente read-only: ConfigDB corre
	// AutoMigrate, así que solo --apply inicializa schema.
	var db *gorm.DB
	if opts.apply {
		db, err = postgresdb.ConfigDB(tgt.db)
	} else {
		db, err = openReadOnly(tgt.db)
	}
	if err != nil {
		fatal("no se pudo conectar a la base %s/%s: %v", tgt.db.Host, tgt.db.Name, err)
	}

	p, err := buildPlan(db, sc)
	if err != nil {
		fatal("%v", err)
	}
	printPlan(p)

	if !opts.apply {
		fmt.Println("\n[dry-run] no se escribió nada. Pasá --apply para borrar en una transacción.")
		return
	}

	applied, err := applyPlan(db, p)
	if err != nil {
		fatal("se revirtió todo (rollback): %v", err)
	}
	printApplied(applied)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "delete_training_data: "+format+"\n", args...)
	os.Exit(1)
}

type target struct {
	remote bool
	db     config.DB
}

func resolveTarget(opts options) (target, error) {
	if config.IsProductionStage() {
		return target{}, fmt.Errorf("producción está prohibida para este comando")
	}
	if opts.db != "" {
		db, err := parseDSN(opts.db)
		if err != nil {
			return target{}, fmt.Errorf("--db inválido: %w", err)
		}
		fmt.Printf("Aviso: la base viene de --db (%s/%s), no del stage.\n", db.Host, db.Name)
		return target{db: db}, nil
	}
	if opts.target == "testing" {
		return resolveTestingTarget(opts)
	}
	if !config.IsLocalStage() {
		return target{}, fmt.Errorf("--target=local requiere --stage=local (o pasá --target=testing/--db)")
	}
	if config.MyDB.Host == "" || config.MyDB.Name == "" {
		return target{}, fmt.Errorf("--stage=local no resolvió DATABASE_URL; revisá .env.local o pasá --db")
	}
	return target{db: config.MyDB}, nil
}

// resolveTestingTarget replica la guarda de seed_runner1: nunca producción y
// siempre con dos opt-ins explícitos (env + --yes).
func resolveTestingTarget(opts options) (target, error) {
	if !envBool("SEEDING_ALLOW_REMOTE") {
		return target{}, fmt.Errorf("--target=testing requiere SEEDING_ALLOW_REMOTE=true")
	}
	if !opts.yes {
		return target{}, fmt.Errorf("--target=testing requiere --yes explícito")
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

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// openReadOnly abre la conexión sin AutoMigrate ni backfills.
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

// newScope valida y arma el alcance pedido, sin tocar la base.
func newScope(opts options) (scope, error) {
	from, err := parseDateArg("--from", opts.from)
	if err != nil {
		return scope{}, err
	}
	var to *time.Time
	if strings.TrimSpace(opts.to) != "" {
		parsed, err := parseDateArg("--to", opts.to)
		if err != nil {
			return scope{}, err
		}
		if parsed.Before(from) {
			return scope{}, fmt.Errorf("--to (%s) es anterior a --from (%s)", parsed.Format("2006-01-02"), from.Format("2006-01-02"))
		}
		to = &parsed
	}
	rawKinds := opts.kinds
	if strings.TrimSpace(rawKinds) == "" {
		rawKinds = strings.Join(constants.GetValidGroupCalendarDayKinds(), ",")
	}
	kinds, err := parseKinds(rawKinds)
	if err != nil {
		return scope{}, err
	}
	groups, err := parseIDs(opts.groups)
	if err != nil {
		return scope{}, fmt.Errorf("--groups: %w", err)
	}
	return scope{teamID: opts.team, from: from, to: to, kinds: kinds, groups: groups}, nil
}

func parseDateArg(name, raw string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(raw), gametime.Location)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s debe tener formato YYYY-MM-DD: %q", name, raw)
	}
	return parsed, nil
}

// parseKinds separa, dedupe y valida los kinds contra el catálogo de constants.
func parseKinds(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	valid := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		if _, dup := valid[part]; dup {
			continue
		}
		if !constants.IsValidGroupCalendarDayKind(part) {
			return nil, fmt.Errorf("--kinds: %q no es válido (validos: %s)", part, strings.Join(constants.GetValidGroupCalendarDayKinds(), ", "))
		}
		valid[part] = struct{}{}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("--kinds no puede quedar vacío")
	}
	return out, nil
}
