package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// 3ª rama de HasInstanceAccess (competencias-ruta-intake D8): la instancia
// sintética de una competencia type=route autoriza a sus participantes sin
// filtro de status, sin día de calendario y sin feedback.
func TestSessionInstanceDao_HasInstanceAccess_CompetenciaRoute(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	f := accessFixtureSetup(t, db)

	inst := &dbs.SessionInstance{Name: "Instancia sintética route"}
	require.NoError(t, db.Create(inst).Error)
	comp := newRouteCompetition()
	comp.TeamID = f.teamA.ID
	comp.CreatedByUserID = f.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	comp.RouteSessionInstanceID = &inst.ID
	require.NoError(t, db.Create(comp).Error)
	require.NoError(t, db.Create(&dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: f.athlete.ID}).Error)

	t.Run("participante_sí", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, inst.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("usuario_ajeno_no", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, inst.ID, f.exMember.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("dueño_del_team_no_es_sibling", func(t *testing.T) {
		// El owner del team no es participante: la 3ª rama solo cubre
		// participantes (la competencia pertenece al mismo team pero eso no
		// autoriza por sí solo).
		ok, err := dao.HasInstanceAccess(nil, inst.ID, f.owner.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("competencia_metric_no_autoriza", func(t *testing.T) {
		// Instancia SOLO referenciada por una competencia metric: la 3ª rama
		// exige type=route, así que ni el participante accede por acá.
		metricInst := &dbs.SessionInstance{Name: "Instancia metric"}
		require.NoError(t, db.Create(metricInst).Error)
		metricComp := newRouteCompetition()
		metricComp.Type = competition.TypeMetric
		metricComp.Metric = strPtr("distance")
		metricComp.TeamID = f.teamA.ID
		metricComp.CreatedByUserID = f.owner.ID
		metricComp.CreatedByRole = competition.CreatorRoleTrainer
		metricComp.Status = competition.StatusActive
		metricComp.RouteSessionInstanceID = &metricInst.ID
		require.NoError(t, db.Create(metricComp).Error)
		require.NoError(t, db.Create(&dbs.CompetitionParticipant{CompetitionID: metricComp.ID, UserID: f.athlete.ID}).Error)
		ok, err := dao.HasInstanceAccess(nil, metricInst.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("status_finished_sigue_autorizando", func(t *testing.T) {
		require.NoError(t, db.Model(comp).Updates(map[string]interface{}{"status": competition.StatusFinished}).Error)
		ok, err := dao.HasInstanceAccess(nil, inst.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("instancia_inexistente", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, 999999, f.athlete.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

// Caso de la tarea: runner_session wip SIN filas de feedback autoriza igual
// por la 3ª rama (el pre-start del intento no tiene feedback todavía).
func TestSessionInstanceDao_HasInstanceAccess_WipSinFeedback(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	f := accessFixtureSetup(t, db)

	inst := &dbs.SessionInstance{Name: "Instancia wip pre-start"}
	require.NoError(t, db.Create(inst).Error)
	comp := newRouteCompetition()
	comp.TeamID = f.teamA.ID
	comp.CreatedByUserID = f.owner.ID
	comp.CreatedByRole = competition.CreatorRoleTrainer
	comp.Status = competition.StatusActive
	comp.RouteSessionInstanceID = &inst.ID
	require.NoError(t, db.Create(comp).Error)
	require.NoError(t, db.Create(&dbs.CompetitionParticipant{CompetitionID: comp.ID, UserID: f.athlete.ID}).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: inst.ID, AthleteUserID: f.athlete.ID,
		Status: "wip", StartDate: time.Now(),
	}).Error)

	var fbCount int64
	require.NoError(t, db.Model(&dbs.WorkoutFeedback{}).Where("assigned_session_id = ?", inst.ID).Count(&fbCount).Error)
	assert.Zero(t, fbCount, "precondición: sin feedback sobre la instancia")

	ok, err := dao.HasInstanceAccess(nil, inst.ID, f.athlete.ID)
	require.NoError(t, err)
	assert.True(t, ok)
}
