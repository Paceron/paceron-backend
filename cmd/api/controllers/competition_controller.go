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
	"simple-arq-golang/cmd/api/domains/runnersession"
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
	ListAttempts(c *gin.Context)
	Close(c *gin.Context)
	CreateAttempt(c *gin.Context)
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
// @Description  El creador entrenador (owner del team) crea con status active directo; el creador corredor (miembro del team) propone con status pending_approval. type=metric exige metric (distance, duration, pace, peak_speed) y end_at posterior a start_at; type=route exige route_geojson LineString (máx 500 vértices, máx 100 km) y acceptance_radius_meters en [5, 500]. Al menos 2 participantes, todos miembros activos del team, sin incluir al trainer.
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
// @Description  Accesible para miembros del team u owner; 404 si no existe. type=metric muestra el acumulado de la métrica por participante dentro de [start_at, end_at o now]; type=route muestra el mejor intento por atleta computado on-demand, sin broadcast WS. El nombre del campo "progress" del detalle depende del type de la competencia y swag no puede resolver la interfaz: para type=metric cada ítem de progress es {user_id: int, value: number|null} y para type=route es {user_id: int, compliance_ratio: number|null, duration_ms: int|null, session_instance_id: int|null} (mejor intento del atleta: máximo compliance_ratio, empate → menor duration_ms; los tres nulls si aún no tiene intentos finished (un intento wip/interrupted ya registrado también se ve nulo)).
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

// ListAttempts godoc
// @Summary      Listar los intentos de un atleta en una competencia
// @Description  Guard igual que el detalle: 404 si la competencia no existe (antes de todo guard), 403 si el caller no es miembro de grupo del team ni owner. Sin query athlete_user_id lista los intentos del caller; con la query, los de ese atleta (cualquier miembro puede ver los de otro; no-participante = lista vacía). Ítems orden por id ASC: distance_meters del feedback activo (null si no hay), compliance_ratio solo en finished (0 sin puntos GPS, matching on-demand) y duration_ms end-start server-side (null en wip).
// @Tags         gamification
// @Produce      json
// @Param              id              path  int     true  "ID de la competencia"
// @Param              athlete_user_id query int     false "Atleta cuyos intentos listar (default: el caller)"
// @Success      200  {object}  competition.CompetitionAttemptsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/competitions/{id}/attempts [get]
func (cc *competitionController) ListAttempts(c *gin.Context) {
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
	athleteQuery, err := parseOptionalPositiveQueryParam(c, "athlete_user_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    err.Error(),
		})
		return
	}
	athleteUserID := authUserID
	if athleteQuery != nil {
		athleteUserID = *athleteQuery
	}
	resp, err := cc.competitionService.ListAttempts(c, authUserID, competitionID, athleteUserID)
	if err != nil {
		respondCompetitionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Close godoc
// @Summary      Cerrar una competencia activa
// @Description  Cierre SIEMPRE manual (caller creador — trainer o runner — u owner del team). Solo sobre active; type=metric exige además ventana terminada (end_at pasado), route se cierra cuando el caller decide: status finished + finished_at + winners calculados (metric: mejor valor en ventana con empate exacto = todos ganan; route: mejor % de match con tie-break de menor duración). El ganador recibe stamp; a todos los participantes llegan unseen y push best-effort; a los canales de los intentos registrados sale control:competition_finished.
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

// CreateAttempt godoc
// @Summary      Registrar el intento del atleta en una competencia de ruta
// @Description  Solo competencias type=route en status active y con el caller entre los participantes: crea su estado de sesión wip tagueado con competition_id. El primer intento reusa la instancia sintética compartida de la competencia; los siguientes crean una instancia propia nueva ("Competencia ruta N (intento M)") cada vez, hasta agotar max_attempts si la competencia configuró uno (400 con "llegaste al límite de intentos de esta competencia"). El finish de cada intento es el PATCH /session-instances/{id}/runner existente. NO aplica el gate presencial: la instancia sintética no pertenece a ningún día de calendario.
// @Tags         gamification
// @Produce      json
// @Param        id    path  int  true  "ID de la competencia"
// @Success      201  {object}  runnersession.MutationResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/competitions/{id}/attempts [post]
func (cc *competitionController) CreateAttempt(c *gin.Context) {
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
	rs, err := cc.competitionService.CreateAttempt(c, authUserID, competitionID)
	if err != nil {
		respondCompetitionError(c, err)
		return
	}
	response := toRunnerSessionResponse(rs)
	c.JSON(http.StatusCreated, runnersession.MutationResponse{
		Message: competition.MsgCompetitionAttemptCreated,
		Data:    &response,
	})
}
