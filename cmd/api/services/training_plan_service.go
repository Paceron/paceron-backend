package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrPlanNotFound          = errors.New("plan no encontrado")
	ErrPlanInvalidDayCount   = errors.New("el plan debe tener entre 2 y 31 días")
	ErrPlanInvalidSequence   = errors.New("sequence_no debe cubrir 1..N sin huecos ni repetidos")
	ErrPlanInvalidDayKind    = errors.New("kind de día inválido")
	ErrPlanDayFieldMismatch  = errors.New("combinación de campos inválida para el kind del día")
	ErrPlanSessionNotFound   = errors.New("session_id referenciado no encontrado")
	ErrPlanInvalidTimeFormat = errors.New("default_time debe tener formato HH:MM")
)

type TrainingPlanServiceInterface interface {
	Create(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error)
	Update(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error)
	Delete(ctx *gin.Context, id, callerID int64) error
	Clone(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error)
	Get(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error)
	List(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error)
}

type trainingPlanService struct {
	trainingPlanDao daos.TrainingPlanDaoInterface
	planDayDao      daos.PlanDayDaoInterface
	sessionDao      daos.SessionDaoInterface
}

func NewTrainingPlanService(trainingPlanDao daos.TrainingPlanDaoInterface, planDayDao daos.PlanDayDaoInterface, sessionDao daos.SessionDaoInterface) TrainingPlanServiceInterface {
	return &trainingPlanService{trainingPlanDao: trainingPlanDao, planDayDao: planDayDao, sessionDao: sessionDao}
}

func (s *trainingPlanService) validateAndBuildDays(ctx *gin.Context, days []trainingplan.PlanDayRequest) ([]dbs.PlanDay, error) {
	n := len(days)
	if n < 2 || n > 31 {
		return nil, ErrPlanInvalidDayCount
	}
	seen := make(map[int]bool, n)
	for _, d := range days {
		if d.SequenceNo < 1 || d.SequenceNo > n || seen[d.SequenceNo] {
			return nil, ErrPlanInvalidSequence
		}
		seen[d.SequenceNo] = true
	}

	rows := make([]dbs.PlanDay, n)
	for i, d := range days {
		if !constants.IsValidPlanDayKind(d.Kind) {
			return nil, ErrPlanInvalidDayKind
		}
		switch d.Kind {
		case string(constants.PlanDayKindTraining):
			if d.SessionID == nil || d.OtherName != nil {
				return nil, ErrPlanDayFieldMismatch
			}
			sessionDB, err := s.sessionDao.FindByID(ctx, *d.SessionID)
			if err != nil {
				return nil, fmt.Errorf("error al validar sesión del día")
			}
			if sessionDB == nil {
				return nil, ErrPlanSessionNotFound
			}
		case string(constants.PlanDayKindOther):
			if d.OtherName == nil || d.SessionID != nil {
				return nil, ErrPlanDayFieldMismatch
			}
		case string(constants.PlanDayKindRest):
			if d.OtherName != nil || d.SessionID != nil {
				return nil, ErrPlanDayFieldMismatch
			}
		}

		defaultPresencial := false
		if d.DefaultPresencial != nil {
			defaultPresencial = *d.DefaultPresencial
		}

		row := dbs.PlanDay{
			SequenceNo:        d.SequenceNo,
			Kind:              d.Kind,
			OtherName:         d.OtherName,
			SessionID:         d.SessionID,
			DefaultPresencial: defaultPresencial,
		}

		if defaultPresencial {
			if d.DefaultTime == nil || d.DefaultLocation == nil {
				return nil, ErrPlanDayFieldMismatch
			}
			parsedTime, err := time.Parse("15:04", *d.DefaultTime)
			if err != nil {
				return nil, ErrPlanInvalidTimeFormat
			}
			row.DefaultTime = &parsedTime
			locationJSON, err := json.Marshal(d.DefaultLocation)
			if err != nil {
				return nil, fmt.Errorf("error al serializar la ubicación del día")
			}
			locationStr := string(locationJSON)
			row.DefaultLocation = &locationStr
		}

		rows[i] = row
	}
	return rows, nil
}

func toPlanDayResponse(d dbs.PlanDay) trainingplan.PlanDayResponse {
	resp := trainingplan.PlanDayResponse{
		ID: d.ID, SequenceNo: d.SequenceNo, Kind: d.Kind, OtherName: d.OtherName,
		SessionID: d.SessionID, DefaultPresencial: d.DefaultPresencial,
	}
	if d.DefaultTime != nil {
		formatted := d.DefaultTime.Format("15:04")
		resp.DefaultTime = &formatted
	}
	if d.DefaultLocation != nil {
		var loc trainingplan.Location
		if err := json.Unmarshal([]byte(*d.DefaultLocation), &loc); err == nil {
			resp.DefaultLocation = &loc
		}
	}
	return resp
}

func (s *trainingPlanService) toResponse(ctx *gin.Context, planDB *dbs.TrainingPlan) (*trainingplan.TrainingPlanResponse, error) {
	days, err := s.planDayDao.FindByPlan(ctx, planDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error loading plan days", err, customlogger.TagMethod("toResponse"))
		return nil, fmt.Errorf("error al armar la respuesta del plan")
	}
	dayResponses := make([]trainingplan.PlanDayResponse, len(days))
	for i, d := range days {
		dayResponses[i] = toPlanDayResponse(d)
	}
	return &trainingplan.TrainingPlanResponse{
		ID: planDB.ID, OwnerID: planDB.OwnerID, Name: planDB.Name, Description: planDB.Description,
		Days: dayResponses, CreatedAt: planDB.CreatedAt, UpdatedAt: planDB.UpdatedAt,
	}, nil
}

func (s *trainingPlanService) Create(ctx *gin.Context, callerID int64, req trainingplan.TrainingPlanRequest) (*trainingplan.TrainingPlanResponse, error) {
	if req.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	rows, err := s.validateAndBuildDays(ctx, req.Days)
	if err != nil {
		return nil, err
	}
	planDB := &dbs.TrainingPlan{OwnerID: req.OwnerID, Name: req.Name, Description: req.Description}
	if err := s.trainingPlanDao.Create(ctx, planDB); err != nil {
		customlogger.Error(ctx, "error creating training plan", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear plan")
	}
	if err := s.planDayDao.ReplaceForPlan(ctx, planDB.ID, rows); err != nil {
		customlogger.Error(ctx, "error setting plan days", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear plan")
	}
	return s.toResponse(ctx, planDB)
}

func (s *trainingPlanService) Update(ctx *gin.Context, id, callerID int64, req trainingplan.TrainingPlanUpdateRequest) (*trainingplan.TrainingPlanResponse, error) {
	planDB, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar plan")
	}
	if planDB == nil {
		return nil, ErrPlanNotFound
	}
	if planDB.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if req.Name != nil {
		planDB.Name = *req.Name
	}
	if req.Description != nil {
		planDB.Description = req.Description
	}
	if err := s.trainingPlanDao.Update(ctx, planDB); err != nil {
		customlogger.Error(ctx, "error updating training plan", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar plan")
	}
	if req.Days != nil {
		rows, err := s.validateAndBuildDays(ctx, *req.Days)
		if err != nil {
			return nil, err
		}
		if err := s.planDayDao.ReplaceForPlan(ctx, id, rows); err != nil {
			customlogger.Error(ctx, "error replacing plan days", err, customlogger.TagMethod("Update"))
			return nil, fmt.Errorf("error al editar plan")
		}
	}
	return s.toResponse(ctx, planDB)
}

func (s *trainingPlanService) Delete(ctx *gin.Context, id, callerID int64) error {
	planDB, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar plan")
	}
	if planDB == nil {
		return ErrPlanNotFound
	}
	if planDB.OwnerID != callerID {
		return ErrCatalogForbidden
	}
	if err := s.trainingPlanDao.Delete(ctx, id); err != nil {
		customlogger.Error(ctx, "error deleting training plan", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar plan")
	}
	return nil
}

func (s *trainingPlanService) Clone(ctx *gin.Context, id, callerID int64) (*trainingplan.TrainingPlanResponse, error) {
	original, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	if original == nil {
		return nil, ErrPlanNotFound
	}
	if original.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	originalDays, err := s.planDayDao.FindByPlan(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error loading plan days", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	clone := &dbs.TrainingPlan{OwnerID: original.OwnerID, Name: original.Name + " (copia)", Description: original.Description}
	if err := s.trainingPlanDao.Create(ctx, clone); err != nil {
		customlogger.Error(ctx, "error creating cloned plan", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	clonedDays := make([]dbs.PlanDay, len(originalDays))
	for i, d := range originalDays {
		clonedDays[i] = dbs.PlanDay{
			SequenceNo: d.SequenceNo, Kind: d.Kind, OtherName: d.OtherName, SessionID: d.SessionID,
			DefaultPresencial: d.DefaultPresencial, DefaultTime: d.DefaultTime, DefaultLocation: d.DefaultLocation,
		}
	}
	if err := s.planDayDao.ReplaceForPlan(ctx, clone.ID, clonedDays); err != nil {
		customlogger.Error(ctx, "error setting cloned plan days", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar plan")
	}
	return s.toResponse(ctx, clone)
}

func (s *trainingPlanService) Get(ctx *gin.Context, id int64) (*trainingplan.TrainingPlanResponse, error) {
	planDB, err := s.trainingPlanDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding training plan", err, customlogger.TagMethod("Get"))
		return nil, fmt.Errorf("error al buscar plan")
	}
	if planDB == nil {
		return nil, ErrPlanNotFound
	}
	return s.toResponse(ctx, planDB)
}

func (s *trainingPlanService) List(ctx *gin.Context, ownerID int64) ([]trainingplan.TrainingPlanResponse, error) {
	plans, err := s.trainingPlanDao.FindByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error listing training plans", err, customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar planes")
	}
	responses := make([]trainingplan.TrainingPlanResponse, len(plans))
	for i := range plans {
		resp, err := s.toResponse(ctx, &plans[i])
		if err != nil {
			return nil, err
		}
		responses[i] = *resp
	}
	return responses, nil
}
