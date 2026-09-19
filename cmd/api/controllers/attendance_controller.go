package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/attendance"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// AttendanceController define los handlers HTTP del módulo de asistencias.
type AttendanceController interface {
	GenerateQR(c *gin.Context)
	RegisterAttendance(c *gin.Context)
	Search(c *gin.Context)
}

type attendanceController struct {
	attendanceService services.AttendanceServiceInterface
}

// NewAttendanceController crea una nueva instancia de AttendanceController.
func NewAttendanceController(attendanceService services.AttendanceServiceInterface) AttendanceController {
	return &attendanceController{
		attendanceService: attendanceService,
	}
}

// parsePositiveQueryParam parsea un query param opcional como int64 estrictamente
// mayor a 0. Devuelve nil si el param está ausente; error si viene con un valor
// inválido (no numérico o <= 0), que el caller mapea a 400.
func parsePositiveQueryParam(c *gin.Context, name string) (*int64, error) {
	raw := c.Query(name)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return nil, fmt.Errorf("%s debe ser un número entero mayor a 0", name)
	}
	return &value, nil
}

// GenerateQR godoc
// @Summary      Generar QR de asistencia
// @Description  Genera el código QR determinista para registrar asistencias de una sesión de entrenamiento de un equipo. Requiere autenticación. Mismos inputs siempre producen el mismo QR.
// @Tags         attendance
// @Accept       json
// @Produce      json
// @Param        team_id              query   int  true  "ID del equipo"
// @Param        training_session_id  query   int  true  "ID de la sesión de entrenamiento"
// @Success      200  {object}  attendance.QRResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/attendance/qr [get]
func (ac *attendanceController) GenerateQR(c *gin.Context) {
	if _, ok := utils.GetAuthUserID(c); !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	teamID, err := parsePositiveQueryParam(c, "team_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	sessionID, err := parsePositiveQueryParam(c, "training_session_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	if teamID == nil || sessionID == nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id y training_session_id son obligatorios",
		})
		return
	}

	response, err := ac.attendanceService.GenerateQR(c, *teamID, *sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// RegisterAttendance godoc
// @Summary      Registrar asistencia
// @Description  Registra la asistencia del usuario autenticado a la sesión de entrenamiento. Idempotente: la primera vez responde 201, si ya estaba registrada responde 200 sin duplicar.
// @Tags         attendance
// @Accept       json
// @Produce      json
// @Param        team_id              path  int  true  "ID del equipo"
// @Param        training_session_id  path  int  true  "ID de la sesión de entrenamiento"
// @Success      201  {object}  attendance.RegisterResponse
// @Success      200  {object}  attendance.RegisterResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/attendance/team/{team_id}/session/{training_session_id} [post]
func (ac *attendanceController) RegisterAttendance(c *gin.Context) {
	if _, ok := utils.GetAuthUserID(c); !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	teamID, err := strconv.ParseInt(c.Param("team_id"), 10, 64)
	if err != nil || teamID <= 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id debe ser un número entero mayor a 0",
		})
		return
	}
	sessionID, err := strconv.ParseInt(c.Param("training_session_id"), 10, 64)
	if err != nil || sessionID <= 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "training_session_id debe ser un número entero mayor a 0",
		})
		return
	}

	authUserID, _ := utils.GetAuthUserID(c)
	created, err := ac.attendanceService.Register(c, authUserID, teamID, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    err.Error(),
		})
		return
	}

	statusCode := http.StatusCreated
	message := attendance.MessageRegistered
	if !created {
		statusCode = http.StatusOK
		message = attendance.MessageAlreadyExists
	}
	c.JSON(statusCode, attendance.RegisterResponse{Message: message})
}

// Search godoc
// @Summary      Buscar asistencias
// @Description  Busca asistencias de un equipo. Obligatorio enviar al menos un query param y el team_id. El usuario del token debe ser entrenador o corredor del equipo: el entrenador ve todas las asistencias del equipo, el corredor solo las suyas.
// @Tags         attendance
// @Accept       json
// @Produce      json
// @Param        team_id              query  int  true  "ID del equipo (obligatorio)"
// @Param        training_session_id  query  int  false  "ID de la sesión de entrenamiento"
// @Param        user_id              query  int  false  "ID del usuario"
// @Success      200  {object}  attendance.SearchResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/attendance/search [get]
func (ac *attendanceController) Search(c *gin.Context) {
	if _, ok := utils.GetAuthUserID(c); !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	teamID, err := parsePositiveQueryParam(c, "team_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	sessionID, err := parsePositiveQueryParam(c, "training_session_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	userID, err := parsePositiveQueryParam(c, "user_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	if teamID == nil && sessionID == nil && userID == nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "debe venir al menos un query param",
		})
		return
	}

	if teamID == nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id es obligatorio",
		})
		return
	}

	authUserID, _ := utils.GetAuthUserID(c)
	records, err := ac.attendanceService.Search(c, authUserID, attendance.SearchFilters{
		TeamID:            teamID,
		TrainingSessionID: sessionID,
		UserID:            userID,
	})
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		switch {
		case errors.Is(err, services.ErrForbiddenAttendance):
			statusCode = http.StatusForbidden
			code = "Forbidden"
		case errors.Is(err, services.ErrTeamNotFound):
			statusCode = http.StatusNotFound
			code = "Not Found"
		}
		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, attendance.SearchResponse{Data: records})
}
