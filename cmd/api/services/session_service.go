package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
	sessionDao          daos.SessionDaoInterface
	sessionExerciseDao  daos.SessionExerciseDaoInterface
	exerciseDao         daos.ExerciseDaoInterface
	groupCalendarDayDao daos.GroupCalendarDaoInterface
	db                  *gorm.DB
}

func NewSessionService(
	sessionDao daos.SessionDaoInterface,
	sessionExerciseDao daos.SessionExerciseDaoInterface,
	exerciseDao daos.ExerciseDaoInterface,
	groupCalendarDayDao daos.GroupCalendarDaoInterface,
	db *gorm.DB,
) SessionServiceInterface {
	return &sessionService{
		sessionDao: sessionDao, sessionExerciseDao: sessionExerciseDao, exerciseDao: exerciseDao,
		groupCalendarDayDao: groupCalendarDayDao, db: db,
	}
}

// cloneSessionInternal crea una copia de original (sesión + ejercicios) usando
// las DAOs recibidas — permite reusar la misma lógica tanto sobre las DAOs
// "normales" del service (Clone) como sobre DAOs frescas atadas a una
// transacción (rama de divergencia en Update).
//
// deepCloneExercises controla si además se clona cada Exercise referenciado
// (congelamiento histórico, ver design.md D4/D6 de congelar-ejercicio-en-clon):
// false para el clonado manual (POST /sessions/{id}/clone, comparte
// exercise_id a propósito), true para cualquier congelamiento por cierre de
// día (D8 manual, D13 automático, o el trigger nuevo desde Exercise.Update) —
// sin esto, el clon de sesión seguiría apuntando a Exercise vivos y el
// congelamiento no protegería contra editar el Exercise directamente.
func cloneSessionInternal(sessionDao daos.SessionDaoInterface, sessionExerciseDao daos.SessionExerciseDaoInterface, exerciseDao daos.ExerciseDaoInterface, ctx *gin.Context, original *dbs.Session, name, description *string, deepCloneExercises bool) (*dbs.Session, error) {
	rows, err := sessionExerciseDao.FindBySession(ctx, original.ID)
	if err != nil {
		return nil, fmt.Errorf("error al leer ejercicios de la sesión original")
	}
	cloneName := original.Name + " (copia)"
	if name != nil {
		cloneName = *name
	}
	cloneDescription := original.Description
	if description != nil {
		cloneDescription = description
	}
	clone := &dbs.Session{OwnerID: original.OwnerID, Name: cloneName, Description: cloneDescription}
	if err := sessionDao.Create(ctx, clone); err != nil {
		return nil, fmt.Errorf("error al crear sesión clonada")
	}
	clonedExerciseIDs := map[int64]int64{}
	clonedRows := make([]dbs.SessionExercise, len(rows))
	for i, r := range rows {
		exerciseID := r.ExerciseID
		if deepCloneExercises {
			if cachedID, ok := clonedExerciseIDs[r.ExerciseID]; ok {
				exerciseID = cachedID
			} else {
				originalExercise, err := exerciseDao.FindByID(ctx, r.ExerciseID)
				if err != nil {
					return nil, fmt.Errorf("error al leer ejercicio original para congelar")
				}
				if originalExercise != nil {
					clonedExercise := &dbs.Exercise{
						OwnerID: originalExercise.OwnerID, Name: originalExercise.Name, Description: originalExercise.Description,
						Kind: originalExercise.Kind, Intensity: originalExercise.Intensity, Minutes: originalExercise.Minutes,
						DistanceM: originalExercise.DistanceM, SpeedKph: originalExercise.SpeedKph, MuscleGroup: originalExercise.MuscleGroup,
						VideoURL: originalExercise.VideoURL,
					}
					if err := exerciseDao.Create(ctx, clonedExercise); err != nil {
						return nil, fmt.Errorf("error al congelar ejercicio de la sesión")
					}
					exerciseID = clonedExercise.ID
				}
				clonedExerciseIDs[r.ExerciseID] = exerciseID
			}
		}
		clonedRows[i] = dbs.SessionExercise{ExerciseID: exerciseID, Role: r.Role, RepeatCount: r.RepeatCount, RestMinutes: r.RestMinutes}
	}
	if err := sessionExerciseDao.ReplaceForSession(ctx, clone.ID, clonedRows); err != nil {
		return nil, fmt.Errorf("error al copiar ejercicios al clon")
	}
	return clone, nil
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

	referencingDays, err := s.groupCalendarDayDao.FindBySessionID(ctx, id)
	if err != nil {
		customlogger.Error(ctx, "error finding calendar days referencing session", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión")
	}

	excludeGroupIDs := []int64{}
	if req.ExcludeGroupIDs != nil {
		excludeGroupIDs = *req.ExcludeGroupIDs
	}
	excludeSet := make(map[int64]bool, len(excludeGroupIDs))
	for _, gid := range excludeGroupIDs {
		excludeSet[gid] = true
	}

	now := time.Now()
	var autoClosedDayIDs []int64
	for _, day := range referencingDays {
		if excludeSet[day.GroupID] {
			continue
		}
		if isCalendarDayClosed(day, now) {
			autoClosedDayIDs = append(autoClosedDayIDs, day.ID)
		}
	}

	if len(excludeGroupIDs) == 0 && len(autoClosedDayIDs) == 0 {
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

	err = s.db.Transaction(func(tx *gorm.DB) error {
		txSessionDao := daos.NewSessionDao(tx)
		txSessionExerciseDao := daos.NewSessionExerciseDao(tx)
		txExerciseDao := daos.NewExerciseDao(tx)
		txCalendarDao := daos.NewGroupCalendarDayDao(tx)

		clone, err := cloneSessionInternal(txSessionDao, txSessionExerciseDao, txExerciseDao, ctx, existing, req.CloneName, req.CloneDescription, true)
		if err != nil {
			return err
		}
		if len(excludeGroupIDs) > 0 {
			if err := txCalendarDao.RepointSessionForGroups(ctx, excludeGroupIDs, id, clone.ID); err != nil {
				return fmt.Errorf("error al repuntear grupos excluidos")
			}
		}
		if len(autoClosedDayIDs) > 0 {
			if err := txCalendarDao.RepointDaysByID(ctx, autoClosedDayIDs, clone.ID); err != nil {
				return fmt.Errorf("error al repuntear días ya cerrados")
			}
		}
		existing.Name = req.Name
		existing.Description = req.Description
		if err := txSessionDao.Update(ctx, existing); err != nil {
			return fmt.Errorf("error al editar sesión original")
		}
		return txSessionExerciseDao.ReplaceForSession(ctx, id, toSessionExerciseRows(req.Exercises))
	})
	if err != nil {
		customlogger.Error(ctx, "error in divergence-clone update", err, customlogger.TagMethod("Update"))
		return nil, fmt.Errorf("error al editar sesión con exclusión de grupos")
	}
	return s.toResponse(ctx, existing)
}

// isCalendarDayClosed decide si un GroupCalendarDay ya no debe recibir la
// edición en vivo de la sesión que referencia — ver D13 del change de
// calendario. Sin cron: se calcula al vuelo contra `now` en cada PUT.
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
	clone, err := cloneSessionInternal(s.sessionDao, s.sessionExerciseDao, s.exerciseDao, ctx, existing, nil, nil, false)
	if err != nil {
		customlogger.Error(ctx, "error cloning session", err, customlogger.TagMethod("Clone"))
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
