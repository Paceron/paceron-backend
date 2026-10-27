package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/config"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/testutils"
)

// Ids del scope del test: altos a propósito. El seed los inserta explícitos y la
// transacción del test nunca se committea, así que un id bajo podría chocar con el
// autoincrement de otro paquete corriendo en paralelo contra la misma base.
const (
	testSeedTeamID  int64 = 999_000_001
	testSeedGroupID int64 = 999_000_002
)

// TestSeed_ProduceLosSeisContadores corre el seed completo contra Postgres y
// comprueba lo que §9.3 declara: el dataset tiene que producir los seis
// contadores de data_quality, no un dataset limpio donde todos dan cero.
//
// Es el test que hace que los números del runbook no se pudran: el plan de
// defectos y las fórmulas del dominio tienen que seguir de acuerdo.
func TestSeed_ProduceLosSeisContadores(t *testing.T) {
	db := testutils.SetupTestDB(t)
	// El stamp de fechas pasadas depende de la bandera de §2; sin ella el
	// calendario cierra toda fecha anterior a hoy y el seed no tiene nada que
	// sembrar.
	previousSeeding := config.Seeding
	config.Seeding = true
	t.Cleanup(func() { config.Seeding = previousSeeding })

	s := newTestSeeder(t, db)
	require.NoError(t, s.resolveScope())
	require.NoError(t, s.run())

	quality := summaryDataQuality(t, db, s)
	// Los valores exactos salen del plan de defectos de generator.go. Si cambia
	// uno, este test es el que avisa de que hay que actualizar el runbook.
	assert.Equal(t, 4, quality.InvalidSeries, "series no calculables (2 sin distancia + 2 sin tiempo)")
	// 5 series sin active_duration_ms y 2 de las que no tienen ningún tiempo: la
	// degradación y la invalidez se solapan, y está bien que así sea.
	assert.Equal(t, 7, quality.DegradedTime)
	assert.Equal(t, 3, quality.UnknownStatus)
	assert.Equal(t, 2, quality.OpenSessions)
	assert.Positive(t, quality.LegacySessions)
	assert.False(t, quality.InsufficientData, "con el scope completo hay suficientes series")

	// El caso de insufficient_data se ve filtrando por el corredor con una sola
	// sesión, que es como lo va a preguntar el front.
	enProgreso := findAthleteByArchetype(t, s, archetypeEnProgreso)
	filtered := summaryDataQualityFor(t, db, s, []int64{enProgreso.UserID})
	assert.True(t, filtered.InsufficientData)
}

// El caso 2 de legacy_sessions: instancias sin source_exercise_id que el fallback
// por nombre tiene que resolver. Sin esto, /exercises/{id}/series perdería el
// histórico en silencio y el contador no lo señalaría.
func TestSeed_DejaInstanciasParaElFallbackPorNombre(t *testing.T) {
	db := testutils.SetupTestDB(t)
	previousSeeding := config.Seeding
	config.Seeding = true
	t.Cleanup(func() { config.Seeding = previousSeeding })

	s := newTestSeeder(t, db)
	require.NoError(t, s.resolveScope())
	require.NoError(t, s.run())

	dao := daos.NewDashboardDao(db)
	unlinked, err := dao.UnlinkedExerciseInstances(nil, []int64{s.groupID})
	require.NoError(t, err)
	require.NotEmpty(t, unlinked, "el seed tiene que dejar instancias sin source_exercise_id")

	team, err := daos.NewTeamDao(db).FindByID(nil, s.teamID)
	require.NoError(t, err)
	require.NotNil(t, team)
	catalog, err := dao.FindCatalogExercisesByOwner(nil, team.OwnerID, constants.CurrentExerciseKinds())
	require.NoError(t, err)

	entries := make([]dashboard.CatalogExercise, 0, len(catalog))
	for _, exercise := range catalog {
		entries = append(entries, dashboard.CatalogExercise{ID: exercise.ID, OwnerID: exercise.OwnerID, Name: exercise.Name})
	}
	index := dashboard.NewNameIndex(entries)

	resolved := 0
	for _, instance := range unlinked {
		if _, _, ok := index.Resolve(team.OwnerID, instance.Name); ok {
			resolved++
		}
	}
	assert.Equal(t, len(unlinked), resolved,
		"toda instancia sin link tiene que resolver por nombre contra el catálogo del mismo dueño")
}

func newTestSeeder(t *testing.T, db *gorm.DB) *seeder {
	t.Helper()
	opts := options{seed: true, bootstrap: true, teamID: testSeedTeamID, groupID: testSeedGroupID}
	return &seeder{
		db: db, opts: opts, window: defaultWindow(time.Now()),
		teamID: opts.teamID, groupID: opts.groupID,
		counts: map[string]int64{},
	}
}

func findAthleteByArchetype(t *testing.T, s *seeder, key string) seedAthlete {
	t.Helper()
	for _, athlete := range s.roster {
		if athlete.Archetype.Key == key {
			return athlete
		}
	}
	t.Fatalf("el roster no tiene ningún corredor %q", key)
	return seedAthlete{}
}

func dashboardServiceFor(db *gorm.DB) services.DashboardServiceInterface {
	return services.NewDashboardService(
		daos.NewDashboardDao(db),
		services.NewDashboardScopeService(daos.NewGroupDao(db), daos.NewGroupUserDao(db), daos.NewTeamDao(db)),
		daos.NewUserDao(db),
		daos.NewGroupUserDao(db),
	)
}

func summaryDataQuality(t *testing.T, db *gorm.DB, s *seeder) dashboard.DataQuality {
	t.Helper()
	return summaryDataQualityFor(t, db, s, nil)
}

func summaryDataQualityFor(t *testing.T, db *gorm.DB, s *seeder, athleteIDs []int64) dashboard.DataQuality {
	t.Helper()
	groupID := s.groupID
	query := services.MetricsQuery{
		TeamID:      s.teamID,
		GroupID:     &groupID,
		From:        s.window.from().Format(dateLayout),
		To:          s.window.to().Format(dateLayout),
		Granularity: "session",
		Tolerance:   "0.05",
	}
	query.AthleteUserIDs = athleteIDs
	summary, err := dashboardServiceFor(db).Summary(nil, s.ownerID, query)
	require.NoError(t, err)
	return summary.DataQuality
}
