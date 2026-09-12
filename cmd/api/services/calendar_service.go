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
	if isPresencial && (req.PresencialTime == nil || req.PresencialLocation == nil) {
		return ErrCalendarFieldMismatch
	}
	return nil
}

func (s *calendarService) buildRow(ctx *gin.Context, groupID int64, date time.Time, req calendar.CalendarDayRequest) (*dbs.GroupCalendarDay, error) {
	row := &dbs.GroupCalendarDay{GroupID: groupID, Date: date, Kind: req.Kind}
	switch req.Kind {
	case string(constants.GroupCalendarDayKindOther):
		row.OtherName = req.OtherName
	case string(constants.GroupCalendarDayKindTraining):
		row.SessionID = req.SessionID
	case string(constants.GroupCalendarDayKindCancelled):
		row.CancelledReason = req.CancelledReason
	}
	if req.IsPresencial != nil && *req.IsPresencial {
		row.IsPresencial = true
		parsedTime, err := time.Parse("15:04", *req.PresencialTime)
		if err != nil {
			return nil, fmt.Errorf("presencial_time debe tener formato HH:MM")
		}
		row.PresencialTime = &parsedTime
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
		OtherName: d.OtherName, SessionID: d.SessionID, CancelledReason: d.CancelledReason,
		IsPresencial: d.IsPresencial, SourcePlanID: d.SourcePlanID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
	if d.PresencialTime != nil {
		formatted := d.PresencialTime.Format("15:04")
		resp.PresencialTime = &formatted
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

// Stamp, Bulk, BulkClear, Shift, NextSession, CalendarSummary y
// AssignedGroups son placeholders intencionales — Tasks 5, 6 y 7
// reemplazan cada uno con la lógica real. Sin estos métodos el struct
// calendarService no implementaría CalendarServiceInterface y el build
// no compilaría.

func (s *calendarService) Stamp(ctx *gin.Context, groupID, callerID int64, req calendar.StampRequest) ([]calendar.CalendarDayResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 5")
}
func (s *calendarService) Bulk(ctx *gin.Context, groupID, callerID int64, req calendar.BulkRequest) ([]calendar.CalendarDayResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 6")
}
func (s *calendarService) BulkClear(ctx *gin.Context, groupID, callerID int64, req calendar.BulkClearRequest) error {
	return fmt.Errorf("no implementado todavía — ver Task 6")
}
func (s *calendarService) Shift(ctx *gin.Context, groupID, callerID int64, req calendar.ShiftRequest) ([]calendar.CalendarDayResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 6")
}
func (s *calendarService) NextSession(ctx *gin.Context, userID int64) (*calendar.NextSessionResponse, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 7")
}
func (s *calendarService) CalendarSummary(ctx *gin.Context, userID int64) ([]calendar.CalendarSummaryItem, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 7")
}
func (s *calendarService) AssignedGroups(ctx *gin.Context, sessionID int64) ([]calendar.CalendarSummaryItem, error) {
	return nil, fmt.Errorf("no implementado todavía — ver Task 7")
}
