package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
	"simple-arq-golang/cmd/api/utils/routematch"
)

func createCompetitionForValidation(t *testing.T, db *gorm.DB, status string) *dbs.Competition {
	t.Helper()
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	row := &dbs.Competition{
		TeamID: 10, Type: "metric", StartAt: start, EndAt: &end,
		CreatedByUserID: 1, CreatedByRole: "trainer", Status: status,
	}
	require.NoError(t, db.Create(row).Error)
	return row
}

func TestValidateFullCompetitionFinalState(t *testing.T) {
	db := testutils.SetupTestDB(t)

	require.Error(t, validateFullCompetitionFinalState(db, 999999, fullCompetitionSpec{status: "active"}))

	row := createCompetitionForValidation(t, db, "pending_approval")
	spec := fullCompetitionSpec{status: "active"}
	err := validateFullCompetitionFinalState(db, row.ID, spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "estado final")

	require.NoError(t, db.Model(row).Update("status", "rejected").Error)
	spec = fullCompetitionSpec{status: "rejected", rejectionReason: "sin cupo"}
	err = validateFullCompetitionFinalState(db, row.ID, spec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "motivo de rechazo")

	reason := "sin cupo"
	require.NoError(t, db.Model(row).Update("rejection_reason", reason).Error)
	require.NoError(t, validateFullCompetitionFinalState(db, row.ID, spec))
}

func TestValidateFullCompetitionSharedInstance(t *testing.T) {
	db := testutils.SetupTestDB(t)
	row := createCompetitionForValidation(t, db, "active")

	missing := int64(999999)
	row.RouteSessionInstanceID = &missing
	require.Error(t, validateFullCompetitionSharedInstance(db, row))

	instance := &dbs.SessionInstance{Name: "otra"}
	require.NoError(t, db.Create(instance).Error)
	row.RouteSessionInstanceID = &instance.ID
	err := validateFullCompetitionSharedInstance(db, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "identidad esperada")

	instance.Name = "Competencia ruta " + itoa(row.ID)
	require.NoError(t, db.Save(instance).Error)
	err = validateFullCompetitionSharedInstance(db, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ejercicio sintético")

	exercise := &dbs.ExerciseInstance{Name: "Otra", Kind: "liviano"}
	require.NoError(t, db.Create(exercise).Error)
	require.NoError(t, db.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: instance.ID, ExerciseInstanceID: exercise.ID,
		Role: "main", RepeatCount: 1, RestMinutes: 0,
	}).Error)
	err = validateFullCompetitionSharedInstance(db, row)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no es sintético")

	exercise.Name = "Corrida"
	exercise.Kind = "velocidad_distancia"
	require.NoError(t, db.Save(exercise).Error)
	require.NoError(t, validateFullCompetitionSharedInstance(db, row))
}

func TestEnsureFullCompetitionAttemptInstance_CreaYValida(t *testing.T) {
	db := testutils.SetupTestDB(t)
	now := time.Date(2026, time.October, 13, 9, 0, 0, 0, gametime.Location)

	require.NoError(t, db.Create(&dbs.SessionInstance{Name: "duplicada"}).Error)
	require.NoError(t, db.Create(&dbs.SessionInstance{Name: "duplicada"}).Error)
	_, err := ensureFullCompetitionAttemptInstance(db, "duplicada", now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicadas")

	require.NoError(t, db.Create(&dbs.SessionInstance{Name: "sin marker"}).Error)
	_, err = ensureFullCompetitionAttemptInstance(db, "sin marker", now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sin marker full")

	id, err := ensureFullCompetitionAttemptInstance(db, "intento nuevo", now)
	require.NoError(t, err)
	assert.Positive(t, id)

	sameID, err := ensureFullCompetitionAttemptInstance(db, "intento nuevo", now)
	require.NoError(t, err)
	assert.Equal(t, id, sameID)
}

func TestEnsureFullCompetitionFeedback_CreaYCompara(t *testing.T) {
	db := testutils.SetupTestDB(t)
	annotations := fullSeedMarker + " " + fullCompetitionEvidenceMarker + " evidencia"
	teamID := int64(10)
	distance := 5000.0
	duration := int64(60000)
	expected := dbs.WorkoutFeedback{
		TeamID: &teamID, AssignedSessionID: 100, AssignedExerciseID: 200,
		AthleteUserID: 1, FeedbackOwnerUserID: 2, ReportSource: "corredor",
		SessionDate: time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		SetNumber:   1, DistanceMeters: &distance, DurationMs: &duration,
		Annotations: &annotations, MediaURLs: emptyMediaURLs(t),
	}

	id, err := ensureFullCompetitionFeedback(db, expected)
	require.NoError(t, err)
	assert.Positive(t, id)

	var stored dbs.WorkoutFeedback
	require.NoError(t, db.First(&stored, id).Error)
	_, err = ensureFullCompetitionFeedback(db, stored)
	require.NoError(t, err)

	other := 6000.0
	stored.DistanceMeters = &other
	_, err = ensureFullCompetitionFeedback(db, stored)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "feedback distinto")
}

func TestEnsureFullCompetitionGPSPoints_CreaYCompara(t *testing.T) {
	db := testutils.SetupTestDB(t)
	startedAt := time.Date(2026, time.October, 13, 9, 0, 0, 0, gametime.Location)
	points := []routematch.Point{{Lat: -31.4, Lng: -64.2}, {Lat: -31.41, Lng: -64.21}}

	require.NoError(t, ensureFullCompetitionGPSPoints(db, 1, 2, 3, points, startedAt))

	var count int64
	require.NoError(t, db.Model(&dbs.WorkoutFeedbackPoint{}).Where("feedback_id = ?", 1).Count(&count).Error)
	assert.Equal(t, int64(2), count)

	require.NoError(t, ensureFullCompetitionGPSPoints(db, 1, 2, 3, points, startedAt))

	changed := []routematch.Point{{Lat: -31.5, Lng: -64.2}, {Lat: -31.41, Lng: -64.21}}
	err := ensureFullCompetitionGPSPoints(db, 1, 2, 3, changed, startedAt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "punto GPS")

	err = ensureFullCompetitionGPSPoints(db, 1, 2, 3, points[:1], startedAt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cantidad de puntos")
}

func itoa(value int64) string {
	return fmt.Sprintf("%d", value)
}
