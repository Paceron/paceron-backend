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
	ListAttendanceSessions(c *gin.Context)
	GetSessionAttendance(c *gin.Context)
	BulkSaveAttendance(c *gin.Context)
	DeleteAttendance(c *gin.Context)
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
	// Se resuelve el id del usuario (y no solo se chequea que exista) porque
	// GenerateQR ahora exige ser entrenador del equipo, no solo estar autenticado.
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
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

	response, err := ac.attendanceService.GenerateQR(c, authUserID, *teamID, *sessionID)
	if err != nil {
		// 403 si no es entrenador del equipo; 422 si la sesión no es válida
		// (no presencial, cancelada, de otro equipo o inexistente); 404 si el
		// equipo no existe.
		attendanceWriteError(c, err)
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

	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	created, err := ac.attendanceService.Register(c, authUserID, teamID, sessionID)
	if err != nil {
		// 403 si el corredor no es miembro activo del grupo de la sesión. La
		// idempotencia (200 en vez de 201) no pasa por acá: esa es la única
		// respuesta de error que el service signaliza con created == false y
		// err == nil.
		if errors.Is(err, services.ErrAttendanceNotGroupMember) {
			c.JSON(http.StatusForbidden, apierror.APIError{
				StatusCode: http.StatusForbidden,
				Code:       "Forbidden",
				Message:    err.Error(),
			})
			return
		}
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

// ListAttendanceSessions godoc
// @Summary      Listar sesiones con asistencia gestionable de un grupo
// @Description  Devuelve las sesiones presenciales ya ocurridas de un grupo, ordenadas de la más reciente a la más antigua, con la cantidad de asistencias de cada una. Solo días training + presencial + fecha <= hoy. Exige que el usuario autenticado sea entrenador del team_id y que el grupo pertenezca a ese equipo.
// @Tags         attendance
// @Accept       json
// @Produce      json
// @Param        id         path   int  true   "ID del grupo"
// @Param        team_id   query  int  true   "ID del equipo (obligatorio)"
// @Success      200  {object}  attendance.SessionAttendanceListResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/groups/{group_id}/attendance-sessions [get]
func (ac *attendanceController) ListAttendanceSessions(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	// El param de la ruta es "id" (no "group_id") por la convención que ya usan
	// las demás rutas de grupos; gin no permite mezclar nombres distintos en el
	// mismo segmento del path.
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || groupID <= 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group_id debe ser un número entero mayor a 0",
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
	if teamID == nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id es obligatorio",
		})
		return
	}

	response, err := ac.attendanceService.ListAttendanceSessions(c, authUserID, *teamID, groupID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		switch {
		case errors.Is(err, services.ErrForbiddenAttendance):
			statusCode = http.StatusForbidden
			code = "Forbidden"
		case errors.Is(err, services.ErrAttendanceGroupNotFound), errors.Is(err, services.ErrTeamNotFound):
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

	c.JSON(http.StatusOK, response)
}

// GetSessionAttendance godoc
// @Summary      Ver la grilla de asistencia de una sesión
// @Description  Devuelve el roster del grupo cruzado con el estado de asistencia de cada corredor, más los agregados de la sesión. El roster son los miembros con membresía activa en la FECHA DE LA SESIÓN (no en la fecha de hoy). Exige que el usuario autenticado sea entrenador del team_id, que la sesión sea presencial y no cancelada, y que pertenezca al grupo y al equipo indicados.
// @Tags         attendance
// @Accept       json
// @Produce      json
// @Param        session_instance_id  path   int  true   "ID de la sesión instanciada"
// @Param        group_id             query  int  true   "ID del grupo"
// @Param        team_id              query  int  true   "ID del equipo (obligatorio)"
// @Success      200  {object}  attendance.SessionAttendanceResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      422  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/attendance/session/{session_instance_id} [get]
func (ac *attendanceController) GetSessionAttendance(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	sessionInstanceID, err := strconv.ParseInt(c.Param("session_instance_id"), 10, 64)
	if err != nil || sessionInstanceID <= 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "session_instance_id debe ser un número entero mayor a 0",
		})
		return
	}
	groupID, err := parsePositiveQueryParam(c, "group_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	if groupID == nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "group_id es obligatorio",
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
	if teamID == nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "team_id es obligatorio",
		})
		return
	}

	response, err := ac.attendanceService.GetSessionAttendance(c, authUserID, *teamID, *groupID, sessionInstanceID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		switch {
		case errors.Is(err, services.ErrForbiddenAttendance):
			statusCode = http.StatusForbidden
			code = "Forbidden"
		case errors.Is(err, services.ErrTeamNotFound), errors.Is(err, services.ErrAttendanceGroupNotFound):
			statusCode = http.StatusNotFound
			code = "Not Found"
		case errors.Is(err, services.ErrAttendanceSessionNotFound),
			errors.Is(err, services.ErrAttendanceSessionNotTraining),
			errors.Is(err, services.ErrAttendanceSessionCancelled),
			errors.Is(err, services.ErrAttendanceSessionNotPresencial),
			errors.Is(err, services.ErrAttendanceSessionWrongTeam),
			errors.Is(err, services.ErrAttendanceGroupMismatch):
			statusCode = http.StatusUnprocessableEntity
			code = "Unprocessable Entity"
		}
		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// BulkSaveAttendance godoc
// @Summary      Carga masiva de asistencia de una sesión
// @Description  Crea o actualiza la asistencia de varios corredores a una misma sesión. Idempotente. Todo-o-nada: si algún corredor no era miembro del grupo en la fecha de la sesión, no se escribe ninguna fila.
// @Tags         attendance
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string                      true  "Bearer token"
// @Param        request        body   attendance.BulkSaveRequest   true  "Corredores a marcar"
// @Success      200            {object}  attendance.BulkSaveResult
// @Failure      400            {object}  apierror.APIError
// @Failure      401            {object}  apierror.APIError
// @Failure      403            {object}  apierror.APIError
// @Failure      404            {object}  apierror.APIError
// @Failure      422            {object}  apierror.APIError
// @Failure      500            {object}  apierror.APIError
// @Router       /api/v1/attendance/bulk [post]
func (ac *attendanceController) BulkSaveAttendance(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	var request attendance.BulkSaveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "body inválido",
		})
		return
	}

	if err := validateBulkSaveRequest(&request); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	userIDs := make([]int64, 0, len(request.Entries))
	for _, entry := range request.Entries {
		userIDs = append(userIDs, entry.UserID)
	}

	result, err := ac.attendanceService.BulkSaveAttendance(c, authUserID, request.TeamID, request.TrainingSessionID, userIDs)
	if err != nil {
		var bulkErr *services.ErrBulkInvalidUsers
		if errors.As(err, &bulkErr) {
			// 422 con los user_id rechazados: el frontend los necesita para
			// marcar en la grilla solo las filas que no corresponden, y no puede
			// derivarlos de un mensaje genérico.
			c.JSON(http.StatusUnprocessableEntity, apierror.APIError{
				StatusCode: http.StatusUnprocessableEntity,
				Code:       "Unprocessable Entity",
				Message:    err.Error(),
				Details:    gin.H{"invalid_user_ids": bulkErr.UserIDs},
			})
			return
		}
		attendanceWriteError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteAttendance godoc
// @Summary      Borrar una asistencia cargada
// @Description  Borra físicamente una asistencia por id. Requiere ser entrenador del equipo indicado. 404 si la asistencia no existe; 403 si es de otro equipo o el usuario no es entrenador.
// @Tags         attendance
// @Produce      json
// @Param        Authorization  header  string  true  "Bearer token"
// @Param        attendance_id  path    int     true  "Id de la asistencia"
// @Param        team_id        query   int     true  "Equipo de la asistencia"
// @Success      204  "No Content"
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/attendance/{attendance_id} [delete]
func (ac *attendanceController) DeleteAttendance(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	attendanceID, err := strconv.ParseInt(c.Param("attendance_id"), 10, 64)
	if err != nil || attendanceID <= 0 {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "attendance_id debe ser un número entero mayor a 0",
		})
		return
	}

	// team_id va en el query y es obligatorio aunque no haya ":team_id" en el
	// path: es contra ese equipo que se autoriza el borrado, y sin él no hay
	// forma de decidir entre 403 y 404.
	teamID, err := parsePositiveQueryParam(c, "team_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
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

	if err := ac.attendanceService.DeleteAttendance(c, authUserID, *teamID, attendanceID); err != nil {
		attendanceWriteError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// validateBulkSaveRequest valida la FORMA del body. Las reglas de negocio (rol del
// usuario, equipo, sesión válida, membresía del grupo) son del service, que las
// corre con datos de la DB.
func validateBulkSaveRequest(request *attendance.BulkSaveRequest) error {
	if request.TeamID <= 0 {
		return errors.New("team_id debe ser un número entero mayor a 0")
	}
	if request.TrainingSessionID <= 0 {
		return errors.New("training_session_id debe ser un número entero mayor a 0")
	}
	// Un `entries` vacío NO es un error: la spec lo define como no-op que
	// responde 200 con los contadores en cero. El service y el DAO lo tratan
	// como tal (BulkUpsertManual devuelve 0,0 sin tocar la DB), así que acá solo
	// se valida la forma de cada entrada cuando hay alguna.
	for i, entry := range request.Entries {
		if entry.UserID <= 0 {
			return fmt.Errorf("entries[%d].user_id debe ser un entero mayor a 0", i)
		}
	}
	return nil
}

// attendanceWriteError mapea a status los errores de los endpoints de escritura.
// Va aparte del mapeo de los endpoints de lectura porque el de carga masiva tiene un
// 422 con payload propio (los user_id rechazados), que el handler de bulk
// intercepta antes de llegar acá.
func attendanceWriteError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	code := "Internal Server Error"
	switch {
	case errors.Is(err, services.ErrForbiddenAttendance), errors.Is(err, services.ErrAttendanceForbiddenTeam):
		statusCode = http.StatusForbidden
		code = "Forbidden"
	case errors.Is(err, services.ErrAttendanceNotFound),
		errors.Is(err, services.ErrTeamNotFound),
		errors.Is(err, services.ErrAttendanceGroupNotFound):
		statusCode = http.StatusNotFound
		code = "Not Found"
	case errors.Is(err, services.ErrAttendanceSessionNotFound),
		errors.Is(err, services.ErrAttendanceSessionNotTraining),
		errors.Is(err, services.ErrAttendanceSessionCancelled),
		errors.Is(err, services.ErrAttendanceSessionNotPresencial),
		errors.Is(err, services.ErrAttendanceSessionWrongTeam),
		errors.Is(err, services.ErrAttendanceGroupMismatch):
		statusCode = http.StatusUnprocessableEntity
		code = "Unprocessable Entity"
	}
	c.JSON(statusCode, apierror.APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    err.Error(),
	})
}
