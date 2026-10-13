package services

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/testutils"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// ---------------------------------------------------------------------------
// Service de rachas (gamificacion-core D2). Determinismo: todos los tests
// fijan el "hoy" con time.Date vía la var gametime.Now (design D1) — nada
// depende de la hora real del runner.
// ---------------------------------------------------------------------------

type mockGamificationDao struct {
	findMembershipsFn  func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error)
	findTrainingDaysFn func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error)
	findFinishedFn     func(ctx *gin.Context, sessionInstanceIDs []int64, athleteUserID int64) ([]dbs.RunnerSession, error)
	hasUnseenFn        func(ctx *gin.Context, userID int64, section string) (bool, error)
}

func (m *mockGamificationDao) FindActiveGroupMemberships(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
	if m.findMembershipsFn != nil {
		return m.findMembershipsFn(ctx, userID)
	}
	return nil, nil
}

func (m *mockGamificationDao) FindTrainingDaysUntil(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
	if m.findTrainingDaysFn != nil {
		return m.findTrainingDaysFn(ctx, groupIDs, until)
	}
	return nil, nil
}

func (m *mockGamificationDao) FindFinishedRunnerSessions(ctx *gin.Context, sessionInstanceIDs []int64, athleteUserID int64) ([]dbs.RunnerSession, error) {
	if m.findFinishedFn != nil {
		return m.findFinishedFn(ctx, sessionInstanceIDs, athleteUserID)
	}
	return nil, nil
}

func (m *mockGamificationDao) HasUnseenSection(ctx *gin.Context, userID int64, section string) (bool, error) {
	if m.hasUnseenFn != nil {
		return m.hasUnseenFn(ctx, userID, section)
	}
	return false, nil
}

// fixStreakToday pisa gametime.Now con time.Date para fijar el "hoy". La
// fecha se da en UTC (12:00) — el service siempre la cae a Cordoba, así que
// cualquier hora real del día da el mismo resultado.
func fixStreakToday(t *testing.T, y int, m time.Month, day int) {
	t.Helper()
	orig := gametime.Now
	t.Cleanup(func() { gametime.Now = orig })
	gametime.Now = func() time.Time { return time.Date(y, m, day, 12, 0, 0, 0, time.UTC) }
}

// mkTrainingDays arma una fila de calendario por instancia asignada al día
// (dos entrenamientos el mismo día = dos filas, hoy distinto grupo). Sin
// instancias queda una fila sin instancia (training que no puede completarse).
// El date se parsea en UTC: misma semántica que el scan del driver (design
// que documenta el TestDateColumnScanProbe del paquete daos).
func mkTrainingDays(date string, instanceIDs ...int64) []dbs.GroupCalendarDay {
	var d time.Time
	d, _ = time.Parse("2006-01-02", date)
	if len(instanceIDs) == 0 {
		return []dbs.GroupCalendarDay{{GroupID: 10, Date: d, Kind: "training"}}
	}
	days := make([]dbs.GroupCalendarDay, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		idCopy := id
		days = append(days, dbs.GroupCalendarDay{GroupID: 10, Date: d, Kind: "training", SessionInstanceID: &idCopy})
	}
	return days
}

func finishedRuns(instanceIDs ...int64) []dbs.RunnerSession {
	out := make([]dbs.RunnerSession, 0, len(instanceIDs))
	for _, id := range instanceIDs {
		end := time.Date(2026, 10, 1, 7, 0, 0, 0, time.UTC)
		out = append(out, dbs.RunnerSession{SessionInstanceID: id, AthleteUserID: 7, Status: "finished", EndDate: &end})
	}
	return out
}

func streakDay(resp *gamification.StreaksResponse, date string) (gamification.StreakDay, bool) {
	for _, d := range resp.Days {
		if d.Date == date {
			return d, true
		}
	}
	return gamification.StreakDay{}, false
}

func assertDay(t *testing.T, resp *gamification.StreaksResponse, date string, wantStrict, wantFlex string) {
	t.Helper()
	day, ok := streakDay(resp, date)
	require.True(t, ok, "falta el día %s en el array", date)
	assert.Equal(t, wantStrict, day.StrictStatus, "strict de %s", date)
	assert.Equal(t, wantFlex, day.FlexibleStatus, "flexible de %s", date)
}

// --- Estados de día ------------------------------------------------------

func TestStreakService_DayStatuses(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	// Oct-10: dos sesiones, 1 finished (spec D2: strict failed, flexible
	// completed). Oct-9: todo finished. Oct-8: training sin finished.
	dao := &mockGamificationDao{
		findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{{GroupID: 10}}, nil
		},
		findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
			var out []dbs.GroupCalendarDay
			out = append(out, mkTrainingDays("2026-10-10", 101, 102)...)
			out = append(out, mkTrainingDays("2026-10-09", 103)...)
			out = append(out, mkTrainingDays("2026-10-08", 104)...)
			return out, nil
		},
		findFinishedFn: func(ctx *gin.Context, ids []int64, athleteID int64) ([]dbs.RunnerSession, error) {
			return finishedRuns(101, 103), nil
		},
	}
	svc := NewStreakService(dao)

	resp, err := svc.GetStreaks(nil, 7, 42)
	require.NoError(t, err)

	// 1 de 2 sesiones finished → strict failed, flexible completed.
	assertDay(t, resp, "2026-10-10", gamification.DayStatusFailed, gamification.DayStatusCompleted)
	// Todas finished → completed en ambos.
	assertDay(t, resp, "2026-10-09", gamification.DayStatusCompleted, gamification.DayStatusCompleted)
	// 0 finished con training asignado → failed en ambos.
	assertDay(t, resp, "2026-10-08", gamification.DayStatusFailed, gamification.DayStatusFailed)
	// Día sin training → skipped en ambos.
	assertDay(t, resp, "2026-10-07", gamification.DayStatusSkipped, gamification.DayStatusSkipped)
	assertDay(t, resp, "2026-09-02", gamification.DayStatusSkipped, gamification.DayStatusSkipped)
	// El día de hoy SIEMPRE pending (aunque llegue data).
	assertDay(t, resp, "2026-10-13", gamification.DayStatusPending, gamification.DayStatusPending)
	// Ventana default 42: de hoy-41 a hoy.
	require.Len(t, resp.Days, 42)
	assert.Equal(t, "2026-09-02", resp.Days[0].Date)
	assert.Equal(t, "2026-10-13", resp.Days[len(resp.Days)-1].Date)
}

// El DAO solo devuelve kind='training': rest/other/cancelled no llegan y
// quedan skipped por ausencia (lo que en producción también cubre días sin
// fila de calendario).
func TestStreakService_DiasSinTrainingSkipped(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	dao := &mockGamificationDao{
		findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{{GroupID: 10}}, nil
		},
		findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
			return nil, nil
		},
	}
	resp, err := NewStreakService(dao).GetStreaks(nil, 7, 7)
	require.NoError(t, err)
	assert.Len(t, resp.Days, 7)
	for _, d := range resp.Days {
		switch d.Date {
		case "2026-10-13":
			assert.Equal(t, gamification.DayStatusPending, d.StrictStatus)
		default:
			assert.Equal(t, gamification.DayStatusSkipped, d.StrictStatus, "%s", d.Date)
		}
	}
}

// --- Rachas: current/best -------------------------------------------------

// Racha de referencia: Oct-12 ok, Oct-11 skipped, Oct-10 failed, Oct-9 ok,
// Oct-8 ok, Oct-7 skipped, Oct-6 ok, Oct-5 skipped, Oct-4/3/2 ok.
// Strict == flexible (la data no distingue): Oct-10 corta hacia atrás →
// current 1; best = 4 (Oct-2..Oct-6, dos skipped de por medio no cortan).
func TestStreakService_FailedCortaSkippedPasa(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	dao := &mockGamificationDao{
		findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{{GroupID: 10}}, nil
		},
		findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
			var out []dbs.GroupCalendarDay
			out = append(out, mkTrainingDays("2026-10-12", 12)...)
			out = append(out, mkTrainingDays("2026-10-10", 10)...)
			out = append(out, mkTrainingDays("2026-10-09", 9)...)
			out = append(out, mkTrainingDays("2026-10-08", 8)...)
			out = append(out, mkTrainingDays("2026-10-06", 6)...)
			out = append(out, mkTrainingDays("2026-10-04", 4)...)
			out = append(out, mkTrainingDays("2026-10-03", 3)...)
			out = append(out, mkTrainingDays("2026-10-02", 2)...)
			return out, nil
		},
		findFinishedFn: func(ctx *gin.Context, ids []int64, athleteID int64) ([]dbs.RunnerSession, error) {
			return finishedRuns(12, 9, 8, 6, 4, 3, 2), nil
		},
	}
	resp, err := NewStreakService(dao).GetStreaks(nil, 7, 42)
	require.NoError(t, err)

	assert.Equal(t, 1, resp.Strict.Current, "current: Oct-12 ok, Oct-11 skipped, Oct-10 failed corta")
	assert.Equal(t, 1, resp.Flexible.Current)
	assert.Equal(t, 6, resp.Strict.Best, "best: Oct-2..Oct-9 son 6 completed puenteados por 2 skipped (skipped no corta), Oct-10 failed")
	assert.Equal(t, 6, resp.Flexible.Best)
}

// days solo recorta el ARRAY (spec: "Solo el array se recorta por la
// ventana"): una racha de 7 días fuera de la ventana days=7 ups voca best=7,
// current=3.
func TestStreakService_BestNoEsLocalALaVentana(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	dao := &mockGamificationDao{
		findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{{GroupID: 10}}, nil
		},
		findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
			var out []dbs.GroupCalendarDay
			for i := 1; i <= 7; i++ {
				out = append(out, mkTrainingDays(fmt.Sprintf("2026-09-%02d", i), int64(i))...)
			}
			out = append(out, mkTrainingDays("2026-10-10", 101)...)
			out = append(out, mkTrainingDays("2026-10-09", 102)...)
			out = append(out, mkTrainingDays("2026-10-08", 103)...)
			// Failed el Oct-6: corta el current pero no el best (fuera de la
			// ventana la racha de Sept sigue entera).
			out = append(out, mkTrainingDays("2026-10-06", 106)...)
			return out, nil
		},
		findFinishedFn: func(ctx *gin.Context, ids []int64, athleteID int64) ([]dbs.RunnerSession, error) {
			out := make([]dbs.RunnerSession, 0, len(ids))
			for _, id := range ids {
				if id == 106 {
					continue // Oct-6 sin finished → failed
				}
				out = append(out, dbs.RunnerSession{SessionInstanceID: id, AthleteUserID: 7, Status: "finished"})
			}
			return out, nil
		},
	}
	resp, err := NewStreakService(dao).GetStreaks(nil, 7, 7)
	require.NoError(t, err)

	assert.Len(t, resp.Days, 7)
	assert.Equal(t, "2026-10-07", resp.Days[0].Date)
	assert.Equal(t, "2026-10-13", resp.Days[len(resp.Days)-1].Date)
	assert.Equal(t, 3, resp.Strict.Current)
	assert.Equal(t, 7, resp.Strict.Best, "best = racha entera de Sept (Oct-6 failed la corta, fuera de la ventana); el array days=7 no la muestra igual")
	assert.Equal(t, 3, resp.Flexible.Current)
	assert.Equal(t, 7, resp.Flexible.Best)
}

// Historia pre-membresía: corredor con alta (date_start) a mitad del
// histórico del grupo. Los días training ANTERIORES a su alta no son suyos:
// skipped en ambos estados (no failed) y no cortan rachas — current/best
// cuentan desde el ingreso.
func TestStreakService_HistoriaPreMembresiaNoCorta(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	dao := &mockGamificationDao{
		findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{{GroupID: 10, DateStart: time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)}}, nil
		},
		findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
			var out []dbs.GroupCalendarDay
			out = append(out, mkTrainingDays("2026-10-02", 2)...)
			out = append(out, mkTrainingDays("2026-10-04", 4)...)
			out = append(out, mkTrainingDays("2026-10-05", 5)...)
			for i := 6; i <= 12; i++ {
				out = append(out, mkTrainingDays(fmt.Sprintf("2026-10-%02d", i), int64(i))...)
			}
			return out, nil
		},
		findFinishedFn: func(ctx *gin.Context, ids []int64, athleteID int64) ([]dbs.RunnerSession, error) {
			return finishedRuns(6, 7, 8, 9, 10, 11, 12), nil
		},
	}
	resp, err := NewStreakService(dao).GetStreaks(nil, 7, 42)
	require.NoError(t, err)

	// Pre-alta: training sin finished pero del grupo → NO failed, skipped.
	assertDay(t, resp, "2026-10-02", gamification.DayStatusSkipped, gamification.DayStatusSkipped)
	assertDay(t, resp, "2026-10-04", gamification.DayStatusSkipped, gamification.DayStatusSkipped)
	assertDay(t, resp, "2026-10-05", gamification.DayStatusSkipped, gamification.DayStatusSkipped)
	// Desde el alta inclusive: completed normal.
	for i := 6; i <= 12; i++ {
		assertDay(t, resp, fmt.Sprintf("2026-10-%02d", i), gamification.DayStatusCompleted, gamification.DayStatusCompleted)
	}
	// Sin failed en todo el histórico: 7 días corridos de Oct-6 a Oct-12.
	assert.Equal(t, 7, resp.Strict.Current)
	assert.Equal(t, 7, resp.Strict.Best)
	assert.Equal(t, 7, resp.Flexible.Current)
	assert.Equal(t, 7, resp.Flexible.Best)
}

// La fecha de alta cuenta INCLUSIVE: un training el mismo día del date_start
// es un día del corredor (la racha arranca ahí, no un día después).
func TestStreakService_AltaIncluidaEnLaRacha(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	dao := &mockGamificationDao{
		findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{{GroupID: 10, DateStart: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)}}, nil
		},
		findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
			var out []dbs.GroupCalendarDay
			out = append(out, mkTrainingDays("2026-10-03", 3)...)
			out = append(out, mkTrainingDays("2026-10-04", 4)...)
			out = append(out, mkTrainingDays("2026-10-05", 5)...)
			for i := 6; i <= 12; i++ {
				out = append(out, mkTrainingDays(fmt.Sprintf("2026-10-%02d", i), int64(i))...)
			}
			return out, nil
		},
		findFinishedFn: func(ctx *gin.Context, ids []int64, athleteID int64) ([]dbs.RunnerSession, error) {
			return finishedRuns(5, 6, 7, 8, 9, 10, 11, 12), nil
		},
	}
	resp, err := NewStreakService(dao).GetStreaks(nil, 7, 42)
	require.NoError(t, err)

	// Día del alta: training terminado → completed (racha desde el alta inclusive).
	assertDay(t, resp, "2026-10-05", gamification.DayStatusCompleted, gamification.DayStatusCompleted)
	// Pre-alta: skipped (no failed, no cappea).
	assertDay(t, resp, "2026-10-03", gamification.DayStatusSkipped, gamification.DayStatusSkipped)
	assertDay(t, resp, "2026-10-04", gamification.DayStatusSkipped, gamification.DayStatusSkipped)
	// Oct-5..Oct-12 = 8 días; si el día de alta quedara afuera sería 7.
	assert.Equal(t, 8, resp.Strict.Current)
	assert.Equal(t, 8, resp.Strict.Best)
	assert.Equal(t, 8, resp.Flexible.Current)
	assert.Equal(t, 8, resp.Flexible.Best)
}

// El día de hoy nunca corta la racha ni la rompe (spec "El día de hoy no
// corta"): ayer completado y hoy sin entrenar → current intacto y hoy pending.
func TestStreakService_TodayNoCorta(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	dao := &mockGamificationDao{
		findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
			return []dbs.GroupUser{{GroupID: 10}}, nil
		},
		findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
			var out []dbs.GroupCalendarDay
			for i := 8; i <= 12; i++ {
				out = append(out, mkTrainingDays(fmt.Sprintf("2026-10-%02d", i), int64(i))...)
			}
			return out, nil
		},
		findFinishedFn: func(ctx *gin.Context, ids []int64, athleteID int64) ([]dbs.RunnerSession, error) {
			return finishedRuns(8, 9, 10, 11, 12), nil
		},
	}
	resp, err := NewStreakService(dao).GetStreaks(nil, 7, 42)
	require.NoError(t, err)
	assert.Equal(t, 5, resp.Strict.Current)
	assert.Equal(t, gamification.DayStatusPending, resp.Days[len(resp.Days)-1].StrictStatus)
	assertDay(t, resp, "2026-10-12", gamification.DayStatusCompleted, gamification.DayStatusCompleted)
}

func TestStreakService_HasUnseen(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	dao := &mockGamificationDao{
		hasUnseenFn: func(ctx *gin.Context, userID int64, section string) (bool, error) {
			return userID == 7 && section == "streaks", nil
		},
	}
	resp, err := NewStreakService(dao).GetStreaks(nil, 7, 3)
	require.NoError(t, err)
	assert.True(t, resp.HasUnseen)
	assert.Len(t, resp.Days, 3)
}

func TestStreakService_SinGruposCeros(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	svc := NewStreakService(&mockGamificationDao{})

	resp, err := svc.GetStreaks(nil, 7, 7)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Strict.Current)
	assert.Equal(t, 0, resp.Strict.Best)
	assert.Equal(t, 0, resp.Flexible.Current)
	assert.Equal(t, 0, resp.Flexible.Best)
	assert.Len(t, resp.Days, 7)
}

// --- Errores --------------------------------------------------------------

func TestStreakService_DBErrorMensajesContractuales(t *testing.T) {
	fixStreakToday(t, 2026, 10, 13)
	boom := errors.New("detalle interno crudo")
	cases := map[string]*mockGamificationDao{
		"unseen": {
			hasUnseenFn: func(ctx *gin.Context, userID int64, section string) (bool, error) { return false, boom },
		},
		"memberships": {
			findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) { return nil, boom },
		},
		"training days": {
			findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
				return []dbs.GroupUser{{GroupID: 10}}, nil
			},
			findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) { return nil, boom },
		},
		"finished": {
			findMembershipsFn: func(ctx *gin.Context, userID int64) ([]dbs.GroupUser, error) {
				return []dbs.GroupUser{{GroupID: 10}}, nil
			},
			findTrainingDaysFn: func(ctx *gin.Context, groupIDs []int64, until time.Time) ([]dbs.GroupCalendarDay, error) {
				return mkTrainingDays("2026-10-10", 101), nil
			},
			findFinishedFn: func(ctx *gin.Context, ids []int64, athleteID int64) ([]dbs.RunnerSession, error) { return nil, boom },
		},
	}
	for name, dao := range cases {
		_, err := NewStreakService(dao).GetStreaks(nil, 7, 42)
		require.Error(t, err, name)
		// Mensaje contractual sin detalle del driver (no-leak).
		require.EqualError(t, err, "error al calcular rachas", name)
	}
}

// --- Batch: pin de conteo de queries y no-leak via FailingDB (Postgres
// real vía SetupTestDB, mismo patrón que calendar_service_batchwrite_test.go).

// seedStreakBatchScene arma runner + grupo + N días training (cada uno con
// instancia propia) y las sesiones finished indicadas.
func seedStreakBatchScene(t *testing.T, db *gorm.DB, tag string, dayCount int, finishedIndexes map[int]bool) (*dbs.User, *dbs.Group) {
	t.Helper()
	runner := &dbs.User{Name: "Batch", Surname: "Streak", Email: "batch-streak-" + tag + "@test.com", DNI: "52" + tag, BirthDate: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Password: "hashed"}
	require.NoError(t, db.Create(runner).Error)
	team := &dbs.Team{Name: "Batch streak " + tag, MaxMembers: 10, OwnerID: runner.ID}
	require.NoError(t, db.Create(team).Error)
	group := &dbs.Group{Name: "Batch streak group " + tag, TeamID: team.ID, IsMain: true}
	require.NoError(t, db.Create(group).Error)
	require.NoError(t, db.Create(&dbs.GroupUser{GroupID: group.ID, UserID: runner.ID, DateStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}).Error)

	for i := 0; i < dayCount; i++ {
		// Días fijos de Sept-Oct 2026: todos anteriores a ayer del "hoy"
		// fijado en el test (Oct-13), así la ventana los cubre.
		inst := &dbs.SessionInstance{Name: fmt.Sprintf("streak inst %s %d", tag, i)}
		require.NoError(t, db.Create(inst).Error)
		day := dbs.GroupCalendarDay{GroupID: group.ID, Date: time.Date(2026, 9, 1+i, 0, 0, 0, 0, time.UTC), Kind: "training", SessionInstanceID: &inst.ID}
		require.NoError(t, db.Create(&day).Error)
		if finishedIndexes[i] {
			require.NoError(t, db.Create(&dbs.RunnerSession{SessionInstanceID: inst.ID, AthleteUserID: runner.ID, Status: "finished", StartDate: time.Date(2026, 9, 1+i, 6, 0, 0, 0, time.UTC)}).Error)
		}
	}
	return runner, group
}

// El cálculo batch de D2: exactamente 4 queries (EXISTS unseen + memberships
// + días + sesiones finished), INDEPENDIENTE de cuántos días trae el
// histórico. Si un N+1 se cuela, esta aserción explota.
func TestStreakService_GetStreaks_QueriesConstantes(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixStreakToday(t, 2026, 10, 13)

	runner, _ := seedStreakBatchScene(t, db, "q4", 4, map[int]bool{0: true, 1: true, 2: true})
	counter := 0
	counting := txCountingDB(t, db, &counter)
	svc := NewStreakService(daos.NewGamificationDao(counting))

	resp, err := svc.GetStreaks(nil, runner.ID, 42)
	require.NoError(t, err)
	require.Len(t, resp.Days, 42)
	dayQueries := counter
	t.Logf("streaks con 4 días históricos: %d queries", dayQueries)

	// El mismo cálculo con 12 días y más sesiones finished NO agrega queries.
	runner12, _ := seedStreakBatchScene(t, db, "q12", 12, map[int]bool{0: true, 3: true, 5: true, 7: true, 9: true, 11: true})
	counter = 0
	respBig, errBig := svc.GetStreaks(nil, runner12.ID, 42)
	require.NoError(t, errBig)
	require.Len(t, respBig.Days, 42)
	t.Logf("streaks con 12 días históricos: %d queries", counter)

	assert.Equal(t, dayQueries, counter, "12 días = exactamente el mismo read path (sin término lineal por día)")
	assert.Equal(t, 4, dayQueries, "4 fijo: EXISTS unseen + memberships + días training + sesiones finished")
}

func TestStreakService_GetStreaks_FailingDBNoLeak(t *testing.T) {
	db := testutils.SetupTestDB(t)
	fixStreakToday(t, 2026, 10, 13)
	runner, _ := seedStreakBatchScene(t, db, "leak", 2, map[int]bool{0: true})

	// 1er select que falla = el EXISTS de unseen → mensaje contractual.
	failing := testutils.FailingDB(t, db, nthFail("select", 1))
	svc := NewStreakService(daos.NewGamificationDao(failing))
	_, err := svc.GetStreaks(nil, runner.ID, 42)
	require.EqualError(t, err, "error al calcular rachas")

	// 2do = memberships; 3ro = días training (idem contract).
	failing = testutils.FailingDB(t, db, nthFail("select", 2))
	svc = NewStreakService(daos.NewGamificationDao(failing))
	_, err = svc.GetStreaks(nil, runner.ID, 42)
	require.EqualError(t, err, "error al calcular rachas")

	failing = testutils.FailingDB(t, db, nthFail("select", 3))
	svc = NewStreakService(daos.NewGamificationDao(failing))
	_, err = svc.GetStreaks(nil, runner.ID, 42)
	require.EqualError(t, err, "error al calcular rachas")
}
