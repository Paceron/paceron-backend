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
	"simple-arq-golang/cmd/api/domains/dbs"
)

// Errores de negocio del módulo de asistencias. El controller los mapea a los
// status HTTP correspondientes vía errors.Is.
var (
	// ErrForbiddenAttendance indica que el usuario autenticado no tiene permiso
	// para ver las asistencias solicitadas (falla de los escenarios B o C).
	ErrForbiddenAttendance = errors.New("no tenés permisos para ver estas asistencias")
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

// Search aplica la matriz de autorización (escenarios A/B/C del spec) y delega la
// búsqueda al DAO con los filtros ya autorizados.
//
//	Escenario A: user_id propio o sin parámetros -> scope self.
//	Escenario B: solo team_id (o team_id + training_session_id) -> el auth_user_id
//	  debe ser el owner del team (404 si no existe, 403 si no es owner).
//	Escenario C: user_id ajeno -> el auth_user_id debe ser owner de al menos un
//	  team al que pertenezca el usuario objetivo (403 si no hay relación).
func (s *attendanceService) Search(ctx *gin.Context, authUserID int64, filters attendance.SearchFilters) ([]dbs.Attendance, error) {
	if filters.UserID != nil {
		if *filters.UserID != authUserID {
			inOwnedTeam, err := s.attendanceDao.ExistsUserInTeamOwnedBy(ctx, *filters.UserID, authUserID)
			if err != nil {
				return nil, err
			}
			if !inOwnedTeam {
				return nil, ErrForbiddenAttendance
			}
		}
		return s.attendanceDao.Search(ctx, daos.AttendanceSearchFilters{
			TeamID:            filters.TeamID,
			TrainingSessionID: filters.TrainingSessionID,
			UserID:            filters.UserID,
		})
	}

	if filters.TeamID != nil {
		exists, err := s.attendanceDao.TeamExists(ctx, *filters.TeamID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrTeamNotFound
		}

		isOwner, err := s.attendanceDao.IsTeamOwner(ctx, *filters.TeamID, authUserID)
		if err != nil {
			return nil, err
		}
		if !isOwner {
			return nil, ErrForbiddenAttendance
		}

		return s.attendanceDao.Search(ctx, daos.AttendanceSearchFilters{
			TeamID:            filters.TeamID,
			TrainingSessionID: filters.TrainingSessionID,
		})
	}

	// Sin parámetros: el usuario consulta sus propias asistencias.
	return s.attendanceDao.Search(ctx, daos.AttendanceSearchFilters{
		TrainingSessionID: filters.TrainingSessionID,
		UserID:            &authUserID,
	})
}
