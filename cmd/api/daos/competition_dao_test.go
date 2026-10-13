package daos

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"

	"simple-arq-golang/cmd/api/testutils"
)

// DAO real de competencias (E4.1) contra Postgres. Escenario base: owner +
// 2 corredores en un grupo del team + un corredor ajeno.

type competitionFixture struct {
	owner     *dbs.User
	runnerA   *dbs.User
	runnerB   *dbs.User
	outsider  *dbs.User
	team      *dbs.Team
	otherTeam *dbs.Team
}

func newCompetitionFixture(t *testing.T, db *gorm.DB, suffix string) *competitionFixture {
	t.Helper()
	owner := persistUser(db, "comp-owner-"+suffix+"@test.com", "9700"+suffix+"1")
	runnerA := persistUser(db, "comp-a-"+suffix+"@test.com", "9700"+suffix+"2")
	runnerB := persistUser(db, "comp-b-"+suffix+"@test.com", "9700"+suffix+"3")
	outsider := persistUser(db, "comp-out-"+suffix+"@test.com", "9700"+suffix+"4")
	team := testTeam(db, "equipo_comp_"+suffix, owner.ID)
	otherTeam := testTeam(db, "equipo_comp_otro_"+suffix, outsider.ID)
	group := &dbs.Group{Name: "grupo_comp_" + suffix, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	for _, u := range []*dbs.User{runnerA, runnerB} {
		require.NoError(t, db.Create(&dbs.GroupUser{
			GroupID: group.ID, UserID: u.ID,
			DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}).Error)
	}
	// El outsider pertenece a un grupo del otro team.
	otherGroup := &dbs.Group{Name: "grupo_comp_otro_" + suffix, TeamID: otherTeam.ID}
	require.NoError(t, db.Create(otherGroup).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{
		GroupID: otherGroup.ID, UserID: outsider.ID,
		DateStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}).Error)
	return &competitionFixture{owner: owner, runnerA: runnerA, runnerB: runnerB, outsider: outsider, team: team, otherTeam: otherTeam}
}

func newMetricCompetition(t *testing.T, db *gorm.DB, fx *competitionFixture, status string, suffix string, groupID *int64) *dbs.Competition {
	t.Helper()
	metric := "distance"
	comp := &dbs.Competition{
		TeamID: fx.team.ID, GroupID: groupID, Type: competition.TypeMetric,
		Metric:          &metric,
		StartAt:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		CreatedByUserID: fx.owner.ID, CreatedByRole: competition.CreatorRoleTrainer,
		Status: status,
	}
	require.NoError(t, db.Create(comp).Error)
	require.NoError(t, db.Create(&[]dbs.CompetitionParticipant{
		{CompetitionID: comp.ID, UserID: fx.runnerA.ID},
		{CompetitionID: comp.ID, UserID: fx.runnerB.ID},
	}).Error)
	return comp
}

func TestCompetitionDAO_CreateYBusqueda(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "01")

	metric := "duration"
	comp := &dbs.Competition{
		TeamID: fx.team.ID, Type: competition.TypeMetric, Metric: &metric,
		StartAt:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		CreatedByUserID: fx.runnerA.ID, CreatedByRole: competition.CreatorRoleRunner,
		Status: competition.StatusPendingApproval,
	}
	err := dao.Create(nil, comp, []int64{fx.runnerA.ID, fx.runnerB.ID})
	require.NoError(t, err)
	require.NotZero(t, comp.ID)

	loaded, err := dao.FindByID(nil, comp.ID)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusPendingApproval, loaded.Status)
	assert.Equal(t, "duration", *loaded.Metric)

	// La FK competition_id de runner_session es opaca: sin constraint que
	// valide, la columna nullable guarda el id.
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: 3001, AthleteUserID: fx.runnerA.ID,
		StartDate:     time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		CompetitionID: &comp.ID,
	}).Error)
	var rs dbs.RunnerSession
	require.NoError(t, db.Where("session_instance_id = 3001").First(&rs).Error)
	require.NotNil(t, rs.CompetitionID)
	assert.Equal(t, comp.ID, *rs.CompetitionID)

	parts, err := dao.ListParticipantsByCompetitionIDs(nil, []int64{comp.ID})
	require.NoError(t, err)
	require.Len(t, parts, 2)

	part, err := dao.IsParticipant(nil, comp.ID, fx.runnerA.ID)
	require.NoError(t, err)
	assert.True(t, part)
	part, err = dao.IsParticipant(nil, comp.ID, fx.outsider.ID)
	require.NoError(t, err)
	assert.False(t, part)

	// 404.
	_, err = dao.FindByID(nil, 999999)
	assert.ErrorIs(t, err, ErrCompetitionNotFound)
}

func TestCompetitionDAO_GuardsDeMembresia(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "02")

	owner, err := dao.IsTeamOwner(nil, fx.team.ID, fx.owner.ID)
	require.NoError(t, err)
	assert.True(t, owner)

	member, err := dao.IsTeamGroupMember(nil, fx.team.ID, fx.runnerA.ID)
	require.NoError(t, err)
	assert.True(t, member)

	member, err = dao.IsTeamGroupMember(nil, fx.team.ID, fx.outsider.ID)
	require.NoError(t, err)
	assert.False(t, member)

	// 2 de los 3 son miembros (el outsider no).
	count, err := dao.CountTeamGroupMembers(nil, fx.team.ID, []int64{fx.owner.ID, fx.runnerB.ID, fx.outsider.ID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	belongs, err := dao.GroupBelongsToTeam(nil, 2, 3)
	require.NoError(t, err)
	assert.False(t, belongs)
}

func TestCompetitionDAO_ListByTeamFiltros(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "03")

	newMetricCompetition(t, db, fx, competition.StatusActive, "a", nil)
	newMetricCompetition(t, db, fx, competition.StatusPendingApproval, "b", nil)
	group := &dbs.Group{Name: "grupo_filtro", TeamID: fx.team.ID}
	require.NoError(t, db.Create(group).Error)
	newMetricCompetition(t, db, fx, competition.StatusActive, "c", &group.ID)

	all, err := dao.ListByTeam(nil, fx.team.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Winner de la única finished (la primera creada del primer new):
	// la lista de winners carga en batch junto a participants.
	winner := &dbs.CompetitionWinner{CompetitionID: all[0].ID, UserID: fx.runnerA.ID}
	require.NoError(t, db.Create(winner).Error)
	winners, err := dao.ListWinnersByCompetitionIDs(nil, []int64{all[0].ID})
	require.NoError(t, err)
	require.Len(t, winners, 1)
	require.Equal(t, fx.runnerA.ID, winners[0].UserID)
	winners, err = dao.ListWinnersByCompetitionIDs(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, winners)

	active, err := dao.ListByTeam(nil, fx.team.ID, strPtr(competition.StatusActive), nil)
	require.NoError(t, err)
	require.Len(t, active, 2)

	byGroup, err := dao.ListByTeam(nil, fx.team.ID, nil, &group.ID)
	require.NoError(t, err)
	require.Len(t, byGroup, 1)

	// Team sin competencias: vacío.
	empty, err := dao.ListByTeam(nil, fx.otherTeam.ID, nil, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestCompetitionDAO_Transiciones(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "04")

	comp := newMetricCompetition(t, db, fx, competition.StatusPendingApproval, "a", nil)

	updated, err := dao.Approve(nil, comp.ID, fx.owner.ID)
	require.NoError(t, err)
	assert.True(t, updated)
	loaded, err := dao.FindByID(nil, comp.ID)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusActive, loaded.Status)
	require.NotNil(t, loaded.ApprovedByUserID)
	assert.Equal(t, fx.owner.ID, *loaded.ApprovedByUserID)

	// Approve ya no es posible desde active.
	updated, err = dao.Approve(nil, comp.ID, fx.owner.ID)
	require.NoError(t, err)
	assert.False(t, updated)

	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	updated, err = dao.Finish(nil, comp.ID, at, nil, nil, nil)
	require.NoError(t, err)
	assert.True(t, updated)
	loaded, err = dao.FindByID(nil, comp.ID)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusFinished, loaded.Status)
	require.NotNil(t, loaded.FinishedAt)

	// Reject solo corre sobre pending.
	rejected, err := dao.Reject(nil, comp.ID, strPtr("no"))
	require.NoError(t, err)
	assert.False(t, rejected)

	// Reject feliz sobre pending, con y sin reason.
	comp2 := newMetricCompetition(t, db, fx, competition.StatusPendingApproval, "b", nil)
	rejected, err = dao.Reject(nil, comp2.ID, nil)
	require.NoError(t, err)
	assert.True(t, rejected)
	loaded, err = dao.FindByID(nil, comp2.ID)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusRejected, loaded.Status)
	assert.Nil(t, loaded.RejectionReason)

	comp3 := newMetricCompetition(t, db, fx, competition.StatusPendingApproval, "c", nil)
	rejected, err = dao.Reject(nil, comp3.ID, strPtr("te pasa"))
	require.NoError(t, err)
	assert.True(t, rejected)
	loaded, err = dao.FindByID(nil, comp3.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.RejectionReason)
	assert.Equal(t, "te pasa", *loaded.RejectionReason)

	// Finish solo sobre active: una rejected no se cierra.
	updated, err = dao.Finish(nil, comp3.ID, at, nil, nil, nil)
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestCompetitionDAO_DeleteFisico(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "05")

	comp := newMetricCompetition(t, db, fx, competition.StatusPendingApproval, "a", nil)
	err := dao.Delete(nil, comp.ID)
	require.NoError(t, err)

	_, err = dao.FindByID(nil, comp.ID)
	assert.ErrorIs(t, err, ErrCompetitionNotFound)
	parts, err := dao.ListParticipantsByCompetitionIDs(nil, []int64{comp.ID})
	require.NoError(t, err)
	assert.Empty(t, parts)
}

func TestCompetitionDAO_UnseenYVentana(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "06")

	has, err := dao.HasUnseenTeam(nil, fx.runnerA.ID, fx.team.ID)
	require.NoError(t, err)
	assert.False(t, has)
	require.NoError(t, db.Create(&dbs.GamificationUnseen{
		UserID:  fx.runnerA.ID,
		Section: "competitions",
		TeamID:  &fx.team.ID,
	}).Error)
	has, err = dao.HasUnseenTeam(nil, fx.runnerA.ID, fx.team.ID)
	require.NoError(t, err)
	assert.True(t, has)

	a1 := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 2001, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(a1).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-05"), "distance_meters": 3000.0, "duration_ms": int64(1200000),
	}).Error)
	b1 := testFeedback(t, db, fx.runnerB.ID, fx.owner.ID, 2002, 1, &fx.team.ID, 0)
	require.NoError(t, db.Model(b1).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-06"), "distance_meters": 5000.0,
	}).Error)
	// Fuera de ventana y de team.
	out := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 2003, 1, &fx.otherTeam.ID, 0)
	require.NoError(t, db.Model(out).Updates(map[string]interface{}{
		"session_date": dateUTC("2026-10-07"), "distance_meters": 42000.0,
	}).Error)

	agg, err := dao.AggregateFeedbackWindow(nil, fx.team.ID, []int64{fx.runnerA.ID, fx.runnerB.ID},
		dateUTC("2026-10-01"), dateUTC("2026-10-10"))
	require.NoError(t, err)
	require.Len(t, agg, 2)
	byUser := map[int64]dbs.CompetitionMetricAgg{}
	for _, r := range agg {
		byUser[r.UserID] = r
	}
	require.NotNil(t, byUser[fx.runnerA.ID].SumDistance)
	assert.InDelta(t, 3000.0, *byUser[fx.runnerA.ID].SumDistance, 0.0001)
	// B solo reportó distance: duration y peak quedan null.
	assert.Nil(t, byUser[fx.runnerB.ID].SumDurationMs)
	assert.Nil(t, byUser[fx.runnerB.ID].MaxPeak)
}

func TestCompetitionDAO_FinishConEfectos(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "07")

	comp := newMetricCompetition(t, db, fx, competition.StatusActive, "ef", nil)
	finishedAt := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	stamps := []dbs.GamificationStamp{{
		UserID: fx.runnerA.ID, Type: "competition", SourceID: comp.ID,
		SourceName: fmt.Sprintf("Competencia %d", comp.ID), GrantedAt: finishedAt, Count: 1,
	}}
	unseen := []dbs.GamificationUnseen{
		{UserID: fx.runnerA.ID, Section: "competitions", TeamID: &fx.team.ID},
		{UserID: fx.runnerB.ID, Section: "competitions", TeamID: &fx.team.ID},
	}
	updated, err := dao.Finish(nil, comp.ID, finishedAt, []int64{fx.runnerA.ID}, stamps, unseen)
	require.NoError(t, err)
	assert.True(t, updated)

	loaded, err := dao.FindByID(nil, comp.ID)
	require.NoError(t, err)
	assert.Equal(t, competition.StatusFinished, loaded.Status)
	require.NotNil(t, loaded.FinishedAt)

	winners, err := dao.ListWinnersByCompetitionIDs(nil, []int64{comp.ID})
	require.NoError(t, err)
	require.Len(t, winners, 1)
	assert.Equal(t, fx.runnerA.ID, winners[0].UserID)

	var stampsCount, unseenCount int64
	require.NoError(t, db.Model(&dbs.GamificationStamp{}).Where("user_id = ?", fx.runnerA.ID).Count(&stampsCount).Error)
	require.NoError(t, db.Model(&dbs.GamificationUnseen{}).Where("section = ? AND team_id = ?", "competitions", fx.team.ID).Count(&unseenCount).Error)
	assert.Equal(t, int64(1), stampsCount)
	assert.Equal(t, int64(2), unseenCount)

	// Re-Finish no toca filas: ni winners ni stamps duplicados (guard del status).
	updated, err = dao.Finish(nil, comp.ID, finishedAt, []int64{fx.runnerA.ID}, stamps, unseen)
	require.NoError(t, err)
	assert.False(t, updated)
	var winnerRows int64
	require.NoError(t, db.Model(&dbs.CompetitionWinner{}).Where("competition_id = ?", comp.ID).Count(&winnerRows).Error)
	assert.Equal(t, int64(1), winnerRows)
}

func TestCompetitionDAO_AttemptSessionsYPoints(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewCompetitionDao(db)
	fx := newCompetitionFixture(t, db, "08")
	comp := newMetricCompetition(t, db, fx, competition.StatusActive, "att", nil)

	compID := comp.ID
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: 771, AthleteUserID: fx.runnerA.ID,
		Status: "finished", StartDate: time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC),
		CompetitionID: &compID,
	}).Error)
	// Intento de otro atleta wip (solo canal).
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: 772, AthleteUserID: fx.runnerB.ID,
		Status: "wip", StartDate: time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC),
		CompetitionID: &compID,
	}).Error)
	// Runner session del mismo atleta SIN competencia: fuera.
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: 773, AthleteUserID: fx.runnerA.ID,
		Status: "finished", StartDate: time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC),
	}).Error)

	fb := testFeedback(t, db, fx.runnerA.ID, fx.owner.ID, 771, 1, &fx.team.ID, 0)
	require.NoError(t, db.Create(&dbs.WorkoutFeedbackPoint{
		FeedbackID: fb.ID, SessionInstanceID: 771, ExerciseInstanceID: 1, Order: 0,
		Latitude: -31.42, Longitude: -64.18, RecordedAt: time.Date(2026, 10, 5, 9, 0, 30, 0, time.UTC),
	}).Error)
	require.NoError(t, db.Create(&dbs.WorkoutFeedbackPoint{
		FeedbackID: fb.ID, SessionInstanceID: 771, ExerciseInstanceID: 1, Order: 1,
		Latitude: -31.421, Longitude: -64.181, RecordedAt: time.Date(2026, 10, 5, 9, 1, 0, 0, time.UTC),
	}).Error)

	sessions, err := dao.ListAttemptSessions(nil, comp.ID)
	require.NoError(t, err)
	require.Len(t, sessions, 2)

	points, err := dao.ListAttemptPoints(nil, []int64{771, 772}, []int64{fx.runnerA.ID, fx.runnerB.ID})
	require.NoError(t, err)
	require.Len(t, points, 2)
	assert.Equal(t, int64(771), points[0].SessionInstanceID)
	assert.Equal(t, fx.runnerA.ID, points[0].AthleteUserID)
	assert.Equal(t, -31.42, points[0].Latitude)
	// ídola nueva instancia fuera del filtro devuelva 0.
	none, err := dao.ListAttemptPoints(nil, []int64{999}, []int64{fx.runnerA.ID})
	require.NoError(t, err)
	assert.Empty(t, none)
	none, err = dao.ListAttemptPoints(nil, nil, nil)
	require.NoError(t, err)
	assert.Nil(t, none)
}
