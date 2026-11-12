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

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
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

func (f *fakeStreakView) ComputeSnapshot(ctx *gin.Context, targetUserID int64) (*StreakSnapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &StreakSnapshot{StrictCurrent: f.strictCurrent, StrictBest: f.strictBest}, nil
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

func (f *fakePushTokenDAO) FindByUserIDs(ctx *gin.Context, userIDs []int64) ([]dbs.PushToken, error) {
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
// los unlocks y sus unseen. alreadyUnlocked modela la idempotencia de la PK
// compuesta.
type fakeAchievementDao struct {
	// agregados por criterio.
	finishedCount  int64
	earlyCount     int64
	sumDistance    float64
	maxSingle      float64
	gpsCount       int64
	attendanceDays int64

	// vitrina.
	listProgressRows []dbs.UserAchievementProgress
	listStampsRows   []dbs.GamificationStamp
	listStampsErr    error

	// ejercito de errores y grabaciones.
	queriesErr      error
	batchErr        error
	rowErrFor       map[string]error
	alreadyUnlocked map[string]bool
	unlocks         []string
	unseen          []dbs.GamificationUnseen
	listProgressErr error
}

func (f *fakeAchievementDao) ListProgress(ctx *gin.Context, userID int64) ([]dbs.UserAchievementProgress, error) {
	if f.listProgressErr != nil {
		return nil, f.listProgressErr
	}
	return f.listProgressRows, nil
}

func (f *fakeAchievementDao) InsertUnlockWithUnseen(ctx *gin.Context, userID int64, achievementID string, unlockedAt time.Time) (bool, error) {
	if f.queriesErr != nil {
		return false, f.queriesErr
	}
	if err := f.rowErrFor[achievementID]; err != nil {
		return false, err
	}
	if f.alreadyUnlocked[achievementID] {
		return false, nil
	}
	f.recordUnlock(userID, achievementID)
	return true, nil
}

func (f *fakeAchievementDao) InsertUnlocksWithUnseen(ctx *gin.Context, userID int64, achievementIDs []string, unlockedAt time.Time) ([]string, error) {
	if f.queriesErr != nil {
		return nil, f.queriesErr
	}
	if f.batchErr != nil {
		return nil, f.batchErr
	}
	inserted := make([]string, 0, len(achievementIDs))
	for _, id := range achievementIDs {
		if f.alreadyUnlocked[id] {
			continue
		}
		f.recordUnlock(userID, id)
		inserted = append(inserted, id)
	}
	return inserted, nil
}

// recordUnlock replica el alta atómica del DAO: el unseen solo existe para un
// unlock nuevo.
func (f *fakeAchievementDao) recordUnlock(userID int64, achievementID string) {
	f.alreadyUnlocked[achievementID] = true
	f.unlocks = append(f.unlocks, achievementID)
	f.unseen = append(f.unseen, dbs.GamificationUnseen{
		UserID: userID, Section: constants.GamificationSectionAchievements,
	})
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

func (f *fakeAchievementDao) CountFinishedSessionStats(ctx *gin.Context, userID int64) (daos.FinishedSessionStats, error) {
	if f.queriesErr != nil {
		return daos.FinishedSessionStats{}, f.queriesErr
	}
	return daos.FinishedSessionStats{Finished: f.finishedCount, Early: f.earlyCount}, nil
}

func (f *fakeAchievementDao) SumDistanceMeters(ctx *gin.Context, userID int64) (float64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.sumDistance, nil
}

func (f *fakeAchievementDao) SumAndMaxDistance(ctx *gin.Context, userID int64) (daos.DistanceStats, error) {
	if f.queriesErr != nil {
		return daos.DistanceStats{}, f.queriesErr
	}
	return daos.DistanceStats{Total: f.sumDistance, MaxSingle: f.maxSingle}, nil
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

func (f *fakeAchievementDao) CountEarlyFinishedSessions(ctx *gin.Context, userID int64) (int64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.earlyCount, nil
}

func (f *fakeAchievementDao) ListFinishedSessionStarts(ctx *gin.Context, userID int64) ([]time.Time, error) {
	panic("el servicio no debe traer el histórico: usa CountEarlyFinishedSessions")
}

func (f *fakeAchievementDao) CountAttendanceDays(ctx *gin.Context, userID int64) (int64, error) {
	if f.queriesErr != nil {
		return 0, f.queriesErr
	}
	return f.attendanceDays, nil
}

func (f *fakeAchievementDao) CountAttendanceDaysByUsers(ctx *gin.Context, userIDs []int64) (map[int64]int64, error) {
	if f.queriesErr != nil {
		return nil, f.queriesErr
	}
	counts := make(map[int64]int64, len(userIDs))
	for _, userID := range userIDs {
		counts[userID] = f.attendanceDays
	}
	return counts, nil
}

func newAchievementsTestService(dao *fakeAchievementDao, gamDao *mockGamificationDao, streaks StreakServiceInterface, tokens *fakePushTokenDAO, client *stubPushClient) AchievementsServiceInterface {
	return NewAchievementsService(dao, gamDao, streaks, tokens, client)
}

func TestAchievementsService_EarlyUnlock(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 5, earlyCount: 5}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	err := svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession)

	require.NoError(t, err)
	assert.Contains(t, dao.unlocks, "early_5")
}

func TestAchievementsService_EarlyNoAlcanza(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 5, earlyCount: 4}
	gamDao := &mockGamificationDao{}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	err := svc.EvaluateEvent(nil, 7, AchievementTriggerRunnerSession)

	require.NoError(t, err)
	assert.NotContains(t, dao.unlocks, "early_5")
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
	require.Len(t, dao.unseen, 3)
	for _, row := range dao.unseen {
		assert.Equal(t, "achievements", row.Section)
		assert.EqualValues(t, 7, row.UserID)
	}
}

func TestAchievementsService_ConSnapshotNoConsultaRachas(t *testing.T) {
	dao := &fakeAchievementDao{alreadyUnlocked: map[string]bool{}, finishedCount: 5}
	gamDao := &mockGamificationDao{}
	// panicStreakView explota en GetStreaks: el snapshot debe evitar la llamada.
	svc := newAchievementsTestService(dao, gamDao, panicStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	err := svc.EvaluateEventWithSnapshot(nil, 7, AchievementTriggerRunnerSession,
		&StreakSnapshot{StrictCurrent: 4, StrictBest: 6})

	require.NoError(t, err)
	// best strict 6 desbloquea streak_3 y no streak_7; 5 sesiones desbloquea
	// first_session y sessions_5.
	assert.Equal(t, []string{"first_session", "streak_3", "sessions_5"}, dao.unlocks)
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
	assert.Empty(t, dao.unseen)
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

	// Re-evaluación con el mismo agregado: idempotente, sin unlocks nuevos.
	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerGPSPoints))
	assert.Equal(t, []string{"gps_first", "gps_10"}, dao.unlocks)

	dao.gpsCount = 50
	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerGPSPoints))
	assert.Equal(t, []string{"gps_first", "gps_10", "gps_50"}, dao.unlocks)

	require.NoError(t, svc.EvaluateEvent(nil, 7, AchievementTriggerAttendance))
	assert.Equal(t, []string{"gps_first", "gps_10", "gps_50", "team_first"}, dao.unlocks)
}

func TestAchievementsService_BatchUnlock_SoloNuevosYOrdenDeCatalogo(t *testing.T) {
	dao := &fakeAchievementDao{
		alreadyUnlocked: map[string]bool{"sessions_5": true},
		finishedCount:   5, earlyCount: 5,
	}
	gamDao := &mockGamificationDao{}
	client := &stubPushClient{}
	tokens := &fakePushTokenDAO{tokens: []dbs.PushToken{{Token: "ExponentPushToken[abc]"}}}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, tokens, client)

	err := svc.EvaluateEventWithSnapshot(nil, 7, AchievementTriggerRunnerSession,
		&StreakSnapshot{StrictCurrent: 6, StrictBest: 6})
	require.NoError(t, err)

	assert.Equal(t, []string{"first_session", "streak_3", "early_5"}, dao.unlocks,
		"solo los insertados nuevos, en orden de catálogo")
	require.Len(t, dao.unseen, 3)
	require.Len(t, client.calls, 3)
	assert.Equal(t, []string{"Primer paso", "Racha de 3", "Madrugador"},
		[]string{client.calls[0].body, client.calls[1].body, client.calls[2].body})

	// Segunda evaluación: idempotente, sin filas nuevas, unseen ni push.
	dao.unseen = nil
	client.calls = nil
	require.NoError(t, svc.EvaluateEventWithSnapshot(nil, 7, AchievementTriggerRunnerSession,
		&StreakSnapshot{StrictCurrent: 6, StrictBest: 6}))
	assert.Equal(t, []string{"first_session", "streak_3", "early_5"}, dao.unlocks)
	assert.Empty(t, dao.unseen)
	assert.Empty(t, client.calls)
}

func TestAchievementsService_BatchError_FallbackBestEffort(t *testing.T) {
	dao := &fakeAchievementDao{
		alreadyUnlocked: map[string]bool{}, batchErr: errors.New("batch caído"),
		finishedCount: 5,
	}
	gamDao := &mockGamificationDao{}
	client := &stubPushClient{}
	tokens := &fakePushTokenDAO{tokens: []dbs.PushToken{{Token: "ExponentPushToken[abc]"}}}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, tokens, client)

	err := svc.EvaluateEventWithSnapshot(nil, 7, AchievementTriggerRunnerSession,
		&StreakSnapshot{StrictCurrent: 6, StrictBest: 6})
	require.NoError(t, err, "el fallback guarda todos los unlocks que puede")

	assert.Equal(t, []string{"first_session", "streak_3", "sessions_5"}, dao.unlocks)
	require.Len(t, dao.unseen, 3)
	assert.Len(t, client.calls, 3)
}

func TestAchievementsService_BatchError_FallbackParcialNoPierdeGuardables(t *testing.T) {
	dao := &fakeAchievementDao{
		alreadyUnlocked: map[string]bool{}, batchErr: errors.New("batch caído"),
		rowErrFor:     map[string]error{"streak_3": errors.New("fila rechazada")},
		finishedCount: 5,
	}
	gamDao := &mockGamificationDao{}
	client := &stubPushClient{}
	tokens := &fakePushTokenDAO{tokens: []dbs.PushToken{{Token: "ExponentPushToken[abc]"}}}
	svc := newAchievementsTestService(dao, gamDao, &fakeStreakView{}, tokens, client)

	err := svc.EvaluateEventWithSnapshot(nil, 7, AchievementTriggerRunnerSession,
		&StreakSnapshot{StrictCurrent: 6, StrictBest: 6})
	require.Error(t, err, "el fallo de una fila se reporta al caller")
	assert.Contains(t, err.Error(), "desbloqueo de logros")

	assert.Equal(t, []string{"first_session", "sessions_5"}, dao.unlocks,
		"las filas guardables sobreviven aunque el batch y una fila fallen")
	require.Len(t, dao.unseen, 2)
	assert.Len(t, client.calls, 2)

	// Reintento posterior: solo se inserta lo que había fallado, sin duplicar.
	dao.batchErr = nil
	dao.rowErrFor = nil
	dao.unseen = nil
	client.calls = nil
	require.NoError(t, svc.EvaluateEventWithSnapshot(nil, 7, AchievementTriggerRunnerSession,
		&StreakSnapshot{StrictCurrent: 6, StrictBest: 6}))
	assert.Equal(t, []string{"first_session", "sessions_5", "streak_3"}, dao.unlocks)
	assert.Len(t, dao.unseen, 1)
	assert.Len(t, client.calls, 1)
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
	require.Len(t, dao.unseen, len(dao.unlocks), "el unseen se persiste junto con cada unlock")
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

func (panicStreakView) ComputeSnapshot(ctx *gin.Context, targetUserID int64) (*StreakSnapshot, error) {
	panic("boom del cálculo de rachas en logros")
}

func TestAchievementsService_OnEvent_NilSafe(t *testing.T) {
	var svc *achievementsService
	require.NotPanics(t, func() { svc.OnEvent(nil, 7, AchievementTriggerRunnerSession) })

	// Con dao nil (y no nil receiver) tampoco dispara: módulo desactivado.
	off := &achievementsService{}
	require.NotPanics(t, func() { off.OnEvent(nil, 7, AchievementTriggerRunnerSession) })
}
