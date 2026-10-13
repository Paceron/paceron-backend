package services

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/utils/gametime"
)

// StreakServiceInterface expone el cálculo exacto de rachas (design.md D2).
type StreakServiceInterface interface {
	GetStreaks(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error)
}

type streakService struct {
	dao daos.GamificationDAOInterface
}

func NewStreakService(dao daos.GamificationDAOInterface) StreakServiceInterface {
	return &streakService{dao: dao}
}

// streakData guarda el histórico crudo traído por las 3 queries batch, para
// mapear por día en Go. La key es la fecha YYYY-MM-DD en TZ Cordoba; assigned
// trae el ID de instancia por entrenamiento asignado (nil = training sin
// instancia, que no puede completarse nunca); finished es el set de instancias
// con runner session finished del usuario.
type streakData struct {
	assigned map[string][]*int64
	finished map[int64]struct{}
}

func (d *streakData) finishedCountFor(key string) (finished, total int) {
	for _, id := range d.assigned[key] {
		total++
		if id != nil {
			if _, ok := d.finished[*id]; ok {
				finished++
			}
		}
	}
	return finished, total
}

func (d *streakData) strictStatus(date time.Time) string {
	finished, total := d.finishedCountFor(date.Format("2006-01-02"))
	if total == 0 {
		return gamification.DayStatusSkipped
	}
	if finished == total {
		return gamification.DayStatusCompleted
	}
	return gamification.DayStatusFailed
}

func (d *streakData) flexibleStatus(date time.Time) string {
	finished, total := d.finishedCountFor(date.Format("2006-01-02"))
	if total == 0 {
		return gamification.DayStatusSkipped
	}
	if finished > 0 {
		return gamification.DayStatusCompleted
	}
	return gamification.DayStatusFailed
}

// GetStreaks recalcula ambas rachas fresco del histórico (hoy nunca corta: la
// ventana de cálculo termina en AYER y el día de hoy se emite pending). La
// ventana days solo recorta el array; current/best van sobre todo el
// histórico hasta ayer. El GET no consulta gamification_streak_state.
func (s *streakService) GetStreaks(ctx *gin.Context, targetUserID int64, days int) (*gamification.StreaksResponse, error) {
	today := gametime.Today()
	windowStart := today.AddDate(0, 0, -(days - 1))
	yesterday := today.AddDate(0, 0, -1)

	resp := &gamification.StreaksResponse{
		Days: []gamification.StreakDay{},
	}

	hasUnseen, err := s.dao.HasUnseenSection(ctx, targetUserID, constants.GamificationSectionStreaks)
	if err != nil {
		customlogger.Error(ctx, "error checking gamification unseen", err, customlogger.TagMethod("GetStreaks"))
		return nil, fmt.Errorf("error al calcular rachas")
	}
	resp.HasUnseen = hasUnseen

	memberships, err := s.dao.FindActiveGroupMemberships(ctx, targetUserID)
	if err != nil {
		customlogger.Error(ctx, "error finding memberships for streaks", err, customlogger.TagMethod("GetStreaks"))
		return nil, fmt.Errorf("error al calcular rachas")
	}
	groupIDs := make([]int64, len(memberships))
	for i, m := range memberships {
		groupIDs[i] = m.GroupID
	}

	data := &streakData{assigned: map[string][]*int64{}, finished: map[int64]struct{}{}}
	earliest := time.Time{}
	if len(groupIDs) > 0 {
		if err := s.loadHistory(ctx, targetUserID, groupIDs, yesterday, data, &earliest); err != nil {
			return nil, err
		}
	}

	resp.Strict.Current = backwardStreak(data, earliest, yesterday, data.strictStatus)
	resp.Flexible.Current = backwardStreak(data, earliest, yesterday, data.flexibleStatus)
	resp.Strict.Best = bestStreak(data, earliest, yesterday, data.strictStatus)
	resp.Flexible.Best = bestStreak(data, earliest, yesterday, data.flexibleStatus)

	resp.Days = windowDays(data, windowStart, today)
	return resp, nil
}

// utcDateToDay lleva un instante escaneado de una columna date (medianoche
// UTC: el DSN del repo fija TimeZone=UTC) a la medianoche equivalente en TZ
// Cordoba, para que las keys del histórico coincidan con los días de la
// ventana (que se arman con gametime.Today()).
func utcDateToDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, gametime.Location)
}

// loadHistory corre las queries (b) y (c): días kind='training' de los
// grupos del usuario hasta ayer (rango abierto al pasado a propósito: best
// cubre TODO el histórico) y las runner sessions finished (batch por IN).
func (s *streakService) loadHistory(ctx *gin.Context, targetUserID int64, groupIDs []int64, yesterday time.Time, data *streakData, earliest *time.Time) error {
	trainingDays, err := s.dao.FindTrainingDaysUntil(ctx, groupIDs, yesterday)
	if err != nil {
		customlogger.Error(ctx, "error finding training days for streaks", err, customlogger.TagMethod("GetStreaks"))
		return fmt.Errorf("error al calcular rachas")
	}

	instanceIDs := make([]int64, 0, len(trainingDays))
	for i := range trainingDays {
		day := trainingDays[i]
		dayDate := utcDateToDay(day.Date)
		key := dayDate.Format("2006-01-02")
		data.assigned[key] = append(data.assigned[key], day.SessionInstanceID)
		if day.SessionInstanceID != nil {
			instanceIDs = append(instanceIDs, *day.SessionInstanceID)
		}
		if earliest.IsZero() || dayDate.Before(*earliest) {
			*earliest = dayDate
		}
	}

	if len(instanceIDs) == 0 {
		return nil
	}
	sessions, err := s.dao.FindFinishedRunnerSessions(ctx, instanceIDs, targetUserID)
	if err != nil {
		customlogger.Error(ctx, "error finding finished sessions for streaks", err, customlogger.TagMethod("GetStreaks"))
		return fmt.Errorf("error al calcular rachas")
	}
	for i := range sessions {
		data.finished[sessions[i].SessionInstanceID] = struct{}{}
	}
	return nil
}

// backwardStreak cuenta la racha que termina en ayer, retrocediendo desde
// ahí: corre con completed, corta con failed; skipped pasa de largo. Un hoy
// pending no interrumpe la racha porque el loop nunca llega a hoy.
func backwardStreak(data *streakData, earliest, yesterday time.Time, statusFor func(time.Time) string) int {
	if earliest.IsZero() {
		return 0
	}
	run := 0
	for d := yesterday; !d.Before(earliest); d = d.AddDate(0, 0, -1) {
		st := statusFor(d)
		if st == gamification.DayStatusFailed {
			break
		}
		if st == gamification.DayStatusCompleted {
			run++
		}
	}
	return run
}

func bestStreak(data *streakData, earliest, yesterday time.Time, statusFor func(time.Time) string) int {
	if earliest.IsZero() {
		return 0
	}
	run, best := 0, 0
	for d := earliest; !d.After(yesterday); d = d.AddDate(0, 0, 1) {
		switch statusFor(d) {
		case gamification.DayStatusCompleted:
			run++
			if run > best {
				best = run
			}
		case gamification.DayStatusFailed:
			run = 0
		}
	}
	return best
}

// windowDays emite el array del query days: desde hoy-(days-1) hasta HOY
// inclusive, con hoy siempre pending (nuestro cálculo termina en ayer).
// Días sin datos del histórico salen skipped.
func windowDays(data *streakData, windowStart, today time.Time) []gamification.StreakDay {
	days := make([]gamification.StreakDay, 0, int(today.Sub(windowStart).Hours()/24)+1)
	for d := windowStart; !d.After(today); d = d.AddDate(0, 0, 1) {
		item := gamification.StreakDay{
			Date:           d.Format("2006-01-02"),
			StrictStatus:   gamification.DayStatusSkipped,
			FlexibleStatus: gamification.DayStatusSkipped,
		}
		if !d.Before(today) {
			item.StrictStatus = gamification.DayStatusPending
			item.FlexibleStatus = gamification.DayStatusPending
		} else {
			item.StrictStatus = data.strictStatus(d)
			item.FlexibleStatus = data.flexibleStatus(d)
		}
		days = append(days, item)
	}
	return days
}
