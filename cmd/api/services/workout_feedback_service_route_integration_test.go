package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/testutils"
)

// Create sobre la instancia sintética de una competencia route sin día de
// calendario: el team se deriva de la competencia (fix acceso route) y un
// team_id explícito que coincide se acepta; uno distinto sigue 400.
func TestWorkoutFeedbackService_Create_RouteInstanceWithoutTeam(t *testing.T) {
	db := testutils.SetupTestDB(t)

	owner := &dbs.User{
		Name: "Route", Surname: "Owner", Email: "route-int-owner@test.com", DNI: "79000001",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed",
	}
	require.NoError(t, db.Create(owner).Error)
	athlete := &dbs.User{
		Name: "Route", Surname: "Athlete", Email: "route-int-athlete@test.com", DNI: "79000002",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed",
	}
	require.NoError(t, db.Create(athlete).Error)
	team := &dbs.Team{Name: "equipo_route_int", MaxMembers: 20, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	require.NoError(t, db.Create(&dbs.TeamUser{
		TeamID: team.ID, UserID: athlete.ID, RoleInTeam: "corredor", AssignmentDate: time.Now(),
	}).Error)

	instance := &dbs.SessionInstance{Name: "Instancia sintética route"}
	require.NoError(t, db.Create(instance).Error)
	exercise := &dbs.ExerciseInstance{Name: "Ruta", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: instance.ID, ExerciseInstanceID: exercise.ID, Role: "main", RepeatCount: 1,
	}).Error)

	route := `{"type":"LineString","coordinates":[[-64.18,-31.42],[-64.19,-31.43]]}`
	radius := 50.0
	comp := &dbs.Competition{
		Type:                   competition.TypeRoute,
		RouteGeoJSON:           &route,
		AcceptanceRadiusMeters: &radius,
		StartAt:                time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TeamID:                 team.ID,
		CreatedByUserID:        owner.ID,
		CreatedByRole:          competition.CreatorRoleTrainer,
		Status:                 competition.StatusActive,
		RouteSessionInstanceID: &instance.ID,
	}
	require.NoError(t, db.Create(comp).Error)
	require.NoError(t, db.Create(&dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: athlete.ID}).Error)

	svc := NewWorkoutFeedbackService(daos.NewWorkoutFeedbackDao(db), daos.NewSessionInstanceDao(db))

	req := workoutfeedback.CreateFeedbackRequest{
		AssignedSessionID:  instance.ID,
		AssignedExerciseID: exercise.ID,
		ReportSource:       "corredor",
		SessionDate:        "2026-01-15",
		SetNumber:          1,
	}

	t.Run("team_derivado_de_la_competencia", func(t *testing.T) {
		created, err := svc.Create(nil, athlete.ID, req)
		require.NoError(t, err)
		require.NotNil(t, created)
		require.NotNil(t, created.TeamID)
		assert.Equal(t, team.ID, *created.TeamID)
	})

	t.Run("team_explicito_coincidente_se_acepta", func(t *testing.T) {
		explicit := req
		explicit.SetNumber = 2
		teamID := team.ID
		explicit.TeamID = &teamID
		created, err := svc.Create(nil, athlete.ID, explicit)
		require.NoError(t, err)
		require.NotNil(t, created.TeamID)
		assert.Equal(t, team.ID, *created.TeamID)
	})

	t.Run("team_explicito_distinto_sigue_400", func(t *testing.T) {
		otherTeam := &dbs.Team{Name: "equipo_route_otro", MaxMembers: 20, OwnerID: owner.ID}
		require.NoError(t, db.Create(otherTeam).Error)
		explicit := req
		explicit.SetNumber = 3
		explicit.TeamID = &otherTeam.ID
		_, err := svc.Create(nil, athlete.ID, explicit)
		require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
		assert.Contains(t, err.Error(), "team_id")
	})
}

// El entrenador dueño del equipo de la competencia puede leer el feedback del
// intento route vía GetSessionFeedback: antes el team_id quedaba nil y la rama
// de feedback de HasInstanceAccess no lo autorizaba.
func TestWorkoutFeedbackService_GetSessionFeedback_RouteAttempt_TeamOwner(t *testing.T) {
	db := testutils.SetupTestDB(t)

	owner := &dbs.User{
		Name: "Route", Surname: "Owner", Email: "route-read-owner@test.com", DNI: "79000011",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed",
	}
	require.NoError(t, db.Create(owner).Error)
	athlete := &dbs.User{
		Name: "Route", Surname: "Athlete", Email: "route-read-athlete@test.com", DNI: "79000012",
		BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed",
	}
	require.NoError(t, db.Create(athlete).Error)
	team := &dbs.Team{Name: "equipo_route_read", MaxMembers: 20, OwnerID: owner.ID}
	require.NoError(t, db.Create(team).Error)
	require.NoError(t, db.Create(&dbs.TeamUser{
		TeamID: team.ID, UserID: athlete.ID, RoleInTeam: "corredor", AssignmentDate: time.Now(),
	}).Error)

	instance := &dbs.SessionInstance{Name: "Instancia route read"}
	require.NoError(t, db.Create(instance).Error)
	exercise := &dbs.ExerciseInstance{Name: "Ruta", Kind: "running"}
	require.NoError(t, db.Create(exercise).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: instance.ID, ExerciseInstanceID: exercise.ID, Role: "main", RepeatCount: 1,
	}).Error)

	route := `{"type":"LineString","coordinates":[[-64.18,-31.42],[-64.19,-31.43]]}`
	radius := 50.0
	comp := &dbs.Competition{
		Type:                   competition.TypeRoute,
		RouteGeoJSON:           &route,
		AcceptanceRadiusMeters: &radius,
		StartAt:                time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TeamID:                 team.ID,
		CreatedByUserID:        owner.ID,
		CreatedByRole:          competition.CreatorRoleTrainer,
		Status:                 competition.StatusActive,
		RouteSessionInstanceID: &instance.ID,
	}
	require.NoError(t, db.Create(comp).Error)
	require.NoError(t, db.Create(&dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: athlete.ID}).Error)

	svc := NewWorkoutFeedbackService(daos.NewWorkoutFeedbackDao(db), daos.NewSessionInstanceDao(db))
	_, err := svc.Create(nil, athlete.ID, workoutfeedback.CreateFeedbackRequest{
		AssignedSessionID:  instance.ID,
		AssignedExerciseID: exercise.ID,
		ReportSource:       "corredor",
		SessionDate:        "2026-01-15",
		SetNumber:          1,
	})
	require.NoError(t, err)

	feedbacks, err := svc.GetSessionFeedback(nil, owner.ID, instance.ID, &athlete.ID)
	require.NoError(t, err)
	require.Len(t, feedbacks, 1)
	assert.Equal(t, team.ID, *feedbacks[0].TeamID)
}
