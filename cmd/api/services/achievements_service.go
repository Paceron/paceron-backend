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
	"simple-arq-golang/cmd/api/utils/gametime"
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
	// OnEvent encola la evaluación async: mismo hook que ejerce
	// AchievementsTriggerInterface (la instancia se comparte).
	OnEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger)
}

// AchievementsTriggerInterface es el contrato de hook nil-safe que llevan los
// controllers (mismo patrón que StreakTriggerServiceInterface en E1).
type AchievementsTriggerInterface interface {
	OnEvent(ctx *gin.Context, userID int64, trigger AchievementTrigger)
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

// earlyStartHour es la frontera de early_5: started_at ANTES de las 08:00
// America/Argentina/Cordoba (D1/gametime), hora del instante en la TZ fija.
const earlyStartHour = 8

// countEarlyStarts es la agregación en Go de early_N: cada instancia con
// started_at antes de las 08:00 de Cordoba cuenta una vez.
func countEarlyStarts(starts []time.Time) int {
	n := 0
	for _, raw := range starts {
		local := raw.In(gametime.Location)
		if local.Hour() < earlyStartHour {
			n++
		}
	}
	return n
}

func (s *achievementsService) valuesFor(ctx *gin.Context, userID int64, trigger AchievementTrigger) (map[string]float64, error) {
	switch trigger {
	case AchievementTriggerRunnerSession:
		finished, err := s.achievementDao.CountFinishedSessions(ctx, userID)
		if err != nil {
			return nil, err
		}
		streaks, err := s.streaks.GetStreaks(ctx, userID, 1)
		if err != nil {
			return nil, err
		}
		bestStrict := streaks.Strict.Current
		if streaks.Strict.Best > bestStrict {
			bestStrict = streaks.Strict.Best
		}
		starts, err := s.achievementDao.ListFinishedSessionStarts(ctx, userID)
		if err != nil {
			return nil, err
		}
		return map[string]float64{
			"first_session": float64(finished), "sessions_5": float64(finished),
			"sessions_25": float64(finished), "sessions_50": float64(finished),
			"sessions_100": float64(finished), "sessions_200": float64(finished),
			"streak_3": float64(bestStrict), "streak_7": float64(bestStrict),
			"streak_14": float64(bestStrict), "streak_30": float64(bestStrict),
			"streak_60": float64(bestStrict),
			"early_5":   float64(countEarlyStarts(starts)),
		}, nil
	case AchievementTriggerFeedback:
		totalMeters, err := s.achievementDao.SumDistanceMeters(ctx, userID)
		if err != nil {
			return nil, err
		}
		maxMeters, err := s.achievementDao.MaxDistanceSingleSession(ctx, userID)
		if err != nil {
			return nil, err
		}
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
	ids, ok := triggerAchievements[trigger]
	if !ok {
		return nil
	}
	values, err := s.valuesFor(ctx, userID, trigger)
	if err != nil {
		return fmt.Errorf("agregados de logros: %w", err)
	}

	// ON CONFLICT DO NOTHING: re-evaluar lo ya desbloqueado no inserta ni
	// cambia unlocked_at; solo la primera inserción genera unseen y push.
	now := time.Now()
	var newlyUnlocked []appconstants.AchievementRow
	for _, id := range ids {
		row, ok := catalogByID[id]
		if !ok {
			continue
		}
		value, ok := values[id]
		if !ok || value < row.Threshold {
			continue
		}
		inserted, err := s.achievementDao.InsertUnlock(ctx, userID, id, now)
		if err != nil {
			return fmt.Errorf("desbloqueo de logros: %w", err)
		}
		if inserted {
			newlyUnlocked = append(newlyUnlocked, row)
		}
	}
	if len(newlyUnlocked) == 0 {
		return nil
	}

	// El fallo de unseen/push no deshace el desbloqueo (best-effort): la
	// vitrina es exacta; el punto rojo y el push son avisos derivables.
	unseen := make([]dbs.GamificationUnseen, 0, len(newlyUnlocked))
	for range newlyUnlocked {
		unseen = append(unseen, dbs.GamificationUnseen{UserID: userID, Section: constants.GamificationSectionAchievements})
	}
	if err := s.gamificationDao.InsertUnseen(ctx, unseen); err != nil {
		customlogger.Error(ctx, "error insertando unseen de logros", err, customlogger.TagMethod("EvaluateEvent"))
	}
	for _, row := range newlyUnlocked {
		sendPushToUser(ctx, s.pushTokenDao, s.pushClient, userID,
			"¡Logro desbloqueado!", row.Name, "achievement_unlocked", "/achievements")
	}
	return nil
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
