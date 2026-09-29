package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/attendance"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/trainingplan"
)

// Errores de negocio del módulo de asistencias. El controller los mapea a los
// status HTTP correspondientes vía errors.Is.
var (
	// ErrForbiddenAttendance indica que el usuario autenticado no tiene permiso
	// para ver las asistencias solicitadas (no es entrenador ni corredor del team,
	// o siendo corredor pidió asistencias de otro).
	ErrForbiddenAttendance = errors.New("no tenés permisos para ver estas asistencias")
	// ErrTeamIDRequired indica que no se envió el team_id obligatorio.
	ErrTeamIDRequired = errors.New("team_id es obligatorio para buscar asistencias")
)

// Observations sobre el determinismo del QR: se usan tamaño (256px), nivel de
// corrección (Medium) y quiet zone por default de la librería, constantes para
// todos los casos; skip2/go-qrcode no inyecta timestamps ni metadata aleatoria,
// por lo que mismos inputs producen el mismo PNG.
const (
	qrSizePx     = 256
	qrErrorLevel = qrcode.Medium

	// El QR codifica una ruta del FRONTEND, no de la API. Antes era
	// `/api/v1/attendance/team/%d/session/%d`, lo que estaba roto para el uso
	// real: el corredor escanea con la app de cámara o Google Lens, el link
	// abre el browser contra la API, y la respuesta es un 401 en crudo —no hay
	// pantalla, ni app, ni mensaje. La API nunca se pone en el QR.
	//
	// Ahora apunta a la pantalla que el corredor abre desde el link:
	// `team_id` y `session_instance_id` viajan como query params porque son los
	// que la pantalla necesita para llamar al endpoint de registro.
	//
	// El prefijo de la ruta es `/attendance`, la misma pantalla que usa el
	// entrenador pero con el sub-segmento `register`: son dos vistas distintas
	// del mismo dominio y `attendance` a secas ya está en el catálogo de rutas
	// con `role: 'trainer'`.
	qrWebPathFmt = "/attendance/register?team_id=%d&session_instance_id=%d"
)

// AttendanceServiceInterface define las operaciones de negocio de asistencias.
type AttendanceServiceInterface interface {
	GenerateQR(ctx *gin.Context, authUserID, teamID, sessionID int64) (*attendance.QRResponse, error)
	Register(ctx *gin.Context, userID, teamID, sessionID int64) (created bool, sessionCtx *daos.AttendanceSessionContext, err error)
	Search(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error)
	ListAttendanceSessions(ctx *gin.Context, authUserID, teamID, groupID int64) (*attendance.SessionAttendanceListResponse, error)
	GetSessionAttendance(ctx *gin.Context, authUserID, teamID, groupID, sessionInstanceID int64) (*attendance.SessionAttendanceResponse, error)
	BulkSaveAttendance(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error)
	DeleteAttendance(ctx *gin.Context, authUserID, teamID, attendanceID int64) error
}

type attendanceService struct {
	attendanceDao daos.AttendanceDAOInterface
	groupDao      daos.GroupDaoInterface
	groupUserDao  daos.GroupUserDaoInterface
	userDao       daos.UserDaoInterface
	baseURL       string
}

// NewAttendanceService crea una nueva instancia de AttendanceService. baseURL es
// la URL pública del backend que se embebe en el QR (config.AttendanceBaseURL).
//
// groupDao y userDao no los usan los endpoints originales de QR y búsqueda: los
// necesita la gestión de asistencia del entrenador, para validar que el grupo sea
// del equipo (404) y para resolver los nombres del roster con UNA sola query en
// vez de una por corredor.
//
// groupUserDao lo usan la carga masiva y el registro por QR, para comprobar que
// un corredor era miembro del grupo EN LA FECHA DE LA SESIÓN.
func NewAttendanceService(attendanceDao daos.AttendanceDAOInterface, groupDao daos.GroupDaoInterface, groupUserDao daos.GroupUserDaoInterface, userDao daos.UserDaoInterface, baseURL string) AttendanceServiceInterface {
	return &attendanceService{
		attendanceDao: attendanceDao,
		groupDao:      groupDao,
		groupUserDao:  groupUserDao,
		userDao:       userDao,
		baseURL:       baseURL,
	}
}

// GenerateQR construye la URL de registro y genera un QR determinista (PNG en
// base64) que la codifica.
// GenerateQR emite el QR de una sesión. Exige ser entrenador del equipo y que la
// sesión sea válida (presencial, no cancelada y del equipo): sin eso, cualquiera
// autenticado podía emitir el QR de un equipo ajeno y dejar.Run de sus corredores.
//
// El orden de las validaciones es el mismo que en el resto de los endpoints del
// entrenador: primero el rol, después la sesión. Así un 403 no depende de que la
// sesión exista.
//
// NO se filtra por fecha pasada: el QR de una sesión próxima es legítimo y es el
// caso normal de uso (el entrenador lo emite antes de la clase). La fecha futura
// no es un error de validación en ningún endpoint de este service.
func (s *attendanceService) GenerateQR(ctx *gin.Context, authUserID, teamID, sessionID int64) (*attendance.QRResponse, error) {
	isCoach, err := s.resolveTrainerRole(ctx, teamID, authUserID)
	if err != nil {
		return nil, err
	}
	if !isCoach {
		return nil, ErrForbiddenAttendance
	}

	if _, err := s.resolveAttendanceSession(ctx, teamID, sessionID); err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(s.baseURL, "/")
	url := fmt.Sprintf("%s%s", baseURL, fmt.Sprintf(qrWebPathFmt, teamID, sessionID))

	png, err := qrcode.Encode(url, qrErrorLevel, qrSizePx)
	if err != nil {
		return nil, fmt.Errorf("error generando el QR: %w", err)
	}

	return &attendance.QRResponse{
		QRCodeBase64: base64.StdEncoding.EncodeToString(png),
		URLEncoded:   url,
	}, nil
}

// Register registra la asistencia del usuario autenticado a la sesión del team.
// Devuelve created=true si fue la primera vez (201) y created=false si ya existía
// (200, idempotente). El insert se hace directo contra la DB y la idempotencia se
// resuelve capturando la violation de la UNIQUE — sin SELECT previo.
//
// Source queda fijo en "qr": este endpoint es el escaneo del corredor, y la
// columna es NOT NULL, así que omitirlo haría fallar el insert en runtime.
//
// RegisteredByUserID es el propio corredor (authUserID), no un tercero: la
// columna registra QUIÉN cargó la fila, y acá ese alguien es el usuario
// autenticado. No queda null en este camino —el null es para el backfill de
// filas preexistentes, donde no hay dato de origen (D10).
// Register registra la asistencia de un corredor al escanear el QR.
//
// Antes de insertar, exige que el corredor sea miembro ACTIVO del grupo al que
// pertenece la sesión (no que sea entrenador, que es el otro camino). La
// membresía se evalúa contra la fecha de la sesión, igual que en la grilla (D7):
// al grupo se pertenecía el día de la clase, no hoy.
//
// Una sesión que no existe (o que no está asignada a ningún día de calendario) se
// responde 403 y no 404 a propósito: no hay grupo al que pertenecer, y un 404
// confirmaría que ese id de sesión existe cuando en realidad no sabemos nada de
// él.
//
// La idempotencia no cambia: la primera vez responde true (201) y si ya existía
// responde false (200) sin insertar un duplicado.
// Devuelve (created, sessionCtx, err). El contexto se agrega a la respuesta
// porque el corredor, después de registrarse, tiene que poder saltar a la sesión
// que acaba de registrar: el deep link del día es /teams/:team/groups/:group/
// calendar/:date, y sin el grupo y la fecha el front no puede armarlo — el QR
// solo trae team_id y session_instance_id. El dato ya estaba cargado acá para
// validar la membresía, así que no es una consulta extra.
func (s *attendanceService) Register(ctx *gin.Context, userID, teamID, sessionID int64) (bool, *daos.AttendanceSessionContext, error) {
	// Se resuelve el contexto de la sesión solo para saber su grupo y su fecha.
	// NO se usa resolveAttendanceSession: esa función valida presencial y no
	// cancelada, que son reglas del camino del entrenador. El escaneo del
	// corredor se describe en la spec únicamente con la exigencia de membresía.
	sessionCtx, err := s.attendanceDao.FindSessionContext(ctx, sessionID)
	if err != nil {
		return false, nil, err
	}
	if sessionCtx == nil {
		return false, nil, ErrAttendanceNotGroupMember
	}

	isMember, err := s.groupUserDao.IsActiveGroupMember(ctx, sessionCtx.GroupID, userID, sessionCtx.Date)
	if err != nil {
		return false, nil, err
	}
	if !isMember {
		return false, nil, ErrAttendanceNotGroupMember
	}

	attendance := &dbs.Attendance{
		TeamID:             teamID,
		TrainingSessionID:  sessionID,
		UserID:             userID,
		Source:             string(constants.AttendanceSourceQR),
		RegisteredByUserID: &userID,
	}

	if err := s.attendanceDao.Create(ctx, attendance); err != nil {
		if errors.Is(err, daos.ErrAttendanceAlreadyExists) {
			// Idempotencia: ya estaba, y el contexto se devuelve igual para que el
			// front pueda llevar al corredor a la misma sesión que en el 201.
			return false, sessionCtx, nil
		}
		return false, nil, fmt.Errorf("error registrando la asistencia: %w", err)
	}
	return true, sessionCtx, nil
}

// Search valida las restricciones de la búsqueda de asistencias y delega al DAO
// con los filtros ya autorizados. team_id es obligatorio y el usuario autenticado
// debe pertenecer al equipo como entrenador o corredor:
//
//	Entrenador -> ve todas las asistencias del equipo.
//	Corredor  -> ve solo las suyas (se fuerza user_id = authUserID en la query).
func (s *attendanceService) Search(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error) {
	if filters.TeamID == nil {
		return nil, ErrTeamIDRequired
	}

	exists, err := s.attendanceDao.TeamExists(ctx, *filters.TeamID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrTeamNotFound
	}

	// Un usuario es entrenador del team si es el owner (teams.owner_id) o si tiene
	// rol "entrenador" en team_users. El owner puede no tener fila en team_users
	// en equipos creados antes de que se registrara la membresía del owner.
	role, err := s.teamRoleFor(ctx, *filters.TeamID, authUserID)
	if err != nil {
		return nil, err
	}
	if role == "" {
		return nil, ErrForbiddenAttendance
	}
	isCoach := role == string(constants.TeamUserRoleEntrenador)

	daoFilters := daos.AttendanceSearchFilters{
		TeamID:            filters.TeamID,
		TrainingSessionID: filters.TrainingSessionID,
	}

	if isCoach {
		// Entrenador: puede ver todas las asistencias del equipo, opcionalmente
		// filtrando por un corredor puntual vía user_id.
		daoFilters.UserID = filters.UserID
		return s.attendanceDao.Search(ctx, daoFilters)
	}

	// Corredor: solo sus propias asistencias. Un user_id ajeno en la request no
	// tiene permitido ver data de otros miembros.
	if filters.UserID != nil && *filters.UserID != authUserID {
		return nil, ErrForbiddenAttendance
	}
	daoFilters.UserID = &authUserID
	return s.attendanceDao.Search(ctx, daoFilters)
}

// teamRoleFor devuelve el rol efectivo de authUserID dentro de teamID, con tres
// estados distinguibles: TeamUserRoleEntrenador si administra el equipo (owner o
// role_in_team = 'entrenador'), el role_in_team si es miembro corredor, o "" si no
// es miembro. Search necesita distinguir esos tres casos para responder 403 a un
// usuario ajeno al equipo en vez de tratarlo como corredor.
//
// Se extrajo de la lógica inline de Search sin cambiar su comportamiento ni su
// número de consultas a la DB: IsTeamOwner se consulta siempre, y
// GetTeamUserRole solo si no es owner (el owner puede no tener fila en
// team_users en equipos creados antes de que se registrara su membresía).
func (s *attendanceService) teamRoleFor(ctx *gin.Context, teamID, authUserID int64) (string, error) {
	isOwner, err := s.attendanceDao.IsTeamOwner(ctx, teamID, authUserID)
	if err != nil {
		return "", err
	}
	if isOwner {
		return string(constants.TeamUserRoleEntrenador), nil
	}
	role, err := s.attendanceDao.GetTeamUserRole(ctx, teamID, authUserID)
	if err != nil {
		return "", err
	}
	return role, nil
}

// resolveTrainerRole responde si authUserID administra teamID, reutilizando
// teamRoleFor. Es la autorización de todos los endpoints de escritura del panel
// de entrenador (alta masiva y borrado): "ser entrenador" tiene que ser
// exactamente la misma regla que ya usa GET /attendance/search, para que un
// usuario no pierda permisos entre endpoints del mismo módulo (D8).
func (s *attendanceService) resolveTrainerRole(ctx *gin.Context, teamID, authUserID int64) (bool, error) {
	role, err := s.teamRoleFor(ctx, teamID, authUserID)
	if err != nil {
		return false, err
	}
	return role == string(constants.TeamUserRoleEntrenador), nil
}

// Errores de validación de la sesión objetivo (spec "Requirement: El sistema
// SHALL validar la sesión objetivo"). Son todos 422 salvo ErrTeamNotFound (404),
// y cada uno tiene un mensaje propio porque el controller los traduce 1:1 a la
// respuesta: el mensaje necesita identificar LA condición incumplida, no un
// "datos inválidos" genérico.
var (
	ErrAttendanceSessionNotFound      = errors.New("la sesión indicada no existe o no está asignada a ningún día de calendario")
	ErrAttendanceSessionNotTraining   = errors.New("la sesión indicada no corresponde a un día de entrenamiento")
	ErrAttendanceSessionCancelled     = errors.New("la sesión indicada fue cancelada")
	ErrAttendanceSessionNotPresencial = errors.New("la sesión indicada no es presencial, y las asistencias solo se registran para sesiones presenciales")
	ErrAttendanceSessionWrongTeam     = errors.New("la sesión indicada no pertenece al equipo indicado")
	ErrAttendanceNotTrainer           = errors.New("solo el entrenador del equipo puede modificar las asistencias")
)

// resolveAttendanceSession resuelve y valida la sesión objetivo de una operación
// de asistencia del entrenador. Es el único lugar donde viven estas 5
// validaciones (spec, D1): los tres endpoints que las necesitan (listado de
// sesiones, grilla, carga masiva, borrado y QR) las comparten para que no puedan
// divergir entre sí.
//
// teamID es el equipo de la operación, no un dato de la sesión: se compara contra
// el team_id del grupo al que pertenece la sesión y se rechaza con 422 si no
// coinciden. No se filtra ni se revela nada del otro equipo más allá del mensaje.
//
// Una sesión FUTURA no es un error: el QR se emite para sesiones próximas y la
// carga masiva admite la sesión de hoy.
func (s *attendanceService) resolveAttendanceSession(ctx *gin.Context, teamID, sessionInstanceID int64) (*daos.AttendanceSessionContext, error) {
	exists, err := s.attendanceDao.TeamExists(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrTeamNotFound
	}

	sessionCtx, err := s.attendanceDao.FindSessionContext(ctx, sessionInstanceID)
	if err != nil {
		return nil, err
	}
	if sessionCtx == nil {
		return nil, ErrAttendanceSessionNotFound
	}

	// Se chequea 'cancelled' ANTES que el kind: un día cancelado tampoco tiene
	// kind == 'training', así que el orden inverso haría inalcanzable el error de
	// cancelación y devolvería el mensaje confuso "no corresponde a un día de
	// entrenamiento" para lo que en realidad es una sesión cancelada.
	if sessionCtx.Kind == string(constants.GroupCalendarDayKindCancelled) {
		return nil, ErrAttendanceSessionCancelled
	}
	if sessionCtx.Kind != string(constants.GroupCalendarDayKindTraining) {
		return nil, ErrAttendanceSessionNotTraining
	}
	if !sessionCtx.IsPresencial {
		return nil, ErrAttendanceSessionNotPresencial
	}
	if sessionCtx.TeamID != teamID {
		return nil, ErrAttendanceSessionWrongTeam
	}

	return sessionCtx, nil
}

// Errores de los endpoints de lectura de la gestión de asistencia.
var (
	// ErrAttendanceGroupNotFound indica que el grupo no existe o no es del equipo
	// de la operación. Se responde 404 y no 403 a propósito: no se le dice al
	// usuario que el grupo existe pero es de otro equipo.
	ErrAttendanceGroupNotFound = errors.New("el grupo indicado no existe en este equipo")
	// ErrAttendanceGroupMismatch indica que el group_id de la request no es el
	// grupo al que pertenece la sesión. Es 422: el grupo existe y es del equipo,
	// pero la combinación pedida no tiene sentido.
	ErrAttendanceGroupMismatch = errors.New("la sesión indicada no pertenece al grupo indicado")
	// ErrAttendanceNotGroupMember indica que el corredor no es miembro activo del
	// grupo de la sesión en la fecha de la sesión. Es el 403 del camino QR.
	ErrAttendanceNotGroupMember = errors.New("no sos miembro del grupo de esta sesión")
	// ErrAttendanceNotFound indica que la asistencia a borrar no existe. Solo se
	// usa 404 para el borrado: el listado no borra nada.
	ErrAttendanceNotFound = errors.New("la asistencia indicada no existe")
)

// ListAttendanceSessions devuelve las sesiones presenciales ya ocurridas del
// grupo, para que el entrenador elija sobre cuál gestionar la asistencia.
//
// teamID es obligatorio y lo valida el controller como 400 antes de llegar acá.
// El grupo tiene que ser del equipo (404 si no) y el usuario tiene que ser
// entrenador de ese equipo (403 si no).
func (s *attendanceService) ListAttendanceSessions(ctx *gin.Context, authUserID, teamID, groupID int64) (*attendance.SessionAttendanceListResponse, error) {
	group, err := s.groupDao.FindByIDAndTeamID(ctx, groupID, teamID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, ErrAttendanceGroupNotFound
	}

	isCoach, err := s.resolveTrainerRole(ctx, teamID, authUserID)
	if err != nil {
		return nil, err
	}
	if !isCoach {
		return nil, ErrForbiddenAttendance
	}

	options, err := s.attendanceDao.FindPastPresencialSessionsForGroup(ctx, groupID, teamID)
	if err != nil {
		return nil, err
	}

	sessions := make([]attendance.SessionAttendanceOption, 0, len(options))
	for _, o := range options {
		sessions = append(sessions, attendance.SessionAttendanceOption{
			SessionInstanceID:  o.SessionInstanceID,
			Name:               o.Name,
			Date:               o.Date,
			PresencialTimeFrom: hhmmPtr(o.PresencialTimeFrom),
			PresencialTimeTo:   hhmmPtr(o.PresencialTimeTo),
			AttendedCount:      o.AttendedCount,
		})
	}
	return &attendance.SessionAttendanceListResponse{Sessions: sessions}, nil
}

// GetSessionAttendance devuelve el roster del grupo cruzado con el estado de
// asistencia de cada corredor para la sesión, más los agregados de la sesión.
//
// El orden de las validaciones es el de menor a mayor fuga de información: primero
// que el equipo exista y el usuario sea su entrenador (404/403), después la
// sesión (422), y recién al final que el group_id pedido sea el de la sesión
// (422). Invertirlo permitiría a un entrenador de un equipo-legítimo sonsacar la
// existencia de sesiones de otro grupo.
func (s *attendanceService) GetSessionAttendance(ctx *gin.Context, authUserID, teamID, groupID, sessionInstanceID int64) (*attendance.SessionAttendanceResponse, error) {
	isCoach, err := s.resolveTrainerRole(ctx, teamID, authUserID)
	if err != nil {
		return nil, err
	}
	if !isCoach {
		return nil, ErrForbiddenAttendance
	}

	sessionCtx, err := s.resolveAttendanceSession(ctx, teamID, sessionInstanceID)
	if err != nil {
		return nil, err
	}
	if sessionCtx.GroupID != groupID {
		return nil, ErrAttendanceGroupMismatch
	}

	rows, aggregates, err := s.attendanceDao.FindGroupRosterWithAttendance(
		ctx, groupID, teamID, sessionInstanceID, sessionCtx.Date)
	if err != nil {
		return nil, err
	}
	if aggregates == nil {
		aggregates = &daos.SessionAttendanceAggregates{}
	}

	roster, err := s.buildRosterRows(ctx, rows)
	if err != nil {
		return nil, err
	}

	return &attendance.SessionAttendanceResponse{
		Session: attendance.SessionInfo{
			SessionInstanceID:  sessionCtx.SessionInstanceID,
			Name:               sessionCtx.SessionName,
			Date:               sessionCtx.Date,
			PresencialTimeFrom: hhmmPtr(sessionCtx.PresencialTimeFrom),
			PresencialTimeTo:   hhmmPtr(sessionCtx.PresencialTimeTo),
			PresencialLocation: parseLocationPtr(sessionCtx.PresencialLocation),
			GroupID:            sessionCtx.GroupID,
			GroupName:          sessionCtx.GroupName,
			TeamID:             sessionCtx.TeamID,
			TeamName:           sessionCtx.TeamName,
		},
		Summary: buildAttendanceSummary(aggregates),
		Roster:  roster,
	}, nil
}

// buildRosterRows resuelve nombre y email de TODAS las filas con UNA sola llamada
// al batch lookup de usuarios, y ordena alfabéticamente por nombre.
//
// El N+1 sería el problema obvio (una query por corredor), pero hay un segundo
// motivo para hacerlo así, que es el que importa: el nombre tiene que armarse con
// EXACTAMENTE la misma regla que ya usa el roster de la pantalla de equipo
// (name + " " + surname, con fallback al email). Si se compusiera el nombre en
// SQL, la grilla y el roster podrían divergir en el recorte de espacios o en el
// fallback, y el mismo corredor aparecería escrito de dos formas en la app.
func (s *attendanceService) buildRosterRows(ctx *gin.Context, rows []daos.SessionAttendanceRow) ([]attendance.SessionAttendanceRow, error) {
	out := make([]attendance.SessionAttendanceRow, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}

	userIDs := make([]int64, 0, len(rows))
	for _, r := range rows {
		userIDs = append(userIDs, r.UserID)
	}
	users, err := s.userDao.FindByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	userByID := make(map[int64]*dbs.User, len(users))
	for _, u := range users {
		userByID[u.ID] = u
	}

	for _, r := range rows {
		row := attendance.SessionAttendanceRow{
			UserID:       r.UserID,
			AttendanceID: r.AttendanceID,
			Source:       r.Source,
			RegisteredAt: r.RegisteredAt,
		}
		if u, ok := userByID[r.UserID]; ok {
			row.Name = composeUserName(u)
			row.Email = u.Email
		}
		// Status se deriva de la existencia de la asistencia, y solo en el camino
		// "attended" puede haber source: es el invariante de la spec.
		if r.AttendanceID != nil {
			row.Status = attendance.SessionAttendanceStatusAttended
		} else {
			row.Status = attendance.SessionAttendanceStatusNotConfirmed
			row.Source = nil
			row.RegisteredAt = nil
		}
		out = append(out, row)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// buildAttendanceSummary deriva los agregados de la respuesta a partir de los dos
// conteos crudos del DAO.
//
// not_confirmed usa max(0, roster_size - attended) y no un conteo directo de filas
// sin asistencia, porque attended es el TOTAL de asistencias de la sesión: si
// alguien que ya no figura en el roster tiene asistencia cargada, la resta puede
// dar negativo y el porcentaje pasarse de 100. El índice único garantiza
// attended <= roster_size solo mientras los conjuntos coincidan.
func buildAttendanceSummary(agg *daos.SessionAttendanceAggregates) attendance.AttendanceSummary {
	notConfirmed := agg.RosterSize - agg.Attended
	if notConfirmed < 0 {
		notConfirmed = 0
	}
	rate := 0.0
	if agg.RosterSize > 0 {
		rate = math.Round(float64(agg.Attended)/float64(agg.RosterSize)*1000) / 10
	}
	return attendance.AttendanceSummary{
		RosterSize:        agg.RosterSize,
		Attended:          agg.Attended,
		NotConfirmed:      notConfirmed,
		AttendanceRatePct: rate,
	}
}

// composeUserName arma el nombre completo con la misma regla que el roster de la
// app: name + " " + surname, recortando espacios sobrantes, con fallback al email
// cuando queda vacío.
func composeUserName(u *dbs.User) string {
	name := strings.TrimSpace(u.Name + " " + u.Surname)
	if name == "" {
		return u.Email
	}
	return name
}

// hhmmPtr formatea un time pointer a "HH:MM" en UTC, o nil si no hay hora. Reusa
// el mismo criterio que el módulo de calendario para que los dos headers
// coincidan.
func hhmmPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.UTC().Format("15:04")
	return &formatted
}

// parseLocationPtr deserializa el jsonb de presencial_location. Un valor
// corrupto no puede romper la grilla entera: devuelve nil y el frontend muestra la
// sesión sin ubicación.
func parseLocationPtr(raw *string) *trainingplan.Location {
	if raw == nil {
		return nil
	}
	loc, err := jsonUnmarshalLocation(*raw)
	if err != nil {
		return nil
	}
	return loc
}

// Errores de los endpoints de escritura de la gestión de asistencia.
var (
	// ErrAttendanceBulkInvalidUsers indica que algún user_id del lote no era
	// miembro del grupo en la fecha de la sesión. Se responde 422 Y no se escribe
	// ninguna fila: el lote es todo-o-nada.
	ErrAttendanceBulkInvalidUsers = errors.New("algunos corredores no son miembros del grupo en la fecha de la sesión")
	// ErrAttendanceForbiddenTeam indica que la fila existe pero es de otro equipo,
	// o que el usuario no es entrenador del equipo consultado. Se responde 403
	// para no revelar la existencia de asistencias de un equipo ajeno.
	ErrAttendanceForbiddenTeam = errors.New("no tenés permisos sobre esta asistencia")
)

// BulkSaveAttendance carga en un solo lote la asistencia de varios corredores a
// una sesión. Es idempotente: reenviar el mismo lote no duplica, refresca la
// procedencia y devuelve los contadores.
//
// El orden de las validaciones es lo que hace segura la operación: se resuelve la
// sesión y se validan los miembros del grupo ANTES de escribir nada, para que un
// lote con un solo user_id ajeno no deje la asistencia de los demás a medias.
func (s *attendanceService) BulkSaveAttendance(ctx *gin.Context, authUserID, teamID, sessionInstanceID int64, userIDs []int64) (*attendance.BulkSaveResult, error) {
	isCoach, err := s.resolveTrainerRole(ctx, teamID, authUserID)
	if err != nil {
		return nil, err
	}
	if !isCoach {
		return nil, ErrForbiddenAttendance
	}

	sessionCtx, err := s.resolveAttendanceSession(ctx, teamID, sessionInstanceID)
	if err != nil {
		return nil, err
	}

	// La membresía se evalúa contra la fecha de la SESIÓN, no contra hoy (D7).
	missing, err := s.groupUserDao.MissingGroupMembers(ctx, sessionCtx.GroupID, userIDs, sessionCtx.Date)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, &ErrBulkInvalidUsers{UserIDs: missing}
	}

	created, updated, err := s.attendanceDao.BulkUpsertManual(ctx, teamID, sessionInstanceID, authUserID, userIDs)
	if err != nil {
		return nil, err
	}
	return &attendance.BulkSaveResult{Created: created, Updated: updated}, nil
}

// ErrBulkInvalidUsers es el 422 de la carga masiva. Lleva adentro los user_id
// rechazados porque la spec exige que la respuesta los identifique, no que diga
// solo "algunos son inválidos".
type ErrBulkInvalidUsers struct {
	UserIDs []int64
}

func (e *ErrBulkInvalidUsers) Error() string {
	return ErrAttendanceBulkInvalidUsers.Error()
}

// Is hace que errors.Is(err, ErrAttendanceBulkInvalidUsers) funcione para el
// controller, sin perder los userIDs del envelope.
func (e *ErrBulkInvalidUsers) Is(target error) bool {
	return target == ErrAttendanceBulkInvalidUsers
}

// DeleteAttendance borra una asistencia ya cargada. El borrado es físico: después
// de un 204 el mismo corredor puede volver a ser marcado para la misma sesión.
//
// El orden importa para no filtrar existencia: primero se busca la fila (404 si
// no está), recién después se compara su equipo (403) y se valida que el usuario
// sea entrenador (403). Nunca se devuelve 403 para algo que no existe, ni 404
// para algo que existe pero es de otro equipo.
func (s *attendanceService) DeleteAttendance(ctx *gin.Context, authUserID, teamID, attendanceID int64) error {
	row, err := s.attendanceDao.FindByID(ctx, attendanceID)
	if err != nil {
		return err
	}
	if row == nil {
		return ErrAttendanceNotFound
	}
	if row.TeamID != teamID {
		return ErrAttendanceForbiddenTeam
	}

	isCoach, err := s.resolveTrainerRole(ctx, teamID, authUserID)
	if err != nil {
		return err
	}
	if !isCoach {
		return ErrAttendanceForbiddenTeam
	}

	deleted, err := s.attendanceDao.DeleteByID(ctx, attendanceID, teamID)
	if err != nil {
		return err
	}
	if !deleted {
		// La fila cambió entre el FindByID y el DELETE (otro request la borró).
		// Para el cliente el resultado observable es el mismo que ya estaba.
		return ErrAttendanceNotFound
	}
	return nil
}
