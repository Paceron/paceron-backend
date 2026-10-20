package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// Task 1 (P1/P2): List de /sessions y /training-plans con detalle en batch —
// 2 queries exactas (owner + batch links), sin detalle por fila, respuesta
// idéntica al path per-row (Get) inclusive con días guardados fuera de
// orden de sequence_no.
func TestSessionService_List_BatchQueriesAndShape(t *testing.T) {
	db := testutils.SetupTestDB(t)
	counter := 0
	batchDB := countingDB(t, db, &counter)

	// seed con DAOs planos: ReplaceFor* va en transacción y el pool de conteo
	// no la soporta; el conteo solo envuelve las lecturas del service
	sessionDao := daos.NewSessionDao(db)
	sessionExerciseDao := daos.NewSessionExerciseDao(db)
	exerciseDao := daos.NewExerciseDao(db)

	created := make([]dbs.Session, 3)
	for i := range created {
		sess := dbs.Session{OwnerID: 42, Name: "S batch"}
		require.NoError(t, sessionDao.Create(nil, &sess))
		rows := []dbs.SessionExercise{
			{ExerciseID: 1, Role: "warmup", RepeatCount: 1, RestMinutes: 30},
			{ExerciseID: 2, Role: "main", RepeatCount: 3, RestMinutes: 90},
		}
		require.NoError(t, sessionExerciseDao.ReplaceForSession(nil, sess.ID, rows))
		created[i] = sess
	}

	svc := NewSessionService(sessionDao, sessionExerciseDao, exerciseDao)
	responses, err := svc.List(nil, 42)
	require.NoError(t, err)
	require.Len(t, responses, 3)
	// respuesta idéntica al path per-row (Get usa FindBySession por fila)
	for i := range created {
		perRow, err := svc.Get(nil, created[i].ID)
		require.NoError(t, err)
		assert.Equal(t, *perRow, responses[i])
	}

	counter = 0
	batchSvc := NewSessionService(daos.NewSessionDao(batchDB), daos.NewSessionExerciseDao(batchDB), exerciseDao)
	batchSvc.List(nil, 42)
	assert.Equal(t, 2, counter, "1 FindByOwner + 1 FindBySessions para N sesiones")

	// caso vacío: solo el FindByOwner, el batch early-returnea sin query
	counter = 0
	empty, err := batchSvc.List(nil, 999)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Equal(t, 1, counter, "owner sin sesiones: 1 query, sin batch")
}

func TestTrainingPlanService_List_BatchQueriesAndShape(t *testing.T) {
	db := testutils.SetupTestDB(t)
	counter := 0
	batchDB := countingDB(t, db, &counter)

	planDao := daos.NewTrainingPlanDao(db)
	planDayDao := daos.NewPlanDayDao(db)
	sessionDao := daos.NewSessionDao(db)
	sess := dbs.Session{OwnerID: 42, Name: "S plan"}
	require.NoError(t, sessionDao.Create(nil, &sess))

	created := make([]dbs.TrainingPlan, 3)
	for i := range created {
		plan := dbs.TrainingPlan{OwnerID: 42, Name: "Plan batch"}
		require.NoError(t, planDao.Create(nil, &plan))
		// días con sequence_no no correlativo de id: el batch debe ordenar
		// por sequence_no dentro de cada plan igual que el per-row
		rows := []dbs.PlanDay{
			{SequenceNo: 3, Kind: "rest"},
			{SequenceNo: 1, Kind: "training", SessionID: &sess.ID},
			{SequenceNo: 2, Kind: "other", OtherName: strPtrTP("Elongación")},
		}
		require.NoError(t, planDayDao.ReplaceForPlan(nil, plan.ID, rows))
		created[i] = plan
	}

	svc := NewTrainingPlanService(planDao, planDayDao, sessionDao, daos.NewGroupCalendarDayDao(db))
	responses, err := svc.List(nil, 42)
	require.NoError(t, err)
	require.Len(t, responses, 3)
	for i := range created {
		perRow, err := svc.Get(nil, created[i].ID)
		require.NoError(t, err)
		assert.Equal(t, *perRow, responses[i])
		require.Len(t, responses[i].Days, 3)
		assert.Equal(t, 1, responses[i].Days[0].SequenceNo)
	}

	counter = 0
	batchSvc := NewTrainingPlanService(daos.NewTrainingPlanDao(batchDB), daos.NewPlanDayDao(batchDB), daos.NewSessionDao(batchDB), daos.NewGroupCalendarDayDao(db))
	batchSvc.List(nil, 42)
	assert.Equal(t, 2, counter, "1 FindByOwner + 1 FindByPlanIDs para N planes")

	counter = 0
	empty, err := batchSvc.List(nil, 999)
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.Equal(t, 1, counter, "owner sin planes: 1 query, sin batch")
}
