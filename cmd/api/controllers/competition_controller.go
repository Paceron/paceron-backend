package controllers

// Competencias (gamificacion-core E4.1, D9): CRUD, moderación y cierre. Task 6
// (E4.2): el cierre emite control:competition_finished por los canales de los
// intentos registrados (best-effort, nil-safe como el resto de los hooks).

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/daos"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/competition"
	"simple-arq-golang/cmd/api/realtime"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// CompetitionController define los handlers HTTP del módulo de competencias.
type CompetitionController interface {
	Create(c *gin.Context)
	Approve(c *gin.Context)
	Reject(c *gin.Context)
	Cancel(c *gin.Context)
	ListByTeam(c *gin.Context)
	GetDetail(c *gin.Context)
	Close(c *gin.Context)
}

type competitionController struct {
	competitionService services.CompetitionServiceInterface
	// notifier es opcional (nil): emite control:competition_finished al
	// cerrar (D9). Sin intentos registrados no emite: el push alcanza al resto.
	notifier realtime.Notifier
}

func NewCompetitionController(competitionService services.CompetitionServiceInterface, notifier realtime.Notifier) CompetitionController {
	return &competitionController{competitionService: competitionService, notifier: notifier}
}

// respondCompetitionError mapea los errores de negocio del service a status HTTP.
func respondCompetitionError(c *gin.Context, err error) {
	statusCode := http.StatusInternalServerError
	code := "Internal Server Error"
	message := "error interno"
	switch {
	case errors.Is(err, services.ErrCompetitionInvalid):
		statusCode = http.StatusBadRequest
		code = "Bad request"
		message = err.Error()
	case errors.Is(err, services.ErrCompetitionForbidden):
		statusCode = http.StatusForbidden
		code = "Forbidden"
		message = err.Error()
	case errors.Is(err, daos.ErrCompetitionNotFound):
		statusCode = http.StatusNotFound
		code = "Not Found"
		message = err.Error()
	case errors.Is(err, services.ErrCompetitionConflict):
		statusCode = http.StatusBadRequest
		code = "Bad request"
		message = err.Error()
	}
	c.JSON(statusCode, apierror.APIError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
	})
}

func requireAuthForCompetition(c *gin.Context) (int64, bool) {
	authUserID, ok := utils.GetAuthUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, apierror.APIError{
			StatusCode: http.StatusUnauthorized,
			Code:       "unauthorized",
			Message:    "no se pudo resolver el usuario autenticado",
		})
		return 0, false
	}
	return authUserID, true
}

func respondCompetitionMutation(c *gin.Context, resp *competition.CompetitionResponse, err error) {
	if err != nil {
		respondCompetitionError(c, err)
		return
	}
	c.JSON(http.StatusOK, competition.CompetitionDetailResponse{Data: *resp})
}

// Create godoc
// @Summary      Crear una competencia en un equipo
// @Description  El creador entrenador (owner del team) crea con status active directo; el creador corredor (miembro del team) propone con status pending_approval. type=metric exige metric (distance, duration, pace, peak_speed) y end_at posterior a start_at; type=route exige route_geojson y acceptance_radius_meters > 0. Al menos 2 participantes, todos miembros activos del team, sin incluir al trainer.
// @Tags         gamification
// @Accept       json
// @Produce      json
// @Param        id    path  int                                          true  "ID del equipo"
// @Param        body  body  competition.CreateCompetitionRequest          true  "Datos de la competencia"
// @Success      200  {object}  competition.CompetitionDetailResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/competitions [post]
func (cc *competitionController) Create(c *gin.Context) {
	authUserID, ok := requireAuthForCompetition(c)
	if !ok {
		return
	}
	teamID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	var req competition.CreateCompetitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	resp, err := cc.competitionService.Create(c, authUserID, teamID, req)
	respondCompetitionMutation(c, resp, err)
}

// Approve godoc
// @Summary      Aprobar una competencia pendiente
// @Description  Caller owner del team (o el trainer creador, caso defensivo). Solo sobre pending_approval: pasa a active. Push best-effort al creador corredor.
// @Tags         gamification
// @Produce      json
// @Param        id    path  int  true  "ID de la competencia"
// @Success      200  {object}  competition.CompetitionDetailResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/competitions/{id}/approve [patch]
func (cc *competitionController) Approve(c *gin.Context) {
	authUserID, ok := requireAuthForCompetition(c)
	if !ok {
		return
	}
	competitionID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	resp, err := cc.competitionService.Approve(c, authUserID, competitionID)
	respondCompetitionMutation(c, resp, err)
}

// Reject godoc
// @Summary      Rechazar una competencia pendiente
// @Description  Caller owner del team (o el trainer creador, caso defensivo). Solo sobre pending_approval: pasa a rejected con reason opcional persistida. Push best-effort al creador corredor.
// @Tags         gamification
// @Accept       json
// @Produce      json
// @Param        id    path  int                                  true  "ID de la competencia"
// @Param        body  body  competition.RejectRequest             false  "Motivo del rechazo (opcional)"
// @Success      200  {object}  competition.CompetitionDetailResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/competitions/{id}/reject [patch]
func (cc *competitionController) Reject(c *gin.Context) {
	authUserID, ok := requireAuthForCompetition(c)
	if !ok {
		return
	}
	competitionID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	var req competition.RejectRequest
	// Body opcional: si viene, se parsea; sin body o malformado se sigue con
	// reason nil (el rechazo no exige motivo).
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && req.Reason == nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    err.Error(),
			})
			return
		}
	}
	resp, err := cc.competitionService.Reject(c, authUserID, competitionID, req.Reason)
	respondCompetitionMutation(c, resp, err)
}

// Cancel godoc
// @Summary      Cancelar una competencia propia pendiente
// @Description  Solo el creador corredor, solo sobre pending_approval: borra físicamente la competencia junto con sus participantes.
// @Tags         gamification
// @Produce      json
// @Param        id    path  int  true  "ID de la competencia"
// @Success      200  {object}  competition.CompetitionCancelResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/competitions/{id}/cancel [delete]
func (cc *competitionController) Cancel(c *gin.Context) {
	authUserID, ok := requireAuthForCompetition(c)
	if !ok {
		return
	}
	competitionID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	_, err = cc.competitionService.Cancel(c, authUserID, competitionID)
	if err != nil {
		respondCompetitionError(c, err)
		return
	}
	c.JSON(http.StatusOK, competition.CompetitionCancelResponse{Message: competition.MsgCompetitionCancelled})
}

// ListByTeam godoc
// @Summary      Listar las competencias de un equipo
// @Description  Accesible para miembros de algún grupo del team u owner (403 otro caso). Filtros opcionales status (pending_approval, active, finished, rejected — 400 fuera de la whitelist) y group_id. Embebe has_unseen de la sección competitions bajo el team_id.
// @Tags         gamification
// @Produce      json
// @Param        id        path  int     true   "ID del equipo"
// @Param        status    query string  false  "pending_approval | active | finished | rejected"
// @Param        group_id  query int     false  "Filtrar por grupo"
// @Success      200  {object}  competition.CompetitionsListResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Router       /api/v1/teams/{id}/competitions [get]
func (cc *competitionController) ListByTeam(c *gin.Context) {
	authUserID, ok := requireAuthForCompetition(c)
	if !ok {
		return
	}
	teamID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	groupID, err := parseOptionalPositiveQueryParam(c, "group_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	resp, err := cc.competitionService.ListByTeam(c, authUserID, teamID, c.Query("status"), groupID)
	if err != nil {
		respondCompetitionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetDetail godoc
// @Summary      Detalle de una competencia con progreso en vivo
// @Description  Accesible para miembros del team u owner; 404 si no existe. type=metric muestra el acumulado de la métrica por participante dentro de [start_at, end_at o now]; type=route muestra el mejor % de match (computado on-demand, sin broadcast WS).
// @Tags         gamification
// @Produce      json
// @Param        id    path  int  true  "ID de la competencia"
// @Success      200  {object}  competition.CompetitionDetailResponse
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/competitions/{id} [get]
func (cc *competitionController) GetDetail(c *gin.Context) {
	authUserID, ok := requireAuthForCompetition(c)
	if !ok {
		return
	}
	competitionID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	resp, err := cc.competitionService.Get(c, authUserID, competitionID)
	if err != nil {
		respondCompetitionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Close godoc
// @Summary      Cerrar una competencia activa
// @Description  Cierre SIEMPRE manual (caller creador — trainer o runner — u owner del team). Solo sobre active y con la ventana terminada si end_at está seteado: status finished + finished_at + winners calculados (metric: mejor valor en ventana con empate exacto = todos ganan; route: mejor % de match con tie-break de menor duración). El ganador recibe stamp; a todos los participantes llegan unseen y push best-effort; a los canales de los intentos registrados sale control:competition_finished.
// @Tags         gamification
// @Produce      json
// @Param        id    path  int  true  "ID de la competencia"
// @Success      200  {object}  competition.CompetitionDetailResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/competitions/{id}/close [patch]
func (cc *competitionController) Close(c *gin.Context) {
	authUserID, ok := requireAuthForCompetition(c)
	if !ok {
		return
	}
	competitionID, err := parsePositivePathParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	resp, attemptInstanceIDs, err := cc.competitionService.Close(c, authUserID, competitionID)
	respondCompetitionMutation(c, resp, err)
	if err == nil {
		cc.emitCompetitionFinished(competitionID, attemptInstanceIDs)
	}
}

// emitCompetitionFinished emite el frame control:competition_finished (D9) por
// los canales session:{instance} de los intentos registrados. nil-safe; sin
// intentos no emite (el push alcanza al resto).
func (cc *competitionController) emitCompetitionFinished(competitionID int64, instanceIDs []int64) {
	if cc.notifier == nil || len(instanceIDs) == 0 {
		return
	}
	for _, id := range instanceIDs {
		channel := sessionChannel(id)
		cc.notifier.Emit(channel, realtime.MarshalControlCompetitionFinished(channel, competitionID))
	}
}
