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
	// el día encontrado (nil si no existe) y el owner resuelto en la misma
	// lectura para reutilizarlos luego sin re-consultar — el hook de apertura
	// los consume (Create del runner, D3).
	CheckAthleteEntry(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, *int64, error)
	// OnRunnerCreated es el hook de apertura (D7): si el día de la instancia
	// es training+presencial y el auth es owner del team del grupo, setea
	// presencial_opened_at (guard NULL, idempotente). Devuelve el día
	// post-write y si el write cambió algo (base del evento WS D10). gateDay y
	// gateOwnerID son los ya resueltos por el gate en este mismo POST: con
	// ambos presentes no se repite la lectura combinada día+owner (D3). Si
	// faltan, el hook resuelve por su cuenta con fallback seguro. Nil si el
	// día no aplica.
	OnRunnerCreated(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay, gateOwnerID *int64) (*dbs.GroupCalendarDay, bool, error)
	// OnRunnerFinished es el hook de cierre (D7): mismas condiciones que la
	// apertura pero seteando presencial_closed_at. El `interrupted` del
	// owner NO cierra (el controller no lo invoca en ese caso).
	OnRunnerFinished(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error)
}

type presencialSessionService struct {
	calendarDayDao daos.GroupCalendarDaoInterface
}

// NewPresencialSessionService crea el gateway del estado presencial.
func NewPresencialSessionService(calendarDayDao daos.GroupCalendarDaoInterface) PresencialSessionServiceInterface {
	return &presencialSessionService{calendarDayDao: calendarDayDao}
}

// presencialDay resuelve el día de la instancia junto con el owner del team en
// una sola lectura (D3). Reusa provided para decidir el gate sin consultar
// cuando no es presencial; si lo es, la lectura combinada aporta el owner.
// Devuelve ok=true solo si el día es training+presencial.
//
// Fallback de errores (paridad con el flujo previo día + group/team): si la
// lectura combinada falla pero el día ya está resuelto (provided) o una
// lectura de día sola lo recupera, el owner queda ausente y el caller hace
// no-op. Solo si también falla la lectura de día se propaga el error.
func (s *presencialSessionService) presencialDay(ctx *gin.Context, sessionInstanceID int64, provided *dbs.GroupCalendarDay) (*dbs.GroupCalendarDay, *int64, bool, error) {
	if provided != nil && !isPresencialCalendarDay(provided) {
		return provided, nil, false, nil
	}
	day, ownerID, err := s.calendarDayDao.FindWithTeamOwnerBySessionInstanceID(ctx, sessionInstanceID)
	if err != nil {
		if provided != nil {
			return provided, nil, true, nil
		}
		dayOnly, dayErr := s.calendarDayDao.FindBySessionInstanceID(ctx, sessionInstanceID)
		if dayErr != nil {
			return nil, nil, false, dayErr
		}
		if dayOnly == nil || !isPresencialCalendarDay(dayOnly) {
			return dayOnly, nil, false, nil
		}
		return dayOnly, nil, true, nil
	}
	if day == nil {
		return nil, nil, false, nil
	}
	if !isPresencialCalendarDay(day) {
		return day, ownerID, false, nil
	}
	return day, ownerID, true, nil
}

func isPresencialCalendarDay(day *dbs.GroupCalendarDay) bool {
	return day.Kind == string(constants.GroupCalendarDayKindTraining) && day.IsPresencial
}

// CheckAthleteEntry gate D9: no-owner sobre día presencial necesita
// opened_at != NULL y closed_at == NULL. El cierre se chequea primero porque
// es el estado final (invariante D5: cerrada implica abierta). Devuelve el
// día encontrado (nil si no hay) para el passthrough al hook de apertura.
func (s *presencialSessionService) CheckAthleteEntry(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, *int64, error) {
	day, ownerID, isPresencial, err := s.presencialDay(ctx, sessionInstanceID, nil)
	if err != nil || !isPresencial {
		return day, ownerID, err
	}
	if ownerID != nil && *ownerID == authUserID {
		return day, ownerID, nil
	}
	if day.PresencialClosedAt != nil {
		return day, ownerID, ErrRunnerSessionClosed
	}
	if day.PresencialOpenedAt == nil {
		return day, ownerID, ErrRunnerSessionNotOpen
	}
	return day, ownerID, nil
}

// OnRunnerCreated hook de apertura D7: owner + día presencial → setear
// opened_at si NULL. Reutiliza el día ya cargado por el gate (mismo POST)
// para el chequeo de gate; la lectura combinada aporta el owner. El
// RETURNING del update entrega el estado post-write para el evento WS.
func (s *presencialSessionService) OnRunnerCreated(ctx *gin.Context, sessionInstanceID, authUserID int64, gateDay *dbs.GroupCalendarDay, gateOwnerID *int64) (*dbs.GroupCalendarDay, bool, error) {
	var (
		day          *dbs.GroupCalendarDay
		ownerID      *int64
		isPresencial bool
	)
	if gateDay != nil && gateOwnerID != nil {
		day = gateDay
		ownerID = gateOwnerID
		isPresencial = isPresencialCalendarDay(gateDay)
	} else {
		var err error
		day, ownerID, isPresencial, err = s.presencialDay(ctx, sessionInstanceID, gateDay)
		if err != nil {
			return nil, false, err
		}
	}
	if !isPresencial || ownerID == nil || *ownerID != authUserID {
		return nil, false, nil
	}
	updated, mutated, err := s.calendarDayDao.SetPresencialOpenedAt(ctx, day.ID, time.Now())
	if err != nil || !mutated {
		return nil, false, err
	}
	return updated, updated != nil, nil
}

// OnRunnerFinished hook de cierre D7: mismo par owner+presencial, con guard
// sobre closed_at (final, sin reopen). El PATCH no corre el gate, así que no
// hay día previo disponible: siempre resuelve el propio.
func (s *presencialSessionService) OnRunnerFinished(ctx *gin.Context, sessionInstanceID, authUserID int64) (*dbs.GroupCalendarDay, bool, error) {
	day, ownerID, isPresencial, err := s.presencialDay(ctx, sessionInstanceID, nil)
	if err != nil || !isPresencial {
		return nil, false, err
	}
	if ownerID == nil || *ownerID != authUserID {
		return nil, false, nil
	}
	updated, mutated, err := s.calendarDayDao.SetPresencialClosedAt(ctx, day.ID, time.Now())
	if err != nil || !mutated {
		return nil, false, err
	}
	return updated, updated != nil, nil
}
