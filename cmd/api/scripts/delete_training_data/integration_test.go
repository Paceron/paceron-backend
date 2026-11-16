package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// TestDeleteTrainingData_Integration corre el plan y el borrado contra Postgres
// real (testutils.SetupTestDB, se skipea sin TEST_DB_HOST) y verifica el
// contrato central del script: borra el rango pedido, conserva lo anterior y no
// toca instancias referenciadas por datos de otro rango.
func TestDeleteTrainingData_Integration(t *testing.T) {
	tx := testutils.SetupTestDB(t)
	fx := seedDeleteFixture(t, tx)

	from := time.Date(2026, 11, 15, 0, 0, 0, 0, gametime.Location)

	p, err := buildPlan(tx, scope{
		teamID: fx.teamID,
		from:   from,
		kinds:  constants.GetValidGroupCalendarDayKinds(),
	})
	require.NoError(t, err)

	// Días del rango: 15, 18 (protegido) y 20.
	assert.Len(t, p.days, 3)
	assert.Len(t, p.feedback, 1, "solo el feedback del 16 entra en el rango")
	assert.Equal(t, fx.feedbackInRange, p.feedback[0].ID)
	assert.Equal(t, int64(1), p.gamification.Achievements, "solo el logro del 16 entra en el rango")
	assert.Equal(t, int64(1), p.gamification.Stamps)
	assert.Equal(t, int64(1), p.gamification.Unseen)
	assert.Equal(t, int64(1), p.gamification.StreakStates)

	// La instancia del día 18 tiene feedback del 10 (fuera del rango): se
	// conserva aunque su día se borre.
	assert.Contains(t, p.cleanup.Kept, fx.instanceProtected)
	assert.NotContains(t, p.cleanup.Deletable, fx.instanceProtected)
	assert.Contains(t, p.cleanup.Deletable, fx.instanceInRange)
	assert.Contains(t, p.cleanup.Deletable, fx.instanceNoData)

	res, err := applyPlan(tx, p)
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.Feedback)
	assert.Equal(t, int64(1), res.Points)
	assert.Equal(t, int64(3), res.Days)
	assert.Equal(t, int64(2), res.Instances)
	assert.Equal(t, int64(2), res.Exercises)
	assert.Equal(t, int64(1), res.Attendances)
	assert.Equal(t, int64(1), res.RunnerSessions)
	assert.Equal(t, int64(1), res.Achievements)
	assert.Equal(t, int64(1), res.Stamps)
	assert.Equal(t, int64(1), res.Unseen)
	assert.Equal(t, int64(1), res.StreakStates)

	// Lo anterior al 15 sigue intacto.
	assert.True(t, exists(t, tx, &dbs.GroupCalendarDay{}, "id = ?", fx.dayBefore))
	assert.True(t, exists(t, tx, &dbs.WorkoutFeedback{}, "id = ?", fx.feedbackBefore))
	assert.True(t, exists(t, tx, &dbs.WorkoutFeedbackPoint{}, "id = ?", fx.pointBefore))
	assert.True(t, exists(t, tx, &dbs.Attendance{}, "id = ?", fx.attendanceBefore))
	assert.True(t, exists(t, tx, &dbs.RunnerSession{}, "id = ?", fx.runnerSessionBefore))
	assert.True(t, exists(t, tx, &dbs.SessionInstance{}, "id = ?", fx.instanceBefore))
	assert.True(t, exists(t, tx, &dbs.UserAchievementProgress{}, "user_id = ? AND achievement_id = ?", fx.athleteID, "before"))

	// Lo del rango desapareció.
	assert.False(t, exists(t, tx, &dbs.WorkoutFeedback{}, "id = ?", fx.feedbackInRange))
	assert.False(t, exists(t, tx, &dbs.WorkoutFeedbackPoint{}, "id = ?", fx.pointInRange))
	assert.False(t, exists(t, tx, &dbs.GroupCalendarDay{}, "id = ?", fx.dayInRange))
	assert.False(t, exists(t, tx, &dbs.GroupCalendarDay{}, "id = ?", fx.dayProtected))
	assert.False(t, exists(t, tx, &dbs.GroupCalendarDay{}, "id = ?", fx.dayNoData))
	assert.False(t, exists(t, tx, &dbs.SessionInstance{}, "id = ?", fx.instanceInRange))
	assert.False(t, exists(t, tx, &dbs.ExerciseInstance{}, "id = ?", fx.exerciseInRange))
	assert.False(t, exists(t, tx, &dbs.SessionExerciseInstance{}, "session_instance_id = ?", fx.instanceInRange))
	assert.False(t, exists(t, tx, &dbs.Attendance{}, "id = ?", fx.attendanceInRange))
	assert.False(t, exists(t, tx, &dbs.RunnerSession{}, "id = ?", fx.runnerSessionInRange))
	assert.False(t, exists(t, tx, &dbs.UserAchievementProgress{}, "user_id = ? AND achievement_id = ?", fx.athleteID, "in-range"))

	// La instancia protegida queda (con su ejercicio y su link), aunque su día
	// de calendario ya no exista.
	assert.True(t, exists(t, tx, &dbs.SessionInstance{}, "id = ?", fx.instanceProtected))
	assert.True(t, exists(t, tx, &dbs.ExerciseInstance{}, "id = ?", fx.exerciseProtected))
	assert.True(t, exists(t, tx, &dbs.SessionExerciseInstance{}, "session_instance_id = ?", fx.instanceProtected))
	assert.True(t, exists(t, tx, &dbs.WorkoutFeedback{}, "id = ?", fx.feedbackProtected))

	// Gamificación del rango limpiada; la anterior al 15 se respeta.
	assert.True(t, exists(t, tx, &dbs.GamificationStamp{}, "user_id = ? AND granted_at < ?", fx.athleteID, from))
	assert.False(t, exists(t, tx, &dbs.GamificationStamp{}, "user_id = ? AND granted_at >= ?", fx.athleteID, from))
	assert.False(t, exists(t, tx, &dbs.GamificationUnseen{}, "user_id = ?", fx.athleteID))
	assert.False(t, exists(t, tx, &dbs.GamificationStreakState{}, "user_id = ?", fx.athleteID))
}

type deleteFixture struct {
	teamID    int64
	athleteID int64

	instanceBefore    int64
	exerciseBefore    int64
	instanceInRange   int64
	exerciseInRange   int64
	instanceProtected int64
	exerciseProtected int64
	instanceNoData    int64
	exerciseNoData    int64

	dayBefore    int64
	dayInRange   int64
	dayProtected int64
	dayNoData    int64

	feedbackBefore    int64
	feedbackInRange   int64
	feedbackProtected int64
	pointBefore       int64
	pointInRange      int64

	attendanceBefore  int64
	attendanceInRange int64

	runnerSessionBefore  int64
	runnerSessionInRange int64
}

func seedDeleteFixture(t *testing.T, tx *gorm.DB) deleteFixture {
	t.Helper()
	suffix := time.Now().UnixNano()
	loc := gametime.Location

	owner := dbs.User{
		Name: "Owner", Surname: "Test", Email: fmt.Sprintf("owner-%d@delete.test", suffix),
		DNI: fmt.Sprintf("o-%d", suffix), BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Password: "x",
	}
	require.NoError(t, tx.Create(&owner).Error)
	athlete := dbs.User{
		Name: "Athlete", Surname: "Test", Email: fmt.Sprintf("athlete-%d@delete.test", suffix),
		DNI: fmt.Sprintf("a-%d", suffix), BirthDate: time.Date(1995, 1, 1, 0, 0, 0, 0, time.UTC),
		Password: "x",
	}
	require.NoError(t, tx.Create(&athlete).Error)

	team := dbs.Team{Name: "Team Delete", OwnerID: owner.ID, MaxMembers: 50}
	require.NoError(t, tx.Create(&team).Error)
	group := dbs.Group{Name: "Grupo Delete", TeamID: team.ID}
	require.NoError(t, tx.Create(&group).Error)
	require.NoError(t, tx.Create(&dbs.TeamUser{
		TeamID: team.ID, UserID: athlete.ID, RoleInTeam: "corredor", AssignmentDate: time.Now(),
	}).Error)

	fx := deleteFixture{teamID: team.ID, athleteID: athlete.ID}

	// Instancias: una con feedback viejo, una con feedback del rango, una
	// protegida por feedback viejo y una sin datos.
	fx.instanceBefore, fx.exerciseBefore = seedInstance(t, tx, "antes")
	fx.instanceInRange, fx.exerciseInRange = seedInstance(t, tx, "rango")
	fx.instanceProtected, fx.exerciseProtected = seedInstance(t, tx, "protegida")
	fx.instanceNoData, fx.exerciseNoData = seedInstance(t, tx, "sin-datos")

	fx.dayBefore = seedDay(t, tx, group.ID, time.Date(2026, 11, 10, 0, 0, 0, 0, loc), fx.instanceBefore)
	fx.dayInRange = seedDay(t, tx, group.ID, time.Date(2026, 11, 15, 0, 0, 0, 0, loc), fx.instanceInRange)
	fx.dayProtected = seedDay(t, tx, group.ID, time.Date(2026, 11, 18, 0, 0, 0, 0, loc), fx.instanceProtected)
	fx.dayNoData = seedDay(t, tx, group.ID, time.Date(2026, 11, 20, 0, 0, 0, 0, loc), fx.instanceNoData)

	fx.feedbackBefore = seedFeedback(t, tx, team.ID, athlete.ID, fx.instanceBefore, fx.exerciseBefore, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), 1)
	fx.feedbackInRange = seedFeedback(t, tx, team.ID, athlete.ID, fx.instanceInRange, fx.exerciseInRange, time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC), 1)
	fx.feedbackProtected = seedFeedback(t, tx, team.ID, athlete.ID, fx.instanceProtected, fx.exerciseProtected, time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC), 2)

	fx.pointBefore = seedPoint(t, tx, fx.feedbackBefore, fx.instanceBefore, fx.exerciseBefore, 1)
	fx.pointInRange = seedPoint(t, tx, fx.feedbackInRange, fx.instanceInRange, fx.exerciseInRange, 1)

	require.NoError(t, tx.Create(&dbs.Attendance{
		TeamID: team.ID, TrainingSessionID: fx.instanceBefore, UserID: athlete.ID, Source: "manual",
	}).Error)
	require.NoError(t, tx.Create(&dbs.Attendance{
		TeamID: team.ID, TrainingSessionID: fx.instanceInRange, UserID: athlete.ID, Source: "manual",
	}).Error)
	var atts []dbs.Attendance
	require.NoError(t, tx.Where("team_id = ?", team.ID).Order("id").Find(&atts).Error)
	require.Len(t, atts, 2)
	fx.attendanceBefore, fx.attendanceInRange = atts[0].ID, atts[1].ID

	require.NoError(t, tx.Create(&dbs.RunnerSession{
		SessionInstanceID: fx.instanceBefore, AthleteUserID: athlete.ID, Status: "finished", StartDate: time.Now(),
	}).Error)
	require.NoError(t, tx.Create(&dbs.RunnerSession{
		SessionInstanceID: fx.instanceInRange, AthleteUserID: athlete.ID, Status: "wip", StartDate: time.Now(),
	}).Error)
	var rss []dbs.RunnerSession
	require.NoError(t, tx.Where("athlete_user_id = ?", athlete.ID).Order("id").Find(&rss).Error)
	require.Len(t, rss, 2)
	fx.runnerSessionBefore, fx.runnerSessionInRange = rss[0].ID, rss[1].ID

	require.NoError(t, tx.Create(&dbs.UserAchievementProgress{
		UserID: athlete.ID, AchievementID: "before", UnlockedAt: time.Date(2026, 11, 1, 12, 0, 0, 0, loc),
	}).Error)
	require.NoError(t, tx.Create(&dbs.UserAchievementProgress{
		UserID: athlete.ID, AchievementID: "in-range", UnlockedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, loc),
	}).Error)
	require.NoError(t, tx.Create(&dbs.GamificationStamp{
		UserID: athlete.ID, Type: "competition", SourceID: 1, GrantedAt: time.Date(2026, 11, 1, 12, 0, 0, 0, loc),
	}).Error)
	require.NoError(t, tx.Create(&dbs.GamificationStamp{
		UserID: athlete.ID, Type: "competition", SourceID: 2, GrantedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, loc),
	}).Error)
	require.NoError(t, tx.Create(&dbs.GamificationUnseen{
		UserID: athlete.ID, Section: "achievements", CreatedAt: time.Date(2026, 11, 16, 12, 0, 0, 0, loc),
	}).Error)
	require.NoError(t, tx.Create(&dbs.GamificationStreakState{UserID: athlete.ID, StrictCurrent: 3}).Error)

	return fx
}

func seedInstance(t *testing.T, tx *gorm.DB, label string) (int64, int64) {
	t.Helper()
	instance := dbs.SessionInstance{Name: "Sesion " + label}
	require.NoError(t, tx.Create(&instance).Error)
	exercise := dbs.ExerciseInstance{Name: "Ejercicio " + label, Kind: string(constants.ExerciseKindVelocidadDistancia)}
	require.NoError(t, tx.Create(&exercise).Error)
	require.NoError(t, tx.Create(&dbs.SessionExerciseInstance{
		SessionInstanceID: instance.ID, ExerciseInstanceID: exercise.ID,
		Role: string(constants.SessionExerciseRoleMain), RepeatCount: 1,
	}).Error)
	return instance.ID, exercise.ID
}

func seedDay(t *testing.T, tx *gorm.DB, groupID int64, date time.Time, instanceID int64) int64 {
	t.Helper()
	day := dbs.GroupCalendarDay{
		GroupID: groupID, Date: date, Kind: string(constants.GroupCalendarDayKindTraining), SessionInstanceID: &instanceID,
	}
	require.NoError(t, tx.Create(&day).Error)
	return day.ID
}

func seedFeedback(t *testing.T, tx *gorm.DB, teamID, athleteID, instanceID, exerciseID int64, date time.Time, set int) int64 {
	t.Helper()
	// MediaURLs se setea explícito: un pgtype.TextArray undefined no codifica.
	var media pgtype.TextArray
	require.NoError(t, media.Set([]string{}))
	f := dbs.WorkoutFeedback{
		TeamID: &teamID, AssignedSessionID: instanceID, AssignedExerciseID: exerciseID,
		AthleteUserID: athleteID, FeedbackOwnerUserID: athleteID, ReportSource: "runner",
		SessionDate: date, SetNumber: set, MediaURLs: media,
	}
	require.NoError(t, tx.Create(&f).Error)
	return f.ID
}

func seedPoint(t *testing.T, tx *gorm.DB, feedbackID, instanceID, exerciseID int64, order int) int64 {
	t.Helper()
	p := dbs.WorkoutFeedbackPoint{
		FeedbackID: feedbackID, SessionInstanceID: instanceID, ExerciseInstanceID: exerciseID,
		Order: order, Latitude: -31.4, Longitude: -64.1, RecordedAt: time.Now(),
	}
	require.NoError(t, tx.Create(&p).Error)
	return p.ID
}

func exists(t *testing.T, db *gorm.DB, model any, where string, args ...any) bool {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(model).Where(where, args...).Count(&count).Error)
	return count > 0
}
