package services

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

// Errores de la comparación planificado vs realizado. El controller los mapea
// a 400, 403 y 404.
var (
	ErrComparisonInvalidQuery = errors.New("parámetros de comparación inválidos")
	ErrComparisonForbidden    = errors.New("no tenés acceso a esta comparación")
	ErrComparisonNotFound     = errors.New("sesión no encontrada")
)

// maxComparisonRangeDays es el tope del rango de la lista (design.md D7).
const maxComparisonRangeDays = 366

const comparisonDateLayout = "2006-01-02"

// ActivityComparisonServiceInterface compara lo planificado con lo realizado
// por un atleta (change comparacion-planificado-realizado).
type ActivityComparisonServiceInterface interface {
	GetSessionComparison(ctx *gin.Context, callerID, sessionInstanceID int64, athleteUserID *int64) (*comparison.SessionComparisonResponse, error)
	ListComparisons(ctx *gin.Context, callerID int64, query comparison.ListQuery) (*comparison.ComparisonListResponse, error)
}

type activityComparisonService struct {
	dao         daos.ActivityComparisonDaoInterface
	instanceDao daos.SessionInstanceDaoInterface
	now         func() time.Time
}

func NewActivityComparisonService(dao daos.ActivityComparisonDaoInterface, instanceDao daos.SessionInstanceDaoInterface) ActivityComparisonServiceInterface {
	return &activityComparisonService{dao: dao, instanceDao: instanceDao, now: time.Now}
}

func (s *activityComparisonService) GetSessionComparison(ctx *gin.Context, callerID, sessionInstanceID int64, athleteUserID *int64) (*comparison.SessionComparisonResponse, error) {
	athleteID := callerID
	if athleteUserID != nil {
		athleteID = *athleteUserID
	}

	day, err := s.dao.FindSessionDay(ctx, sessionInstanceID)
	if err != nil {
		return nil, err
	}
	if day == nil {
		return nil, ErrComparisonNotFound
	}
	if err := s.checkSessionAccess(ctx, callerID, athleteID, sessionInstanceID); err != nil {
		return nil, err
	}

	ids := []int64{sessionInstanceID}
	exercises, err := s.dao.ListPlannedExercises(ctx, ids)
	if err != nil {
		return nil, err
	}
	feedback, err := s.dao.ListFeedback(ctx, athleteID, ids)
	if err != nil {
		return nil, err
	}
	runnerSessions, err := s.dao.ListRunnerSessions(ctx, athleteID, ids)
	if err != nil {
		return nil, err
	}

	// Una instancia sin día toma la fecha del feedback (D8).
	if day.GroupID == nil {
		if date, ok := earliestSessionDate(feedback); ok {
			day.Date = date
		}
	}

	calc := compareSession(exercises, feedback, athleteID)
	header := buildHeader(*day, calc.kinds, runnerSessionFor(runnerSessions, sessionInstanceID))
	resp := &comparison.SessionComparisonResponse{
		SessionHeader: header,
		AthleteUserID: athleteID,
		Exercises:     calc.exercises,
		Totals:        calc.totals,
		CompliancePct: calc.compliancePct,
	}
	resp.StartedAt, resp.EndedAt = sessionSchedule(*day, runnerSessionFor(runnerSessions, sessionInstanceID))

	customlogger.Info(ctx, "session comparison built",
		customlogger.TagMethod("GetSessionComparison"),
		customlogger.Tag("session_instance_id", fmt.Sprintf("%d", sessionInstanceID)))
	return resp, nil
}

// checkSessionAccess aplica la regla de acceso al detalle (design.md D3).
func (s *activityComparisonService) checkSessionAccess(ctx *gin.Context, callerID, athleteID, sessionInstanceID int64) error {
	if athleteID == callerID {
		ok, err := s.instanceDao.HasInstanceAccess(ctx, sessionInstanceID, callerID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrComparisonForbidden
		}
		return nil
	}

	owner, err := s.dao.IsSessionTeamOwner(ctx, sessionInstanceID, callerID)
	if err != nil {
		return err
	}
	if !owner {
		return ErrComparisonForbidden
	}
	athleteAccess, err := s.instanceDao.HasInstanceAccess(ctx, sessionInstanceID, athleteID)
	if err != nil {
		return err
	}
	if !athleteAccess {
		return ErrComparisonForbidden
	}
	return nil
}

func (s *activityComparisonService) ListComparisons(ctx *gin.Context, callerID int64, query comparison.ListQuery) (*comparison.ComparisonListResponse, error) {
	from, to, err := s.parseRange(query.DateFrom, query.DateTo)
	if err != nil {
		return nil, err
	}
	if query.Kind != "" && !constants.IsValidExerciseKind(query.Kind) {
		return nil, fmt.Errorf("%w: kind desconocido", ErrComparisonInvalidQuery)
	}
	if query.GroupID != nil && query.TeamID == nil {
		return nil, fmt.Errorf("%w: group_id requiere team_id", ErrComparisonInvalidQuery)
	}

	athleteID := callerID
	if query.AthleteUserID != nil && *query.AthleteUserID != callerID {
		if query.TeamID == nil {
			return nil, fmt.Errorf("%w: team_id es obligatorio para consultar a otro atleta", ErrComparisonInvalidQuery)
		}
		member, err := s.dao.IsTeamMemberOfOwner(ctx, *query.TeamID, *query.AthleteUserID, callerID)
		if err != nil {
			return nil, err
		}
		if !member {
			return nil, ErrComparisonForbidden
		}
		athleteID = *query.AthleteUserID
	}

	filter := daos.ComparisonRangeFilter{AthleteUserID: athleteID, From: from, To: to, TeamID: query.TeamID, GroupID: query.GroupID}
	sessions, err := s.collectSessions(ctx, filter)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(sessions))
	for _, sess := range sessions {
		ids = append(ids, sess.SessionInstanceID)
	}
	exercises, err := s.dao.ListPlannedExercises(ctx, ids)
	if err != nil {
		return nil, err
	}
	feedback, err := s.dao.ListFeedback(ctx, athleteID, ids)
	if err != nil {
		return nil, err
	}
	runnerSessions, err := s.dao.ListRunnerSessions(ctx, athleteID, ids)
	if err != nil {
		return nil, err
	}

	exercisesBySession := make(map[int64][]daos.ComparisonPlannedExercise)
	for _, ex := range exercises {
		exercisesBySession[ex.SessionInstanceID] = append(exercisesBySession[ex.SessionInstanceID], ex)
	}
	feedbackBySession := make(map[int64][]dbs.WorkoutFeedback)
	for _, fb := range feedback {
		feedbackBySession[fb.AssignedSessionID] = append(feedbackBySession[fb.AssignedSessionID], fb)
	}

	resp := &comparison.ComparisonListResponse{
		DateFrom: from.Format(comparisonDateLayout),
		DateTo:   to.Format(comparisonDateLayout),
		Sessions: make([]comparison.SessionComparisonSummary, 0, len(sessions)),
	}
	today := s.now().In(argentinaTZ).Format(comparisonDateLayout)
	complianceSum := 0.0
	for _, sess := range sessions {
		calc := compareSession(exercisesBySession[sess.SessionInstanceID], feedbackBySession[sess.SessionInstanceID], athleteID)
		if query.Kind != "" && !containsString(calc.kinds, query.Kind) {
			continue
		}
		header := buildHeader(sess, calc.kinds, runnerSessionFor(runnerSessions, sess.SessionInstanceID))
		counts := countsForAverage(header, today)
		resp.Sessions = append(resp.Sessions, comparison.SessionComparisonSummary{
			SessionHeader:    header,
			SessionTotals:    calc.totals,
			CompliancePct:    calc.compliancePct,
			CountsForAverage: counts,
		})
		if counts {
			complianceSum += calc.compliancePct
			resp.Summary.SessionsAveraged++
		}
		if calc.totals.CompletedSets > 0 {
			resp.Summary.SessionsWithActivity++
		}
	}
	resp.Summary.SessionsPlanned = len(resp.Sessions)
	if resp.Summary.SessionsAveraged > 0 {
		resp.Summary.AvgCompliancePct = round1(complianceSum / float64(resp.Summary.SessionsAveraged))
	}
	return resp, nil
}

// countsForAverage dice si una sesión entra en el promedio del rango (design.md
// D9): las de días anteriores a hoy siempre, aunque no se hayan ejecutado; las
// de hoy solo si ya terminaron, para no bajar el promedio con algo que el
// corredor todavía puede hacer.
func countsForAverage(header comparison.SessionHeader, today string) bool {
	return header.Date < today || header.Status == runnersession.RunnerSessionStatusFinished
}

// collectSessions junta los días planificados con las sesiones que solo
// aparecen por el feedback (design.md D7), sin repetir instancias.
func (s *activityComparisonService) collectSessions(ctx *gin.Context, filter daos.ComparisonRangeFilter) ([]daos.ComparisonSessionDay, error) {
	planned, err := s.dao.ListPlannedDays(ctx, filter)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(planned))
	sessions := make([]daos.ComparisonSessionDay, 0, len(planned))
	for _, day := range planned {
		if seen[day.SessionInstanceID] {
			continue
		}
		seen[day.SessionInstanceID] = true
		sessions = append(sessions, day)
	}
	// Con filtro de grupo solo cuentan los días de ese grupo.
	if filter.GroupID != nil {
		return sessions, nil
	}

	fromFeedback, err := s.dao.ListFeedbackSessions(ctx, filter)
	if err != nil {
		return nil, err
	}
	for _, day := range fromFeedback {
		if seen[day.SessionInstanceID] {
			continue
		}
		seen[day.SessionInstanceID] = true
		sessions = append(sessions, day)
	}
	sortSessionDays(sessions)
	return sessions, nil
}

// parseRange valida el rango de la lista: obligatorio, ordenado, de 366 días
// como máximo y sin pasar de hoy en hora argentina.
func (s *activityComparisonService) parseRange(rawFrom, rawTo string) (time.Time, time.Time, error) {
	if rawFrom == "" || rawTo == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_from y date_to son obligatorios", ErrComparisonInvalidQuery)
	}
	from, err := time.ParseInLocation(comparisonDateLayout, rawFrom, argentinaTZ)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_from debe tener formato YYYY-MM-DD", ErrComparisonInvalidQuery)
	}
	to, err := time.ParseInLocation(comparisonDateLayout, rawTo, argentinaTZ)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_to debe tener formato YYYY-MM-DD", ErrComparisonInvalidQuery)
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_from no puede ser posterior a date_to", ErrComparisonInvalidQuery)
	}
	if to.Sub(from) > maxComparisonRangeDays*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: el rango no puede superar %d días", ErrComparisonInvalidQuery, maxComparisonRangeDays)
	}
	today := s.now().In(argentinaTZ).Format(comparisonDateLayout)
	if rawTo > today {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_to no puede ser posterior a hoy", ErrComparisonInvalidQuery)
	}
	return from, to, nil
}

// buildHeader arma la cabecera común del detalle y de la lista.
func buildHeader(day daos.ComparisonSessionDay, kinds []string, rs *dbs.RunnerSession) comparison.SessionHeader {
	status := comparison.SessionStatusNone
	if rs != nil {
		status = rs.Status
	}
	return comparison.SessionHeader{
		SessionInstanceID: day.SessionInstanceID,
		Date:              day.Date.Format(comparisonDateLayout),
		SessionName:       day.SessionName,
		TeamID:            day.TeamID,
		TeamName:          day.TeamName,
		GroupID:           day.GroupID,
		GroupName:         day.GroupName,
		IsPresencial:      day.IsPresencial,
		Kinds:             kinds,
		Status:            status,
	}
}

// sessionSchedule devuelve el horario de la sesión: el del runner_session si
// existe, si no el presencial del día (D8). Formato RFC 3339 en UTC.
func sessionSchedule(day daos.ComparisonSessionDay, rs *dbs.RunnerSession) (*string, *string) {
	if rs != nil {
		start := rs.StartDate.UTC().Format(time.RFC3339)
		var end *string
		if rs.EndDate != nil {
			formatted := rs.EndDate.UTC().Format(time.RFC3339)
			end = &formatted
		}
		return &start, end
	}
	return formatOptionalTime(day.PresencialTimeFrom), formatOptionalTime(day.PresencialTimeTo)
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}

func runnerSessionFor(rows []dbs.RunnerSession, sessionInstanceID int64) *dbs.RunnerSession {
	for i := range rows {
		if rows[i].SessionInstanceID == sessionInstanceID {
			return &rows[i]
		}
	}
	return nil
}

func earliestSessionDate(feedback []dbs.WorkoutFeedback) (time.Time, bool) {
	var earliest time.Time
	found := false
	for _, fb := range feedback {
		if !found || fb.SessionDate.Before(earliest) {
			earliest = fb.SessionDate
			found = true
		}
	}
	return earliest, found
}

func sortSessionDays(days []daos.ComparisonSessionDay) {
	sort.SliceStable(days, func(i, j int) bool {
		if !days[i].Date.Equal(days[j].Date) {
			return days[i].Date.Before(days[j].Date)
		}
		return days[i].SessionInstanceID < days[j].SessionInstanceID
	})
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
