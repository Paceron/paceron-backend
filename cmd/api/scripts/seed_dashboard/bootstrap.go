package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// parseSeedDSN entiende el mismo formato que la app (una URL de Postgres) y
// devuelve la config de conexión del script.
//
// Existe para poder sembrar una base descartable —la de `make test-db-up`, que
// vive en el puerto 5433— sin tocar el `.env.local` de la máquina de cada uno:
// el stage y el guard de SEEDING_ENABLED siguen aplicando igual, lo único que
// cambia es a qué base apuntan. Para sembrar la base de trabajo, no hace falta.
func parseSeedDSN(raw string) (config.DB, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return config.DB{}, fmt.Errorf("no parece una URL de Postgres: %w", err)
	}
	name := strings.TrimPrefix(parsed.Path, "/")
	if parsed.Hostname() == "" || name == "" {
		return config.DB{}, fmt.Errorf("faltan host o nombre de base en %q", raw)
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

// bootstrapScope crea el scope mínimo para poder sembrar una base vacía: un
// entrenador, el equipo, el grupo y los corredores.
//
// Es opt-in (`--bootstrap-scope`) y existe para el caso "no tengo la base de
// desarrollo con datos": sembrar contra una base descartable tiene que ser tan
// fácil como `make test-db-up`. No se reusa ningún dato real: los usuarios se
// crean con emails y DNI del rango de pruebas, así que nunca colisionan con
// usuarios de verdad.
func (s *seeder) bootstrapScope() error {
	if err := s.bootstrapUsers(); err != nil {
		return err
	}
	if err := s.bootstrapTeamAndGroup(); err != nil {
		return err
	}
	return nil
}

// Los correos del scope sintético llevan el id del equipo para que dos seeds sobre
// equipos distintos no compartan entrenador: el stamp detecta la colisión presencial
// entre grupos del mismo dueño, y un entrenador compartido entre dos bases (o entre
// dos equipos sembrados) haría saltar esa guarda sin motivo.
//
// El dominio .local es reservado y no resuelve en DNS (RFC 6762), así que un usuario
// sembrado nunca puede recibir un mail de verdad. Los DNI van en el rango 99xxxxxx,
// fuera del de los usuarios reales.
func (s *seeder) runnerEmail() string { return fmt.Sprintf("entrenador+%d@seed.local", s.teamID) }
func (s *seeder) athleteEmail(i int) string {
	return fmt.Sprintf("corredor+%d-%02d@seed.local", s.teamID, i)
}
func (s *seeder) athleteDNI(i int) string { return fmt.Sprintf("99%07d%02d", s.teamID%1000, i) }

func (s *seeder) bootstrapUsers() error {
	if err := s.createUser(s.runnerEmail(), fmt.Sprintf("99%08d", s.teamID%100000000), "Entrenador", "Semilla"); err != nil {
		return err
	}
	for i := 1; i <= desiredAthletes; i++ {
		if err := s.createUser(s.athleteEmail(i), s.athleteDNI(i),
			"Corredor", fmt.Sprintf("Semilla %02d", i)); err != nil {
			return err
		}
	}
	return nil
}

// desiredAthletes es el tamaño de roster que el seed necesita para tener los
// cinco arquetipos representados y dos destacados por especialidad (tarea 7.8).
const desiredAthletes = 7

func (s *seeder) createUser(email, dni, name, surname string) error {
	var existing dbs.User
	if err := s.db.Where("email = ?", email).First(&existing).Error; err == nil {
		return nil
	}
	user := &dbs.User{
		Name:      name,
		Surname:   surname,
		Email:     email,
		DNI:       dni,
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Password:  "seed",
		Status:    "active",
	}
	return s.db.Create(user).Error
}

func (s *seeder) bootstrapTeamAndGroup() error {
	var trainer dbs.User
	if err := s.db.Where("email = ?", s.runnerEmail()).First(&trainer).Error; err != nil {
		return fmt.Errorf("no se pudo leer el entrenador sembrado: %w", err)
	}
	// Los ids van explícitos: el scope del seed es una constante (tarea 7.1) y
	// no puede depender de en qué punto de la secuencia esté la base.
	team := dbs.Team{
		ID:         s.teamID,
		Name:       "Semilla dashboard",
		OwnerID:    trainer.ID,
		MaxMembers: 30,
		Status:     "active",
	}
	if err := s.db.Where("id = ?", s.teamID).First(&dbs.Team{}).Error; err != nil {
		if err := s.db.Create(&team).Error; err != nil {
			return fmt.Errorf("no se pudo crear el equipo %d: %w", s.teamID, err)
		}
	}
	group := dbs.Group{ID: s.groupID, Name: "Semilla dashboard · grupo", TeamID: s.teamID, IsMain: true}
	if err := s.db.Where("id = ?", s.groupID).First(&dbs.Group{}).Error; err != nil {
		if err := s.db.Create(&group).Error; err != nil {
			return fmt.Errorf("no se pudo crear el grupo %d: %w", s.groupID, err)
		}
	}
	return s.bootstrapMembership(&trainer)
}

// bootstrapMembership mete al entrenador y a los corredores al grupo y al equipo:
// sin filas en group_users la query del dashboard no devuelve nada (la membresía
// se evalúa por ventana de fechas), y el scope quedaría sembrado a ciegas.
func (s *seeder) bootstrapMembership(trainer *dbs.User) error {
	from := s.window.from()
	athletes := make([]int64, 0, desiredAthletes)
	for i := 1; i <= desiredAthletes; i++ {
		var athlete dbs.User
		if err := s.db.Where("email = ?", s.athleteEmail(i)).First(&athlete).Error; err != nil {
			return fmt.Errorf("no se pudo leer el corredor sembrado %d: %w", i, err)
		}
		athletes = append(athletes, athlete.ID)
	}
	// El entrenador va al equipo pero NO al grupo: group_users es el roster de
	// corredores y con el dueño adentro el leaderboard lo mostraría como atleta.
	for _, userID := range append([]int64{trainer.ID}, athletes...) {
		isTrainer := userID == trainer.ID
		var teamUser int64
		err := s.db.Table("team_users").
			Where("team_id = ? AND user_id = ? AND deleted_at IS NULL", s.teamID, userID).
			Pluck("id", &teamUser).Error
		if err != nil {
			return fmt.Errorf("error al leer la asociación al equipo del user %d: %w", userID, err)
		}
		if teamUser != 0 {
			continue
		}
		role := string(constants.TeamUserRoleCorredor)
		assignmentDate := from.AddDate(0, 0, -30)
		if isTrainer {
			role = string(constants.TeamUserRoleEntrenador)
			assignmentDate = from.AddDate(0, 0, -60)
		}
		teamUserRow := dbs.TeamUser{
			TeamID: s.teamID, UserID: userID, RoleInTeam: role,
			Status: "active", AssignmentDate: assignmentDate,
		}
		if err := s.db.Create(&teamUserRow).Error; err != nil {
			return fmt.Errorf("error creando la asociación al equipo del user %d: %w", userID, err)
		}
		if isTrainer {
			continue
		}
		var groupUser int64
		if err := s.db.Table("group_users").
			Where("group_id = ? AND user_id = ? AND deleted_at IS NULL", s.groupID, userID).
			Pluck("id", &groupUser).Error; err != nil {
			return fmt.Errorf("error al leer la membresía del user %d: %w", userID, err)
		}
		if groupUser != 0 {
			continue
		}
		member := dbs.GroupUser{GroupID: s.groupID, UserID: userID, DateStart: from}
		if err := s.db.Create(&member).Error; err != nil {
			return fmt.Errorf("error creando la membresía del user %d: %w", userID, err)
		}
	}
	return nil
}
