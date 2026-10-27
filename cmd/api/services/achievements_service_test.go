package services

// --- Logros (gamificacion-core D5): evaluadores con fixtures por criterio,
// idempotencia, push best-effort con stubs y vitrina.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
)

// fakeStreakView sirve los current/best de racha al evaluador streak_N.
type fakeStreakView struct {
	strictCurrent, strictBest int
	err                       error
}

func (f *fakeStreakView) GetStreaks(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &gamification.StreaksResponse{
		Strict:   gamification.StreakSummary{Current: f.strictCurrent, Best: f.strictBest},
		Flexible: gamification.StreakSummary{},
		Days:     []gamification.StreakDay{},
	}, nil
}

func (f *fakeStreakView) ComputeCurrentStreaks(ctx *gin.Context, targetUserID int64) (int, int, error) {
	return f.strictCurrent, 0, f.err
}

// fakePushTokenDAO devuelve los tokens según fixture (o error).
type fakePushTokenDAO struct {
	tokens  []dbs.PushToken
	findErr error
}

func (f *fakePushTokenDAO) Upsert(ctx *gin.Context, userID int64, token, platform string) error {
	return nil
}

func (f *fakePushTokenDAO) FindByUserID(ctx *gin.Context, userID int64) ([]dbs.PushToken, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.tokens, nil
}

// recordedPush es un Send grabado por el stub del cliente Expo.
type recordedPush struct {
	token, title, body string
	data               map[string]string
}

// stubPushClient graba los Send y puede fallar (best-effort del push).
type stubPushClient struct {
	calls  []recordedPush
	sender func() error
}

func (s *stubPushClient) Send(ctx context.Context, token, title, body string, data map[string]string) error {
	s.calls = append(s.calls, recordedPush{token: token, title: title, body: body, data: data})
	if s.sender != nil {
		return s.sender()
	}
	return nil
}

func (s *stubPushClient) SendBatch(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	for _, token := range tokens {
		if err := s.Send(ctx, token, title, body, data); err != nil {
			return err
		}
	}
	return nil
}

// fakeAchievementDao lleva el snapshot de agregados por criterio + registra
// los InsertUnlock. alreadyUnlocked modela la idempotencia de la PK compuesta.
type fakeAchievementDao struct {
	// agregados por criterio.
	finishedCount  int64
	sumDistance    float64
	maxSingle      float64
	gpsCount       int64
	starts         []time.Time
	attendanceDays int64

	// vitrina.
	listProgressRows []dbs.UserAchievementProgress
	listStampsRows   []dbs.GamificationStamp
	listStampsErr    error

	// ejercito de errores y grabaciones.
	queriesErr      error
	alreadyUnlocked map[string]bool
	unlocks         []string
	listProgressErr error
}

func (f *fakeAchievementDao) ListProgress(ctx *gin.Context, userID int64) ([]dbs.UserAchievementProgress, error) {
	if f.listProgressErr != nil {
		return nil, f.listProgressErr
	}
	return f.listProgressRows, nil
}

func (f *fakeAchievementDao) InsertUnlock(ctx *gin.Context, userID int64, achievementID string, unlockedAt time.Time) (bool, error) {
	if f.queriesErr != nil {
		return false, f.queriesErr
	}
	if f.alreadyUnlocked[achievementID] {
		return false, nil
	}
	f.alreadyUnlocked[achievementID] = true
	f.unlocks = append(f.unlocks, achievementID)
	return true, nil
}

func (f *fakeAchievementDao) ListStamps(ctx *gin.Context, userID int64) ([]dbs.GamificationStamp, error) {
	if f.listStampsErr != nil {
		return nil, f.listStampsErr
	}
	return f.listStampsRows, nil
}

func (f *fakeAchievementDao) CountFinishedSessions(ctx *gin.Context, userID int64) (int64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.finishedCount, nil
}

func (f *fakeAchievementDao) SumDistanceMeters(ctx *gin.Context, userID int64) (float64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.sumDistance, nil
}

func (f *fakeAchievementDao) MaxDistanceSingleSession(ctx *gin.Context, userID int64) (float64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.maxSingle, nil
}

func (f *fakeAchievementDao) CountGPSFinishedSessions(ctx *gin.Context, userID int64) (int64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.gpsCount, nil
}

func (f *fakeAchievementDao) ListFinishedSessionStarts(ctx *gin.Context, userID int64) ([]time.Time, error) {
	if f.queriesErr != nil {
		return nil, f.queriesErr
	}
	return f.starts, nil
}

func (f *fakeAchievementDao) CountAttendanceDays(ctx *gin.Context, userID int64) (int64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.attendanceDays, nil
}

func newAchievementsTestService(dao *fakeAchievementDao, gamDao *mockGamificationDao, streaks StreakServiceInterface, tokens *fakePushTokenDAO, client *stubPushClient) AchievementsServiceInterface {
	return NewAchievementsService(dao, gamDao, streaks, tokens, client)
}

func TestCountEarlyStarts_FronteraOchoHoras(t *testing.T) {
	// TZ Cordoba (UTC-3): 11:00 UTC = 08:00 → NO cuenta; 10:59 UTC = 07:59 → sí.
	starts := []time.Time{
		time.Date(2026, 10, 12, 10, 59, 0, 0, time.UTC),
		time.Date(2026, 10, 12, 11, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),  // 06:00 → sí
		time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC), // 09:00 → no
	}
	assert.Equal(t, 2, countEarlyStarts(starts))
}

func TestAchievementsService_SessionsUnlock(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 5}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{strictCurrent: 4, strictBest: 6}, &fakePushTokenDAO{}, &stubPushClient{})

	err := svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession)

	require.NoError(t, err)
	// 5 finished >= 5 y >= 1; best de racha strict 6 >= 3, < 7; early sin datos.
	assert.Equal(t, []string{"first_session", "streak_3", "sessions_5"}, dao.unlocks)
	// Una fila unseen de la sección achievements por logro desbloqueado.
	require.Len(t, gamDao.insertedUnseen, 1)
	require.Len(t, gamDao.insertedUnseen[0], 3)
	for _, row := range gamDao.insertedUnseen[0] {
		assert.Equal(t, "achievements", row.Section)
		assert.EqualValues(t, 7, row.UserID)
	}
}

func TestAchievementsService_UmbralNoAlcanza(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 3}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	err := svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession)

	require.NoError(t, err)
	// Con 3 sesiones solo first_session (threshold 1) se desbloquea:
	// sessions_5 NO (sin fila) y ninguna racha a 0 corta un hito.
	assert.Equal(t, []string{"first_session"}, dao.unlocks)
	assert.NotContains(t, dao.unlocks, "sessions_5")
}

func TestAchievementsService_ReevaluacionIdempotente(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{
		"first_session": true,
		"sessions_5":    true,
	}, finishedCount: 5}
	gamDao := &mockGamificationDao{}
	client := &stubPushClient{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, client)

	err := svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession)

	require.NoError(t, err)
	assert.NotContains(t, dao.unlocks, "sessions_5", "re-evaluar un logro ya desbloqueado no inserta")
	assert.Empty(t, gamDao.insertedUnseen)
	assert.Empty(t, client.calls, "sin push si nada se desbloqueo recién")
}

func TestAchievementsService_DistanceUnlock(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, sumDistance: 10500, maxSingle: 43000}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	err := svc.EvaluateEvent(nil, 7, AchievementTriggerFeedback)

	require.NoError(t, err)
	// Total 10.5 km → 10k; 100k no; single 43 km ≥ 42.2 → distance_42k_single.
	assert.Equal(t, []string{"distance_10k", "distance_42k_single"}, dao.unlocks)
}

func TestAchievementsService_GPSYTeamUnlock(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, gpsCount: 10, attendanceDays: 2}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerGPSPoints))
	assert.Equal(t, []string{"gps_first", "gps_10"}, dao.unlocks)

	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerAttendance))
	assert.Equal(t, []string{"gps_first", "gps_10", "team_first"}, dao.unlocks)
}

func TestAchievementsService_TriggersAislados(t *testing.T) {
	// El trigger de feedback no debe tocar queries de sesiones/racha.
	fakeStreak := &fakeStreakView{strictCurrent: 60, strictBest: 60}
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, sumDistance: 0}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, fakeStreak, &fakePushTokenDAO{}, &stubPushClient{})

	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerFeedback))
	assert.Empty(t, dao.unlocks, "0 km no desbloquea nada de volume")
}

func TestAchievementsService_PushBestEffort_FalloSilencioso(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 5}
	gamDao := &mockGamificationDao{}
	client := &stubPushClient{sender: func() error { return errors.New("expo rechazado") }}
	tokens := &fakePushTokenDAO{tokens: []dbs.PushToken{{Token: "ExponentPushToken[abc]"}}}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, tokens, client)

	err := svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession)

	require.NoError(t, err, "el fallo del proveedor nunca rompe el flujo de desbloqueo")
	assert.NotEmpty(t, dao.unlocks)
	require.Len(t, gamDao.insertedUnseen, 1, "el unseen se inserta igual")
}

func TestAchievementsService_PushPayload(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 5}
	gamDao := &mockGamificationDao{}
	client := &stubPushClient{}
	tokens := &fakePushTokenDAO{tokens: []dbs.PushToken{{Token: "ExponentPushToken[abc]"}}}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, tokens, client)

	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession))
	require.NotEmpty(t, client.calls)
	for _, call := range client.calls {
		assert.Equal(t, "ExponentPushToken[abc]", call.token)
		assert.Equal(t, "/achievements", call.data["route"])
		assert.Equal(t, "achievement_unlocked", call.data["type"])
	}
}

func TestAchievementsService_SinTokensNoPush(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 5}
	gamDao := &mockGamificationDao{}
	client := &stubPushClient{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, client)

	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession))
	assert.NotEmpty(t, dao.unlocks)
	assert.Empty(t, client.calls, "sin dispositivos registrados el push no se dispara")
}

func TestAchievementsService_GetUserAchievements_Merge(t *testing.T) {
	now := time.Now()
	dao := &fakeAchievementDao{listProgressRows: []dbs.UserAchievementProgress{
		{UserID: 7, AchievementID: "first_session", UnlockedAt: now},
		{UserID: 7, AchievementID: "gps_first", UnlockedAt: now},
	}}
	gamDao := &mockGamificationDao{hasUnseenFn: func(ctx *gin.Context, userID int64, section string) (bool, error) {
		return section == "achievements", nil
	}}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	resp, err := svc.GetUserAchievements(nil, 7)

	require.NoError(t, err)
	require.Len(t, resp.Data, 21)
	require.NotNil(t, resp.HasUnseen)
	assert.True(t, resp.HasUnseen)
	unlocked := 0
	for _, item := range resp.Data {
		if item.UnlockedAt != nil {
			unlocked++
			if item.AchievementID == "first_session" {
				assert.WithinDuration(t, now, *item.UnlockedAt, time.Second)
			}
		} else {
			assert.Nil(t, item.LastUnlockedAt)
		}
		// Shape v1: current_level y count SIEMPRE en null.
		assert.Nil(t, item.CurrentLevel)
		assert.Nil(t, item.Count)
	}
	assert.Equal(t, 2, unlocked, "2 desbloqueados y 19 con unlocked_at null")
}

func TestAchievementsService_GetUserAchievements_Error(t *testing.T) {
	dao := &fakeAchievementDao{listProgressErr: errors.New("SQLSTATE 42P01 detalle crudo")}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	resp, err := svc.GetUserAchievements(nil, 7)
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestAchievementsService_GetUserStamps(t *testing.T) {
	dao := &fakeAchievementDao{}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	resp, err := svc.GetUserStamps(nil, 7)
	require.NoError(t, err)
	assert.Empty(t, resp.Data, "vitrina de estampas vacía hasta E4")

	granted := time.Now()
	dao.listStampsRows = []dbs.GamificationStamp{{ID: 5, UserID: 7, Type: "competition", SourceID: 9, SourceName: "Competencia 9", GrantedAt: granted, Count: 1}}
	resp, err = svc.GetUserStamps(nil, 7)
	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "competition", resp.Data[0].Type)
}

func TestAchievementsService_GetCatalog(t *testing.T) {
	svc := NewAchievementsService(&fakeAchievementDao{}, &mockGamificationDao{}, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	catalog := svc.GetCatalog(nil)
	require.Len(t, catalog, 21)
	for _, row := range catalog {
		assert.NotEmpty(t, row.ID)
		assert.NotEmpty(t, row.Unit)
	}
}

// --- El hook async: nil-safe y con panic-guard (sync acá vía caminos del
// struct; el camino del goroutine real queda cubierto por el patrón E1).

// run (unidad del goroutine): los errores y el panic se loguean best-effort
// y nada estalla; EvaluateEvent feliz ya está cubierto arriba.
func TestAchievementsService_run_Errores(t *testing.T) {
	t.Run("evaluate falla", func(t *testing.T) {
		dao := &fakeAchievementDao{queriesErr: errors.New("boom de agregados")}
		svc := newAchievementsTestService(dao, &mockGamificationDao{}, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{}).(*achievementsService)
		require.NotPanics(t, func() { svc.run(nil, 7, AchievementTriggerRunnerSession) })
		assert.Empty(t, dao.unlocks)
	})
	t.Run("evaluate paniquea", func(t *testing.T) {
		var boomStreak panicStreakView
		svc := NewAchievementsService(&fakeAchievementDao{}, &mockGamificationDao{}, boomStreak, &fakePushTokenDAO{}, &stubPushClient{}).(*achievementsService)
		require.NotPanics(t, func() { svc.run(nil, 7, AchievementTriggerRunnerSession) })
	})
}

// panicStreakView paniquea dentro de GetStreaks pese al guard del service:
// alimenta el camino de panic del run.
type panicStreakView struct{}

func (panicStreakView) GetStreaks(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
	panic("boom del cálculo de rachas en logros")
}

func (panicStreakView) ComputeCurrentStreaks(ctx *gin.Context, targetUserID int64) (int, int, error) {
	return 0, 0, nil
}

func TestAchievementsService_OnEvent_NilSafe(t *testing.T) {
	var svc *achievementsService
	require.NotPanics(t, func() { svc.OnEvent(nil, 7, AchievementTriggerRunnerSession) })

	// Con dao nil (y no nil receiver) tampoco dispara: módulo desactivado.
	off := &achievementsService{}
	require.NotPanics(t, func() { off.OnEvent(nil, 7, AchievementTriggerRunnerSession) })
}
