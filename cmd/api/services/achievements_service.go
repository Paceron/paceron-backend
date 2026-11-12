package services

// Catálogo, merge de progreso y evaluadores de logros (gamificacion-core D5).

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	appconstants "simple-arq-golang/cmd/api/constants"
	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/restclients/expopushclient"
)

// AchievementTrigger identifica el hecho que dispara la re-evaluación: cada
// trigger re-evalúa SOLO los logros de su mapa de criterios.
type AchievementTrigger string

const (
	AchievementTriggerRunnerSession AchievementTrigger = "runner_session"
	AchievementTriggerFeedback      AchievementTrigger = "feedback"
	AchievementTriggerGPSPoints     AchievementTrigger = "gps_points"
	AchievementTriggerAttendance    AchievementTrigger = "attendance"
)

// AchievementsServiceInterface expone la vitrina (catálogo + progreso +
// estampas) y el core síncrono de evaluación que los hooks re-evalúan.
type AchievementsServiceInterface interface {
	GetCatalog(ctx *gin.Context) []gamification.AchievementResponse
	GetUserAchievements(ctx *gin.Context, targetUserID int64) (*gamification.UserAchievementsResponse, error)
	GetUserStamps(ctx *gin.Context, targetUserID int64) (*gamification.StampsResponse, error)
	EvaluateEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger) error
	// EvaluateEventBatch re-evalúa varios usuarios con una sola agregación
	// cuando el trigger la soporta (asistencia).
	EvaluateEventBatch(ctx *gin.Context, userIDs []int64, trigger AchievementTrigger) error
	// EvaluateEventWithSnapshot re-evalúa con el snapshot de rachas ya
	// calculado por el finish (D7); nil cae al cálculo propio.
	EvaluateEventWithSnapshot(ctx *gin.Context, userID int64, trigger AchievementTrigger, snapshot *StreakSnapshot) error
	// OnEvent encola la evaluación async: mismo hook que ejerce
	// AchievementsTriggerInterface (la instancia se comparte).
	OnEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger)
	// OnEventBatch encola una sola evaluación async por lote.
	OnEventBatch(ctx *gin.Context, userIDs []int64, trigger AchievementTrigger)
}

// AchievementsTriggerInterface es el contrato de hook nil-safe que llevan los
// controllers (mismo patrón que StreakTriggerServiceInterface en E1).
type AchievementsTriggerInterface interface {
	OnEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger)
	OnEventBatch(ctx *gin.Context, userIDs []int64, trigger AchievementTrigger)
	EvaluateEventWithSnapshot(ctx *gin.Context, userID int64, trigger AchievementTrigger, snapshot *StreakSnapshot) error
}

type achievementsService struct {
	achievementDao  daos.AchievementDAOInterface
	gamificationDao daos.GamificationDAOInterface
	streaks         StreakServiceInterface
	pushTokenDao    daos.PushTokenDaoInterface
	pushClient      expopushclient.ExpoPushClientInterface
}

func NewAchievementsService(
	achievementDao daos.AchievementDAOInterface,
	gamificationDao daos.GamificationDAOInterface,
	streaks StreakServiceInterface,
	pushTokenDao daos.PushTokenDaoInterface,
	pushClient expopushclient.ExpoPushClientInterface,
) AchievementsServiceInterface {
	return &achievementsService{
		achievementDao:  achievementDao,
		gamificationDao: gamificationDao,
		streaks:         streaks,
		pushTokenDao:    pushTokenDao,
		pushClient:      pushClient,
	}
}

// triggerAchievements mapea cada trigger al subset del catálogo que
// re-evalúa (task 3.4): mismo criterio de familia de queries por trigger.
var triggerAchievements = map[AchievementTrigger][]string{
	AchievementTriggerRunnerSession: {
		"first_session", "streak_3", "streak_7", "streak_14", "streak_30", "streak_60",
		"sessions_5", "sessions_25", "sessions_50", "sessions_100", "sessions_200", "early_5",
	},
	AchievementTriggerFeedback: {
		"distance_10k", "distance_50k", "distance_100k", "distance_42k_single",
	},
	AchievementTriggerGPSPoints: {
		"gps_first", "gps_10", "gps_50",
	},
	AchievementTriggerAttendance: {
		"team_first", "team_10",
	},
}

var catalogByID = func() map[string]appconstants.AchievementRow {
	m := make(map[string]appconstants.AchievementRow, len(appconstants.AchievementCatalog))
	for _, row := range appconstants.AchievementCatalog {
		m[row.ID] = row
	}
	return m
}()

func (s *achievementsService) GetCatalog(ctx *gin.Context) []gamification.AchievementResponse {
	out := make([]gamification.AchievementResponse, 0, len(appconstants.AchievementCatalog))
	for _, row := range appconstants.AchievementCatalog {
		out = append(out, toAchievementResponse(row))
	}
	return out
}

func toAchievementResponse(row appconstants.AchievementRow) gamification.AchievementResponse {
	return gamification.AchievementResponse{
		ID: row.ID, Family: row.Family, Name: row.Name,
		Description: row.Description, IconKey: row.IconKey,
		Threshold: row.Threshold, Unit: row.Unit,
	}
}

func (s *achievementsService) GetUserAchievements(ctx *gin.Context, targetUserID int64) (*gamification.UserAchievementsResponse, error) {
	progress, err := s.achievementDao.ListProgress(ctx, targetUserID)
	if err != nil {
		customlogger.Error(ctx, "error listing achievement progress", err, customlogger.TagMethod("GetUserAchievements"))
		return nil, fmt.Errorf("error al consultar logros")
	}
	unlocked := make(map[string]dbs.UserAchievementProgress, len(progress))
	for _, row := range progress {
		unlocked[row.AchievementID] = row
	}

	hasUnseen, err := s.gamificationDao.HasUnseenSection(ctx, targetUserID, constants.GamificationSectionAchievements)
	if err != nil {
		customlogger.Error(ctx, "error checking gamification unseen", err, customlogger.TagMethod("GetUserAchievements"))
		return nil, fmt.Errorf("error al consultar logros")
	}

	items := make([]gamification.UserAchievementItem, 0, len(appconstants.AchievementCatalog))
	for _, row := range appconstants.AchievementCatalog {
		var unlockedAt *time.Time
		if row, ok := unlocked[row.ID]; ok {
			when := row.UnlockedAt
			unlockedAt = &when
		}
		items = append(items, gamification.UserAchievementItem{
			AchievementID:  row.ID,
			UnlockedAt:     unlockedAt,
			LastUnlockedAt: unlockedAt,
		})
	}
	return &gamification.UserAchievementsResponse{Data: items, HasUnseen: hasUnseen}, nil
}

func (s *achievementsService) GetUserStamps(ctx *gin.Context, targetUserID int64) (*gamification.StampsResponse, error) {
	stamps, err := s.achievementDao.ListStamps(ctx, targetUserID)
	if err != nil {
		customlogger.Error(ctx, "error listing gamification stamps", err, customlogger.TagMethod("GetUserStamps"))
		return nil, fmt.Errorf("error al consultar estampas")
	}
	items := make([]gamification.StampItem, 0, len(stamps))
	for i := range stamps {
		items = append(items, gamification.StampItem{
			ID: stamps[i].ID, Type: stamps[i].Type, SourceID: stamps[i].SourceID,
			SourceName: stamps[i].SourceName, GrantedAt: stamps[i].GrantedAt, Count: stamps[i].Count,
		})
	}
	return &gamification.StampsResponse{Data: items}, nil
}

func (s *achievementsService) valuesFor(ctx *gin.Context, userID int64, trigger AchievementTrigger, snapshot *StreakSnapshot) (map[string]float64, error) {
	switch trigger {
	case AchievementTriggerRunnerSession:
		counts, err := s.achievementDao.CountFinishedSessionStats(ctx, userID)
		if err != nil {
			return nil, err
		}
		finished, early := counts.Finished, counts.Early
		var bestStrict int
		if snapshot != nil {
			bestStrict = snapshot.StrictCurrent
			if snapshot.StrictBest > bestStrict {
				bestStrict = snapshot.StrictBest
			}
		} else {
			streaks, err := s.streaks.GetStreaks(ctx, userID, 1)
			if err != nil {
				return nil, err
			}
			bestStrict = streaks.Strict.Current
			if streaks.Strict.Best > bestStrict {
				bestStrict = streaks.Strict.Best
			}
		}
		return map[string]float64{
			"first_session": float64(finished), "sessions_5": float64(finished),
			"sessions_25": float64(finished), "sessions_50": float64(finished),
			"sessions_100": float64(finished), "sessions_200": float64(finished),
			"streak_3": float64(bestStrict), "streak_7": float64(bestStrict),
			"streak_14": float64(bestStrict), "streak_30": float64(bestStrict),
			"streak_60": float64(bestStrict),
			"early_5":   float64(early),
		}, nil
	case AchievementTriggerFeedback:
		distances, err := s.achievementDao.SumAndMaxDistance(ctx, userID)
		if err != nil {
			return nil, err
		}
		totalMeters, maxMeters := distances.Total, distances.MaxSingle
		return map[string]float64{
			"distance_10k": totalMeters / 1000, "distance_50k": totalMeters / 1000,
			"distance_100k": totalMeters / 1000, "distance_42k_single": maxMeters / 1000,
		}, nil
	case AchievementTriggerGPSPoints:
		n, err := s.achievementDao.CountGPSFinishedSessions(ctx, userID)
		if err != nil {
			return nil, err
		}
		return map[string]float64{
			"gps_first": float64(n), "gps_10": float64(n), "gps_50": float64(n),
		}, nil
	case AchievementTriggerAttendance:
		days, err := s.achievementDao.CountAttendanceDays(ctx, userID)
		if err != nil {
			return nil, err
		}
		return map[string]float64{
			"team_first": float64(days), "team_10": float64(days),
		}, nil
	default:
		return map[string]float64{}, nil
	}
}

// EvaluateEvent es el core síncrono: re-evalúa el mapa de criterios del
// trigger contra los agregados frescos de la DB, inserta los desbloqueos
// (idempotente por PK compuesta), emite unseen por logro nuevo y push
// best-effort por cada desbloqueo. Stamps NO participan en E2 (solo E4).
func (s *achievementsService) EvaluateEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger) error {
	return s.evaluateEvent(ctx, userID, trigger, nil)
}

// EvaluateEventWithSnapshot evita releer el histórico de rachas cuando el
// finish ya lo calculó una vez para ambos hooks (D7).
func (s *achievementsService) EvaluateEventWithSnapshot(ctx *gin.Context, userID int64, trigger AchievementTrigger, snapshot *StreakSnapshot) error {
	return s.evaluateEvent(ctx, userID, trigger, snapshot)
}

func (s *achievementsService) evaluateEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger, snapshot *StreakSnapshot) error {
	ids, ok := triggerAchievements[trigger]
	if !ok {
		return nil
	}
	values, err := s.valuesFor(ctx, userID, trigger, snapshot)
	if err != nil {
		return fmt.Errorf("agregados de logros: %w", err)
	}
	return s.evaluateValues(ctx, userID, ids, values)
}

// EvaluateEventBatch re-evalúa varios usuarios sin multiplicar lecturas: para
// asistencia calcula los días distintos de todo el roster en una sola query.
func (s *achievementsService) EvaluateEventBatch(ctx *gin.Context, userIDs []int64, trigger AchievementTrigger) error {
	ids, ok := triggerAchievements[trigger]
	if !ok || len(userIDs) == 0 {
		return nil
	}
	if trigger != AchievementTriggerAttendance {
		return s.evaluateEventSequential(ctx, userIDs, trigger)
	}

	days, err := s.achievementDao.CountAttendanceDaysByUsers(ctx, userIDs)
	if err != nil {
		return fmt.Errorf("agregados de logros: %w", err)
	}

	var firstErr error
	for _, userID := range userIDs {
		n := days[userID]
		values := map[string]float64{"team_first": float64(n), "team_10": float64(n)}
		if err := s.evaluateValues(ctx, userID, ids, values); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// evaluateEventSequential preserva la semántica para triggers que todavía no
// tienen un agregado por lote.
func (s *achievementsService) evaluateEventSequential(ctx *gin.Context, userIDs []int64, trigger AchievementTrigger) error {
	var firstErr error
	for _, userID := range userIDs {
		if err := s.evaluateEvent(ctx, userID, trigger, nil); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *achievementsService) evaluateValues(ctx *gin.Context, userID int64, ids []string, values map[string]float64) error {
	// ON CONFLICT DO NOTHING: re-evaluar lo ya desbloqueado no inserta ni
	// cambia unlocked_at; solo la primera inserción genera unseen y push.
	now := time.Now()
	eligible := make([]string, 0, len(ids))
	for _, id := range ids {
		row, ok := catalogByID[id]
		if !ok {
			continue
		}
		value, ok := values[id]
		if !ok || value < row.Threshold {
			continue
		}
		eligible = append(eligible, id)
	}
	if len(eligible) == 0 {
		return nil
	}

	inserted, err := s.achievementDao.InsertUnlocksWithUnseen(ctx, userID, eligible, now)
	if err != nil {
		customlogger.Error(ctx, "batch de unlocks falló; reintentando por logro", err, customlogger.TagMethod("EvaluateEvent"))
		inserted, err = s.insertUnlocksFallback(ctx, userID, eligible, now)
	}

	// El orden del catálogo se impone acá: la inserción batch no garantiza
	// el orden de RETURNING y push debe salir en el orden original.
	insertedSet := make(map[string]struct{}, len(inserted))
	for _, id := range inserted {
		insertedSet[id] = struct{}{}
	}
	newlyUnlocked := make([]appconstants.AchievementRow, 0, len(insertedSet))
	for _, id := range eligible {
		if _, ok := insertedSet[id]; ok {
			newlyUnlocked = append(newlyUnlocked, catalogByID[id])
		}
	}
	if len(newlyUnlocked) > 0 {
		// El unseen ya quedó durable junto con cada unlock en el DAO. El push
		// sigue siendo best-effort: un fallo del proveedor no deshace nada.
		for _, row := range newlyUnlocked {
			sendPushToUser(ctx, s.pushTokenDao, s.pushClient, userID,
				"¡Logro desbloqueado!", row.Name, "achievement_unlocked", "/achievements")
		}
	}
	if err != nil {
		return fmt.Errorf("desbloqueo de logros: %w", err)
	}
	return nil
}

// insertUnlocksFallback conserva el mejor esfuerzo cuando el batch falla:
// reintenta logro por logro (idempotente, también atómico con su unseen) para
// no perder los que sí se pueden guardar, y devuelve solo los insertados junto
// con el primer error.
func (s *achievementsService) insertUnlocksFallback(ctx *gin.Context, userID int64, ids []string, unlockedAt time.Time) ([]string, error) {
	inserted := make([]string, 0, len(ids))
	var firstErr error
	for _, id := range ids {
		ok, err := s.achievementDao.InsertUnlockWithUnseen(ctx, userID, id, unlockedAt)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if ok {
			inserted = append(inserted, id)
		}
	}
	return inserted, firstErr
}

// run es la unidad que el goroutine ejecuta: panic-guard + log y nada más.
// Los tests la invocan directo para los caminos de error sin depender del
// scheduling del goroutine.
func (s *achievementsService) run(hookCtx *gin.Context, userID int64, trigger AchievementTrigger) {
	defer func() {
		if r := recover(); r != nil {
			customlogger.Error(hookCtx, "panic en trigger de logros", fmt.Errorf("%v", r), customlogger.TagMethod("AchievementsTrigger"))
		}
	}()
	if err := s.EvaluateEvent(hookCtx, userID, trigger); err != nil {
		customlogger.Error(hookCtx, "trigger de logros falló", err, customlogger.TagMethod("AchievementsTrigger"))
	}
}

// OnEvent encola la evaluación async post-write: nil-safe, panic-guard y con
// copia del contexto (el goroutine sobrevive a la respuesta).
func (s *achievementsService) OnEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger) {
	if s == nil || s.achievementDao == nil {
		return
	}
	hookCtx := ctx
	if ctx != nil {
		hookCtx = ctx.Copy()
	}
	go func() {
		s.run(hookCtx, userID, trigger)
	}()
}

// runBatch es la unidad del goroutine de lote: un solo trabajo async por
// request, panic-guard y sin concurrencia interna (limita presión al pool).
func (s *achievementsService) runBatch(hookCtx *gin.Context, userIDs []int64, trigger AchievementTrigger) {
	defer func() {
		if r := recover(); r != nil {
			customlogger.Error(hookCtx, "panic en trigger de logros en lote", fmt.Errorf("%v", r), customlogger.TagMethod("AchievementsTriggerBatch"))
		}
	}()
	if err := s.EvaluateEventBatch(hookCtx, userIDs, trigger); err != nil {
		customlogger.Error(hookCtx, "trigger de logros en lote falló", err, customlogger.TagMethod("AchievementsTriggerBatch"))
	}
}

// OnEventBatch encola una sola evaluación por lote, con copia del contexto y
// copia defensiva de los IDs (nil-safe como OnEvent).
func (s *achievementsService) OnEventBatch(ctx *gin.Context, userIDs []int64, trigger AchievementTrigger) {
	if s == nil || s.achievementDao == nil || len(userIDs) == 0 {
		return
	}
	hookCtx := ctx
	if ctx != nil {
		hookCtx = ctx.Copy()
	}
	users := append([]int64(nil), userIDs...)
	go func() {
		s.runBatch(hookCtx, users, trigger)
	}()
}
