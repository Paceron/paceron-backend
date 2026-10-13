package services

// Leaderboard por team (gamificacion-core design.md D7).

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/gamification"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/utils/gametime"
)

var (
	ErrLeaderboardInvalid   = errors.New("parámetros de leaderboard inválidos")
	ErrLeaderboardForbidden = errors.New("no tenés acceso al leaderboard de este equipo")
)

// LeaderboardServiceInterface expone GET /teams/{id}/leaderboard: guard de
// caller (miembro de algún grupo del team O owner), whitelists de metric y
// period, y armado del ranking con desempates, viewer_rank y record.
type LeaderboardServiceInterface interface {
	GetLeaderboard(ctx *gin.Context, callerID, teamID int64, metric, period string, from, to *string) (*gamification.LeaderboardResponse, error)
}

type leaderboardService struct {
	dao daos.LeaderboardDAOInterface
}

func NewLeaderboardService(dao daos.LeaderboardDAOInterface) LeaderboardServiceInterface {
	return &leaderboardService{dao: dao}
}

// leaderboardEntry es la fila interna del ranking antes de resolver nombres.
type leaderboardEntry struct {
	userID int64
	value  float64
	// tiebreak: menos entrenamientos contribuyentes (D7); para sessions_count
	// y attendance_count es el instante específico del desempate (empezó/
	// registró más temprano su última actividad).
	tiebreak int64
	// achievedAt alimenta el record (period=all): la fecha en que la fila
	// alcanzó el valor.
	achievedAt *time.Time
}

func (s *leaderboardService) GetLeaderboard(ctx *gin.Context, callerID, teamID int64, metric, period string, from, to *string) (*gamification.LeaderboardResponse, error) {
	if err := validateLeaderboardQuery(metric, period, from, to); err != nil {
		return nil, err
	}
	rangeFrom, rangeTo, err := leaderboardRange(period, from, to)
	if err != nil {
		return nil, err
	}

	// Guard: miembro de algún grupo del team O owner (403 otro caso).
	isMember, err := s.dao.IsTeamGroupMember(ctx, teamID, callerID)
	if err != nil {
		return nil, fmt.Errorf("error al consultar el leaderboard")
	}
	if !isMember {
		isOwner, err := s.dao.IsTeamOwner(ctx, teamID, callerID)
		if err != nil {
			return nil, fmt.Errorf("error al consultar el leaderboard")
		}
		if !isOwner {
			return nil, ErrLeaderboardForbidden
		}
	}

	entries, err := s.aggregate(ctx, teamID, metric, rangeFrom, rangeTo)
	if err != nil {
		customlogger.Error(ctx, "error aggregating leaderboard", err, customlogger.TagMethod("GetLeaderboard"))
		return nil, fmt.Errorf("error al consultar el leaderboard")
	}
	if len(entries) == 0 {
		return &gamification.LeaderboardResponse{Rows: []gamification.LeaderboardRow{}, ViewerRank: nil}, nil
	}

	sortLeaderboardEntries(entries, metric)
	ranks := assignLeaderboardRanks(entries)

	resp := &gamification.LeaderboardResponse{Rows: []gamification.LeaderboardRow{}, ViewerRank: nil}

	usersByID, err := s.loadUsers(ctx, entries)
	if err != nil {
		return nil, fmt.Errorf("error al consultar el leaderboard")
	}

	top := gamification.LeaderboardTopSize
	if top > len(entries) {
		top = len(entries)
	}
	for _, e := range entries[:top] {
		resp.Rows = append(resp.Rows, gamification.LeaderboardRow{
			UserID: e.userID, Name: userNameByID(usersByID, e.userID),
			AvatarURL: userAvatarByID(usersByID, e.userID),
			Value:     e.value, Rank: ranks[e.userID],
		})
	}

	if e, ok := findLeaderboardEntry(entries, callerID); ok {
		resp.ViewerRank = &gamification.LeaderboardViewerRank{
			UserID: callerID, Value: e.value, Rank: ranks[callerID],
		}
	}

	if period == gamification.LeaderboardPeriodAll {
		resp.Record = buildLeaderboardRecord(entries, metric, usersByID)
	}
	return resp, nil
}

// aggregate corre UNA query de agregación según la métrica (D7: sin N+1) y
// mapea a entries con el desempate y el achieved_at de cada familia.
func (s *leaderboardService) aggregate(ctx *gin.Context, teamID int64, metric string, from, to *string) ([]leaderboardEntry, error) {
	switch metric {
	case gamification.LeaderboardMetricDistance, gamification.LeaderboardMetricDuration,
		gamification.LeaderboardMetricPace, gamification.LeaderboardMetricPeakSpeed:
		rows, err := s.dao.AggregateFeedback(ctx, teamID, from, to)
		if err != nil {
			return nil, err
		}
		return mapFeedbackAgg(rows, metric), nil
	case gamification.LeaderboardMetricSessions:
		rows, err := s.dao.AggregateRunnerSessions(ctx, teamID, from, to)
		if err != nil {
			return nil, err
		}
		return mapSessionsAgg(rows), nil
	case gamification.LeaderboardMetricAttendance:
		rows, err := s.dao.AggregateAttendances(ctx, teamID, from, to)
		if err != nil {
			return nil, err
		}
		return mapAttendanceAgg(rows), nil
	}
	return nil, fmt.Errorf("métrica sin agregación: %s", metric)
}

func mapFeedbackAgg(rows []dbs.LeaderboardFeedbackAgg, metric string) []leaderboardEntry {
	entries := make([]leaderboardEntry, 0, len(rows))
	for i := range rows {
		r := rows[i]
		e := leaderboardEntry{userID: r.UserID}
		switch metric {
		case gamification.LeaderboardMetricDistance:
			if r.DistanceCount == 0 {
				continue
			}
			e.value = derefFloat(r.SumDistance)
			e.tiebreak = r.DistanceCount
			e.achievedAt = r.DistanceLastDate
		case gamification.LeaderboardMetricDuration:
			if r.DurationCount == 0 {
				continue
			}
			e.value = float64(derefInt(r.SumDurationMs))
			e.tiebreak = r.DurationCount
			e.achievedAt = r.DurationLastDate
		case gamification.LeaderboardMetricPace:
			if r.PaceCount == 0 {
				continue
			}
			sumDuration := float64(derefInt(r.SumDurationMs))
			sumDistance := derefFloat(r.SumDistance)
			if sumDistance <= 0 || sumDuration <= 0 {
				continue
			}
			e.value = sumDuration / sumDistance
			e.tiebreak = r.PaceCount
			e.achievedAt = r.PaceLastDate
		case gamification.LeaderboardMetricPeakSpeed:
			if r.PeakCount == 0 || r.MaxPeak == nil {
				continue
			}
			e.value = *r.MaxPeak
			e.tiebreak = r.PeakCount
			e.achievedAt = r.PeakArgmaxDate
		}
		entries = append(entries, e)
	}
	return entries
}

func mapSessionsAgg(rows []dbs.LeaderboardSessionsAgg) []leaderboardEntry {
	entries := make([]leaderboardEntry, 0, len(rows))
	for i := range rows {
		e := leaderboardEntry{
			userID: rows[i].UserID,
			value:  float64(rows[i].SessionsCount),
			// Desempate específico: a igual cantidad, empezó antes (last_start ASC).
			tiebreak:   timeToNanos(rows[i].LastStart),
			achievedAt: rows[i].LastStart,
		}
		entries = append(entries, e)
	}
	return entries
}

func mapAttendanceAgg(rows []dbs.LeaderboardAttendanceAgg) []leaderboardEntry {
	entries := make([]leaderboardEntry, 0, len(rows))
	for i := range rows {
		e := leaderboardEntry{
			userID: rows[i].UserID,
			value:  float64(rows[i].AttendanceCount),
			// Desempate específico: a igual cantidad, registró antes su última asistencia.
			tiebreak:   timeToNanos(rows[i].LastRegisteredAt),
			achievedAt: rows[i].LastRegisteredAt,
		}
		entries = append(entries, e)
	}
	return entries
}

// sortLeaderboardEntries ordena por valor (desc; pace ASC porque menor es
// mejor) y desempata: métricas de feedback por MENOS actividades
// contribuyentes; sessions_count/attendance_count por su instante específico.
func sortLeaderboardEntries(entries []leaderboardEntry, metric string) {
	less := func(a, b leaderboardEntry) bool {
		if a.value != b.value {
			if metric == gamification.LeaderboardMetricPace {
				return a.value < b.value
			}
			return a.value > b.value
		}
		if a.tiebreak != b.tiebreak {
			return a.tiebreak < b.tiebreak
		}
		return a.userID < b.userID
	}
	sort.SliceStable(entries, func(i, j int) bool { return less(entries[i], entries[j]) })
}

// assignLeaderboardRanks asigna ranks de competencia: a igual valor exacto
// comparten posición y la siguiente saltea N (1,1,3...).
func assignLeaderboardRanks(entries []leaderboardEntry) map[int64]int {
	ranks := make(map[int64]int, len(entries))
	rank := 0
	for i := range entries {
		if i == 0 || entries[i].value != entries[i-1].value {
			rank = i + 1
		}
		ranks[entries[i].userID] = rank
	}
	return ranks
}

func findLeaderboardEntry(entries []leaderboardEntry, userID int64) (leaderboardEntry, bool) {
	for i := range entries {
		if entries[i].userID == userID {
			return entries[i], true
		}
	}
	return leaderboardEntry{}, false
}

// loadUsers resuelve nombre/avatar en batch (1 query IN).
func (s *leaderboardService) loadUsers(ctx *gin.Context, entries []leaderboardEntry) (map[int64]*dbs.User, error) {
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.userID)
	}
	users, err := s.dao.UsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*dbs.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	return byID, nil
}

// buildLeaderboardRecord arma el record de period=all: el mejor valor de la
// métrica (para pace el MÁS BAJO) y, a empate, el achieved_at más reciente.
func buildLeaderboardRecord(entries []leaderboardEntry, metric string, usersByID map[int64]*dbs.User) *gamification.LeaderboardRecord {
	best := entries[0]
	for i := 1; i < len(entries); i++ {
		e := entries[i]
		better := false
		if metric == gamification.LeaderboardMetricPace {
			better = e.value < best.value
		} else {
			better = e.value > best.value
		}
		if better {
			best = e
			continue
		}
		if e.value == best.value && laterAchieved(e.achievedAt, best.achievedAt) {
			best = e
		}
	}
	return &gamification.LeaderboardRecord{
		UserID: best.userID, Name: userNameByID(usersByID, best.userID), Value: best.value,
		AchievedAt: achievedAtString(best.achievedAt),
	}
}

func userNameByID(usersByID map[int64]*dbs.User, userID int64) string {
	if u := usersByID[userID]; u != nil {
		return u.Name
	}
	return ""
}

func userAvatarByID(usersByID map[int64]*dbs.User, userID int64) *string {
	if u := usersByID[userID]; u != nil {
		return buildMediaURL(u.PhotoKey, u.PhotoUpdatedAt)
	}
	return nil
}

func laterAchieved(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return a.After(*b)
}

func achievedAtString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func timeToNanos(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixNano()
}

func derefFloat(f *float64) float64 {
	if f == nil {
		return 0
	}
	return *f
}

func derefInt(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// validateLeaderboardQuery aplica las whitelists del D7 y las reglas de
// custom: ambos bordes presentes, formato YYYY-MM-DD y from <= to.
func validateLeaderboardQuery(metric, period string, from, to *string) error {
	validMetric := false
	for _, m := range gamification.GetValidLeaderboardMetrics() {
		if m == metric {
			validMetric = true
			break
		}
	}
	if !validMetric {
		return fmt.Errorf("%w: metric debe ser una de: distance, duration, pace, peak_speed, sessions_count, attendance_count", ErrLeaderboardInvalid)
	}
	validPeriod := false
	for _, p := range gamification.GetValidLeaderboardPeriods() {
		if p == period {
			validPeriod = true
			break
		}
	}
	if !validPeriod {
		return fmt.Errorf("%w: period debe ser una de: week, month, year, custom, all", ErrLeaderboardInvalid)
	}
	if period == gamification.LeaderboardPeriodCustom {
		if from == nil || to == nil {
			return fmt.Errorf("%w: period=custom exige from y to (YYYY-MM-DD)", ErrLeaderboardInvalid)
		}
		parsedFrom, errFrom := time.Parse("2006-01-02", *from)
		parsedTo, errTo := time.Parse("2006-01-02", *to)
		if errFrom != nil || errTo != nil {
			return fmt.Errorf("%w: from y to deben tener formato YYYY-MM-DD", ErrLeaderboardInvalid)
		}
		if parsedFrom.After(parsedTo) {
			return fmt.Errorf("%w: from no puede ser posterior a to", ErrLeaderboardInvalid)
		}
	}
	return nil
}

// leaderboardRange deriva el rango del período desde "hoy" en TZ Cordoba
// (D1/gametime). week/month/year son calendarios completos (los días futuros
// del período no traen datos, es solo la frontera del rango).
func leaderboardRange(period string, from, to *string) (*string, *string, error) {
	switch period {
	case gamification.LeaderboardPeriodAll:
		return nil, nil, nil
	case gamification.LeaderboardPeriodCustom:
		return from, to, nil
	}

	today := gametime.Today()
	var start, end time.Time
	switch period {
	case gamification.LeaderboardPeriodWeek:
		offset := (int(today.Weekday()) + 6) % 7 // lunes = 0
		start = today.AddDate(0, 0, -offset)
		end = start.AddDate(0, 0, 6)
	case gamification.LeaderboardPeriodMonth:
		start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, gametime.Location)
		end = start.AddDate(0, 1, -1)
	case gamification.LeaderboardPeriodYear:
		start = time.Date(today.Year(), 1, 1, 0, 0, 0, 0, gametime.Location)
		end = time.Date(today.Year(), 12, 31, 0, 0, 0, 0, gametime.Location)
	default:
		return nil, nil, fmt.Errorf("%w: period desconocido", ErrLeaderboardInvalid)
	}
	return periodDatePtr(start), periodDatePtr(end), nil
}

func periodDatePtr(d time.Time) *string {
	s := d.Format("2006-01-02")
	return &s
}
