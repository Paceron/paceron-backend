package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
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
	ErrCalendarUserMismatch            = errors.New("no podés consultar los datos de otro usuario")
	ErrCalendarInvalidTimeFormat       = errors.New("presencial_time_from/presencial_time_to deben tener formato HH:MM")
	ErrCalendarInvalidTimeRange        = errors.New("presencial_time_to debe ser posterior a presencial_time_from")
)

// CalendarServiceInterface se completa en Tasks 5-7 (Stamp, Bulk/BulkClear/
// Shift, NextSession/CalendarSummary/AssignedGroups) — todos métodos del
// mismo archivo/struct, no una interfaz separada.
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
	AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error)
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

func (s *calendarService) validateDayFields(req calendar.CalendarDayRequest, currentKind string) error {
	if !constants.IsValidGroupCalendarDayKind(req.Kind) {
		return ErrCalendarInvalidKind
	}
	switch req.Kind {
	case string(constants.GroupCalendarDayKindOther):
		if req.OtherName == nil {
			return ErrCalendarFieldMismatch
		}
	case string(constants.GroupCalendarDayKindTraining):
		if req.SessionID == nil {
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
	case string(constants.GroupCalendarDayKindTraining):
		row.SessionInstanceID = req.SessionID
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
		responses[i] = toCalendarDayResponse(d)
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
	if existing != nil {
		currentKind = existing.Kind
	}
	if err := s.validateDayFields(req, currentKind); err != nil {
		return nil, err
	}
	row, err := s.buildRow(ctx, groupID, date, req)
	if err != nil {
		return nil, err
	}
	if err := s.calendarDao.Upsert(ctx, row); err != nil {
		customlogger.Error(ctx, "error upserting calendar day", err, customlogger.TagMethod("UpsertDay"))
		return nil, fmt.Errorf("error al guardar día de calendario")
	}
	resp := toCalendarDayResponse(*row)
	return &resp, nil
}

func (s *calendarService) DeleteDay(ctx *gin.Context, groupID, callerID int64, date time.Time) error {
	if err := s.isGroupOwner(ctx, groupID, callerID); err != nil {
		return err
	}
	if err := s.calendarDao.Delete(ctx, groupID, date); err != nil {
		customlogger.Error(ctx, "error deleting calendar day", err, customlogger.TagMethod("DeleteDay"))
		return fmt.Errorf("error al borrar día de calendario")
	}
	return nil
}

func toCalendarDayResponse(d dbs.GroupCalendarDay) calendar.CalendarDayResponse {
	resp := calendar.CalendarDayResponse{
		ID: d.ID, GroupID: d.GroupID, Date: d.Date.Format("2006-01-02"), Kind: d.Kind,
		OtherName: d.OtherName, SessionID: d.SessionInstanceID, CancelledReason: d.CancelledReason,
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
	return resp
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

	rows := make([]dbs.GroupCalendarDay, len(planDays))
	targetDates := make([]time.Time, len(planDays))
	for i, pd := range planDays {
		date := startDate.AddDate(0, 0, pd.SequenceNo-1)
		targetDates[i] = date
		row := dbs.GroupCalendarDay{
			GroupID: groupID, Date: date, Kind: pd.Kind, OtherName: pd.OtherName, SessionInstanceID: pd.SessionID,
			IsPresencial: pd.DefaultPresencial, PresencialTimeFrom: pd.DefaultTimeFrom, PresencialTimeTo: pd.DefaultTimeTo, PresencialLocation: pd.DefaultLocation,
			SourcePlanID: &req.PlanID,
		}
		rows[i] = row
	}

	if !req.Force {
		minDate, maxDate := targetDates[0], targetDates[0]
		for _, d := range targetDates {
			if d.Before(minDate) {
				minDate = d
			}
			if d.After(maxDate) {
				maxDate = d
			}
		}
		existing, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, minDate, maxDate)
		if err != nil {
			return nil, fmt.Errorf("error al validar conflictos")
		}
		occupied := make(map[string]bool, len(existing))
		for _, e := range existing {
			occupied[e.Date.Format("2006-01-02")] = true
		}
		conflict := false
		for _, d := range targetDates {
			if occupied[d.Format("2006-01-02")] {
				conflict = true
				break
			}
		}
		if conflict {
			return nil, ErrCalendarStampConflict
		}
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		for i := range rows {
			if err := txCalendarDao.Upsert(ctx, &rows[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		customlogger.Error(ctx, "error stamping calendar day", err, customlogger.TagMethod("Stamp"))
		return nil, fmt.Errorf("error al estampar plan")
	}

	responses := make([]calendar.CalendarDayResponse, len(rows))
	for i := range rows {
		responses[i] = toCalendarDayResponse(rows[i])
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
	if err := s.validateDayFields(dayReq, ""); err != nil {
		return nil, err
	}
	responses := make([]calendar.CalendarDayResponse, 0, len(req.Dates))
	// validationErr distingue un error de validación de request (fecha/día
	// inválido) de un error real de DB — GORM's Transaction solo devuelve el
	// error del closure, sin tipo propio para diferenciarlos afuera.
	var validationErr error
	err := s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		for _, dateStr := range req.Dates {
			date, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				validationErr = fmt.Errorf("fecha inválida en dates: %s", dateStr)
				return validationErr
			}
			row, err := s.buildRow(ctx, groupID, date, dayReq)
			if err != nil {
				validationErr = err
				return err
			}
			if err := txCalendarDao.Upsert(ctx, row); err != nil {
				return err
			}
			responses = append(responses, toCalendarDayResponse(*row))
		}
		return nil
	})
	if validationErr != nil {
		return nil, validationErr
	}
	if err != nil {
		customlogger.Error(ctx, "error bulk-upserting calendar day", err, customlogger.TagMethod("Bulk"))
		return nil, fmt.Errorf("error al aplicar bulk")
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
	if err := s.calendarDao.DeleteByDates(ctx, groupID, dates); err != nil {
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
	farFuture := fromDate.AddDate(1, 0, 0)
	affected, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, fromDate, farFuture)
	if err != nil {
		return nil, fmt.Errorf("error al buscar filas a correr")
	}
	before, err := s.calendarDao.FindByGroupAndRange(ctx, groupID, fromDate.AddDate(0, 0, -365), fromDate.AddDate(0, 0, -1))
	if err != nil {
		return nil, fmt.Errorf("error al validar colisiones")
	}
	unaffectedDates := make(map[string]bool, len(before))
	for _, b := range before {
		unaffectedDates[b.Date.Format("2006-01-02")] = true
	}
	for _, a := range affected {
		newDate := a.Date.AddDate(0, 0, req.Days)
		if unaffectedDates[newDate.Format("2006-01-02")] {
			return nil, ErrCalendarShiftCollision
		}
	}
	responses := make([]calendar.CalendarDayResponse, len(affected))
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)
		for i, a := range affected {
			newDate := a.Date.AddDate(0, 0, req.Days)
			if err := txCalendarDao.UpdateDatesForShift(ctx, groupID, a.Date, newDate); err != nil {
				return err
			}
			a.Date = newDate
			responses[i] = toCalendarDayResponse(a)
		}
		return nil
	})
	if err != nil {
		customlogger.Error(ctx, "error shifting calendar day", err, customlogger.TagMethod("Shift"))
		return nil, fmt.Errorf("error al correr fechas")
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
		GroupID: day.GroupID, Date: day.Date.Format("2006-01-02"), SessionID: day.SessionInstanceID, IsPresencial: day.IsPresencial,
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

func (s *calendarService) AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error) {
	groupIDs, err := s.calendarDao.FindDistinctGroupsBySession(ctx, sessionID)
	if err != nil {
		customlogger.Error(ctx, "error finding assigned groups", err, customlogger.TagMethod("AssignedGroups"))
		return nil, fmt.Errorf("error al buscar grupos asignados")
	}
	items := make([]calendar.CalendarSummaryItem, 0, len(groupIDs))
	for _, id := range groupIDs {
		group, err := s.groupDao.FindByID(ctx, id)
		if err != nil || group == nil {
			continue
		}
		items = append(items, calendar.CalendarSummaryItem{GroupID: group.ID, GroupName: group.Name})
	}
	return items, nil
}
