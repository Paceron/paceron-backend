package services

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
)

// Errores del gate presencial (D9): el controller de runner session los mapea
// a 409 con los slugs session_not_opened / session_closed en el campo Code.
var (
	ErrRunnerSessionNotOpen = errors.New("la sesión presencial todavía no fue abierta por el entrenador")
	ErrRunnerSessionClosed  = errors.New("la sesión presencial ya fue cerrada")
)

// PresencialSessionServiceInterface encapsula el estado presencial del día de
// calendario para el flujo del corredor (Gap 26): gate de entrada (D9) y
// hooks de apertura/cierre por el owner (D7). El controller de runner session
// la usa como dependencia mínima (CalendarGateway del design): ni daos ni el
// calendar service directo.
type PresencialSessionServiceInterface interface {
	// CheckAthleteEntry aplica el gate del corredor (D9) sobre el día
	// training+presencial que referencia la instancia: error nil si el día no
	// es presencial, el auth es owner del team del grupo o la sesión está
	// abierta; ErrRunnerSessionClosed/ErrRunnerSessionNotOpen si no. Devuelve
	// el día encontrado (nil si no existe) para reutilizarlo luego sin
	// re-consultar — el hook de apertura lo consume (Create del runner).
	CheckAthleteEntry(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error)
	// OnRunnerCreated es el hook de apertura (D7): si el día de la instancia
	// es training+presencial y el auth es owner del team del grupo, setea
	// presencial_opened_at (guard NULL, idempotente). Devuelve el día
	// post-write y si el write cambió algo (base del evento WS D10). gateDay
	// es el día ya cargado por el gate en este mismo POST (nil-able: si es
	// nil se re-resuelve por la instancia). Nil si el día no aplica.
	OnRunnerCreated(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error)
	// OnRunnerFinished es el hook de cierre (D7): mismas condiciones que la
	// apertura pero seteando presencial_closed_at. El `interrupted` del
	// owner NO cierra (el controller no lo invoca en ese caso).
	OnRunnerFinished(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error)
}

type presencialSessionService struct {
	calendarDayDao daos.GroupCalendarDaoInterface
	groupDao       daos.GroupDaoInterface
	teamDao        daos.TeamDaoInterface
}

// NewPresencialSessionService crea el gateway del estado presencial.
func NewPresencialSessionService(calendarDayDao daos.GroupCalendarDaoInterface, groupDao daos.GroupDaoInterface, teamDao daos.TeamDaoInterface) PresencialSessionServiceInterface {
	return &presencialSessionService{
		calendarDayDao: calendarDayDao,
		groupDao:       groupDao,
		teamDao:        teamDao,
	}
}

// presencialDay resuelve el día de la instancia (reusando provided si no es
// nil — así el gate y el hook comparten la misma lectura) y devuelve ok=true
// solo si es training+presencial. Cuando la fila existe pero no aplica, el día
// igual se devuelve para que el caller decida sin volver a consultar.
func (s *presencialSessionService) presencialDay(ctx *gin.Context, sessionInstanceID int64, provided *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error) {
	if provided == nil {
		var err error
		provided, err = s.calendarDayDao.FindBySessionInstanceID(ctx, sessionInstanceID)
		if err != nil || provided == nil {
			return nil, false, err
		}
	}
	if provided.Kind != string(constants.GroupCalendarDayKindTraining) || !provided.IsPresencial {
		return provided, false, nil
	}
	return provided, true, nil
}

// isDayTeamOwner resuelve dueño del team del grupo del día (D7/D9: owner del
// team del grupo — no "membresía en cualquier team" como resolveAthlete).
func (s *presencialSessionService) isDayTeamOwner(ctx *gin.Context, day *dbs.GroupCalendarDay, authUserID int64) bool {
	group, err := s.groupDao.FindByID(ctx, day.GroupID)
	if err != nil || group == nil {
		return false
	}
	team, err := s.teamDao.FindByID(ctx, group.TeamID)
	if err != nil || team == nil {
		return false
	}
	return team.OwnerID == authUserID
}

// CheckAthleteEntry gate D9: no-owner sobre día presencial necesita
// opened_at != NULL y closed_at == NULL. El cierre se chequea primero porque
// es el estado final (invariante D5: cerrada implica abierta). Devuelve el
// día encontrado (nil si no hay) para el passthrough al hook de apertura.
func (s *presencialSessionService) CheckAthleteEntry(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, error) {
	day, isPresencial, err := s.presencialDay(ctx, sessionInstanceID, nil)
	if err != nil || !isPresencial {
		return day, err
	}
	if s.isDayTeamOwner(ctx, day, authUserID) {
		return day, nil
	}
	if day.PresencialClosedAt != nil {
		return day, ErrRunnerSessionClosed
	}
	if day.PresencialOpenedAt == nil {
		return day, ErrRunnerSessionNotOpen
	}
	return day, nil
}

// OnRunnerCreated hook de apertura D7: owner + día presencial → setear
// opened_at si NULL. Reutiliza el día ya cargado por el gate (mismo POST,
// sin segundo lookup); si es nil lo resuelve por la instancia. Post-write
// re-lee el día para el evento WS (estado post-write, no el pre-write).
func (s *presencialSessionService) OnRunnerCreated(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, bool, error) {
	day, isPresencial, err := s.presencialDay(ctx, sessionInstanceID, gateDay)
	if err != nil || !isPresencial {
		return nil, false, err
	}
	if !s.isDayTeamOwner(ctx, day, authUserID) {
		return nil, false, nil
	}
	mutated, err := s.calendarDayDao.SetPresencialOpenedAt(ctx, day.ID, time.Now())
	if err != nil {
		return nil, false, err
	}
	if !mutated {
		return nil, false, nil
	}
	updated, err := s.calendarDayDao.FindBySessionInstanceID(ctx, sessionInstanceID)
	return updated, updated != nil, err
}

// OnRunnerFinished hook de cierre D7: mismo par owner+presencial, con guard
// sobre closed_at (final, sin reopen). El PATCH no corre el gate, así que no
// hay día previo disponible: siempre resuelve el propio.
func (s *presencialSessionService) OnRunnerFinished(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
	day, isPresencial, err := s.presencialDay(ctx, sessionInstanceID, nil)
	if err != nil || !isPresencial {
		return nil, false, err
	}
	if !s.isDayTeamOwner(ctx, day, authUserID) {
		return nil, false, nil
	}
	mutated, err := s.calendarDayDao.SetPresencialClosedAt(ctx, day.ID, time.Now())
	if err != nil {
		return nil, false, err
	}
	if !mutated {
		return nil, false, nil
	}
	updated, err := s.calendarDayDao.FindBySessionInstanceID(ctx, sessionInstanceID)
	return updated, updated != nil, err
}
