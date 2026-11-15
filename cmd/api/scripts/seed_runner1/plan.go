package main

import (
	"fmt"
	"time"

	appconstants "simple-arq-golang/cmd/api/constants"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
)

const (
	defaultEmail        = "corredor.paceron.1@gmail.com"
	timelineDays        = 30
	fixtureMarkerPrefix = "[runner1_data:"
	runTag              = "runner1_data"
)

// expectedUnlockedAchievements es el set exacto que el fixture debe dejar
// desbloqueado. El resto del catálogo queda bloqueado.
var expectedUnlockedAchievements = []string{
	"first_session", "sessions_5", "sessions_25",
	"streak_3", "streak_7", "streak_14", "early_5",
	"distance_10k", "distance_50k",
	"gps_first", "gps_10",
	"team_first", "team_10",
}

// expectedLockedAchievements es el complemento del catálogo. No se inserta:
// existe para preflight/verificación.
var expectedLockedAchievements = []string{
	"streak_30", "streak_60",
	"sessions_50", "sessions_100", "sessions_200",
	"distance_100k", "distance_42k_single",
	"gps_50",
}

// dayBlueprint describe un día relativo al ancla (offset 1 = ayer). Es puro:
// no toca DB ni reloj, así los tests fijan el resultado sin Postgres.
type dayBlueprint struct {
	Offset          int
	DayStatus       string
	RunnerStatus    string
	StartHour       int
	StartMinute     int
	HasFeedback     bool
	DistanceMeters  float64
	HasGPS          bool
	HasAttendance   bool
	DurationMinutes int
}

type fixtureDay struct {
	Offset         int
	Date           time.Time
	Marker         string
	DayStatus      string
	RunnerStatus   string
	Start          time.Time
	End            *time.Time
	HasFeedback    bool
	DistanceMeters float64
	HasGPS         bool
	HasAttendance  bool
}

type fixturePlan struct {
	Anchor time.Time
	Days   []fixtureDay

	ExpectedFinishedSessions int
	ExpectedEarlySessions    int
	ExpectedDistanceMeters   float64
	ExpectedMaxSingleMeters  float64
	ExpectedGPSDays          int
	ExpectedAttendanceDays   int
	ExpectedStreakCurrent    int
	ExpectedStreakBest       int
}

func dayBlueprintForOffset(offset int) dayBlueprint {
	switch {
	case offset >= 1 && offset <= 10:
		startHour := 18
		startMinute := 30
		if offset <= 5 {
			startHour = 6
			startMinute = 45
		}
		return dayBlueprint{
			Offset: offset, DayStatus: "completed",
			RunnerStatus: runnersession.RunnerSessionStatusFinished,
			StartHour:    startHour, StartMinute: startMinute,
			HasFeedback: true, DistanceMeters: 4000, HasGPS: true,
			HasAttendance: true, DurationMinutes: 30,
		}
	case offset == 11:
		return dayBlueprint{
			Offset: offset, DayStatus: "failed",
			RunnerStatus: runnersession.RunnerSessionStatusWip,
			StartHour:    18, StartMinute: 30, DurationMinutes: 30,
		}
	case offset >= 12 && offset <= 25:
		blueprint := dayBlueprint{
			Offset: offset, DayStatus: "completed",
			RunnerStatus: runnersession.RunnerSessionStatusFinished,
			StartHour:    18, StartMinute: 30, DurationMinutes: 30,
		}
		switch offset {
		case 24:
			blueprint.HasFeedback = true
			blueprint.DistanceMeters = 4000
		case 25:
			blueprint.HasFeedback = true
			blueprint.DistanceMeters = 21000
			blueprint.DurationMinutes = 105
		}
		return blueprint
	case offset == 26:
		return dayBlueprint{
			Offset: offset, DayStatus: "failed",
			RunnerStatus: runnersession.RunnerSessionStatusWip,
			StartHour:    18, StartMinute: 30, DurationMinutes: 30,
		}
	case offset >= 27 && offset <= timelineDays:
		return dayBlueprint{
			Offset: offset, DayStatus: "completed",
			RunnerStatus: runnersession.RunnerSessionStatusFinished,
			StartHour:    18, StartMinute: 30, DurationMinutes: 30,
		}
	default:
		return dayBlueprint{Offset: offset, DayStatus: "pending"}
	}
}

func buildFixturePlan(anchor time.Time) fixturePlan {
	anchor = dateOnly(anchor)
	plan := fixturePlan{
		Anchor:                   anchor,
		ExpectedFinishedSessions: 28,
		ExpectedEarlySessions:    5,
		ExpectedDistanceMeters:   65000,
		ExpectedMaxSingleMeters:  21000,
		ExpectedGPSDays:          10,
		ExpectedAttendanceDays:   10,
		ExpectedStreakCurrent:    10,
		ExpectedStreakBest:       14,
	}
	plan.Days = make([]fixtureDay, 0, timelineDays)
	for offset := 1; offset <= timelineDays; offset++ {
		blueprint := dayBlueprintForOffset(offset)
		date := anchor.AddDate(0, 0, -offset)
		start := time.Date(date.Year(), date.Month(), date.Day(), blueprint.StartHour, blueprint.StartMinute, 0, 0, gametime.Location)
		var end *time.Time
		if blueprint.RunnerStatus == runnersession.RunnerSessionStatusFinished {
			finishedAt := start.Add(time.Duration(blueprint.DurationMinutes) * time.Minute)
			end = &finishedAt
		}
		plan.Days = append(plan.Days, fixtureDay{
			Offset:         offset,
			Date:           date,
			Marker:         markerFor(date),
			DayStatus:      blueprint.DayStatus,
			RunnerStatus:   blueprint.RunnerStatus,
			Start:          start,
			End:            end,
			HasFeedback:    blueprint.HasFeedback,
			DistanceMeters: blueprint.DistanceMeters,
			HasGPS:         blueprint.HasGPS,
			HasAttendance:  blueprint.HasAttendance,
		})
	}
	return plan
}

// markerFor es la clave natural del fixture: estable para la misma fecha, así
// una segunda corrida reutiliza las instancias en vez de duplicarlas.
func markerFor(date time.Time) string {
	return fmt.Sprintf("%s%s]", fixtureMarkerPrefix, dateOnly(date).Format("2006-01-02"))
}

func isFixtureMarker(description *string) bool {
	if description == nil {
		return false
	}
	return len(*description) >= len(fixtureMarkerPrefix) && (*description)[:len(fixtureMarkerPrefix)] == fixtureMarkerPrefix
}

func stampSourceName(index int, name string) string {
	return fmt.Sprintf("%s %d · %s", runTag, index, name)
}

func isFixtureStampName(name *string) bool {
	if name == nil {
		return false
	}
	return len(*name) > len(runTag) && (*name)[:len(runTag)] == runTag
}

func dateOnly(value time.Time) time.Time {
	local := value.In(gametime.Location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, gametime.Location)
}

// expectedAchievementSets valida que unlocked+locked cubran el catálogo sin
// solaparse. Se usa en tests y en preflight.
func expectedAchievementSets() (unlocked, locked []string, err error) {
	catalog := make(map[string]struct{}, len(expectedUnlockedAchievements)+len(expectedLockedAchievements))
	for _, id := range expectedUnlockedAchievements {
		if _, exists := catalog[id]; exists {
			return nil, nil, fmt.Errorf("logro %q duplicado en el set esperado", id)
		}
		catalog[id] = struct{}{}
	}
	for _, id := range expectedLockedAchievements {
		if _, exists := catalog[id]; exists {
			return nil, nil, fmt.Errorf("logro %q está en desbloqueados y bloqueados", id)
		}
		catalog[id] = struct{}{}
	}
	if len(catalog) != len(appconstants.AchievementCatalog) {
		return nil, nil, fmt.Errorf("el set esperado cubre %d logros y el catálogo tiene %d", len(catalog), len(appconstants.AchievementCatalog))
	}
	for _, row := range appconstants.AchievementCatalog {
		if _, ok := catalog[row.ID]; !ok {
			return nil, nil, fmt.Errorf("el catálogo tiene %q fuera del set esperado", row.ID)
		}
	}
	return expectedUnlockedAchievements, expectedLockedAchievements, nil
}
