package services

// --- Evaluación de asistencia en lote (optimizaciones-metricas D7) ---

import (
	"errors"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/testutils"
)

// perUserAttendanceDao agrega estado por usuario sobre el fake base: el
// evaluador individual y el batch deben terminar con los mismos unlocks.
type perUserAttendanceDao struct {
	*fakeAchievementDao
	days       map[int64]int64
	batchErr   error
	batchPanic bool
	batchCalls int
	unlocked   map[int64]map[string]bool
	unlocks    map[int64][]string
	unseen     map[int64]int
}

func newPerUserAttendanceDao(days map[int64]int64) *perUserAttendanceDao {
	return &perUserAttendanceDao{
		fakeAchievementDao: &fakeAchievementDao{},
		days:               days,
		unlocked:           map[int64]map[string]bool{},
		unlocks:            map[int64][]string{},
		unseen:             map[int64]int{},
	}
}

func (d *perUserAttendanceDao) CountAttendanceDays(ctx *gin.Context, userID int64) (int64, error) {
	return d.days[userID], nil
}

func (d *perUserAttendanceDao) CountAttendanceDaysByUsers(ctx *gin.Context, userIDs []int64) (map[int64]int64, error) {
	d.batchCalls++
	if d.batchPanic {
		panic("boom del agregado batch")
	}
	if d.batchErr != nil {
		return nil, d.batchErr
	}
	counts := make(map[int64]int64, len(userIDs))
	for _, userID := range userIDs {
		counts[userID] = d.days[userID]
	}
	return counts, nil
}

func (d *perUserAttendanceDao) InsertUnlocksWithUnseen(ctx *gin.Context, userID int64, achievementIDs []string, unlockedAt time.Time) ([]string, error) {
	if d.unlocked[userID] == nil {
		d.unlocked[userID] = map[string]bool{}
	}
	inserted := make([]string, 0, len(achievementIDs))
	for _, id := range achievementIDs {
		if d.unlocked[userID][id] {
			continue
		}
		d.unlocked[userID][id] = true
		d.unlocks[userID] = append(d.unlocks[userID], id)
		d.unseen[userID]++
		inserted = append(inserted, id)
	}
	return inserted, nil
}

func (d *perUserAttendanceDao) InsertUnlockWithUnseen(ctx *gin.Context, userID int64, achievementID string, unlockedAt time.Time) (bool, error) {
	inserted, err := d.InsertUnlocksWithUnseen(ctx, userID, []string{achievementID}, unlockedAt)
	return len(inserted) == 1, err
}

func TestAchievementsService_EvaluateEventBatch_AttendanceParidad(t *testing.T) {
	days := map[int64]int64{101: 1, 102: 10, 103: 0, 104: 9}
	users := []int64{101, 102, 103, 104}

	batchDao := newPerUserAttendanceDao(days)
	batchClient := &stubPushClient{}
	tokens := &fakePushTokenDAO{tokens: []dbs.PushToken{{Token: "ExponentPushToken[abc]"}}}
	batchSvc := NewAchievementsService(batchDao, &mockGamificationDao{}, &fakeStreakView{}, tokens, batchClient)

	require.NoError(t, batchSvc.EvaluateEventBatch(nil, users, AchievementTriggerAttendance))
	assert.Equal(t, 1, batchDao.batchCalls, "una sola agregación para todo el roster")
	assert.Equal(t, []string{"team_first"}, batchDao.unlocks[101])
	assert.Equal(t, []string{"team_first", "team_10"}, batchDao.unlocks[102])
	assert.Empty(t, batchDao.unlocks[103])
	assert.Equal(t, []string{"team_first"}, batchDao.unlocks[104])
	assert.Equal(t, map[int64]int{101: 1, 102: 2, 104: 1}, batchDao.unseen)

	individualDao := newPerUserAttendanceDao(days)
	individualClient := &stubPushClient{}
	individualSvc := NewAchievementsService(individualDao, &mockGamificationDao{}, &fakeStreakView{}, tokens, individualClient)
	for _, userID := range users {
		require.NoError(t, individualSvc.EvaluateEvent(nil, userID, AchievementTriggerAttendance))
	}
	assert.Equal(t, individualDao.unlocks, batchDao.unlocks, "mismos logros por usuario que la evaluación individual")
	assert.Equal(t, individualDao.unseen, batchDao.unseen)
	assert.Equal(t, len(individualClient.calls), len(batchClient.calls), "mismos pushes")

	// Re-evaluación idempotente: no duplica unlocks, unseen ni push.
	batchClient.calls = nil
	require.NoError(t, batchSvc.EvaluateEventBatch(nil, users, AchievementTriggerAttendance))
	assert.Equal(t, []string{"team_first", "team_10"}, batchDao.unlocks[102])
	assert.Equal(t, map[int64]int{101: 1, 102: 2, 104: 1}, batchDao.unseen)
	assert.Empty(t, batchClient.calls)
}

func TestAchievementsService_EvaluateEventBatch_ErrorAgregadoNoDesbloquea(t *testing.T) {
	dao := newPerUserAttendanceDao(map[int64]int64{101: 10})
	dao.batchErr = errors.New("agregado caído")
	svc := NewAchievementsService(dao, &mockGamificationDao{}, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{})

	err := svc.EvaluateEventBatch(nil, []int64{101, 102}, AchievementTriggerAttendance)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agregados de logros")
	assert.Empty(t, dao.unlocks)
}

func TestAchievementsService_OnEventBatch_NilSafeYRecover(t *testing.T) {
	var svc *achievementsService
	require.NotPanics(t, func() { svc.OnEventBatch(nil, []int64{101}, AchievementTriggerAttendance) })

	off := &achievementsService{}
	require.NotPanics(t, func() { off.OnEventBatch(nil, []int64{101}, AchievementTriggerAttendance) })

	dao := newPerUserAttendanceDao(map[int64]int64{101: 10})
	dao.batchPanic = true
	live := NewAchievementsService(dao, &mockGamificationDao{}, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{}).(*achievementsService)
	require.NotPanics(t, func() { live.runBatch(nil, []int64{101}, AchievementTriggerAttendance) })
}

func TestAchievementsService_EvaluateEventBatch_QueryCountConstante(t *testing.T) {
	db := testutils.SetupTestDB(t)
	queries := 0
	svc := NewAchievementsService(
		daos.NewAchievementDao(countingDB(t, db, &queries)),
		&mockGamificationDao{}, &fakeStreakView{}, &fakePushTokenDAO{}, &stubPushClient{},
	)

	require.NoError(t, svc.EvaluateEventBatch(nil, []int64{900001}, AchievementTriggerAttendance))
	assert.Equal(t, 1, queries, "roster chico: una lectura agrupada")

	queries = 0
	large := make([]int64, 40)
	for i := range large {
		large[i] = int64(900100 + i)
	}
	require.NoError(t, svc.EvaluateEventBatch(nil, large, AchievementTriggerAttendance))
	assert.Equal(t, 1, queries, "roster grande: misma lectura agrupada")
}
