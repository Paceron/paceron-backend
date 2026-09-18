package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/dbs"
	"simple-arq-golang/cmd/api/domains/workoutfeedback"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// WorkoutFeedbackController define los handlers HTTP del módulo de feedback.
type WorkoutFeedbackController interface {
	Create(c *gin.Context)
	GetByID(c *gin.Context)
	Search(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
}

type workoutFeedbackController struct {
	workoutFeedbackService services.WorkoutFeedbackServiceInterface
}

// NewWorkoutFeedbackController crea una nueva instancia de WorkoutFeedbackController.
func NewWorkoutFeedbackController(workoutFeedbackService services.WorkoutFeedbackServiceInterface) WorkoutFeedbackController {
	return &workoutFeedbackController{
		workoutFeedbackService: workoutFeedbackService,
	}
}

// parsePositivePathParam parsea un path param como int64 estrictamente mayor a 0.
func parsePositivePathParam(c *gin.Context, name string) (int64, error) {
	value, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s debe ser un número entero mayor a 0", name)
	}
	return value, nil
}

// parseOptionalQueryParamBase parsea un query param opcional como int64 > 0. nil si ausente.
func parseOptionalPositiveQueryParam(c *gin.Context, name string) (*int64, error) {
	return parsePositiveQueryParam(c, name)
}

// respondFeedbackError mapea los errores de negocio del service a status HTTP.
func respondFeedbackError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	code := "Internal Server Error"
	switch {
	case errors.Is(err, services.ErrWorkoutFeedbackInvalid):
		statusCode = http.StatusBadRequest
		code = "Bad request"
	case errors.Is(err, services.ErrWorkoutFeedbackForbidden):
		statusCode = http.StatusForbidden
		code = "Forbidden"
	case errors.Is(err, daos.ErrWorkoutFeedbackNotFound), errors.Is(err, services.ErrTeamNotFound):
		statusCode = http.StatusNotFound
		code = "Not Found"
	case errors.Is(err, daos.ErrWorkoutFeedbackDuplicate):
		statusCode = http.StatusConflict
		code = "Conflict"
	}
	c.JSON(statusCode, apierror.APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    err.Error(),
	})
}

// toWorkoutFeedbackResponse mapea el modelo a la respuesta plana (media_urls []string).
func toWorkoutFeedbackResponse(feedback *dbs.WorkoutFeedback) workoutfeedback.WorkoutFeedbackResponse {
	mediaURLs := []string{}
	for _, element := range feedback.MediaURLs.Elements {
		if element.Status == pgtype.Present {
			mediaURLs = append(mediaURLs, element.String)
		}
	}
	return workoutfeedback.WorkoutFeedbackResponse{
		ID:                  feedback.ID,
		TeamID:              feedback.TeamID,
		AssignedSessionID:   feedback.AssignedSessionID,
		AssignedExerciseID:  feedback.AssignedExerciseID,
		AthleteUserID:       feedback.AthleteUserID,
		FeedbackOwnerUserID: feedback.FeedbackOwnerUserID,
		ReportSource:        feedback.ReportSource,
		SessionDate:         feedback.SessionDate.Format("2006-01-02"),
		SetNumber:           feedback.SetNumber,
		StartedAt:           feedback.StartedAt,
		EndedAt:             feedback.EndedAt,
		DurationMs:          feedback.DurationMs,
		ActiveDurationMs:    feedback.ActiveDurationMs,
		WeightKg:            feedback.WeightKg,
		Reps:                feedback.Reps,
		DistanceMeters:      feedback.DistanceMeters,
		RPE:                 feedback.RPE,
		AvgHeartRate:        feedback.AvgHeartRate,
		MaxHeartRate:        feedback.MaxHeartRate,
		CompletionStatus:    feedback.CompletionStatus,
		ElevationGainMeters: feedback.ElevationGainMeters,
		Cadence:             feedback.Cadence,
		Annotations:         feedback.Annotations,
		MediaURLs:           mediaURLs,
		CreatedAt:           feedback.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:           feedback.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// Create godoc
// @Summary      Crear feedback de entrenamiento
// @Description  Crea el feedback de un entrenamiento. El reportante es siempre el usuario autenticado; si athlete_user_id viene y es distinto, solo un entrenador (owner de un equipo del atleta) puede reportar por él. Duplicado del mismo set activo → 409.
// @Tags         workout-feedback
// @Accept       json
// @Produce      json
// @Param        body  body  workoutfeedback.CreateFeedbackRequest  true  "Datos del feedback"
// @Success      201  {object}  workoutfeedback.MutationResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/workout-feedback [post]
func (fc *workoutFeedbackController) Create(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	var req workoutfeedback.CreateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	feedback, err := fc.workoutFeedbackService.Create(c, authUserID, req)
	if err != nil {
		respondFeedbackError(c, err)
		return
	}

	response := toWorkoutFeedbackResponse(feedback)
	c.JSON(http.StatusCreated, workoutfeedback.MutationResponse{
		Message: workoutfeedback.MsgFeedbackCreated,
		Data:    &response,
	})
}

// GetByID godoc
// @Summary      Obtener feedback por id
// @Description  Devuelve el feedback activo con ese id. Visible solo para el atleta, el reportante o el owner del equipo.
// @Tags         workout-feedback
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "ID del feedback"
// @Success      200  {object}  workoutfeedback.WorkoutFeedbackResponse
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/workout-feedback/{id} [get]
func (fc *workoutFeedbackController) GetByID(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	feedbackID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	feedback, err := fc.workoutFeedbackService.GetByID(c, authUserID, feedbackID)
	if err != nil {
		respondFeedbackError(c, err)
		return
	}

	c.JSON(http.StatusOK, toWorkoutFeedbackResponse(feedback))
}

// Search godoc
// @Summary      Buscar feedbacks
// @Description  Busca feedbacks aplicando la matriz de autorización. Sin parámetros devuelve los del usuario autenticado (como atleta o reportante); con team_id se requiere ser owner; con athlete_user_id ajeno, owner de un equipo del atleta.
// @Tags         workout-feedback
// @Accept       json
// @Produce      json
// @Param        team_id               query  int     false  "ID del equipo"
// @Param        athlete_user_id       query  int     false  "ID del atleta"
// @Param        feedback_owner_user_id query  int     false  "ID del reportante"
// @Param        assigned_session_id   query  int     false  "ID de la sesión asignada"
// @Param        assigned_exercise_id  query  int     false  "ID del ejercicio asignado"
// @Param        session_date_from     query  string  false  "Desde (YYYY-MM-DD)"
// @Param        session_date_to       query  string  false  "Hasta (YYYY-MM-DD)"
// @Success      200  {object}  workoutfeedback.SearchResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/workout-feedback/search [get]
func (fc *workoutFeedbackController) Search(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	filters, err := parseSearchFilters(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	records, err := fc.workoutFeedbackService.Search(c, authUserID, filters)
	if err != nil {
		respondFeedbackError(c, err)
		return
	}

	response := make([]workoutfeedback.WorkoutFeedbackResponse, 0, len(records))
	for i := range records {
		response = append(response, toWorkoutFeedbackResponse(&records[i]))
	}
	c.JSON(http.StatusOK, workoutfeedback.SearchResponse{Data: response})
}

// parseSearchFilters lee y valida los query params de búsqueda.
func parseSearchFilters(c *gin.Context) (workoutfeedback.SearchFilters, error) {
	var filters workoutfeedback.SearchFilters
	var err error

	if filters.TeamID, err = parseOptionalPositiveQueryParam(c, "team_id"); err != nil {
		return filters, err
	}
	if filters.AthleteUserID, err = parseOptionalPositiveQueryParam(c, "athlete_user_id"); err != nil {
		return filters, err
	}
	if filters.FeedbackOwnerUserID, err = parseOptionalPositiveQueryParam(c, "feedback_owner_user_id"); err != nil {
		return filters, err
	}
	if filters.AssignedSessionID, err = parseOptionalPositiveQueryParam(c, "assigned_session_id"); err != nil {
		return filters, err
	}
	if filters.AssignedExerciseID, err = parseOptionalPositiveQueryParam(c, "assigned_exercise_id"); err != nil {
		return filters, err
	}

	if from := c.Query("session_date_from"); from != "" {
		fromCopy := from
		filters.SessionDateFrom = &fromCopy
	}
	if to := c.Query("session_date_to"); to != "" {
		toCopy := to
		filters.SessionDateTo = &toCopy
	}
	return filters, nil
}

// Update godoc
// @Summary      Editar feedback
// @Description  Edita parcialmente el feedback activo (solo los campos provistos). Utilizable por el atleta, el reportante o el owner del equipo. athlete_user_id / feedback_owner_user_id no son editables.
// @Tags         workout-feedback
// @Accept       json
// @Produce      json
// @Param        id    path  int                                  true  "ID del feedback"
// @Param        body  body  workoutfeedback.UpdateFeedbackRequest  true  "Campos a editar"
// @Success      200  {object}  workoutfeedback.MutationResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      409  {object}  apierror.APIError
// @Router       /api/v1/workout-feedback/{id} [put]
func (fc *workoutFeedbackController) Update(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	feedbackID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	var req workoutfeedback.UpdateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	feedback, err := fc.workoutFeedbackService.Update(c, authUserID, feedbackID, req)
	if err != nil {
		respondFeedbackError(c, err)
		return
	}

	response := toWorkoutFeedbackResponse(feedback)
	c.JSON(http.StatusOK, workoutfeedback.MutationResponse{
		Message: workoutfeedback.MsgFeedbackUpdated,
		Data:    &response,
	})
}

// Delete godoc
// @Summary      Eliminar feedback (baja lógica)
// @Description  Da de baja lógicamente el feedback activo (deleted_at = now). No borra filas; un mismo set puede volver a reportarse tras la baja. Utilizable por el atleta, el reportante o el owner del equipo.
// @Tags         workout-feedback
// @Accept       json
// @Produce      json
// @Param        id  path  int  true  "ID del feedback"
// @Success      204  {string}  string
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/workout-feedback/{id} [delete]
func (fc *workoutFeedbackController) Delete(c *gin.Context) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return
	}

	feedbackID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}

	if err := fc.workoutFeedbackService.SoftDelete(c, authUserID, feedbackID); err != nil {
		respondFeedbackError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}