package services

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

var (
	ErrSessionNotFound         = errors.New("sesión no encontrada")
	ErrSessionMissingRole      = errors.New("la sesión debe tener al menos un ejercicio de cada rol")
	ErrSessionInvalidRole      = errors.New("role inválido")
	ErrSessionExerciseNotFound = errors.New("ejercicio referenciado no encontrado")
)

type SessionServiceInterface interface {
	Create(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	Update(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error)
	Delete(ctx *gin.Context, id, callerID int64) error
	Clone(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error)
	Get(ctx *gin.Context, id int64) (*session.SessionResponse, error)
	List(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error)
}

type sessionService struct {
	sessionDao         daos.SessionDaoInterface
	sessionExerciseDao daos.SessionExerciseDaoInterface
	exerciseDao        daos.ExerciseDaoInterface
}

func NewSessionService(sessionDao daos.SessionDaoInterface, sessionExerciseDao daos.SessionExerciseDaoInterface, exerciseDao daos.ExerciseDaoInterface) SessionServiceInterface {
	return &sessionService{sessionDao: sessionDao, sessionExerciseDao: sessionExerciseDao, exerciseDao: exerciseDao}
}

func (s *sessionService) validateExercises(ctx *gin.Context, items []session.SessionExerciseRequest) error {
	roleCounts := map[string]int{}
	for _, item := range items {
		if !constants.IsValidSessionExerciseRole(item.Role) {
			return ErrSessionInvalidRole
		}
		roleCounts[item.Role]++
		ex, err := s.exerciseDao.FindByID(ctx, item.ExerciseID)
		if err != nil {
			return fmt.Errorf("error al validar ejercicios de la sesión")
		}
		if ex == nil {
			return ErrSessionExerciseNotFound
		}
	}
	for _, role := range constants.GetValidSessionExerciseRoles() {
		if roleCounts[role] < 1 {
			return ErrSessionMissingRole
		}
	}
	return nil
}

func toSessionExerciseRows(items []session.SessionExerciseRequest) []dbs.SessionExercise {
	rows := make([]dbs.SessionExercise, len(items))
	for i, item := range items {
		repeatCount := 1
		if item.RepeatCount != nil {
			repeatCount = *item.RepeatCount
		}
		restMinutes := 0
		if item.RestMinutes != nil {
			restMinutes = *item.RestMinutes
		}
		rows[i] = dbs.SessionExercise{ExerciseID: item.ExerciseID, Role: item.Role, RepeatCount: repeatCount, RestMinutes: restMinutes}
	}
	return rows
}

func (s *sessionService) toResponse(ctx *gin.Context, sessionDB *dbs.Session) (*session.SessionResponse, error) {
	rows, err := s.sessionExerciseDao.FindBySession(ctx, sessionDB.ID)
	if err != nil {
		customlogger.Error(ctx, "error loading session exercises", err, customlogger.TagMethod("toResponse"))
		return nil, fmt.Errorf("error al armar la respuesta de la sesión")
	}
	exercises := make([]session.SessionExerciseResponse, len(rows))
	for i, r := range rows {
		exercises[i] = session.SessionExerciseResponse{ID: r.ID, ExerciseID: r.ExerciseID, Role: r.Role, RepeatCount: r.RepeatCount, RestMinutes: r.RestMinutes}
	}
	return &session.SessionResponse{
		ID: sessionDB.ID, OwnerID: sessionDB.OwnerID, Name: sessionDB.Name, Description: sessionDB.Description,
		Exercises: exercises, CreatedAt: sessionDB.CreatedAt, UpdatedAt: sessionDB.UpdatedAt,
	}, nil
}

func (s *sessionService) Create(ctx *gin.Context, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	if req.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := s.validateExercises(ctx, req.Exercises); err != nil {
		return nil, err
	}
	sessionDB := &dbs.Session{OwnerID: req.OwnerID, Name: req.Name, Description: req.Description}
	if err := s.sessionDao.Create(ctx, sessionDB); err != nil {
		customlogger.Error(ctx, "error creating session", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear sesión")
	}
	if err := s.sessionExerciseDao.ReplaceForSession(ctx, sessionDB.ID, toSessionExerciseRows(req.Exercises)); err != nil {
		customlogger.Error(ctx, "error setting session exercises", err, customlogger.TagMethod("Create"))
		return nil, fmt.Errorf("error al crear sesión")
	}
	return s.toResponse(ctx, sessionDB)
}

func (s *sessionService) Update(ctx *gin.Context, id, callerID int64, req session.SessionRequest) (*session.SessionResponse, error) {
	existing, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión")
	}
	if existing == nil {
		return nil, ErrSessionNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	if err := s.validateExercises(ctx, req.Exercises); err != nil {
		return nil, err
	}
	existing.Name = req.Name
	existing.Description = req.Description
	if err := s.sessionDao.Update(ctx, existing); err != nil {
		customlogger.Error(ctx, "error updating session", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión")
	}
	if err := s.sessionExerciseDao.ReplaceForSession(ctx, id, toSessionExerciseRows(req.Exercises)); err != nil {
		customlogger.Error(ctx, "error replacing session exercises", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión")
	}
	return s.toResponse(ctx, existing)
}

func (s *sessionService) Delete(ctx *gin.Context, id, callerID int64) error {
	existing, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar sesión")
	}
	if existing == nil {
		return ErrSessionNotFound
	}
	if existing.OwnerID != callerID {
		return ErrCatalogForbidden
	}
	if err := s.sessionDao.SoftDelete(ctx, id); err != nil {
		customlogger.Error(ctx, "error soft-deleting session", err, customlogger.TagMethod("Delete"))
		return fmt.Errorf("error al borrar sesión")
	}
	return nil
}

func (s *sessionService) Clone(ctx *gin.Context, id, callerID int64) (*session.SessionResponse, error) {
	existing, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	if existing == nil {
		return nil, ErrSessionNotFound
	}
	if existing.OwnerID != callerID {
		return nil, ErrCatalogForbidden
	}
	rows, err := s.sessionExerciseDao.FindBySession(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error loading session exercises", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	clone := &dbs.Session{OwnerID: existing.OwnerID, Name: existing.Name + " (copia)", Description: existing.Description}
	if err := s.sessionDao.Create(ctx, clone); err != nil {
		customlogger.Error(ctx, "error cloning session", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	clonedRows := make([]dbs.SessionExercise, len(rows))
	for i, r := range rows {
		clonedRows[i] = dbs.SessionExercise{ExerciseID: r.ExerciseID, Role: r.Role, RepeatCount: r.RepeatCount, RestMinutes: r.RestMinutes}
	}
	if err := s.sessionExerciseDao.ReplaceForSession(ctx, clone.ID, clonedRows); err != nil {
		customlogger.Error(ctx, "error setting cloned session exercises", err, customlogger.TagMethod("Clone"))
		return nil, fmt.Errorf("error al clonar sesión")
	}
	return s.toResponse(ctx, clone)
}

func (s *sessionService) Get(ctx *gin.Context, id int64) (*session.SessionResponse, error) {
	sessionDB, err := s.sessionDao.FindByID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding session", err, customlogger.TagMethod("Get"))
		return nil, fmt.Errorf("error al buscar sesión")
	}
	if sessionDB == nil {
		return nil, ErrSessionNotFound
	}
	return s.toResponse(ctx, sessionDB)
}

func (s *sessionService) List(ctx *gin.Context, ownerID int64) ([]session.SessionResponse, error) {
	sessions, err := s.sessionDao.FindByOwner(ctx, ownerID)
	if err != nil {
		customlogger.Error(ctx, "error listing sessions", err, customlogger.TagMethod("List"))
		return nil, fmt.Errorf("error al listar sesiones")
	}
	responses := make([]session.SessionResponse, len(sessions))
	for i := range sessions {
		resp, err := s.toResponse(ctx, &sessions[i])
		if err != nil {
			return nil, err
		}
		responses[i] = *resp
	}
	return responses, nil
}
