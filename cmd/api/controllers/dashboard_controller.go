package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/dashboard"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// DashboardController define los handlers HTTP del dashboard analítico.
type DashboardController interface {
	Filters(c *gin.Context)
	Summary(c *gin.Context)
	TimeSeries(c *gin.Context)
	Leaderboard(c *gin.Context)
	ExerciseSeries(c *gin.Context)
	Comparisons(c *gin.Context)
	Periods(c *gin.Context)
}

// Los tipos de respuesta se nombran acá para que swag los resuelva.
var (
	_ *dashboard.FiltersResponse
	_ *dashboard.SummaryResponse
	_ *dashboard.TimeseriesResponse
	_ *dashboard.LeaderboardResponse
	_ *dashboard.SeriesResponse
	_ *dashboard.ComparisonsResponse
	_ *dashboard.PeriodsResponse
)

type dashboardController struct {
	service services.DashboardServiceInterface
}

func NewDashboardController(service services.DashboardServiceInterface) DashboardController {
	return &dashboardController{service: service}
}

// Filters godoc
// @Summary      Filtros del dashboard
// @Description  Equipos, grupos, corredores, tipos de ejercicio, catálogo con su objetivo VIGENTE y el rango de fechas real con datos, todo recortado por el scope del solicitante. El entrenador ve sus corredores; el corredor, solo su propia entrada.
// @Tags         dashboard
// @Produce      json
// @Param        team_id  query     int64  true  "Equipo del que se está mirando el dashboard"
// @Success      200  {object}  dashboard.FiltersResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Router       /api/v1/dashboard/filters [get]
func (dc *dashboardController) Filters(c *gin.Context) {
	callerID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondDashboardUnauthorized(c)
		return
	}
	teamID, ok := requiredInt64Query(c, "team_id")
	if !ok {
		return
	}

	resp, err := dc.service.Filters(c, callerID, teamID)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Summary godoc
// @Summary      Resumen de métricas
// @Description  KPIs del período: totales, cumplimiento (porcentaje 0-100), mejor tiempo, consistencia, tendencia y estado de los datos. `compliance_rate` es porcentaje y `delta_pct` va con signo positivo hacia la mejora, incluso en el eje temporal.
// @Tags         dashboard
// @Produce      json
// @Param        team_id       query     int64    true   "Equipo del que se está mirando el dashboard"
// @Param        group_id      query     int64    false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        exercise_id   query     int64    false  "Un ejercicio de catálogo (excluyente con exercise_ids y kinds)"
// @Param        exercise_ids  query     []int64  false  "Varios ejercicios de catálogo (excluyente con exercise_id y kinds)"
// @Param        kinds         query     []string false  "Tipos vigentes (excluyente con exercise_id y exercise_ids)"  Enums(liviano, elongacion, ritmo_continuo, velocidad_tiempo, velocidad_distancia)
// @Param        from          query     string   true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to            query     string   false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance     query     number   false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Success      200  {object}  dashboard.SummaryResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/dashboard/metrics/summary [get]
func (dc *dashboardController) Summary(c *gin.Context) {
	callerID, query, ok := dashboardRequest(c)
	if !ok {
		return
	}
	resp, err := dc.service.Summary(c, callerID, query)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// TimeSeries godoc
// @Summary      Métricas por período
// @Description  Buckets de calendario (session/week/month/quarter) con los mismos KPIs del resumen. Los anclajes son de calendario: un bucket puede empezar antes del `from` y el recorte nunca parte una semana ni un trimestre por la mitad.
// @Tags         dashboard
// @Produce      json
// @Param        team_id       query     int64    true   "Equipo del que se está mirando el dashboard"
// @Param        group_id      query     int64    false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        exercise_id   query     int64    false  "Un ejercicio de catálogo (excluyente con exercise_ids y kinds)"
// @Param        exercise_ids  query     []int64  false  "Varios ejercicios de catálogo (excluyente con exercise_id y kinds)"
// @Param        kinds         query     []string false  "Tipos vigentes (excluyente con exercise_id y exercise_ids)"  Enums(liviano, elongacion, ritmo_continuo, velocidad_tiempo, velocidad_distancia)
// @Param        from          query     string   true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to            query     string   false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance     query     number   false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Param        granularity   query     string   false  "Anclaje del bucket (default session)"  Enums(session, week, month, quarter)
// @Success      200  {object}  dashboard.TimeseriesResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/dashboard/metrics/timeseries [get]
func (dc *dashboardController) TimeSeries(c *gin.Context) {
	callerID, query, ok := dashboardRequest(c)
	if !ok {
		return
	}
	resp, err := dc.service.TimeSeries(c, callerID, query)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Leaderboard godoc
// @Summary      Ranking de corredores
// @Description  Ranking paginado del scope con la posición global de cada corredor (`position`), calculada sobre todas las filas y con el mismo criterio de orden que la página. Solo el dueño del equipo.
// @Tags         dashboard
// @Produce      json
// @Param        team_id       query     int64    true   "Equipo del que se está mirando el dashboard"
// @Param        group_id      query     int64    false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        exercise_id   query     int64    false  "Un ejercicio de catálogo (excluyente con exercise_ids y kinds)"
// @Param        exercise_ids  query     []int64  false  "Varios ejercicios de catálogo (excluyente con exercise_id y kinds)"
// @Param        kinds         query     []string false  "Tipos vigentes (excluyente con exercise_id y exercise_ids)"  Enums(liviano, elongacion, ritmo_continuo, velocidad_tiempo, velocidad_distancia)
// @Param        from          query     string   true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to            query     string   false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance     query     number   false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Param        page          query     int      false  "Página (default 1)"
// @Param        page_size     query     int      false  "Tamaño de página, máximo 100 (default 20)"
// @Param        sort          query     string   false  "Columna de orden (default best_time_ms)"  Enums(best_time_ms, mean_speed_kph, total_distance_m, compliance_rate)
// @Param        order         query     string   false  "Sentido del orden (default asc)"  Enums(asc, desc)
// @Success      200  {object}  dashboard.LeaderboardResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/dashboard/leaderboard [get]
func (dc *dashboardController) Leaderboard(c *gin.Context) {
	callerID, query, ok := dashboardRequest(c)
	if !ok {
		return
	}
	query.Page = c.Query("page")
	query.PageSize = c.Query("page_size")
	query.Sort = c.Query("sort")
	query.Order = c.Query("order")

	resp, err := dc.service.Leaderboard(c, callerID, query)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ExerciseSeries godoc
// @Summary      Detalle de series de un ejercicio
// @Description  Serie por serie con objetivo congelado, cumplimiento, banda y calidad de tiempo. El `{id}` es el `exercises.id` de catálogo, el mismo que expone `/dashboard/filters`. Las instancias anteriores al change de instancia-referencia-catálogo se resuelven por nombre normalizado contra el catálogo del mismo dueño.
// @Tags         dashboard
// @Produce      json
// @Param        id         path      int64   true   "Ejercicio de catálogo (exercises.id)"
// @Param        team_id    query     int64   true   "Equipo del que se está mirando el dashboard"
// @Param        group_id   query     int64   false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        from       query     string  true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to         query     string  false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance  query     number  false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Param        page       query     int     false  "Página (default 1)"
// @Param        page_size  query     int     false  "Tamaño de página, máximo 100 (default 20)"
// @Success      200  {object}  dashboard.SeriesResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/dashboard/exercises/{id}/series [get]
func (dc *dashboardController) ExerciseSeries(c *gin.Context) {
	callerID, query, ok := dashboardRequest(c)
	if !ok {
		return
	}
	exerciseID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || exerciseID <= 0 {
		respondDashboardParamError(c, "INVALID_RANGE", "el id del ejercicio debe ser un entero mayor a cero")
		return
	}
	query.Page = c.Query("page")
	query.PageSize = c.Query("page_size")

	resp, err := dc.service.ExerciseSeries(c, callerID, exerciseID, query)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Comparisons godoc
// @Summary      Comparación entre corredores
// @Description  Una fila por corredor comparable del scope, con su tendencia y el `data_quality` común. Solo el dueño del equipo. Cada `athlete_user_id` se valida por separado: si uno está fuera de scope responde 403 nombrándolo, sin devolver los datos de los válidos.
// @Tags         dashboard
// @Accept       json
// @Produce      json
// @Param        request  body      dashboard.ComparisonsRequest  true  "Filtros de la comparación"
// @Success      200  {object}  dashboard.ComparisonsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/dashboard/comparisons [post]
func (dc *dashboardController) Comparisons(c *gin.Context) {
	callerID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondDashboardUnauthorized(c)
		return
	}
	var request dashboard.ComparisonsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondDashboardParamError(c, "INVALID_REQUEST", "body inválido: %s", err.Error())
		return
	}

	resp, err := dc.service.Comparisons(c, callerID, request)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Periods godoc
// @Summary      Comparación de períodos
// @Description  Compara el período actual contra el anterior de la misma longitud; sin `previous`, el backend lo deriva desplazando `current` hacia atrás. `compliance_pp` son puntos porcentuales y `best_time_pct` tiene el signo invertido: bajar el tiempo es mejorar. Solo el dueño del equipo.
// @Tags         dashboard
// @Accept       json
// @Produce      json
// @Param        request  body      dashboard.PeriodsRequest  true  "Períodos y filtros de la comparación"
// @Success      200  {object}  dashboard.PeriodsResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Router       /api/v1/dashboard/comparisons/periods [post]
func (dc *dashboardController) Periods(c *gin.Context) {
	callerID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondDashboardUnauthorized(c)
		return
	}
	var request dashboard.PeriodsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondDashboardParamError(c, "INVALID_REQUEST", "body inválido: %s", err.Error())
		return
	}

	resp, err := dc.service.Periods(c, callerID, request)
	if err != nil {
		respondDashboardError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ---------- helpers ----------

// dashboardRequest arma la query común de los endpoints GET: team_id
// obligatorio, filtro de ejercicio y rango. Los parámetros ausentes se dejan vacíos
// y los defaults los aplica el service, que es donde vive la validación.
func dashboardRequest(c *gin.Context) (int64, services.MetricsQuery, bool) {
	callerID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondDashboardUnauthorized(c)
		return 0, services.MetricsQuery{}, false
	}
	teamID, ok := requiredInt64Query(c, "team_id")
	if !ok {
		return 0, services.MetricsQuery{}, false
	}
	query := services.MetricsQuery{
		TeamID:      teamID,
		From:        c.Query("from"),
		To:          c.Query("to"),
		Tolerance:   c.Query("tolerance"),
		Granularity: c.Query("granularity"),
	}
	if raw := c.Query("group_id"); raw != "" {
		groupID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || groupID <= 0 {
			respondDashboardParamError(c, "INVALID_RANGE", "group_id debe ser un entero mayor a cero")
			return 0, services.MetricsQuery{}, false
		}
		query.GroupID = &groupID
	}
	if raw := c.Query("exercise_id"); raw != "" {
		exerciseID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || exerciseID <= 0 {
			respondDashboardParamError(c, "INVALID_RANGE", "exercise_id debe ser un entero mayor a cero")
			return 0, services.MetricsQuery{}, false
		}
		query.ExerciseID = &exerciseID
	}
	query.ExerciseIDs = int64Query(c, "exercise_ids")
	query.Kinds = trimmedQuery(c, "kinds")
	return callerID, query, true
}

func requiredInt64Query(c *gin.Context, name string) (int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		respondDashboardParamError(c, "MISSING_TEAM", "%s es obligatorio", name)
		return 0, false
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		respondDashboardParamError(c, "INVALID_RANGE", "%s debe ser un entero mayor a cero", name)
		return 0, false
	}
	return value, true
}

func int64Query(c *gin.Context, name string) []int64 {
	values := c.QueryArray(name)
	out := make([]int64, 0, len(values))
	for _, raw := range values {
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || parsed <= 0 {
			continue
		}
		out = append(out, parsed)
	}
	return out
}

func trimmedQuery(c *gin.Context, name string) []string {
	values := c.QueryArray(name)
	out := make([]string, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func respondDashboardUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, apierror.APIError{
		StatusCode: http.StatusUnauthorized, Code: "Unauthorized", Message: "no autenticado",
	})
}

func respondDashboardParamError(c *gin.Context, code, format string, args ...interface{}) {
	c.JSON(http.StatusBadRequest, apierror.APIError{
		StatusCode: http.StatusBadRequest,
		Code:       code,
		Message:    fmt.Sprintf(format, args...),
	})
}

// respondDashboardError mapea los errores del service a status. El 400 de
// parámetro lleva su code estable: el cliente no debe ofrecer reintentar la misma
// request.
func respondDashboardError(c *gin.Context, err error) {
	var paramErr *services.DashboardParamError
	if errors.As(err, &paramErr) {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest, Code: paramErr.Code, Message: paramErr.Message,
		})
		return
	}
	var athleteErr *services.DashboardAthleteOutOfScope
	if errors.As(err, &athleteErr) {
		c.JSON(http.StatusForbidden, apierror.APIError{
			StatusCode: http.StatusForbidden,
			Code:       "ATHLETE_OUT_OF_SCOPE",
			Message:    athleteErr.Error(),
			Details:    map[string]interface{}{"athlete_user_id": athleteErr.AthleteUserID},
		})
		return
	}
	switch {
	case errors.Is(err, services.ErrDashboardForbidden):
		c.JSON(http.StatusForbidden, apierror.APIError{
			StatusCode: http.StatusForbidden, Code: "FORBIDDEN", Message: err.Error(),
		})
	case errors.Is(err, services.ErrDashboardNotFound):
		c.JSON(http.StatusNotFound, apierror.APIError{
			StatusCode: http.StatusNotFound, Code: "NOT_FOUND", Message: err.Error(),
		})
	default:
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError, Code: "Internal Server Error", Message: err.Error(),
		})
	}
}
