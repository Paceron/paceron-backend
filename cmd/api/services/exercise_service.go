package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/exercise"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrExerciseNotFound           = errors.New("ejercicio no encontrado")
	ErrExerciseInvalidKind        = errors.New("kind inválido")
	ErrExerciseInvalidIntensity   = errors.New("intensity inválido")
	ErrExerciseInvalidMuscleGroup = errors.New("muscle_group inválido")
	// ErrCatalogForbidden es compartido por ExerciseService/SessionService/
	// TrainingPlanService (D9) — declarado una sola vez acá.
	ErrCatalogForbidden = errors.New("no autorizado")
)

type ExerciseServiceInterface interface {
	Create(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	Update(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error)
	Delete(ctx *gin.Context, id, callerID int64) error
	Clone(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error)
	Get(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error)
	List(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error)
}

type exerciseService struct {
	exerciseDao daos.ExerciseDaoInterface
}

func NewExerciseService(exerciseDao daos.ExerciseDaoInterface) ExerciseServiceInterface {
	return &exerciseService{exerciseDao: exerciseDao}
}

func validateExerciseRequest(req exercise.ExerciseRequest) error {
	if !constants.IsValidExerciseKind(req.Kind) {
		return ErrExerciseInvalidKind
	}
	if req.Intensity != nil && !constants.IsValidExerciseIntensity(*req.Intensity) {
		return ErrExerciseInvalidIntensity
	}
	if req.MuscleGroup != nil && !constants.IsValidMuscleGroup(*req.MuscleGroup) {
		return ErrExerciseInvalidMuscleGroup
	}
	return nil
}

func (s *exerciseService) Create(ctx *gin.Context, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	if req.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := validateExerciseRequest(req); err != nil {
		return nil, err
	}
	e := &dbs.Exercise{
		OwnerID: req.OwnerID, Name: req.Name, Description: req.Description, Kind: req.Kind,
		Intensity: req.Intensity, Minutes: req.Minutes, DistanceM: req.DistanceM,
		SpeedKph: req.SpeedKph, MuscleGroup: req.MuscleGroup,
	}
	if err := s.exerciseDao.Create(ctx, e); err != nil {
		customlogger.Error(ctx, "error creating exercise", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear ejercicio")
	}
	return toExerciseResponse(e), nil
}

func (s *exerciseService) Update(ctx *gin.Context, id, callerID int64, req exercise.ExerciseRequest) (*exercise.ExerciseResponse, error) {
	existing, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar ejercicio")
	}
	if existing == nil {
		return nil, ErrExerciseNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := validateExerciseRequest(req); err != nil {
		return nil, err
	}
	existing.Name = req.Name
	existing.Description = req.Description
	existing.Kind = req.Kind
	existing.Intensity = req.Intensity
	existing.Minutes = req.Minutes
	existing.DistanceM = req.DistanceM
	existing.SpeedKph = req.SpeedKph
	existing.MuscleGroup = req.MuscleGroup
	if err := s.exerciseDao.Update(ctx, existing); err != nil {
		customlogger.Error(ctx, "error updating exercise", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar ejercicio")
	}
	return toExerciseResponse(existing), nil
}

func (s *exerciseService) Delete(ctx *gin.Context, id, callerID int64) error {
	existing, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar ejercicio")
	}
	if existing == nil {
		return ErrExerciseNotFound
	}
	if existing.OwnerID != callerID {
		return ErrCatalogForbidden
	}
	if err := s.exerciseDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error soft-deleting exercise", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar ejercicio")
	}
	return nil
}

func (s *exerciseService) Clone(ctx *gin.Context, id, callerID int64) (*exercise.ExerciseResponse, error) {
	existing, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar ejercicio")
	}
	if existing == nil {
		return nil, ErrExerciseNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	clone := &dbs.Exercise{
		OwnerID: existing.OwnerID, Name: existing.Name + " (copia)", Description: existing.Description,
		Kind: existing.Kind, Intensity: existing.Intensity, Minutes: existing.Minutes,
		DistanceM: existing.DistanceM, SpeedKph: existing.SpeedKph, MuscleGroup: existing.MuscleGroup,
	}
	if err := s.exerciseDao.Create(ctx, clone); err != nil {
		customlogger.Error(ctx, "error cloning exercise", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar ejercicio")
	}
	return toExerciseResponse(clone), nil
}

func (s *exerciseService) Get(ctx *gin.Context, id int64) (*exercise.ExerciseResponse, error) {
	e, err := s.exerciseDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding exercise", err, customlogger.TagMethod("Get"))
		return nil, fmt.Errorf("error al buscar ejercicio")
	}
	if e == nil {
		return nil, ErrExerciseNotFound
	}
	return toExerciseResponse(e), nil
}

func (s *exerciseService) List(ctx *gin.Context, ownerID int64) ([]exercise.ExerciseResponse, error) {
	exercises, err := s.exerciseDao.FindByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error listing exercises", err, customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar ejercicios")
	}
	responses := make([]exercise.ExerciseResponse, len(exercises))
	for i := range exercises {
		responses[i] = *toExerciseResponse(&exercises[i])
	}
	return responses, nil
}

func toExerciseResponse(e *dbs.Exercise) *exercise.ExerciseResponse {
	return &exercise.ExerciseResponse{
		ID: e.ID, OwnerID: e.OwnerID, Name: e.Name, Description: e.Description, Kind: e.Kind,
		Intensity: e.Intensity, Minutes: e.Minutes, DistanceM: e.DistanceM, SpeedKph: e.SpeedKph,
		MuscleGroup: e.MuscleGroup, VideoURL: e.VideoURL, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}
