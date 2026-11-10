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

// Create sobre la instancia sintética de una competencia route (sin día de
// calendario ni team derivable): el feedback se persiste con team nil y un
// team_id explícito se rechaza (optimizaciones D2).
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

	created, err := svc.Create(nil, athlete.ID, req)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Nil(t, created.TeamID)

	explicitTeam := team.ID
	req.TeamID = &explicitTeam
	_, err = svc.Create(nil, athlete.ID, req)
	require.ErrorIs(t, err, ErrWorkoutFeedbackInvalid)
	assert.Contains(t, err.Error(), "team_id")
}
