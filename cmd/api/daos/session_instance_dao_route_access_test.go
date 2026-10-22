package daos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

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

// escenarioIntentos arma dos competencias route de teams/usuarios separados:
// comp1 ancla sharedInst con athlete participante; comp2 (del caller other)
// ancla instAnchor con other participante. Las instancias extras (inst2/instX)
// solo tienen runner_sessions tagueadas — simulan los intentos 2+ sin día ni
// feedback.
type escenarioIntentos struct {
	db        *gorm.DB
	f         *accessFixture
	other     *dbs.User
	comp1     *dbs.Competition
	comp2     *dbs.Competition
	sharedInt *dbs.SessionInstance // ancla de comp1 (intento 1 compartida)
	inst2     *dbs.SessionInstance // intento 2 del athlete en comp1 (wip)
	instFin   *dbs.SessionInstance // intento finished del athlete en comp1
	instX     *dbs.SessionInstance // rs tagueada con comp2 (athlete ajeno a comp2)
	instAnch2 *dbs.SessionInstance // ancla de comp2
}

func intentosFixtureSetup(t *testing.T, db *gorm.DB) *escenarioIntentos {
	t.Helper()
	f := accessFixtureSetup(t, db)
	other := persistUser(db, "access-other-comp@test.com", "70000008")

	sharedInt := &dbs.SessionInstance{Name: "Intento compartida"}
	require.NoError(t, db.Create(sharedInt).Error)
	comp1 := newRouteCompetition()
	comp1.TeamID = f.teamA.ID
	comp1.CreatedByUserID = f.owner.ID
	comp1.CreatedByRole = competition.CreatorRoleTrainer
	comp1.Status = competition.StatusActive
	comp1.RouteSessionInstanceID = &sharedInt.ID
	require.NoError(t, db.Create(comp1).Error)
	require.NoError(t, db.Create(&dbs.CompetitionParticipant{CompetitionID: comp1.ID, UserID: f.athlete.ID}).Error)

	inst2 := &dbs.SessionInstance{Name: "Intento 2"}
	require.NoError(t, db.Create(inst2).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: inst2.ID, AthleteUserID: f.athlete.ID,
		Status: "wip", StartDate: time.Now(), CompetitionID: &comp1.ID,
	}).Error)

	instFin := &dbs.SessionInstance{Name: "Intento finished"}
	require.NoError(t, db.Create(instFin).Error)
	end := time.Now().Add(time.Hour)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: instFin.ID, AthleteUserID: f.athlete.ID,
		Status: "finished", StartDate: time.Now(), EndDate: &end, CompetitionID: &comp1.ID,
	}).Error)

	instAnch2 := &dbs.SessionInstance{Name: "Ancla comp2"}
	require.NoError(t, db.Create(instAnch2).Error)
	comp2 := newRouteCompetition()
	comp2.TeamID = f.teamA.ID
	comp2.CreatedByUserID = other.ID
	comp2.CreatedByRole = competition.CreatorRoleTrainer
	comp2.Status = competition.StatusActive
	comp2.RouteSessionInstanceID = &instAnch2.ID
	require.NoError(t, db.Create(comp2).Error)
	require.NoError(t, db.Create(&dbs.CompetitionParticipant{CompetitionID: comp2.ID, UserID: other.ID}).Error)

	instX := &dbs.SessionInstance{Name: "Intento de comp2"}
	require.NoError(t, db.Create(instX).Error)
	require.NoError(t, db.Create(&dbs.RunnerSession{
		SessionInstanceID: instX.ID, AthleteUserID: other.ID,
		Status: "wip", StartDate: time.Now(), CompetitionID: &comp2.ID,
	}).Error)

	return &escenarioIntentos{db: db, f: f, other: other, comp1: comp1, comp2: comp2,
		sharedInt: sharedInt, inst2: inst2, instFin: instFin, instX: instX, instAnch2: instAnch2}
}

// D3: la instancia de un intento 2+ (solo runner_session tagueada, sin día ni
// feedback) autoriza al participante de la competencia del intento. La
// competencia anclada y la del intento pueden ser DIFERENTES: cada lado del
// OR chequea participante contra SU competencia.
func TestSessionInstanceDao_HasInstanceAccess_IntentosMultiples(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	s := intentosFixtureSetup(t, db)
	f := s.f

	t.Run("participante_intento_2_wip", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, s.inst2.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("participante_intento_finished", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, s.instFin.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("compartida_sigue_autorizando", func(t *testing.T) {
		ok, err := dao.HasInstanceAccess(nil, s.sharedInt.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("ajeno_sin_participacion_no", func(t *testing.T) {
		// f.exMember no es participante de comp1: la instancia del intento 2
		// no lo autoriza ni por otra rama.
		ok, err := dao.HasInstanceAccess(nil, s.inst2.ID, f.exMember.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("participante_de_otra_competencia_no_cruza", func(t *testing.T) {
		// El caso trampa del OR: instX tiene rs tagueada con comp2. El caller
		// es participante de comp1 únicamente — un join que mezcle ambos lados
		// lo autorizaría mal; el check va contra la competencia del intento.
		ok, err := dao.HasInstanceAccess(nil, s.instX.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.False(t, ok)

		ok, err = dao.HasInstanceAccess(nil, s.instX.ID, s.other.ID)
		require.NoError(t, err)
		assert.True(t, ok, "participante de comp2 sí accede a la instancia de su intento")
	})
	t.Run("ancla_ajena_no_autoriza", func(t *testing.T) {
		// instAnch2 está anclada por comp2 al que el athlete no pertenece.
		ok, err := dao.HasInstanceAccess(nil, s.instAnch2.ID, f.athlete.ID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

// Espejo batch de la rama extendida: singular y batch no pueden divergir.
func TestSessionInstanceDao_HasInstanceAccessBatch_IntentosMultiples(t *testing.T) {
	db := testutils.SetupTestDB(t)
	dao := NewSessionInstanceDao(db)
	s := intentosFixtureSetup(t, db)
	f := s.f

	users := []int64{f.athlete.ID, s.other.ID, f.exMember.ID, f.stranger.ID}

	t.Run("intento_2_wip", func(t *testing.T) {
		access, err := dao.HasInstanceAccessBatch(nil, s.inst2.ID, users)
		require.NoError(t, err)
		require.Len(t, access, len(users))
		assert.True(t, access[f.athlete.ID])
		assert.False(t, access[s.other.ID])
		assert.False(t, access[f.exMember.ID])
		assert.False(t, access[f.stranger.ID])
	})
	t.Run("intento_finished", func(t *testing.T) {
		access, err := dao.HasInstanceAccessBatch(nil, s.instFin.ID, users)
		require.NoError(t, err)
		assert.True(t, access[f.athlete.ID])
		assert.False(t, access[s.other.ID])
	})
	t.Run("instancia_de_otra_competencia", func(t *testing.T) {
		access, err := dao.HasInstanceAccessBatch(nil, s.instX.ID, users)
		require.NoError(t, err)
		assert.False(t, access[f.athlete.ID], "participante de comp1 no cruza a comp2")
		assert.True(t, access[s.other.ID])
	})
	t.Run("compartida", func(t *testing.T) {
		access, err := dao.HasInstanceAccessBatch(nil, s.sharedInt.ID, users)
		require.NoError(t, err)
		assert.True(t, access[f.athlete.ID])
		assert.False(t, access[s.other.ID])
	})
}
