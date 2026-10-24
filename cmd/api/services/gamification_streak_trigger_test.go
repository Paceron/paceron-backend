package services

// --- Trigger de racha (gamificacion-core D3, tasks 2.2-2.4): contrato con
// mock DAO + fake calculator; la integración con Postgres real y el pin de
// queries van al final del archivo.

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/testutils"
)

type fakeStreakCalculator struct {
	strict, flexible int
	err              error
}

func (f *fakeStreakCalculator) GetStreaks(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
	return nil, nil
}

func (f *fakeStreakCalculator) ComputeCurrentStreaks(ctx *gin.Context, targetUserID int64) (int, int, error) {
	if f.err != nil {
		return 0, 0, f.err
	}
	return f.strict, f.flexible, nil
}

// panicCalculator también implementa StreakServiceInterface, paniqueando en
// el cálculo: alimenta el test de panic-guard del run del goroutine.
type panicCalculator struct{}

func (panicCalculator) GetStreaks(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
	return nil, nil
}

func (panicCalculator) ComputeCurrentStreaks(ctx *gin.Context, targetUserID int64) (int, int, error) {
	panic("boom del cálculo de rachas")
}

// prevStreakState arma el mock del estado persistido "antes" del write.
func prevStreakState(userID int64, strict, flexible int) func(*gin.Context, int64) (*dbs.GamificationStreakState, error) {
	return func(ctx *gin.Context, id int64) (*dbs.GamificationStreakState, error) {
		return &dbs.GamificationStreakState{UserID: userID, StrictCurrent: strict, FlexibleCurrent: flexible}, nil
	}
}

func TestStreakTrigger_ResetEstrictaGeneraUnseen(t *testing.T) {
	dao := &mockGamificationDao{getStateFn: prevStreakState(7, 5, 0)}
	tr := NewStreakTriggerService(dao, &fakeStreakCalculator{strict: 0, flexible: 3})

	err := tr.EvaluateRunnerCompletion(nil, 7)

	require.NoError(t, err)
	// Solo la strict cruza: 5→0 es reset; flexible sin racha previa no emite.
	require.Len(t, dao.insertedUnseen, 1)
	rows := dao.insertedUnseen[0]
	require.Len(t, rows, 1)
	assert.Equal(t, int64(7), rows[0].UserID)
	assert.Equal(t, constants.GamificationSectionStreaks, rows[0].Section)
	assert.Equal(t, [][]int{{0, 3}}, dao.upsertdAt, "upsert persiste el estado fresco igual con o sin evento")
}

func TestStreakTrigger_AmbasRachasUnaFilaPorRacha(t *testing.T) {
	dao := &mockGamificationDao{getStateFn: prevStreakState(7, 3, 4)}
	tr := NewStreakTriggerService(dao, &fakeStreakCalculator{strict: 0, flexible: 0})

	err := tr.EvaluateRunnerCompletion(nil, 7)

	require.NoError(t, err)
	require.Len(t, dao.insertedUnseen, 1)
	rows := dao.insertedUnseen[0]
	require.Len(t, rows, 2, "una fila por racha: máx 2 filas aunque ambas caigan a 0 en la misma mutación")
	for _, row := range rows {
		assert.Equal(t, int64(7), row.UserID)
		assert.Equal(t, constants.GamificationSectionStreaks, row.Section)
	}
	assert.Equal(t, [][]int{{0, 0}}, dao.upsertdAt)
}

func TestStreakTrigger_HitoCruzadoDesdeCero(t *testing.T) {
	dao := &mockGamificationDao{}
	tr := NewStreakTriggerService(dao, &fakeStreakCalculator{strict: 10, flexible: 0})

	err := tr.EvaluateRunnerCompletion(nil, 7)

	require.NoError(t, err)
	require.Len(t, dao.insertedUnseen, 1)
	require.Len(t, dao.insertedUnseen[0], 1)
	assert.Equal(t, constants.GamificationSectionStreaks, dao.insertedUnseen[0][0].Section)
	assert.Equal(t, [][]int{{10, 0}}, dao.upsertdAt)
}

func TestStreakTrigger_SaltoMultiHitoUnaFila(t *testing.T) {
	dao := &mockGamificationDao{getStateFn: prevStreakState(7, 0, 15)}
	tr := NewStreakTriggerService(dao, &fakeStreakCalculator{strict: 0, flexible: 55})

	err := tr.EvaluateRunnerCompletion(nil, 7)

	require.NoError(t, err)
	// 15→55 pisa los hitos 20 y 50: UNA fila para esa racha (dedupe), + la
	// strict estable no emite.
	require.Len(t, dao.insertedUnseen, 1)
	require.Len(t, dao.insertedUnseen[0], 1)
}

func TestStreakTrigger_SinEventoNoGeneraUnseen(t *testing.T) {
	cases := []struct {
		name string
		prev dbs.GamificationStreakState
		next fakeStreakCalculator
	}{
		{"estable", dbs.GamificationStreakState{StrictCurrent: 11, FlexibleCurrent: 3}, fakeStreakCalculator{strict: 11, flexible: 3}},
		{"no cruza el hito aún", dbs.GamificationStreakState{StrictCurrent: 15}, fakeStreakCalculator{strict: 19, flexible: 0}},
		{"baja sin resetear", dbs.GamificationStreakState{FlexibleCurrent: 15}, fakeStreakCalculator{strict: 0, flexible: 5}},
		{"hito ya cruzado antes", dbs.GamificationStreakState{StrictCurrent: 25}, fakeStreakCalculator{strict: 25, flexible: 0}},
	}
	for _, tc := range cases {
		dao := &mockGamificationDao{getStateFn: prevStreakState(7, tc.prev.StrictCurrent, tc.prev.FlexibleCurrent)}
		err := NewStreakTriggerService(dao, &tc.next).EvaluateRunnerCompletion(nil, 7)
		require.NoError(t, err, tc.name)
		assert.Empty(t, dao.insertedUnseen, tc.name)
		require.Len(t, dao.upsertdAt, 1, tc.name)
	}
}

// --- Errores (best-effort del hook): contracto ErrStreakTrigger y sin
// mutaciones parciales donde corresponde.

func TestStreakTrigger_Errores(t *testing.T) {
	boom := errors.New("SQLSTATE 57014: detalle crudo del driver")
	t.Run("estado previo", func(t *testing.T) {
		dao := &mockGamificationDao{getStateFn: func(ctx *gin.Context, userID int64) (*dbs.GamificationStreakState, error) {
			return nil, boom
		}}
		err := NewStreakTriggerService(dao, &fakeStreakCalculator{}).EvaluateRunnerCompletion(nil, 7)
		require.ErrorIs(t, err, ErrStreakTrigger)
		assert.Empty(t, dao.insertedUnseen)
		assert.Empty(t, dao.upsertdAt)
	})
	t.Run("recalculo", func(t *testing.T) {
		dao := &mockGamificationDao{}
		err := NewStreakTriggerService(dao, &fakeStreakCalculator{err: fmt.Errorf("detalle interno")}).EvaluateRunnerCompletion(nil, 7)
		require.ErrorIs(t, err, ErrStreakTrigger)
		assert.Empty(t, dao.insertedUnseen)
		assert.Empty(t, dao.upsertdAt)
	})
	t.Run("insert unseen", func(t *testing.T) {
		dao := &mockGamificationDao{
			getStateFn:     prevStreakState(7, 5, 0),
			insertUnseenFn: func(ctx *gin.Context, unseen []dbs.GamificationUnseen) error { return boom },
		}
		err := NewStreakTriggerService(dao, &fakeStreakCalculator{}).EvaluateRunnerCompletion(nil, 7)
		require.ErrorIs(t, err, ErrStreakTrigger)
		assert.Empty(t, dao.upsertdAt, "upsert no corre tras el insert fallido: el estado queda para el próximo hook")
	})
	t.Run("upsert estado", func(t *testing.T) {
		dao := &mockGamificationDao{upsertStateFn: func(ctx *gin.Context, userID int64, strict, flexible int) error { return boom }}
		err := NewStreakTriggerService(dao, &fakeStreakCalculator{}).EvaluateRunnerCompletion(nil, 7)
		require.ErrorIs(t, err, ErrStreakTrigger)
	})
}

// run (unidad del goroutine): el fallo de debajo se loguea best-effort y
// nada estalla; errores y éxitos se ejercitan sync acá (el camino de panic
// está en TestStreakTrigger_run_SobrevivePanic).
func TestStreakTrigger_run_Errores(t *testing.T) {
	t.Run("evaluate falla", func(t *testing.T) {
		dao := &mockGamificationDao{}
		tr := &streakTriggerService{dao: dao, streaks: &fakeStreakCalculator{err: errors.New("boom del cálculo")}}
		require.NotPanics(t, func() { tr.run(nil, 7) })
		assert.Empty(t, dao.upsertdAt)
	})
	t.Run("evaluate feliz", func(t *testing.T) {
		dao := &mockGamificationDao{}
		tr := &streakTriggerService{dao: dao, streaks: &fakeStreakCalculator{strict: 7, flexible: 9}}
		require.NotPanics(t, func() { tr.run(nil, 7) })
		assert.Empty(t, dao.insertedUnseen)
		assert.Len(t, dao.upsertdAt, 1)
	})
}

// --- Integración Postgres real --------------------------------------------

// seedTriggerScene arma runner + grupo + 12 días training Oct 1..12 del
// "hoy" fijado (Oct-13), cada uno con su instancia; finishedDays marca qué
// días tienen su sesión finished. Con Oct 3..12 finished el recálculo da una
// racha de current 10 (los días sin finished quedan failed y cortan desde
// ayer hacia atrás).
func seedTriggerScene(t *testing.T, db *gorm.DB, tag string, finishedDays map[int]bool) *dbs.User {
	t.Helper()
	runner := &dbs.User{Name: "Trigger", Surname: "Streak", Email: "trigger-streak-" + tag + "@test.com", DNI: "61" + tag, BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(runner).Error)
	team := &dbs.Team{Name: "Trigger team " + tag, MaxMembers: 10, OwnerID: runner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Trigger group " + tag, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}).Error)
	dayCount := 12
	for i := 0; i < dayCount; i++ {
		inst := &dbs.SessionInstance{Name: fmt.Sprintf("trigger inst %s %d", tag, i)}
		require.NoError(t, db.Create(inst).Error)
		day := dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 10, 1+i, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &inst.ID}
		require.NoError(t, db.Create(&day).Error)
		if finishedDays[i] {
			end := time.Date(2026, 10, 1+i, 10, 0, 0, 0, time.UTC)
			require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: inst.ID, AthleteUserID: runner.ID, Status: "finished", StartDate: day.Date.Add(9 * time.Hour), EndDate: &end}).Error)
		}
	}
	return runner
}

// seedStreakStateRow persiste el baseline "antes" del hook.
func seedStreakStateRow(t *testing.T, db *gorm.DB, userID int64, strict, flexible int) {
	t.Helper()
	require.NoError(t, db.Create(&dbs.GamificationStreakState{UserID: userID, StrictCurrent: strict, FlexibleCurrent: flexible}).Error)
}

func TestStreakTrigger_Evaluate_Integration_HitoYReset(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixStreakToday(t, 2026, 10, 13)

	// Hito: Oct 3..12 finished (10 días corridos = current 10 exacto; Oct
	// 1-2 quedan sin finished → failed y cortan igual porque el loop de
	// backward ya contó 10 antes de llegar a ellos).
	runner := seedTriggerScene(t, db, "hito", map[int]bool{2: true, 3: true, 4: true, 5: true, 6: true, 7: true, 8: true, 9: true, 10: true, 11: true})
	seedStreakStateRow(t, db, runner.ID, 9, 9)

	streakSvc := NewStreakService(daos.NewGamificationDao(db))
	tr := NewStreakTriggerService(daos.NewGamificationDao(db), streakSvc)
	require.NoError(t, tr.EvaluateRunnerCompletion(nil, runner.ID))

	var unseen []dbs.GamificationUnseen
	require.NoError(t, db.Where("user_id = ?", runner.ID).Find(&unseen).Error)
	require.Len(t, unseen, 2, "ambas rachas 9→10 cruzan el hito 10: una fila por racha")
	for _, row := range unseen {
		assert.Equal(t, constants.GamificationSectionStreaks, row.Section)
	}
	var state dbs.GamificationStreakState
	require.NoError(t, db.Where("user_id = ?", runner.ID).First(&state).Error)
	assert.Equal(t, 10, state.StrictCurrent)
	assert.Equal(t, 10, state.FlexibleCurrent)

	// Reset: otro corredor con racha previa (estado 5/2) y todo el histórico
	// de training sin finished.
	other := seedTriggerScene(t, db, "reset", nil)
	seedStreakStateRow(t, db, other.ID, 5, 2)
	require.NoError(t, tr.EvaluateRunnerCompletion(nil, other.ID))

	// 12 días training, NINGUNO finished: el anterior a ayer es failed →
	// 0/0: ambas rachas 5→0 y 2→0 reset, 2 filas unseen + estado 0/0.
	var otherUnseen []dbs.GamificationUnseen
	require.NoError(t, db.Where("user_id = ?", other.ID).Find(&otherUnseen).Error)
	assert.Len(t, otherUnseen, 2)
	var otherState dbs.GamificationStreakState
	require.NoError(t, db.Where("user_id = ?", other.ID).First(&otherState).Error)
	assert.Equal(t, 0, otherState.StrictCurrent)
	assert.Equal(t, 0, otherState.FlexibleCurrent)
}

// OnRunnerSessionCompletion async: el goroutine corre el chequeo a
// post-respuesta y NO usa el ctx de la request (que muere con ella), sino la
// copia. Sincronizado por canal: determinista y -race-friendly sin contender
// la tx de test desde dos corrientes (poll concurrente daría conn busy).
func TestStreakTrigger_AsyncConCopiaDeCtx(t *testing.T) {
	upsertd := make(chan int64, 1)
	dao := &mockGamificationDao{upsertStateFn: func(ctx *gin.Context, userID int64, strict, flexible int) error {
		upsertd <- userID
		return nil
	}}
	tr := NewStreakTriggerService(dao, &fakeStreakCalculator{strict: 4, flexible: 2})

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/api/v1/session-instances/5/runner", nil)

	tr.OnRunnerSessionCompletion(c, 7)

	select {
	case got := <-upsertd:
		assert.Equal(t, int64(7), got)
	case <-time.After(2 * time.Second):
		t.Fatal("el hook async no terminó")
	}
}

// Nil-safe del hook async: ctx nil es un caso válido de tests y la unidad no
// entra en panic; la sincronización va por canal (sin compartir el mock con
// el goroutine).
func TestStreakTrigger_AsyncNilCtx(t *testing.T) {
	upsertd := make(chan [2]int, 1)
	dao := &mockGamificationDao{upsertStateFn: func(ctx *gin.Context, userID int64, strict, flexible int) error {
		upsertd <- [2]int{strict, flexible}
		return nil
	}}
	tr := NewStreakTriggerService(dao, &fakeStreakCalculator{strict: 4, flexible: 2})

	tr.OnRunnerSessionCompletion(nil, 7)

	select {
	case got := <-upsertd:
		assert.Equal(t, [2]int{4, 2}, got)
	case <-time.After(2 * time.Second):
		t.Fatal("el hook async no terminó")
	}
}

// --- Pin de queries (sin N+1): el hook corre round-trips fijos, sin término
// lineal por día del histórico.

func TestStreakTrigger_Evaluate_QueriesConstantes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixStreakToday(t, 2026, 10, 13)

	// Escenas con día failed cerca de ayer (rachas 0) y estado previo 5/2:
	// el evaluate dispara reset → insert unseen + upsert.
	runnerSmall, _ := seedStreakBatchScene(t, db, "trigq3", 3, map[int]bool{0: true})
	seedStreakStateRow(t, db, runnerSmall.ID, 5, 2)
	runnerBig, _ := seedStreakBatchScene(t, db, "trigq12", 12, map[int]bool{3: true, 5: true})
	seedStreakStateRow(t, db, runnerBig.ID, 5, 2)

	counter := 0
	counting := txCountingDB(t, db, &counter)
	trCounting := NewStreakTriggerService(daos.NewGamificationDao(counting), NewStreakService(daos.NewGamificationDao(counting)))

	// Corrida con evento, 3 días históricos.
	counter = 0
	require.NoError(t, trCounting.EvaluateRunnerCompletion(nil, runnerSmall.ID))
	queriesSmall := counter
	t.Logf("trigger con 3 días históricos: %d queries", queriesSmall)

	// 12 días: MISMO read path.
	counter = 0
	require.NoError(t, trCounting.EvaluateRunnerCompletion(nil, runnerBig.ID))
	queriesBig := counter
	t.Logf("trigger con 12 días históricos: %d queries", queriesBig)
	assert.Equal(t, queriesSmall, queriesBig, "12 días = mismo read path (sin término lineal por día)")
	// 6 = old state + memberships + días + finished + 1 INSERT unseen (2
	// filas, ambos resets) + 1 upsert estado.
	assert.Equal(t, 6, queriesSmall)
}
