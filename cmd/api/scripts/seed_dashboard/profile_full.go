package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const (
	dashboardProfile = "dashboard"
	fullProfile      = "full"
	fullSeedMarker   = "[seed_full]"
)

const (
	fullTeamAID int64 = 1001 + iota
	fullTeamBID
	fullTeamCID
	fullTeamDID
	fullTeamEID
	fullTeamFID
)

const (
	fullGroupAMainID int64 = 2001 + iota
	fullGroupASecondaryID
	fullGroupBID
	fullGroupCID
	fullGroupDID
	fullGroupEID
	fullGroupFID
)

type fullScope struct {
	teamAID, teamBID, teamCID, teamDID, teamEID, teamFID int64
	groupAMainID, groupASecondaryID                      int64
	groupBID, groupCID, groupDID, groupEID, groupFID     int64
}

type fullSeedIDs struct {
	users             map[string]int64
	roles             map[string]int64
	tiers             map[string]int64
	subscriptions     map[string]int64
	exercises         map[string]int64
	sessions          map[string]int64
	trainingPlans     map[string]int64
	calendarDays      map[string]int64
	sessionInstances  map[string]int64
	exerciseInstances map[string]int64
	runnerSessions    map[string]int64
	workoutFeedback   map[string]int64
	competitions      map[string]int64
	payments          map[string]int64
}

type fullSeeder struct {
	db    *gorm.DB
	opts  options
	scope fullScope
	now   time.Time
	ids   fullSeedIDs
}

func validateSeedProfile(profile string) error {
	if profile != dashboardProfile && profile != fullProfile {
		return fmt.Errorf("--profile inválido %q: usar dashboard|full", profile)
	}
	return nil
}

func newFullSeeder(db *gorm.DB, opts options) *fullSeeder {
	return &fullSeeder{
		db:   db,
		opts: opts,
		scope: fullScope{
			teamAID: fullTeamAID, teamBID: fullTeamBID, teamCID: fullTeamCID,
			teamDID: fullTeamDID, teamEID: fullTeamEID, teamFID: fullTeamFID,
			groupAMainID: fullGroupAMainID, groupASecondaryID: fullGroupASecondaryID,
			groupBID: fullGroupBID, groupCID: fullGroupCID, groupDID: fullGroupDID,
			groupEID: fullGroupEID, groupFID: fullGroupFID,
		},
		now: gametime.Now().In(gametime.Location),
		ids: fullSeedIDs{
			users: make(map[string]int64), roles: make(map[string]int64),
			tiers: make(map[string]int64), subscriptions: make(map[string]int64),
			exercises: make(map[string]int64), sessions: make(map[string]int64),
			trainingPlans: make(map[string]int64), calendarDays: make(map[string]int64),
			sessionInstances: make(map[string]int64), exerciseInstances: make(map[string]int64),
			runnerSessions: make(map[string]int64), workoutFeedback: make(map[string]int64),
			competitions: make(map[string]int64), payments: make(map[string]int64),
		},
	}
}

func runFullSeed(db *gorm.DB, opts options) error {
	if err := validateSeedProfile(opts.profile); err != nil {
		return err
	}
	s := newFullSeeder(db, opts)
	if err := s.guardScope(); err != nil {
		return err
	}
	switch {
	case opts.cleanup:
		return fmt.Errorf("perfil full incompleto: cleanupFull todavía no está implementado")
	case opts.verify:
		return fmt.Errorf("perfil full incompleto: verifyFull todavía no está implementado")
	default:
		// Orden D14: identidad, equipos, catálogo/calendario, actividad, gamificación, competencias y pagos/mensajes.
		return fmt.Errorf("perfil full incompleto: las fases de siembra todavía no están conectadas; no se sembró ningún stage")
	}
}

func (s *fullSeeder) guardScope() error {
	teams := []struct {
		name string
		id   int64
	}{
		{"team A", s.scope.teamAID}, {"team B", s.scope.teamBID},
		{"team C", s.scope.teamCID}, {"team D", s.scope.teamDID},
		{"team E", s.scope.teamEID}, {"team F", s.scope.teamFID},
	}
	for _, scope := range teams {
		var team dbs.Team
		err := s.db.Select("id", "description").Where("id = ?", scope.id).Take(&team).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error al leer %s reservado %d: %w", scope.name, scope.id, err)
		}
		if !strings.Contains(team.Description, fullSeedMarker) {
			return fmt.Errorf("%s reservado %d ya existe sin el marker %s; se aborta para no adoptar datos ajenos", scope.name, scope.id, fullSeedMarker)
		}
	}

	groups := []struct {
		name string
		id   int64
	}{
		{"group A principal", s.scope.groupAMainID}, {"group A secundario", s.scope.groupASecondaryID},
		{"group B", s.scope.groupBID}, {"group C", s.scope.groupCID},
		{"group D", s.scope.groupDID}, {"group E", s.scope.groupEID},
		{"group F", s.scope.groupFID},
	}
	for _, scope := range groups {
		var group dbs.Group
		err := s.db.Select("id", "description").Where("id = ?", scope.id).Take(&group).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("error al leer %s reservado %d: %w", scope.name, scope.id, err)
		}
		if !strings.Contains(group.Description, fullSeedMarker) {
			return fmt.Errorf("%s reservado %d ya existe sin el marker %s; se aborta para no adoptar datos ajenos", scope.name, scope.id, fullSeedMarker)
		}
	}
	return nil
}
