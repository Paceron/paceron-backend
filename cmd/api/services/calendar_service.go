package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/instance"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrCalendarGroupNotFound           = errors.New("grupo no encontrado")
	ErrCalendarForbidden               = errors.New("no autorizado")
	ErrCalendarInvalidKind             = errors.New("kind inválido")
	ErrCalendarFieldMismatch           = errors.New("combinación de campos inválida para el kind del día")
	ErrCalendarInvalidCancelTransition = errors.New("solo se puede cancelar un día que está en training")
	ErrCalendarPlanNotFound            = errors.New("plan no encontrado")
	ErrCalendarPlanForbidden           = errors.New("el plan no pertenece al entrenador dueño del grupo")
	ErrCalendarStampConflict           = errors.New("hay fechas con contenido existente")
	ErrCalendarShiftCollision          = errors.New("el corrimiento haría chocar dos fechas")
	ErrCalendarDayClosed               = errors.New("el día de calendario está cerrado")
	ErrCalendarSessionNotFound         = errors.New("sesión de catálogo no encontrada")
	ErrCalendarUserMismatch            = errors.New("no podés consultar los datos de otro usuario")
	ErrCalendarInvalidTimeFormat       = errors.New("presencial_time_from/presencial_time_to deben tener formato HH:MM")
	ErrCalendarInvalidTimeRange        = errors.New("presencial_time_to debe ser posterior a presencial_time_from")
	ErrCalendarTrainingWithoutInstance = errors.New("los días indicados no tienen una sesión instanciada que conservar")
	ErrCalendarInvalidDate             = errors.New("exclude_dates debe tener formato YYYY-MM-DD")
	ErrCalendarPresencialCollision     = errors.New("colisión presencial con otro equipo")
)

// CalendarServiceInterface reúne las operaciones de lectura y escritura del
// calendario; las escrituras comparten las transacciones del service.
type CalendarServiceInterface interface {
	GetRange(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error)
	UpsertDay(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error)
	DeleteDay(ctx *gin.Context, groupID, callerID int64, date time.Time) error
	Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error)
	Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error)
	BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error
	Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error)
	NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error)
	CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error)
}

type calendarClosedDaysError struct {
	dates []string
}

func (e *calendarClosedDaysError) Error() string {
	return fmt.Sprintf("%s: %s", ErrCalendarDayClosed, strings.Join(e.dates, ", "))
}

func (e *calendarClosedDaysError) Unwrap() error { return ErrCalendarDayClosed }

func newCalendarClosedDaysError(dates []string) error {
	if len(dates) == 0 {
		return ErrCalendarDayClosed
	}
	return &calendarClosedDaysError{dates: dates}
}

type calendarTrainingWithoutInstanceError struct {
	dates []string
}

func (e *calendarTrainingWithoutInstanceError) Error() string {
	return fmt.Sprintf("%s: %s", ErrCalendarTrainingWithoutInstance, strings.Join(e.dates, ", "))
}

func (e *calendarTrainingWithoutInstanceError) Unwrap() error {
	return ErrCalendarTrainingWithoutInstance
}

func newCalendarTrainingWithoutInstanceError(dates []string) error {
	if len(dates) == 0 {
		return ErrCalendarTrainingWithoutInstance
	}
	return &calendarTrainingWithoutInstanceError{dates: dates}
}

func isCalendarValidationError(err error) bool {
	for _, target := range []error{
		ErrCalendarInvalidKind, ErrCalendarFieldMismatch, ErrCalendarInvalidCancelTransition,
		ErrCalendarInvalidTimeFormat, ErrCalendarInvalidTimeRange, ErrCalendarSessionNotFound,
		ErrSessionExerciseNotFound, ErrCalendarTrainingWithoutInstance,
	} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

type calendarStampConflictError struct {
	dates []string
}

func (e *calendarStampConflictError) Error() string {
	return fmt.Sprintf("%s: %s", ErrCalendarStampConflict, strings.Join(e.dates, ", "))
}

func (e *calendarStampConflictError) Unwrap() error { return ErrCalendarStampConflict }

type calendarPresencialCollisionError struct {
	conflicts []calendar.PresencialConflict
}

func (e *calendarPresencialCollisionError) Error() string {
	parts := make([]string, 0, len(e.conflicts))
	for _, c := range e.conflicts {
		parts = append(parts, fmt.Sprintf("%s grupo %s (%s) %s-%s", c.Date, c.GroupName, c.TeamName, c.PresencialTimeFrom, c.PresencialTimeTo))
	}
	return fmt.Sprintf("%s: %s", ErrCalendarPresencialCollision, strings.Join(parts, "; "))
}

func (e *calendarPresencialCollisionError) Unwrap() error { return ErrCalendarPresencialCollision }

func newCalendarPresencialCollisionError(conflicts []calendar.PresencialConflict) error {
	if len(conflicts) == 0 {
		return ErrCalendarPresencialCollision
	}
	return &calendarPresencialCollisionError{conflicts: conflicts}
}

// PresencialCollisionConflicts extrae la lista de conflictos de un error de
// colisión presencial (para el body JSON del 409 en el controller).
func PresencialCollisionConflicts(err error) []calendar.PresencialConflict {
	var typed *calendarPresencialCollisionError
	if errors.As(err, &typed) {
		return typed.conflicts
	}
	return nil
}

func presencialTimeHHMM(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("15:04")
}

func sameCalendarDate(a, b time.Time) bool {
	ya, ma, da := a.Date()
	yb, mb, db := b.Date()
	return ya == yb && ma == mb && da == db
}

// presencialRangesOverlap aplica el overlap medio-abierto de design.md D1:
// colisión sii fromA < toB && fromB < toA (los bordes que se tocan no chocan;
// los strings "HH:MM" comparan lexicográficamente igual que el reloj).
func presencialRangesOverlap(fromA, toA, fromB, toB string) bool {
	return fromA < toB && fromB < toA
}

// findPresencialCollisions detecta colisiones presenciales (D3): días
// training+presencial de TODOS los grupos activos de los equipos del owner en
// las fechas pedidas, cruzados contra los días candidatos que se van a
// escribir. Cada colisionante se clasifica relative al/los grupo(s) escrito(s)
// por team: equipo distinto → cross (bloqueante); mismo equipo → same
// (warnings). Corre contra el db/tx que recibe (evita TOCTOU básico en
// escrituras transaccionales).
func (s *calendarService) findPresencialCollisions(
	ctx *gin.Context,
	db *gorm.DB,
	ownerID int64,
	excludeGroupID *int64,
	excludeDayIDs []int64,
	dates []time.Time,
	candidates []dbs.GroupCalendarDay,
) ([]calendar.PresencialConflict, []calendar.PresencialConflict, error) {
	groups, err := s.groupDao.FindByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, nil, fmt.Errorf("error al buscar grupos del owner")
	}
	groupByID := make(map[int64]dbs.Group, len(groups))
	groupIDs := make([]int64, 0, len(groups))
	for _, g := range groups {
		groupByID[g.ID] = g
		groupIDs = append(groupIDs, g.ID)
	}
	teams, err := s.teamDao.GetAllByOwnerID(ctx, ownerID)
	if err != nil {
		return nil, nil, fmt.Errorf("error al buscar equipos del owner")
	}
	teamNameByID := make(map[int64]string, len(teams))
	for _, t := range teams {
		teamNameByID[t.ID] = t.Name
	}

	existing, err := daos.NewGroupCalendarDayDao(db).FindPresencialForGroupsInRange(ctx, groupIDs, dates)
	if err != nil {
		return nil, nil, fmt.Errorf("error al buscar días presenciales")
	}

	excludedDaySet := make(map[int64]struct{}, len(excludeDayIDs))
	for _, id := range excludeDayIDs {
		excludedDaySet[id] = struct{}{}
	}

	var cross, same []calendar.PresencialConflict
	seen := make(map[string]struct{})
	for _, day := range existing {
		if _, skip := excludedDaySet[day.ID]; skip {
			continue
		}
		if excludeGroupID != nil && day.GroupID == *excludeGroupID {
			continue
		}
		dayGroup, ok := groupByID[day.GroupID]
		if !ok {
			continue
		}
		dayFrom := presencialTimeHHMM(day.PresencialTimeFrom)
		dayTo := presencialTimeHHMM(day.PresencialTimeTo)
		for _, cand := range candidates {
			if cand.GroupID == day.GroupID {
				continue
			}
			if !cand.IsPresencial || cand.Kind != string(constants.GroupCalendarDayKindTraining) {
				continue
			}
			if !sameCalendarDate(cand.Date, day.Date) {
				continue
			}
			candGroup, ok := groupByID[cand.GroupID]
			if !ok {
				continue
			}
			candFrom := presencialTimeHHMM(cand.PresencialTimeFrom)
			candTo := presencialTimeHHMM(cand.PresencialTimeTo)
			if !presencialRangesOverlap(candFrom, candTo, dayFrom, dayTo) {
				continue
			}
			conflict := calendar.PresencialConflict{
				GroupID:            day.GroupID,
				GroupName:          dayGroup.Name,
				TeamID:             dayGroup.TeamID,
				TeamName:           teamNameByID[dayGroup.TeamID],
				Date:               day.Date.Format("2006-01-02"),
				PresencialTimeFrom: dayFrom,
				PresencialTimeTo:   dayTo,
			}
			key := fmt.Sprintf("%d|%s|%s|%s", conflict.GroupID, conflict.Date, conflict.PresencialTimeFrom, conflict.PresencialTimeTo)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			if candGroup.TeamID != dayGroup.TeamID {
				cross = append(cross, conflict)
			} else {
				same = append(same, conflict)
			}
		}
	}
	return cross, same, nil
}

type calendarService struct {
	calendarDao     daos.GroupCalendarDaoInterface
	groupDao        daos.GroupDaoInterface
	teamDao         daos.TeamDaoInterface
	groupUserDao    daos.GroupUserDaoInterface
	teamUserDao     daos.TeamUserDaoInterface
	trainingPlanDao daos.TrainingPlanDaoInterface
	planDayDao      daos.PlanDayDaoInterface
	sessionDao      daos.SessionDaoInterface
	db              *gorm.DB
}

func NewCalendarService(
	calendarDao daos.GroupCalendarDaoInterface,
	groupDao daos.GroupDaoInterface,
	teamDao daos.TeamDaoInterface,
	groupUserDao daos.GroupUserDaoInterface,
	teamUserDao daos.TeamUserDaoInterface,
	trainingPlanDao daos.TrainingPlanDaoInterface,
	planDayDao daos.PlanDayDaoInterface,
	sessionDao daos.SessionDaoInterface,
	db *gorm.DB,
) CalendarServiceInterface {
	return &calendarService{
		calendarDao: calendarDao, groupDao: groupDao, teamDao: teamDao, groupUserDao: groupUserDao,
		teamUserDao: teamUserDao, trainingPlanDao: trainingPlanDao, planDayDao: planDayDao, sessionDao: sessionDao, db: db,
	}
}

// isGroupOwner devuelve nil si callerID es el entrenador dueño del equipo del grupo.
func (s *calendarService) isGroupOwner(ctx *gin.Context, groupID, callerID int64) error {
	group, err := s.groupDao.FindByID(ctx, groupID)
	if err != nil {
		return fmt.Errorf("error al buscar grupo")
	}
	if group == nil {
		return ErrCalendarGroupNotFound
	}
	team, err := s.teamDao.FindByID(ctx, group.TeamID)
	if err != nil {
		return fmt.Errorf("error al buscar equipo")
	}
	if team == nil || team.OwnerID != callerID {
		return ErrCalendarForbidden
	}
	return nil
}

// isGroupOwnerOrMember devuelve nil si callerID es el dueño o un miembro
// activo del grupo (lectura, spec §4).
func (s *calendarService) isGroupOwnerOrMember(ctx *gin.Context, groupID, callerID int64) error {
	if err := s.isGroupOwner(ctx, groupID, callerID); err == nil {
		return nil
	} else if !errors.Is(err, ErrCalendarForbidden) {
		return err
	}
	member, err := s.groupUserDao.FindByGroupAndUser(ctx, groupID, callerID)
	if err != nil {
		return fmt.Errorf("error al validar membresía")
	}
	if member == nil {
		return ErrCalendarForbidden
	}
	return nil
}

// isCalendarDayClosed applies the calendar's date/time rule to a stored day.
// It is a write guard now; it no longer decides whether catalog content must
// be cloned.
func isCalendarDayClosed(day dbs.GroupCalendarDay, now time.Time) bool {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayDate := time.Date(day.Date.Year(), day.Date.Month(), day.Date.Day(), 0, 0, 0, 0, now.Location())
	if dayDate.Before(today) {
		return true
	}
	if dayDate.After(today) {
		return false
	}
	if !day.IsPresencial {
		return true
	}
	if day.PresencialTimeFrom == nil {
		return false
	}
	threshold := time.Date(now.Year(), now.Month(), now.Day(), day.PresencialTimeFrom.UTC().Hour(), day.PresencialTimeFrom.UTC().Minute(), 0, 0, now.Location())
	return !now.Before(threshold)
}

// validateDayFields valida el request contra el kind actual del día.
// hasExistingInstance indica si el día ya tiene una SessionInstance propia:
// con training y session_id nil, la instancia existente se conserva (no es
// error); solo sin instancia previa se exige session_id.
func (s *calendarService) validateDayFields(req calendar.CalendarDayRequest, currentKind string, hasExistingInstance bool) error {
	if !constants.IsValidGroupCalendarDayKind(req.Kind) {
		return ErrCalendarInvalidKind
	}
	switch req.Kind {
	case string(constants.GroupCalendarDayKindOther):
		if req.OtherName == nil {
			return ErrCalendarFieldMismatch
		}
	case string(constants.GroupCalendarDayKindTraining):
		if req.SessionID == nil && !hasExistingInstance {
			return ErrCalendarFieldMismatch
		}
	case string(constants.GroupCalendarDayKindCancelled):
		if req.CancelledReason == nil {
			return ErrCalendarFieldMismatch
		}
		if currentKind != string(constants.GroupCalendarDayKindTraining) {
			return ErrCalendarInvalidCancelTransition
		}
	}
	isPresencial := req.IsPresencial != nil && *req.IsPresencial
	if isPresencial && (req.PresencialTimeFrom == nil || req.PresencialTimeTo == nil || req.PresencialLocation == nil) {
		return ErrCalendarFieldMismatch
	}
	if isPresencial {
		from, err := time.Parse("15:04", *req.PresencialTimeFrom)
		if err != nil {
			return ErrCalendarInvalidTimeFormat
		}
		to, err := time.Parse("15:04", *req.PresencialTimeTo)
		if err != nil {
			return ErrCalendarInvalidTimeFormat
		}
		if !to.After(from) {
			return ErrCalendarInvalidTimeRange
		}
	}
	return nil
}

func (s *calendarService) buildRow(ctx *gin.Context, groupID int64, date time.Time, req calendar.CalendarDayRequest) (*dbs.GroupCalendarDay, error) {
	row := &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: req.Kind}
	switch req.Kind {
	case string(constants.GroupCalendarDayKindOther):
		row.OtherName = req.OtherName
	case string(constants.GroupCalendarDayKindCancelled):
		row.CancelledReason = req.CancelledReason
	}
	if req.IsPresencial != nil && *req.IsPresencial {
		row.IsPresencial = true
		parsedFrom, err := time.Parse("15:04", *req.PresencialTimeFrom)
		if err != nil {
			return nil, fmt.Errorf("presencial_time_from debe tener formato HH:MM")
		}
		parsedTo, err := time.Parse("15:04", *req.PresencialTimeTo)
		if err != nil {
			return nil, fmt.Errorf("presencial_time_to debe tener formato HH:MM")
		}
		row.PresencialTimeFrom = &parsedFrom
		row.PresencialTimeTo = &parsedTo
		locationJSON, err := jsonMarshalLocation(req.PresencialLocation)
		if err != nil {
			return nil, err
		}
		row.PresencialLocation = locationJSON
	}
	return row, nil
}

func calendarRequestFromPlanDay(day dbs.PlanDay) (calendar.CalendarDayRequest, error) {
	req := calendar.CalendarDayRequest{Kind: day.Kind, SessionID: day.SessionID, OtherName: day.OtherName}
	if day.DefaultPresencial {
		isPresencial := true
		req.IsPresencial = &isPresencial
		if day.DefaultTimeFrom != nil {
			from := day.DefaultTimeFrom.UTC().Format("15:04")
			req.PresencialTimeFrom = &from
		}
		if day.DefaultTimeTo != nil {
			to := day.DefaultTimeTo.UTC().Format("15:04")
			req.PresencialTimeTo = &to
		}
		if day.DefaultLocation != nil {
			location, err := jsonUnmarshalLocation(*day.DefaultLocation)
			if err != nil {
				return calendar.CalendarDayRequest{}, ErrCalendarFieldMismatch
			}
			req.PresencialLocation = location
		}
	}
	return req, nil
}

// instantiateSession copies catalog content into the instance tables. The
// caller supplies the transaction so every row created for one calendar save
// participates in the same atomic operation.
func (s *calendarService) instantiateSession(ctx *gin.Context, tx *gorm.DB, sessionID int64) (*dbs.SessionInstance, error) {
	sessionDao := daos.NewSessionDao(tx)
	sessionExerciseDao := daos.NewSessionExerciseDao(tx)
	exerciseDao := daos.NewExerciseDao(tx)
	sessionInstanceDao := daos.NewSessionInstanceDao(tx)
	exerciseInstanceDao := daos.NewExerciseInstanceDao(tx)
	linkInstanceDao := daos.NewSessionExerciseInstanceDao(tx)

	catalogSession, err := sessionDao.FindByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar sesión para instanciar: %w", err)
	}
	if catalogSession == nil {
		return nil, ErrCalendarSessionNotFound
	}
	catalogLinks, err := sessionExerciseDao.FindBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar ejercicios para instanciar: %w", err)
	}

	result := &dbs.SessionInstance{
		Name: catalogSession.Name, Description: catalogSession.Description,
		SourceSessionID: &sessionID,
	}
	if err := sessionInstanceDao.Create(ctx, result); err != nil {
		return nil, fmt.Errorf("error al crear sesión instancia: %w", err)
	}
	for _, catalogLink := range catalogLinks {
		catalogExercise, err := exerciseDao.FindByID(ctx, catalogLink.ExerciseID)
		if err != nil {
			return nil, fmt.Errorf("error al buscar ejercicio para instanciar: %w", err)
		}
		if catalogExercise == nil {
			return nil, fmt.Errorf("%w: %d", ErrSessionExerciseNotFound, catalogLink.ExerciseID)
		}
		exerciseInstance := &dbs.ExerciseInstance{
			Name: catalogExercise.Name, Description: catalogExercise.Description, Kind: catalogExercise.Kind,
			Intensity: catalogExercise.Intensity, Minutes: catalogExercise.Minutes, DistanceM: catalogExercise.DistanceM,
			SpeedKph: catalogExercise.SpeedKph, MuscleGroup: catalogExercise.MuscleGroup, VideoURL: catalogExercise.VideoURL,
			SourceExerciseID: &catalogLink.ExerciseID,
		}
		if err := exerciseInstanceDao.Create(ctx, exerciseInstance); err != nil {
			return nil, fmt.Errorf("error al crear ejercicio instancia: %w", err)
		}
		linkInstance := &dbs.SessionExerciseInstance{
			SessionInstanceID: result.ID, ExerciseInstanceID: exerciseInstance.ID,
			Role: catalogLink.Role, RepeatCount: catalogLink.RepeatCount, RestMinutes: catalogLink.RestMinutes,
		}
		if err := linkInstanceDao.Create(ctx, linkInstance); err != nil {
			return nil, fmt.Errorf("error al crear vínculo de ejercicio instancia: %w", err)
		}
	}
	return result, nil
}

func (s *calendarService) sessionInstanceResponse(ctx *gin.Context, database *gorm.DB, id *int64) (*instance.SessionInstanceResponse, error) {
	if id == nil {
		return nil, nil
	}
	if database == nil {
		// Unit tests that exercise calendar row mapping can use a nil DB for
		// rows whose instance detail is supplied only by the integration path.
		return nil, nil
	}
	sessionDao := daos.NewSessionInstanceDao(database)
	linkDao := daos.NewSessionExerciseInstanceDao(database)
	exerciseDao := daos.NewExerciseInstanceDao(database)
	sessionInstance, err := sessionDao.FindByID(ctx, *id)
	if err != nil {
		return nil, err
	}
	if sessionInstance == nil {
		return nil, fmt.Errorf("sesión instancia %d no encontrada", *id)
	}
	links, err := linkDao.FindBySessionInstance(ctx, *id)
	if err != nil {
		return nil, err
	}
	exerciseIDs := make([]int64, len(links))
	for i, link := range links {
		exerciseIDs[i] = link.ExerciseInstanceID
	}
	exercises, err := exerciseDao.FindByIDs(ctx, exerciseIDs)
	if err != nil {
		return nil, err
	}
	response, err := instance.NewSessionResponse(*sessionInstance, links, exercises)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

// deleteSupersededInstance follows D10 and keeps an instance orphaned when
// feedback already points to either the session or one of its exercises.
func (s *calendarService) deleteSupersededInstance(ctx *gin.Context, tx *gorm.DB, id int64) error {
	if id == 0 {
		return nil
	}
	sessionInstanceDao := daos.NewSessionInstanceDao(tx)
	linkDao := daos.NewSessionExerciseInstanceDao(tx)
	exerciseDao := daos.NewExerciseInstanceDao(tx)
	hasFeedback, err := sessionInstanceDao.HasFeedback(ctx, id)
	if err != nil {
		return err
	}
	links, err := linkDao.FindBySessionInstance(ctx, id)
	if err != nil {
		return err
	}
	if hasFeedback {
		return nil
	}
	for _, link := range links {
		hasFeedback, err := exerciseDao.HasFeedback(ctx, link.ExerciseInstanceID)
		if err != nil {
			return err
		}
		if hasFeedback {
			return nil
		}
	}
	if err := linkDao.DeleteBySessionInstance(ctx, id); err != nil {
		return err
	}
	for _, link := range links {
		if err := exerciseDao.Delete(ctx, link.ExerciseInstanceID); err != nil {
			return err
		}
	}
	return sessionInstanceDao.Delete(ctx, id)
}

func closedDayForRequest(existing *dbs.GroupCalendarDay, candidate *dbs.GroupCalendarDay, now time.Time) bool {
	if existing != nil {
		return isCalendarDayClosed(*existing, now)
	}
	return candidate != nil && isCalendarDayClosed(*candidate, now)
}

func (s *calendarService) GetRange(ctx *gin.Context, groupID, callerID int64, from, to time.Time) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwnerOrMember(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	days, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, from, to)
	if err != nil {
		customlogger.Error(ctx, "error listing calendar range", err, customlogger.TagMethod("GetRange"))
		return nil, fmt.Errorf("error al listar calendario")
	}
	responses := make([]calendar.CalendarDayResponse, len(days))
	for i, d := range days {
		response, err := s.toCalendarDayResponse(ctx, d)
		if err != nil {
			return nil, err
		}
		responses[i] = response
	}
	return responses, nil
}

func (s *calendarService) UpsertDay(ctx *gin.Context, groupID, callerID int64, date time.Time, req calendar.CalendarDayRequest) (*calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	existing, err := s.calendarDao.FindByGroupAndDate(ctx, groupID, date)
	if err != nil {
		return nil, fmt.Errorf("error al buscar día de calendario")
	}
	currentKind := ""
	hasExistingInstance := false
	if existing != nil {
		currentKind = existing.Kind
		hasExistingInstance = existing.SessionInstanceID != nil
	}
	if err := s.validateDayFields(req, currentKind, hasExistingInstance); err != nil {
		return nil, err
	}
	row, err := s.buildRow(ctx, groupID, date, req)
	if err != nil {
		return nil, err
	}
	if req.Kind != string(constants.GroupCalendarDayKindCancelled) && closedDayForRequest(existing, row, time.Now()) {
		return nil, newCalendarClosedDaysError([]string{date.Format("2006-01-02")})
	}
	if s.db == nil {
		if existing != nil && (req.Kind == string(constants.GroupCalendarDayKindCancelled) ||
			(req.Kind == string(constants.GroupCalendarDayKindTraining) && req.SessionID == nil)) {
			row.SessionInstanceID = existing.SessionInstanceID
		}
		if err := s.calendarDao.Upsert(ctx, row); err != nil {
			return nil, fmt.Errorf("error al guardar día de calendario")
		}
		response, err := s.toCalendarDayResponse(ctx, *row)
		if err != nil {
			return nil, err
		}
		return &response, nil
	}
	var saved dbs.GroupCalendarDay
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		txExisting, err := txCalendarDao.FindByGroupAndDate(ctx, groupID, date)
		if err != nil {
			return err
		}
		if req.Kind != string(constants.GroupCalendarDayKindCancelled) && closedDayForRequest(txExisting, row, time.Now()) {
			return newCalendarClosedDaysError([]string{date.Format("2006-01-02")})
		}
		txHasExistingInstance := txExisting != nil && txExisting.SessionInstanceID != nil
		if err := s.validateDayFields(req, func() string {
			if txExisting == nil {
				return ""
			}
			return txExisting.Kind
		}(), txHasExistingInstance); err != nil {
			return err
		}
		preservada := false
		if req.Kind == string(constants.GroupCalendarDayKindTraining) {
			if req.SessionID == nil {
				row.SessionInstanceID = txExisting.SessionInstanceID
				preservada = true
			} else {
				newInstance, err := s.instantiateSession(ctx, tx, *req.SessionID)
				if err != nil {
					return err
				}
				row.SessionInstanceID = &newInstance.ID
			}
		} else if req.Kind == string(constants.GroupCalendarDayKindCancelled) && txExisting != nil {
			row.SessionInstanceID = txExisting.SessionInstanceID
		}
		if err := txCalendarDao.Upsert(ctx, row); err != nil {
			return err
		}
		if txExisting != nil && txExisting.SessionInstanceID != nil && !preservada && req.Kind != string(constants.GroupCalendarDayKindCancelled) {
			if err := s.deleteSupersededInstance(ctx, tx, *txExisting.SessionInstanceID); err != nil {
				return err
			}
		}
		saved = *row
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCalendarDayClosed) || errors.Is(err, ErrCalendarSessionNotFound) || errors.Is(err, ErrSessionExerciseNotFound) {
			return nil, err
		}
		customlogger.Error(ctx, "error upserting calendar day", err, customlogger.TagMethod("UpsertDay"))
		return nil, fmt.Errorf("error al guardar día de calendario")
	}
	response, err := s.toCalendarDayResponse(ctx, saved)
	if err != nil {
		return nil, err
	}
	return &response, nil
}

func (s *calendarService) DeleteDay(ctx *gin.Context, groupID, callerID int64, date time.Time) error {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return err
	}
	existing, err := s.calendarDao.FindByGroupAndDate(ctx, groupID, date)
	if err != nil {
		return fmt.Errorf("error al buscar día de calendario")
	}
	if existing == nil {
		if s.db == nil {
			if err := s.calendarDao.Delete(ctx, groupID, date); err != nil {
				return fmt.Errorf("error al borrar día de calendario")
			}
		}
		return nil
	}
	if isCalendarDayClosed(*existing, time.Now()) {
		return newCalendarClosedDaysError([]string{date.Format("2006-01-02")})
	}
	if s.db == nil {
		if err := s.calendarDao.Delete(ctx, groupID, date); err != nil {
			return fmt.Errorf("error al borrar día de calendario")
		}
		return nil
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		txExisting, err := txCalendarDao.FindByGroupAndDate(ctx, groupID, date)
		if err != nil {
			return err
		}
		if txExisting == nil {
			return nil
		}
		if isCalendarDayClosed(*txExisting, time.Now()) {
			return newCalendarClosedDaysError([]string{date.Format("2006-01-02")})
		}
		if err := txCalendarDao.Delete(ctx, groupID, date); err != nil {
			return err
		}
		if txExisting.SessionInstanceID != nil {
			return s.deleteSupersededInstance(ctx, tx, *txExisting.SessionInstanceID)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCalendarDayClosed) {
			return err
		}
		customlogger.Error(ctx, "error deleting calendar day", err, customlogger.TagMethod("DeleteDay"))
		return fmt.Errorf("error al borrar día de calendario")
	}
	return nil
}

func (s *calendarService) toCalendarDayResponse(ctx *gin.Context, d dbs.GroupCalendarDay) (calendar.CalendarDayResponse, error) {
	resp := calendar.CalendarDayResponse{
		ID: d.ID, GroupID: d.GroupID, Date: d.Date.Format("2006-01-02"), Kind: d.Kind,
		OtherName: d.OtherName, CancelledReason: d.CancelledReason,
		IsPresencial: d.IsPresencial, SourcePlanID: d.SourcePlanID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if d.PresencialTimeFrom != nil {
		formatted := d.PresencialTimeFrom.UTC().Format("15:04")
		resp.PresencialTimeFrom = &formatted
	}
	if d.PresencialTimeTo != nil {
		formatted := d.PresencialTimeTo.UTC().Format("15:04")
		resp.PresencialTimeTo = &formatted
	}
	if d.PresencialLocation != nil {
		loc, err := jsonUnmarshalLocation(*d.PresencialLocation)
		if err == nil {
			resp.PresencialLocation = loc
		}
	}
	var err error
	resp.SessionInstance, err = s.sessionInstanceResponse(ctx, s.db, d.SessionInstanceID)
	if err != nil {
		return calendar.CalendarDayResponse{}, err
	}
	return resp, nil
}

func jsonMarshalLocation(loc *trainingplan.Location) (*string, error) {
	b, err := json.Marshal(loc)
	if err != nil {
		return nil, fmt.Errorf("error al serializar ubicación")
	}
	s := string(b)
	return &s, nil
}

func jsonUnmarshalLocation(s string) (*trainingplan.Location, error) {
	var loc trainingplan.Location
	if err := json.Unmarshal([]byte(s), &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// parseStampExcludeDates convierte exclude_dates en un set por fecha
// ("YYYY-MM-DD" exacto). Cualquier elemento con otro formato aborta con
// ErrCalendarInvalidDate antes de que Stamp toque nada.
func parseStampExcludeDates(dates []string) (map[string]bool, error) {
	set := make(map[string]bool, len(dates))
	for _, d := range dates {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return nil, ErrCalendarInvalidDate
		}
		set[d] = true
	}
	return set, nil
}

func (s *calendarService) Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	plan, err := s.trainingPlanDao.FindByID(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar plan")
	}
	if plan == nil {
		return nil, ErrCalendarPlanNotFound
	}
	if plan.OwnerID != callerID {
		return nil, ErrCalendarPlanForbidden
	}
	planDays, err := s.planDayDao.FindByPlan(ctx, req.PlanID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar días del plan")
	}
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("start_date debe tener formato YYYY-MM-DD")
	}
	excluded, err := parseStampExcludeDates(req.ExcludeDates)
	if err != nil {
		return nil, err
	}

	if len(planDays) == 0 {
		return nil, fmt.Errorf("el plan no tiene días")
	}
	// Subconjunto a estampar: días del plan cuyas fechas objetivo no están
	// en exclude_dates. Todo lo que sigue (guards, escritura, respuesta)
	// opera solo sobre este subconjunto; las fechas excluidas no se tocan.
	idx := make([]int, 0, len(planDays))
	targetDates := make([]time.Time, 0, len(planDays))
	for i, pd := range planDays {
		date := startDate.AddDate(0, 0, pd.SequenceNo-1)
		if excluded[date.Format("2006-01-02")] {
			continue
		}
		idx = append(idx, i)
		targetDates = append(targetDates, date)
	}
	if len(idx) == 0 {
		return []calendar.CalendarDayResponse{}, nil
	}
	for _, planDay := range idx {
		planReq, err := calendarRequestFromPlanDay(planDays[planDay])
		if err != nil {
			return nil, err
		}
		if err := s.validateDayFields(planReq, "", false); err != nil {
			return nil, err
		}
	}
	if s.db == nil {
		minDate, maxDate := targetDates[0], targetDates[0]
		for _, date := range targetDates[1:] {
			if date.Before(minDate) {
				minDate = date
			}
			if date.After(maxDate) {
				maxDate = date
			}
		}
		existing, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, minDate, maxDate)
		if err != nil {
			return nil, fmt.Errorf("error al validar conflictos")
		}
		occupied := make(map[string]bool, len(existing))
		for _, day := range existing {
			occupied[day.Date.Format("2006-01-02")] = true
		}
		conflicts := make([]string, 0)
		for _, date := range targetDates {
			if occupied[date.Format("2006-01-02")] {
				conflicts = append(conflicts, date.Format("2006-01-02"))
			}
		}
		if !req.Force && len(conflicts) > 0 {
			return nil, &calendarStampConflictError{dates: conflicts}
		}
		return nil, fmt.Errorf("no hay DB disponible para estampar plan")
	}
	rows := make([]dbs.GroupCalendarDay, len(idx))
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		existingByDate := make(map[string]*dbs.GroupCalendarDay, len(idx))
		for j, date := range targetDates {
			i := idx[j]
			existing, err := txCalendarDao.FindByGroupAndDate(ctx, groupID, date)
			if err != nil {
				return err
			}
			existingByDate[date.Format("2006-01-02")] = existing
			planReq, err := calendarRequestFromPlanDay(planDays[i])
			if err != nil {
				return err
			}
			currentKind := ""
			if existing != nil {
				currentKind = existing.Kind
			}
			if err := s.validateDayFields(planReq, currentKind, false); err != nil {
				return err
			}
			row := dbs.GroupCalendarDay{
				GroupID: groupID, Date: date, Kind: planDays[i].Kind, OtherName: planDays[i].OtherName,
				IsPresencial: planDays[i].DefaultPresencial, PresencialTimeFrom: planDays[i].DefaultTimeFrom,
				PresencialTimeTo: planDays[i].DefaultTimeTo, PresencialLocation: planDays[i].DefaultLocation,
				SourcePlanID: &req.PlanID,
			}
			rows[j] = row
		}

		closed := make([]string, 0)
		conflicts := make([]string, 0)
		for i, row := range rows {
			existing := existingByDate[row.Date.Format("2006-01-02")]
			if closedDayForRequest(existing, &row, time.Now()) {
				closed = append(closed, row.Date.Format("2006-01-02"))
			}
			if existing != nil {
				conflicts = append(conflicts, targetDates[i].Format("2006-01-02"))
			}
		}
		if len(closed) > 0 {
			return newCalendarClosedDaysError(closed)
		}
		if !req.Force && len(conflicts) > 0 {
			return &calendarStampConflictError{dates: conflicts}
		}

		for i := range rows {
			planDay := planDays[idx[i]]
			if planDay.Kind == string(constants.GroupCalendarDayKindTraining) {
				newInstance, err := s.instantiateSession(ctx, tx, *planDay.SessionID)
				if err != nil {
					return err
				}
				rows[i].SessionInstanceID = &newInstance.ID
			}
			if err := txCalendarDao.Upsert(ctx, &rows[i]); err != nil {
				return err
			}
			if existing := existingByDate[rows[i].Date.Format("2006-01-02")]; existing != nil && existing.SessionInstanceID != nil {
				if err := s.deleteSupersededInstance(ctx, tx, *existing.SessionInstanceID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCalendarDayClosed) || errors.Is(err, ErrCalendarStampConflict) || isCalendarValidationError(err) {
			return nil, err
		}
		customlogger.Error(ctx, "error stamping calendar day", err, customlogger.TagMethod("Stamp"))
		return nil, fmt.Errorf("error al estampar plan")
	}
	responses := make([]calendar.CalendarDayResponse, len(rows))
	for i := range rows {
		responses[i], err = s.toCalendarDayResponse(ctx, rows[i])
		if err != nil {
			return nil, err
		}
	}
	return responses, nil
}
func (s *calendarService) Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	dayReq := calendar.CalendarDayRequest{
		Kind: req.Kind, SessionID: req.SessionID, OtherName: req.OtherName,
		IsPresencial: req.IsPresencial, PresencialTimeFrom: req.PresencialTimeFrom, PresencialTimeTo: req.PresencialTimeTo, PresencialLocation: req.PresencialLocation,
	}
	dates := make([]time.Time, len(req.Dates))
	for i, dateStr := range req.Dates {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("fecha inválida en dates: %s", dateStr)
		}
		dates[i] = date
	}
	if s.db == nil {
		existing := make([]*dbs.GroupCalendarDay, len(dates))
		rows := make([]*dbs.GroupCalendarDay, len(dates))
		missing := make([]string, 0)
		for i, date := range dates {
			var err error
			existing[i], err = s.calendarDao.FindByGroupAndDate(ctx, groupID, date)
			if err != nil {
				return nil, err
			}
			if dayReq.Kind == string(constants.GroupCalendarDayKindTraining) && dayReq.SessionID == nil &&
				(existing[i] == nil || existing[i].SessionInstanceID == nil) {
				missing = append(missing, date.Format("2006-01-02"))
				continue
			}
			if err := s.validateDayFields(dayReq, func() string {
				if existing[i] == nil {
					return ""
				}
				return existing[i].Kind
			}(), existing[i] != nil && existing[i].SessionInstanceID != nil); err != nil {
				return nil, err
			}
			rows[i], err = s.buildRow(ctx, groupID, date, dayReq)
			if err != nil {
				return nil, err
			}
			if dayReq.Kind == string(constants.GroupCalendarDayKindTraining) && dayReq.SessionID == nil {
				rows[i].SessionInstanceID = existing[i].SessionInstanceID
			}
		}
		if len(missing) > 0 {
			return nil, newCalendarTrainingWithoutInstanceError(missing)
		}
		closed := make([]string, 0)
		for i, date := range dates {
			if dayReq.Kind != string(constants.GroupCalendarDayKindCancelled) && closedDayForRequest(existing[i], rows[i], time.Now()) {
				closed = append(closed, date.Format("2006-01-02"))
			}
			if dayReq.Kind == string(constants.GroupCalendarDayKindCancelled) && existing[i] == nil {
				return nil, ErrCalendarInvalidCancelTransition
			}
			if dayReq.Kind == string(constants.GroupCalendarDayKindCancelled) {
				rows[i].SessionInstanceID = existing[i].SessionInstanceID
			}
		}
		if len(closed) > 0 {
			return nil, newCalendarClosedDaysError(closed)
		}
		responses := make([]calendar.CalendarDayResponse, 0, len(rows))
		for _, row := range rows {
			if err := s.calendarDao.Upsert(ctx, row); err != nil {
				return nil, err
			}
			response, err := s.toCalendarDayResponse(ctx, *row)
			if err != nil {
				return nil, err
			}
			responses = append(responses, response)
		}
		return responses, nil
	}
	rows := make([]dbs.GroupCalendarDay, len(dates))
	err := s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		existing := make([]*dbs.GroupCalendarDay, len(dates))
		missing := make([]string, 0)
		for i, date := range dates {
			var err error
			existing[i], err = txCalendarDao.FindByGroupAndDate(ctx, groupID, date)
			if err != nil {
				return err
			}
			if dayReq.Kind == string(constants.GroupCalendarDayKindTraining) && dayReq.SessionID == nil &&
				(existing[i] == nil || existing[i].SessionInstanceID == nil) {
				missing = append(missing, date.Format("2006-01-02"))
				continue
			}
			if err := s.validateDayFields(dayReq, func() string {
				if existing[i] == nil {
					return ""
				}
				return existing[i].Kind
			}(), existing[i] != nil && existing[i].SessionInstanceID != nil); err != nil {
				return err
			}
			builtRow, err := s.buildRow(ctx, groupID, date, dayReq)
			if err != nil {
				return err
			}
			if dayReq.Kind == string(constants.GroupCalendarDayKindTraining) && dayReq.SessionID == nil {
				builtRow.SessionInstanceID = existing[i].SessionInstanceID
			}
			rows[i] = *builtRow
		}
		if len(missing) > 0 {
			return newCalendarTrainingWithoutInstanceError(missing)
		}
		closed := make([]string, 0)
		for i := range rows {
			if dayReq.Kind != string(constants.GroupCalendarDayKindCancelled) && closedDayForRequest(existing[i], &rows[i], time.Now()) {
				closed = append(closed, dates[i].Format("2006-01-02"))
			}
		}
		if len(closed) > 0 {
			return newCalendarClosedDaysError(closed)
		}
		for i := range rows {
			preservada := false
			if dayReq.Kind == string(constants.GroupCalendarDayKindTraining) {
				if dayReq.SessionID == nil {
					preservada = true
				} else {
					newInstance, err := s.instantiateSession(ctx, tx, *dayReq.SessionID)
					if err != nil {
						return err
					}
					rows[i].SessionInstanceID = &newInstance.ID
				}
			} else if dayReq.Kind == string(constants.GroupCalendarDayKindCancelled) {
				rows[i].SessionInstanceID = existing[i].SessionInstanceID
			}
			if err := txCalendarDao.Upsert(ctx, &rows[i]); err != nil {
				return err
			}
			if !preservada && existing[i] != nil && existing[i].SessionInstanceID != nil && dayReq.Kind != string(constants.GroupCalendarDayKindCancelled) {
				if err := s.deleteSupersededInstance(ctx, tx, *existing[i].SessionInstanceID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCalendarDayClosed) || isCalendarValidationError(err) {
			return nil, err
		}
		customlogger.Error(ctx, "error bulk-upserting calendar day", err, customlogger.TagMethod("Bulk"))
		return nil, fmt.Errorf("error al aplicar bulk")
	}
	responses := make([]calendar.CalendarDayResponse, len(rows))
	for i := range rows {
		responses[i], err = s.toCalendarDayResponse(ctx, rows[i])
		if err != nil {
			return nil, err
		}
	}
	return responses, nil
}

func (s *calendarService) BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return err
	}
	dates := make([]time.Time, 0, len(req.Dates))
	for _, dateStr := range req.Dates {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return fmt.Errorf("fecha inválida en dates: %s", dateStr)
		}
		dates = append(dates, date)
	}
	if s.db == nil {
		closed := make([]string, 0)
		for _, date := range dates {
			day, err := s.calendarDao.FindByGroupAndDate(ctx, groupID, date)
			if err != nil {
				return err
			}
			if day != nil && isCalendarDayClosed(*day, time.Now()) {
				closed = append(closed, date.Format("2006-01-02"))
			}
		}
		if len(closed) > 0 {
			return newCalendarClosedDaysError(closed)
		}
		return s.calendarDao.DeleteByDates(ctx, groupID, dates)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		existing := make([]*dbs.GroupCalendarDay, len(dates))
		closed := make([]string, 0)
		for i, date := range dates {
			var err error
			existing[i], err = txCalendarDao.FindByGroupAndDate(ctx, groupID, date)
			if err != nil {
				return err
			}
			if existing[i] != nil && isCalendarDayClosed(*existing[i], time.Now()) {
				closed = append(closed, date.Format("2006-01-02"))
			}
		}
		if len(closed) > 0 {
			return newCalendarClosedDaysError(closed)
		}
		if err := txCalendarDao.DeleteByDates(ctx, groupID, dates); err != nil {
			return err
		}
		for _, day := range existing {
			if day != nil && day.SessionInstanceID != nil {
				if err := s.deleteSupersededInstance(ctx, tx, *day.SessionInstanceID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCalendarDayClosed) {
			return err
		}
		customlogger.Error(ctx, "error bulk-clearing calendar days", err, customlogger.TagMethod("BulkClear"))
		return fmt.Errorf("error al limpiar fechas")
	}
	return nil
}

func (s *calendarService) Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error) {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return nil, err
	}
	fromDate, err := time.Parse("2006-01-02", req.FromDate)
	if err != nil {
		return nil, fmt.Errorf("from_date debe tener formato YYYY-MM-DD")
	}
	if req.Days <= 0 {
		return nil, fmt.Errorf("days debe ser un entero positivo")
	}
	farFuture := time.Date(9999, 12, 31, 0, 0, 0, 0, fromDate.Location())
	if s.db == nil {
		affected, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, fromDate, farFuture)
		if err != nil {
			return nil, fmt.Errorf("error al buscar filas a correr")
		}
		closed := make([]string, 0)
		for _, a := range affected {
			if isCalendarDayClosed(a, time.Now()) {
				closed = append(closed, a.Date.Format("2006-01-02"))
			}
		}
		if len(closed) > 0 {
			return nil, newCalendarClosedDaysError(closed)
		}
		return nil, fmt.Errorf("no hay DB disponible para correr fechas")
	}
	var affected []dbs.GroupCalendarDay
	// Updating from the latest date backwards avoids transient unique-key
	// collisions while shifting several rows forward by the same amount.
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		var err error
		affected, err = txCalendarDao.FindByGroupAndRange(ctx, groupID, fromDate, farFuture)
		if err != nil {
			return err
		}
		closed := make([]string, 0)
		for _, a := range affected {
			if isCalendarDayClosed(a, time.Now()) {
				closed = append(closed, a.Date.Format("2006-01-02"))
			}
		}
		if len(closed) > 0 {
			return newCalendarClosedDaysError(closed)
		}
		ordered := append([]dbs.GroupCalendarDay(nil), affected...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].Date.After(ordered[j].Date) })
		// Collision guard: re-chequear dentro de la tx que ninguna fecha destino
		// esté ocupada por una fila que NO va a moverse en este shift (p. ej.
		// escritura concurrente). Si lo está, rollback con 409 en vez de
		// explotar contra idx_group_calendar_day_group_date como 500.
		movedDates := make(map[string]bool, len(ordered))
		for _, a := range ordered {
			movedDates[a.Date.Format("2006-01-02")] = true
		}
		for _, a := range ordered {
			newDate := a.Date.AddDate(0, 0, req.Days)
			occupant, ferr := txCalendarDao.FindByGroupAndDate(ctx, groupID, newDate)
			if ferr != nil {
				return ferr
			}
			if occupant != nil && !movedDates[occupant.Date.Format("2006-01-02")] {
				return ErrCalendarShiftCollision
			}
		}
		for _, a := range ordered {
			newDate := a.Date.AddDate(0, 0, req.Days)
			if err := txCalendarDao.UpdateDatesForShift(ctx, groupID, a.Date, newDate); err != nil {
				// Defensivo: una escritura concurrente puede chocar el unique
				// index a pesar del guard de arriba (ventana entre el SELECT del
				// guard y el UPDATE). Mapear a colisión, no a 500.
				if strings.Contains(err.Error(), "duplicate key") {
					return ErrCalendarShiftCollision
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrCalendarDayClosed) || errors.Is(err, ErrCalendarShiftCollision) {
			return nil, err
		}
		customlogger.Error(ctx, "error shifting calendar day", err, customlogger.TagMethod("Shift"))
		return nil, fmt.Errorf("error al correr fechas")
	}
	responses := make([]calendar.CalendarDayResponse, len(affected))
	for i := range affected {
		a := affected[i]
		a.Date = a.Date.AddDate(0, 0, req.Days)
		responses[i], err = s.toCalendarDayResponse(ctx, a)
		if err != nil {
			return nil, err
		}
	}
	return responses, nil
}
func (s *calendarService) NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
	memberships, err := s.groupUserDao.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar grupos del usuario")
	}
	groupIDs := make([]int64, len(memberships))
	for i, m := range memberships {
		groupIDs[i] = m.GroupID
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day, err := s.calendarDao.FindNextSessionForGroups(ctx, groupIDs, today)
	if err != nil {
		customlogger.Error(ctx, "error finding next session", err, customlogger.TagMethod("NextSession"))
		return nil, fmt.Errorf("error al buscar próxima sesión")
	}
	if day == nil {
		return nil, nil
	}
	resp := &calendar.NextSessionResponse{
		GroupID: day.GroupID, Date: day.Date.Format("2006-01-02"), IsPresencial: day.IsPresencial,
	}
	if day.PresencialTimeFrom != nil {
		formatted := day.PresencialTimeFrom.UTC().Format("15:04")
		resp.PresencialTimeFrom = &formatted
	}
	if day.PresencialTimeTo != nil {
		formatted := day.PresencialTimeTo.UTC().Format("15:04")
		resp.PresencialTimeTo = &formatted
	}
	if day.PresencialLocation != nil {
		loc, err := jsonUnmarshalLocation(*day.PresencialLocation)
		if err == nil {
			resp.PresencialLocation = loc
		}
	}
	resp.SessionInstance, err = s.sessionInstanceResponse(ctx, s.db, day.SessionInstanceID)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *calendarService) CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
	memberships, err := s.groupUserDao.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("error al buscar grupos del usuario")
	}
	items := make([]calendar.CalendarSummaryItem, 0, len(memberships))
	for _, m := range memberships {
		group, err := s.groupDao.FindByID(ctx, m.GroupID)
		if err != nil || group == nil {
			continue
		}
		items = append(items, calendar.CalendarSummaryItem{GroupID: group.ID, GroupName: group.Name})
	}
	return items, nil
}
