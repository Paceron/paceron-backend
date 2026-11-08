package main

import (
	"flag"
	"fmt"
	"os"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/infrastructure/postgresdb"
)

const (
	seedTeamID  int64 = 29
	seedGroupID int64 = 39
)

type options struct {
	seed    bool
	dryRun  bool
	yes     bool
	target  string
	teamID  int64
	groupID int64
	count   int
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
	fs.String("stage", "", "ignorado acá: config lo lee de os.Args")
	fs.BoolVar(&opts.seed, "seed", false, "sembrar cuotas (sin el flag solo informa)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "no escribir nada")
	fs.BoolVar(&opts.yes, "yes", false, "confirmación explícita")
	fs.StringVar(&opts.target, "target", "local", "destino: local o testing")
	fs.Int64Var(&opts.teamID, "team-id", seedTeamID, "equipo")
	fs.Int64Var(&opts.groupID, "group-id", seedGroupID, "grupo")
	fs.IntVar(&opts.count, "count", 3, "cuotas por corredor")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

func main() {
	opts := parseOptions()
	customlogger.CustomConfig(customlogger.DebugLevel, true, true, true)

	remote := opts.target == "testing"
	if opts.target != "local" && opts.target != "testing" {
		fatal("--target inválido %q", opts.target)
	}
	if remote {
		config.AssertSeedingRemoteAllowed()
		if opts.teamID == 0 || opts.groupID == 0 {
			fatal("--team-id y --group-id obligatorios en modo remoto")
		}
	} else {
		config.AssertSeedingStage()
	}
	if !config.SeedingEnabled() {
		fatal("necesita SEEDING_ENABLED=true")
	}

	dbConfig := config.MyDB
	db, err := postgresdb.ConfigDB(dbConfig)
	if err != nil {
		fatal("no se pudo conectar: %v", err)
	}

	if opts.seed && !opts.dryRun && remote && !opts.yes {
		fmt.Printf("Modo remoto. team=%d group=%d. Escribir? (yes/no): ", opts.teamID, opts.groupID)
		var answer string
		fmt.Scanln(&answer)
		if answer != "yes" {
			fatal("abortado")
		}
	}

	s := &seeder{db: db, opts: opts}
	if err := s.run(); err != nil {
		fatal("%v", err)
	}
	s.report()
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "seed_payments: "+format+"\n", args...)
	os.Exit(1)
}

type seeder struct {
	db   *gorm.DB
	opts options
}

func (s *seeder) run() error {
	fmt.Printf("Seed payments: team=%d group=%d count=%d\n", s.opts.teamID, s.opts.groupID, s.opts.count)
	if s.opts.dryRun || !s.opts.seed {
		fmt.Println("[dry-run] no se escribió nada. Pasá --seed para sembrar.")
		return nil
	}
	// TODO: implementar creación de cuotas
	fmt.Println("Implementación pendiente.")
	return nil
}

func (s *seeder) report() {}