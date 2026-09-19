package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/attendance"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/dbs"
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
	qrAPIPathFmt = "/api/v1/attendance/team/%d/session/%d"
)

// AttendanceServiceInterface define las operaciones de negocio de asistencias.
type AttendanceServiceInterface interface {
	GenerateQR(ctx *gin.Context, teamID, sessionID int64) (*attendance.QRResponse, error)
	Register(ctx *gin.Context, userID, teamID, sessionID int64) (created bool, err error)
	Search(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error)
}

type attendanceService struct {
	attendanceDao daos.AttendanceDAOInterface
	baseURL       string
}

// NewAttendanceService crea una nueva instancia de AttendanceService. baseURL es
// la URL pública del backend que se embebe en el QR (config.AttendanceBaseURL).
func NewAttendanceService(attendanceDao daos.AttendanceDAOInterface, baseURL string) AttendanceServiceInterface {
	return &attendanceService{
		attendanceDao: attendanceDao,
		baseURL:       baseURL,
	}
}

// GenerateQR construye la URL de registro y genera un QR determinista (PNG en
// base64) que la codifica.
func (s *attendanceService) GenerateQR(ctx *gin.Context, teamID, sessionID int64) (*attendance.QRResponse, error) {
	baseURL := strings.TrimRight(s.baseURL, "/")
	url := fmt.Sprintf("%s%s", baseURL, fmt.Sprintf(qrAPIPathFmt, teamID, sessionID))

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
func (s *attendanceService) Register(ctx *gin.Context, userID, teamID, sessionID int64) (bool, error) {
	attendance := &dbs.Attendance{TeamID: teamID, TrainingSessionID: sessionID, UserID: userID}

	err := s.attendanceDao.Create(ctx, attendance)
	if err != nil {
		if errors.Is(err, daos.ErrAttendanceAlreadyExists) {
			return false, nil
		}
		return false, fmt.Errorf("error registrando la asistencia: %w", err)
	}
	return true, nil
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
	isCoach, err := s.attendanceDao.IsTeamOwner(ctx, *filters.TeamID, authUserID)
	if err != nil {
		return nil, err
	}
	if !isCoach {
		role, err := s.attendanceDao.GetTeamUserRole(ctx, *filters.TeamID, authUserID)
		if err != nil {
			return nil, err
		}
		if role == "" {
			return nil, ErrForbiddenAttendance
		}
		isCoach = role == string(constants.TeamUserRoleEntrenador)
	}

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
