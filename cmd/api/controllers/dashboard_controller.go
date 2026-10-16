package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

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
// @Description  KPIs del período: totales, cumplimiento (porcentaje 0-100), mejor tiempo, consistencia, tendencia y estado de los datos. `compliance_rate` es porcentaje y `delta_pct` va con signo positivo hacia la mejora, incluso en el eje temporal. Los cuatro filtros de rango acotan qué series entran al cálculo y se aplican sobre el tiempo logrado y la distancia lograda; una serie que queda afuera no cuenta como inválida.
// @Tags         dashboard
// @Produce      json
// @Param        team_id          query     int64    true   "Equipo del que se está mirando el dashboard"
// @Param        group_id         query     int64    false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        exercise_id      query     int64    false  "Un ejercicio de catálogo (excluyente con exercise_ids y kind)"
// @Param        exercise_ids     query     []int64  false  "Varios ejercicios de catálogo (excluyente con exercise_id y kind)"
// @Param        kind             query     string   false  "Un tipo con métrica (excluyente con exercise_id y exercise_ids)"  Enums(ritmo_continuo, velocidad_tiempo, velocidad_distancia)
// @Param        from             query     string   true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to               query     string   false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance        query     number   false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Param        distance_from_m  query     int64    false  "Distancia mínima de la serie, en metros enteros"
// @Param        distance_to_m    query     int64    false  "Distancia máxima de la serie, en metros enteros"
// @Param        time_from_min    query     int64    false  "Tiempo logrado mínimo, en minutos enteros"
// @Param        time_to_min      query     int64    false  "Tiempo logrado máximo, en minutos enteros"
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
// @Param        team_id          query     int64    true   "Equipo del que se está mirando el dashboard"
// @Param        group_id         query     int64    false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        exercise_id      query     int64    false  "Un ejercicio de catálogo (excluyente con exercise_ids y kind)"
// @Param        exercise_ids     query     []int64  false  "Varios ejercicios de catálogo (excluyente con exercise_id y kind)"
// @Param        kind             query     string   false  "Un tipo con métrica (excluyente con exercise_id y exercise_ids)"  Enums(ritmo_continuo, velocidad_tiempo, velocidad_distancia)
// @Param        from             query     string   true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to               query     string   false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance        query     number   false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Param        distance_from_m  query     int64    false  "Distancia mínima de la serie, en metros enteros"
// @Param        distance_to_m    query     int64    false  "Distancia máxima de la serie, en metros enteros"
// @Param        time_from_min    query     int64    false  "Tiempo logrado mínimo, en minutos enteros"
// @Param        time_to_min      query     int64    false  "Tiempo logrado máximo, en minutos enteros"
// @Param        granularity      query     string   false  "Anclaje del bucket (default session)"  Enums(session, week, month, quarter)
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
// @Param        team_id          query     int64    true   "Equipo del que se está mirando el dashboard"
// @Param        group_id         query     int64    false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        exercise_id      query     int64    false  "Un ejercicio de catálogo (excluyente con exercise_ids y kind)"
// @Param        exercise_ids     query     []int64  false  "Varios ejercicios de catálogo (excluyente con exercise_id y kind)"
// @Param        kind             query     string   false  "Un tipo con métrica (excluyente con exercise_id y exercise_ids)"  Enums(ritmo_continuo, velocidad_tiempo, velocidad_distancia)
// @Param        from             query     string   true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to               query     string   false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance        query     number   false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Param        distance_from_m  query     int64    false  "Distancia mínima de la serie, en metros enteros"
// @Param        distance_to_m    query     int64    false  "Distancia máxima de la serie, en metros enteros"
// @Param        time_from_min    query     int64    false  "Tiempo logrado mínimo, en minutos enteros"
// @Param        time_to_min      query     int64    false  "Tiempo logrado máximo, en minutos enteros"
// @Param        page             query     int      false  "Página (default 1)"
// @Param        page_size        query     int      false  "Tamaño de página, máximo 100 (default 20)"
// @Param        sort             query     string   false  "Columna de orden (default best_time_ms)"  Enums(best_time_ms, mean_speed_kph, total_distance_m, compliance_rate)
// @Param        order            query     string   false  "Sentido del orden (default asc)"  Enums(asc, desc)
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
// @Param        id              path      int64   true   "Ejercicio de catálogo (exercises.id)"
// @Param        team_id         query     int64   true   "Equipo del que se está mirando el dashboard"
// @Param        group_id        query     int64   false  "Acotar a un grupo del scope (no lo amplía)"
// @Param        from            query     string  true   "Desde (YYYY-MM-DD, obligatorio)"
// @Param        to              query     string  false  "Hasta (YYYY-MM-DD; sin él, cierra en from)"
// @Param        tolerance       query     number  false  "Tolerancia de cumplimiento, entre 0.01 y 0.20 (default 0.05)"
// @Param        distance_from_m query     int64   false  "Distancia mínima de la serie, en metros enteros"
// @Param        distance_to_m   query     int64   false  "Distancia máxima de la serie, en metros enteros"
// @Param        time_from_min   query     int64   false  "Tiempo logrado mínimo, en minutos enteros"
// @Param        time_to_min     query     int64   false  "Tiempo logrado máximo, en minutos enteros"
// @Param        page            query     int     false  "Página (default 1)"
// @Param        page_size       query     int     false  "Tamaño de página, máximo 100 (default 20)"
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
// @Description  Una fila por corredor comparable del scope, con su tendencia y el `data_quality` común. Solo el dueño del equipo. Cada `athlete_user_id` se valida por separado: si uno está fuera de scope responde 403 nombrándolo, sin devolver los datos de los válidos. `kind` es un valor único entre los tres tipos con métrica, y los cuatro filtros de rango acotan las series igual que en los GET.
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
	if err := bindDashboardBody(c, &request); err != nil {
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
// @Description  Compara el período actual contra el anterior de la misma longitud; sin `previous`, el backend lo deriva desplazando `current` hacia atrás. `compliance_pp` son puntos porcentuales y `best_time_pct` tiene el signo invertido: bajar el tiempo es mejorar. Solo el dueño del equipo. Los cuatro filtros de rango acotan las series de los dos períodos por igual.
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
	if err := bindDashboardBody(c, &request); err != nil {
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

// bindDashboardBody decodifica el body de los dos POST rechazando los campos que no
// existen en el DTO. ShouldBindJSON los ignoraba en silencio, y para un filtro eso es
// lo peor que puede pasar: un `kinds` viejo —o cualquier otro nombre mal escrito—
// devolvía el scope entero y el cliente creía que había filtrado.
//
// La validación de `binding:"required"` la hace binding.Validator a mano porque
// ShouldBindJSON la corría en el mismo paso: sin ella, un team_id ausente pasaría de
// 400 a 403.
func bindDashboardBody(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return binding.Validator.ValidateStruct(target)
}

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
	kind, ok := singleKindQuery(c)
	if !ok {
		return 0, services.MetricsQuery{}, false
	}
	query.Kind = kind
	if !seriesRangeQuery(c, &query) {
		return 0, services.MetricsQuery{}, false
	}
	return callerID, query, true
}

// seriesRangeQuery lee los cuatro filtros de rango de datos. Comparten la misma regla
// —entero opcional, y el 0 es un valor legítimo—, así que van en una sola pasada y
// en orden fijo para que un request con dos parámetros malos siempre reporte el
// mismo.
func seriesRangeQuery(c *gin.Context, query *services.MetricsQuery) bool {
	for _, bound := range []struct {
		name   string
		target **int64
	}{
		{"distance_from_m", &query.DistanceFromM},
		{"distance_to_m", &query.DistanceToM},
		{"time_from_min", &query.TimeFromMin},
		{"time_to_min", &query.TimeToMin},
	} {
		value, ok := optionalIntQuery(c, bound.name)
		if !ok {
			return false
		}
		*bound.target = value
	}
	return true
}

// singleKindQuery lee el filtro de tipo, que es un valor único. Repetido es el array
// que el contrato no tiene, y el `kinds` viejo —con o sin corchetes— tampoco se
// ignora en silencio: ignorarlo devolvería el scope entero y el cliente creería que
// filtró por tipo.
func singleKindQuery(c *gin.Context) (string, bool) {
	for name := range c.Request.URL.Query() {
		if strings.TrimSuffix(name, "[]") == "kinds" {
			respondDashboardParamError(c, "INVALID_REQUEST",
				"kinds ya no es un parámetro: el filtro de tipo es `kind`, y es un valor único")
			return "", false
		}
	}
	values := c.QueryArray("kind")
	switch len(values) {
	case 0:
		return "", true
	case 1:
		return strings.TrimSpace(values[0]), true
	default:
		respondDashboardParamError(c, "INVALID_REQUEST",
			"kind es un valor único: mandá un solo tipo")
		return "", false
	}
}

// optionalIntQuery lee un entero opcional. El 0 es un valor válido —"cualquier
// distancia" se puede pedir explícitamente—, así que lo que marca la ausencia es el
// parámetro vacío, no el valor.
func optionalIntQuery(c *gin.Context, name string) (*int64, bool) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		respondDashboardParamError(c, "INVALID_RANGE_FILTER", "%s debe ser un entero", name)
		return nil, false
	}
	return &value, true
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
