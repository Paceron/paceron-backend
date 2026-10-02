package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
)

// Estados de runner_session (string sin constraint en DB; el POST crea en wip).
const (
	runnerSessionStatusWip         = "wip"
	runnerSessionStatusFinished    = "finished"
	runnerSessionStatusInterrupted = "interrupted"
)

// Errores de negocio del módulo de estado de sesión del corredor. El controller
// los mapea a 400/403/404 vía errors.Is.
var (
	ErrRunnerSessionInvalid    = errors.New("datos inválidos")
	ErrRunnerSessionForbidden  = errors.New("no tenés permisos para operar sobre este estado de sesión")
	ErrSessionInstanceNotFound = errors.New("sesión asignada no encontrada")
)

// RunnerSessionServiceInterface define las operaciones de negocio del estado de
// sesión del corredor (runner_session). El atleta es siempre el auth salvo que
// un entrenador autorizado (owner de un equipo al que pertenezca ese atleta) lo
// indique explícitamente; la creación es idempotente.
type RunnerSessionServiceInterface interface {
	// Create crea el estado en wip (start_date provista por el cliente) o, si ya
	// existía la fila (idempotencia), devuelve la actual. second return: true si
	// se creó, false si ya existía.
	Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error)
	// Finish aplica la transición de status pedida (finished|interrupted) con
	// end_date = now() del servidor. Idempotente si la fila ya está en el
	// status destino; finished→interrupted es rechazada (400).
	Finish(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error)
	// Get devuelve el estado actual (404 si no existe la fila).
	Get(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) (*dbs.RunnerSession, error)
}

type runnerSessionService struct {
	runnerSessionDao daos.RunnerSessionDAOInterface
	// presencial es opcional (nil en tests = sin gate ni hooks calendario):
	// el gate D9 del día presencial corre dentro de Create para respetar el
	// orden 404 instancia → 403 atleta ajeno → 409 del gate.
	presencial PresencialSessionServiceInterface
}

// NewRunnerSessionService crea una nueva instancia de RunnerSessionService.
func NewRunnerSessionService(runnerSessionDao daos.RunnerSessionDAOInterface, presencial PresencialSessionServiceInterface) RunnerSessionServiceInterface {
	return &runnerSessionService{
		runnerSessionDao: runnerSessionDao,
		presencial:       presencial,
	}
}

// Create valida la sesión, resuelve el atleta y crea el estado en wip de forma
// idempotente: si la fila ya existía devuelve la actual (created=false), sin
// pisar start_date ni bajar de finished a wip.
func (s *runnerSessionService) Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, bool, error) {
	exists, err := s.runnerSessionDao.SessionInstanceExists(ctx, sessionInstanceID)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, ErrSessionInstanceNotFound
	}
	if req.StartDate.IsZero() {
		return nil, false, fmt.Errorf("%w: start_date es obligatorio", ErrRunnerSessionInvalid)
	}

	athleteUserID, err := s.resolveAthlete(ctx, authUserID, req.AthleteUserID)
	if err != nil {
		return nil, false, err
	}

	// Gate presencial D9: después de resolveAthlete (403 precede al 409) y
	// antes del write — un día cerrado/sin abrir no acepta entrada del
	// corredor. Owner exento (CheckAthleteEntry lo resuelve).
	if s.presencial != nil {
		if err := s.presencial.CheckAthleteEntry(ctx, sessionInstanceID, authUserID); err != nil {
			return nil, false, err
		}
	}

	rs := &dbs.RunnerSession{
		SessionInstanceID: sessionInstanceID,
		AthleteUserID:     athleteUserID,
		Status:            "wip",
		StartDate:         req.StartDate,
	}
	created, err := s.runnerSessionDao.Create(ctx, rs)
	if err != nil {
		return nil, false, err
	}
	if !created {
		current, err := s.runnerSessionDao.GetBySessionAndAthlete(ctx, sessionInstanceID, athleteUserID)
		if err != nil {
			return nil, false, err
		}
		return current, false, nil
	}
	return rs, true, nil
}

// Finish aplica la transición de status pedida (finished|interrupted) con
// end_date seteada por el servidor. Idempotente: pedir el status que ya tiene
// la fila la devuelve sin cambios. finished→interrupted es ilegal (400) y una
// fila finished jamás cambia. Sin fila → ErrRunnerSessionNotFound (404).
//
// Legales: wip→finished, wip→interrupted, interrupted→finished (re-setea
// end_date = now, momento del finish explícito).
func (s *runnerSessionService) Finish(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
	if req.Status != runnerSessionStatusFinished && req.Status != runnerSessionStatusInterrupted {
		return nil, fmt.Errorf("%w: status solo admite 'finished' o 'interrupted'", ErrRunnerSessionInvalid)
	}

	athleteUserID, err := s.resolveAthlete(ctx, authUserID, req.AthleteUserID)
	if err != nil {
		return nil, err
	}

	current, err := s.runnerSessionDao.GetBySessionAndAthlete(ctx, sessionInstanceID, athleteUserID)
	if err != nil {
		return nil, err
	}
	if current.Status == req.Status {
		return current, nil
	}
	if current.Status == runnerSessionStatusFinished {
		return nil, fmt.Errorf("%w: no se puede interrumpir una sesión ya finalizada", ErrRunnerSessionInvalid)
	}

	now := time.Now()
	if err := s.runnerSessionDao.UpdateStatus(ctx, current.ID, req.Status, []string{runnerSessionStatusWip, runnerSessionStatusInterrupted}, now); err != nil {
		return nil, err
	}
	return s.runnerSessionDao.GetBySessionAndAthlete(ctx, sessionInstanceID, athleteUserID)
}

// Get devuelve el estado actual de la sesión para el atleta resuelto (self por
// default). Sin fila → 404.
func (s *runnerSessionService) Get(ctx *gin.Context, authUserID, sessionInstanceID int64, athleteUserID *int64) (*dbs.RunnerSession, error) {
	target, err := s.resolveAthlete(ctx, authUserID, athleteUserID)
	if err != nil {
		return nil, err
	}
	return s.runnerSessionDao.GetBySessionAndAthlete(ctx, sessionInstanceID, target)
}

// resolveAthlete aplica la matriz: sin athlete_user_id el atleta es el auth;
// si viene ajeno, el auth debe ser owner de un equipo al que pertenezca ese
// atleta (misma lógica que el módulo de feedback).
func (s *runnerSessionService) resolveAthlete(ctx *gin.Context, authUserID int64, athleteUserID *int64) (int64, error) {
	if athleteUserID == nil || *athleteUserID == authUserID {
		return authUserID, nil
	}
	if *athleteUserID <= 0 {
		return 0, fmt.Errorf("%w: athlete_user_id debe ser un número entero mayor a 0", ErrRunnerSessionInvalid)
	}
	inOwnedTeam, err := s.runnerSessionDao.ExistsUserInTeamOwnedBy(ctx, *athleteUserID, authUserID)
	if err != nil {
		return 0, err
	}
	if !inOwnedTeam {
		return 0, ErrRunnerSessionForbidden
	}
	return *athleteUserID, nil
}
