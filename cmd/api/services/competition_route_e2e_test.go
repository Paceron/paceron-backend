package services

// Test e2e del flujo completo de intentos múltiples (Gap 36) con service real
// + DAO reales contra Postgres: alta route con max_attempts, intentos 1/2
// (compartida/instancia nueva), límite, cierre y winners por mejor intento.

import (
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/testutils"
)

type attemptsFlowFixture struct {
	owner   *dbs.User
	runnerA *dbs.User
	runnerB *dbs.User
	team    *dbs.Team
	group   *dbs.Group
}

func attemptsFlowUser(t *testing.T, db *gorm.DB, tag string) *dbs.User {
	t.Helper()
	u := &dbs.User{Name: tag, Surname: "Intentos", Email: tag + "@attempts.test.com", DNI: "83" + tag}
	u.BirthDate = time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	u.Password = "hashed"
	require.NoError(t, db.Create(u).Error)
	return u
}

func attemptsFlowSetup(t *testing.T, db *gorm.DB, tag string) *attemptsFlowFixture {
	t.Helper()
	f := &attemptsFlowFixture{
		owner:   attemptsFlowUser(t, db, "ow"+tag),
		runnerA: attemptsFlowUser(t, db, "ra"+tag),
		runnerB: attemptsFlowUser(t, db, "rb"+tag),
	}
	f.team = &dbs.Team{Name: "Team " + tag, MaxMembers: 10, OwnerID: f.owner.ID}
	require.NoError(t, db.Create(f.team).Error)
	f.group = &dbs.Group{Name: "Group " + tag, TeamID: f.team.ID, IsMain: true}
	require.NoError(t, db.Create(f.group).Error)
	for _, u := range []*dbs.User{f.runnerA, f.runnerB} {
		require.NoError(t, db.Create(&dbs.GroupUser{GroupID: f.group.ID, UserID: u.ID,
			DateStart: time.Now().AddDate(0, 0, -1)}).Error)
	}
	return f
}

func routeFlowGeoJSON() string {
	return fmt.Sprintf(`{"type":"LineString","coordinates":[[0,0],[0,%v]]}`, 400.0/metrosPorGradoLat)
}

// attachAttemptEvidence persiste el feedback activo del atleta sobre la
// instancia con distance y los primeros n puntos del spread (la evidencia GPS
// del matching de route vive de workout_feedback + workout_feedback_points).
func attachAttemptEvidence(t *testing.T, db *gorm.DB, athleteID, instanceID int64, teamID *int64, distance float64, n int) {
	t.Helper()
	var media pgtype.TextArray
	require.NoError(t, media.Set(nil))
	end := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	fb := &dbs.WorkoutFeedback{
		TeamID: teamID, AssignedSessionID: instanceID, AssignedExerciseID: 1,
		AthleteUserID: athleteID, FeedbackOwnerUserID: athleteID, ReportSource: "corredor",
		SessionDate: end, SetNumber: 1, DistanceMeters: &distance, MediaURLs: media,
	}
	require.NoError(t, db.Create(fb).Error)
	points := attemptSpreadGPS(instanceID, athleteID)
	if n > 0 && n < len(points) {
		points = points[:n]
	}
	for i, p := range points {
		require.NoError(t, db.Create(&dbs.WorkoutFeedbackPoint{
			FeedbackID: fb.ID, SessionInstanceID: instanceID, ExerciseInstanceID: 1,
			Order: i, Latitude: p.Latitude, Longitude: p.Longitude,
			RecordedAt: end.Add(time.Duration(i) * time.Minute),
		}).Error)
	}
}

// pinAttemptSpan fija start/end de un intento ya persistido: las duraciones
// reales entre creates/finaliza son ~0 ms y el desempate del mejor intento
// quedaria a merced del scheduler.
func pinAttemptSpan(t *testing.T, db *gorm.DB, runnerSessionID int64, start, end time.Time) {
	t.Helper()
	require.NoError(t, db.Model(&dbs.RunnerSession{}).Where("id = ?", runnerSessionID).
		Updates(map[string]interface{}{"start_date": start, "end_date": end}).Error)
}

func attemptInstanceName(t *testing.T, db *gorm.DB, instanceID int64) string {
	t.Helper()
	var inst dbs.SessionInstance
	require.NoError(t, db.Where("id = ?", instanceID).First(&inst).Error)
	return inst.Name
}

func TestCompetitionService_CreateAttemptE2E_FlujoCompleto(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fx := attemptsFlowSetup(t, db, "flu36")

	compDao := daos.NewCompetitionDao(db)
	compSvc := NewCompetitionService(compDao, nil, nil)
	runnerSvc := NewRunnerSessionService(daos.NewRunnerSessionDao(db), nil, compDao)
	teamID := fx.team.ID

	// Alta por el owner (trainer): active directo, max_attempts 2.
	maxAttempts := int64(2)
	created, err := compSvc.Create(nil, fx.owner.ID, fx.team.ID, competition.CreateCompetitionRequest{
		Type:                   competition.TypeRoute,
		RouteGeoJSON:           strPtr(routeFlowGeoJSON()),
		AcceptanceRadiusMeters: fptr(50),
		StartAt:                time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		ParticipantIDs:         []int64{fx.runnerA.ID, fx.runnerB.ID},
		MaxAttempts:            &maxAttempts,
	})
	require.NoError(t, err)
	assert.Equal(t, competition.TypeRoute, created.Type)
	require.NotNil(t, created.MaxAttempts)
	assert.Equal(t, maxAttempts, *created.MaxAttempts)
	require.NotNil(t, created.SessionInstanceID)
	shared := *created.SessionInstanceID
	assert.Equal(t, fmt.Sprintf("Competencia ruta %d", created.ID), attemptInstanceName(t, db, shared),
		"la instancia compartida del alta va sin (intento)")

	// Intento 1 de A: reusa la instancia compartida.
	rs1, err := compSvc.CreateAttempt(nil, fx.runnerA.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, shared, rs1.SessionInstanceID)
	assert.Equal(t, runnersession.RunnerSessionStatusWip, rs1.Status)

	// PATCH/finish real vía runnerSessionService + evidencia GPS completa.
	finished, err := runnerSvc.Finish(nil, fx.runnerA.ID, shared, runnersession.RunnerStatusRequest{Status: runnersession.RunnerSessionStatusFinished})
	require.NoError(t, err)
	assert.Equal(t, runnersession.RunnerSessionStatusFinished, finished.Status)
	require.NotNil(t, finished.EndDate)
	attachAttemptEvidence(t, db, fx.runnerA.ID, shared, &teamID, 400, 11)
	base := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	pinAttemptSpan(t, db, rs1.ID, base, base.Add(5*time.Minute))

	// Detalle participante: 1 intento usado, puede intentar de nuevo.
	detail, err := compSvc.Get(nil, fx.runnerA.ID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, detail.Data.MaxAttempts)
	assert.Equal(t, maxAttempts, *detail.Data.MaxAttempts)
	require.NotNil(t, detail.Data.AttemptsUsed)
	assert.Equal(t, int64(1), *detail.Data.AttemptsUsed)
	assert.True(t, detail.Data.CanAttempt)

	// Intento 2 de A: instancia nueva "(intento 2)".
	rs2, err := compSvc.CreateAttempt(nil, fx.runnerA.ID, created.ID)
	require.NoError(t, err)
	require.NotEqual(t, shared, rs2.SessionInstanceID)
	assert.Equal(t, fmt.Sprintf("Competencia ruta %d (intento 2)", created.ID), attemptInstanceName(t, db, rs2.SessionInstanceID))
	_, err = runnerSvc.Finish(nil, fx.runnerA.ID, rs2.SessionInstanceID, runnersession.RunnerStatusRequest{Status: runnersession.RunnerSessionStatusFinished})
	require.NoError(t, err)

	detail, err = compSvc.Get(nil, fx.runnerA.ID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, detail.Data.AttemptsUsed)
	assert.Equal(t, int64(2), *detail.Data.AttemptsUsed)
	assert.False(t, detail.Data.CanAttempt)

	// Tercer intento de A: límite, sin filas nuevas.
	_, err = compSvc.CreateAttempt(nil, fx.runnerA.ID, created.ID)
	require.ErrorIs(t, err, ErrCompetitionInvalid)
	assert.Contains(t, err.Error(), "llegaste al límite de intentos de esta competencia")

	var rsCountA int64
	require.NoError(t, db.Model(&dbs.RunnerSession{}).
		Where("competition_id = ? AND athlete_user_id = ?", created.ID, fx.runnerA.ID).
		Count(&rsCountA).Error)
	assert.Equal(t, int64(2), rsCountA)
	var thirdInstances int64
	require.NoError(t, db.Model(&dbs.SessionInstance{}).
		Where("name LIKE ?", fmt.Sprintf("Competencia ruta %d (intento 3", created.ID)).
		Count(&thirdInstances).Error)
	assert.Zero(t, thirdInstances)

	// B: mejor resultado distinto por intento (ranking). Intento 1 apoyado en
	// la compartida con GPS parcial; intento 2 completo pero más lento que A.
	b1, err := compSvc.CreateAttempt(nil, fx.runnerB.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, shared, b1.SessionInstanceID, "el intento 1 de B también corre sobre la compartida")
	_, err = runnerSvc.Finish(nil, fx.runnerB.ID, shared, runnersession.RunnerStatusRequest{Status: runnersession.RunnerSessionStatusFinished})
	require.NoError(t, err)
	attachAttemptEvidence(t, db, fx.runnerB.ID, shared, &teamID, 400, 4)

	b2, err := compSvc.CreateAttempt(nil, fx.runnerB.ID, created.ID)
	require.NoError(t, err)
	require.NotEqual(t, shared, b2.SessionInstanceID)
	_, err = runnerSvc.Finish(nil, fx.runnerB.ID, b2.SessionInstanceID, runnersession.RunnerStatusRequest{Status: runnersession.RunnerSessionStatusFinished})
	require.NoError(t, err)
	attachAttemptEvidence(t, db, fx.runnerB.ID, b2.SessionInstanceID, &teamID, 400, 11)
	pinAttemptSpan(t, db, b2.ID, base, base.Add(7*time.Minute))

	// Cierre por el creador: winners por mejor intento + stamps.
	closeResp, instances, err := compSvc.Close(nil, fx.owner.ID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusFinished, closeResp.Status)
	assert.Equal(t, []int64{fx.runnerA.ID}, closeResp.WinnerUserIDs)
	// Caller del cierre = owner (no compite): attempts_used null, can_attempt false.
	assert.Nil(t, closeResp.AttemptsUsed)
	assert.False(t, closeResp.CanAttempt)
	assert.ElementsMatch(t, []int64{shared, rs2.SessionInstanceID, b2.SessionInstanceID}, instances,
		"canales WS: todas las instancias de intentos registrados, sin duplicar la compartida")

	var winnerRows int64
	require.NoError(t, db.Model(&dbs.CompetitionWinner{}).Where("competition_id = ?", created.ID).Count(&winnerRows).Error)
	assert.Equal(t, int64(1), winnerRows)
	var stampRows int64
	require.NoError(t, db.Model(&dbs.GamificationStamp{}).Where("user_id = ?", fx.runnerA.ID).Count(&stampRows).Error)
	assert.Equal(t, int64(1), stampRows)
	var unseenRows int64
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).
		Where("section = ? AND team_id = ?", "competitions", fx.team.ID).Count(&unseenRows).Error)
	assert.Equal(t, int64(2), unseenRows)

	// Participante post-cierre: usados 2 y sin cupo (status finished).
	detailA, err := compSvc.Get(nil, fx.runnerA.ID, created.ID)
	require.NoError(t, err)
	require.NotNil(t, detailA.Data.AttemptsUsed)
	assert.Equal(t, int64(2), *detailA.Data.AttemptsUsed)
	assert.False(t, detailA.Data.CanAttempt, "finished: participante con intentos sin cupo")
}
