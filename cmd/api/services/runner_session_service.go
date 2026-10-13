package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/runnersession"
	"simple-arq-golang/cmd/api/utils/gametime"
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
	// Create crea el estado en wip (start_date provista por el cliente) o, si
	// ya existía la fila (idempotencia), devuelve la actual. created es true
	// solo si se creó. calendarDay es el día cargado por el gate presencial
	// (nil si no hay gate, día inexistente o no presencial): el controller lo
	// pasa al hook de apertura para evitar re-consultarlo.
	Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error)
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
	// competitionDAO es opcional (nil en tests viejos): valida la marca de
	// competencia del intento (E4.1) antes del write.
	competitionDAO daos.CompetitionDAOInterface
}

// NewRunnerSessionService crea una nueva instancia de RunnerSessionService.
// competitionDAO es un parámetro variadic opcional (E4.1): los call sites
// históricos de dos argumentos quedan intactos.
func NewRunnerSessionService(runnerSessionDao daos.RunnerSessionDAOInterface, presencial PresencialSessionServiceInterface, competitionDAO ...daos.CompetitionDAOInterface) RunnerSessionServiceInterface {
	svc := &runnerSessionService{
		runnerSessionDao: runnerSessionDao,
		presencial:       presencial,
	}
	if len(competitionDAO) > 0 && competitionDAO[0] != nil {
		svc.competitionDAO = competitionDAO[0]
	}
	return svc
}

// validateCompetitionIntent aplica las reglas del intento tagueado (D9): la
// competencia debe existir, estar active, el caller ser participante, el
// instante actual estar dentro del rango activo y respetar el anclaje de
// instancia en los dos sentidos (una route solo taguea sobre su propia
// instancia sintética; una instancia sintética solo acepta a su dueña, lo que
// también bloquea el mixeo de tipos). Cualquier falla es 400.
func (s *runnerSessionService) validateCompetitionIntent(ctx *gin.Context, sessionInstanceID, authUserID int64, competitionID *int64) error {
	if s.competitionDAO == nil {
		return fmt.Errorf("%w: competition_id no se puede resolver en este ámbito", ErrRunnerSessionInvalid)
	}
	comp, err := s.competitionDAO.FindByID(ctx, *competitionID)
	if err != nil {
		if errors.Is(err, daos.ErrCompetitionNotFound) {
			return fmt.Errorf("%w: la competencia indicada no existe", ErrRunnerSessionInvalid)
		}
		return fmt.Errorf("error al validar la competencia del intento")
	}
	if comp.Status != competition.StatusActive {
		return fmt.Errorf("%w: la competencia no está activa", ErrRunnerSessionInvalid)
	}
	isParticipant, err := s.competitionDAO.IsParticipant(ctx, comp.ID, authUserID)
	if err != nil {
		return fmt.Errorf("error al validar la competencia del intento")
	}
	if !isParticipant {
		return fmt.Errorf("%w: no sos participante de la competencia", ErrRunnerSessionInvalid)
	}
	now := gametime.Now()
	if now.Before(comp.StartAt) {
		return fmt.Errorf("%w: la competencia todavía no empezó", ErrRunnerSessionInvalid)
	}
	if comp.EndAt != nil && now.After(*comp.EndAt) {
		return fmt.Errorf("%w: la competencia ya terminó", ErrRunnerSessionInvalid)
	}
	if comp.Type == competition.TypeRoute {
		// Route SIEMPRE tiene instancia sintética (D1); el nil-check es
		// defensivo. Anchor distinto = instancia de otra competencia.
		if comp.RouteSessionInstanceID == nil || sessionInstanceID != *comp.RouteSessionInstanceID {
			return fmt.Errorf("%w: la competencia indicada no corresponde a esta sesión", ErrRunnerSessionInvalid)
		}
	}
	// Anclaje inverso: si la instancia pedida es la sintética de una route
	// (dueña en DB), solo la dueña puede taguearse sobre ella — cubre el
	// mixeo de tipos (comp metric/ajena sobre instancia de route). Sin el
	// intento tagueado no se pasa por acá, y una instancia regular de
	// calendario no ancla a ninguna.
	anchor, err := s.competitionDAO.FindByRouteSessionInstance(ctx, sessionInstanceID)
	if err != nil && !errors.Is(err, daos.ErrCompetitionNotFound) {
		return fmt.Errorf("error al validar la competencia del intento")
	}
	if anchor != nil && anchor.ID != comp.ID {
		return fmt.Errorf("%w: la competencia indicada no corresponde a esta sesión", ErrRunnerSessionInvalid)
	}
	return nil
}

// Create valida la sesión, resuelve el atleta y crea el estado en wip de forma
// idempotente: si la fila ya existía devuelve la actual (created=false), sin
// pisar start_date ni bajar de finished a wip. Devuelve además el día cargado
// por el gate presencial (nil si no se cargó) para el hook de apertura.
func (s *runnerSessionService) Create(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.CreateRunnerSessionRequest) (*dbs.RunnerSession, *dbs.GroupCalendarDay, bool, error) {
	exists, err := s.runnerSessionDao.SessionInstanceExists(ctx, sessionInstanceID)
	if err != nil {
		return nil, nil, false, err
	}
	if !exists {
		return nil, nil, false, ErrSessionInstanceNotFound
	}
	if req.StartDate.IsZero() {
		return nil, nil, false, fmt.Errorf("%w: start_date es obligatorio", ErrRunnerSessionInvalid)
	}

	athleteUserID, err := s.resolveAthlete(ctx, authUserID, req.AthleteUserID)
	if err != nil {
		return nil, nil, false, err
	}

	// Gate presencial D9: después de resolveAthlete (403 precede al 409) y
	// antes del write — un día cerrado/sin abrir no acepta entrada del
	// corredor. Owner exento (CheckAthleteEntry lo resuelve). El día cargado
	// viaja hacia arriba: el hook de apertura del controller lo reutiliza.
	var calendarDay *dbs.GroupCalendarDay
	if s.presencial != nil {
		calendarDay, err = s.presencial.CheckAthleteEntry(ctx, sessionInstanceID, authUserID)
		if err != nil {
			return nil, nil, false, err
		}
	}

	// Marca de competencia (E4.1): validar ANTES del write; sin el campo el
	// flujo es byte-idéntico al de siempre.
	if req.CompetitionID != nil {
		if err := s.validateCompetitionIntent(ctx, sessionInstanceID, athleteUserID, req.CompetitionID); err != nil {
			return nil, nil, false, err
		}
	}

	rs := &dbs.RunnerSession{
		SessionInstanceID: sessionInstanceID,
		AthleteUserID:     athleteUserID,
		Status:            runnersession.RunnerSessionStatusWip,
		StartDate:         req.StartDate,
		CompetitionID:     req.CompetitionID,
	}
	created, err := s.runnerSessionDao.Create(ctx, rs)
	if err != nil {
		return nil, nil, false, err
	}
	if !created {
		current, err := s.runnerSessionDao.GetBySessionAndAthlete(ctx, sessionInstanceID, athleteUserID)
		if err != nil {
			return nil, nil, false, err
		}
		return current, calendarDay, false, nil
	}
	return rs, calendarDay, true, nil
}

// Finish aplica la transición de status pedida (finished|interrupted) con
// end_date seteada por el servidor. Idempotente: pedir el status que ya tiene
// la fila la devuelve sin cambios. finished→interrupted es ilegal (400) y una
// fila finished jamás cambia. Sin fila → ErrRunnerSessionNotFound (404).
//
// Legales: wip→finished, wip→interrupted, interrupted→finished (re-setea
// end_date = now, momento del finish explícito).
func (s *runnerSessionService) Finish(ctx *gin.Context, authUserID, sessionInstanceID int64, req runnersession.RunnerStatusRequest) (*dbs.RunnerSession, error) {
	if req.Status != runnersession.RunnerSessionStatusFinished && req.Status != runnersession.RunnerSessionStatusInterrupted {
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
	if current.Status == runnersession.RunnerSessionStatusFinished {
		return nil, fmt.Errorf("%w: no se puede interrumpir una sesión ya finalizada", ErrRunnerSessionInvalid)
	}

	now := time.Now()
	if err := s.runnerSessionDao.UpdateStatus(ctx, current.ID, req.Status, []string{runnersession.RunnerSessionStatusWip, runnersession.RunnerSessionStatusInterrupted}, now); err != nil {
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
