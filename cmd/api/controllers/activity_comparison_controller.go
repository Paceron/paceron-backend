package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/comparison"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

// ActivityComparisonController expone la comparación planificado vs realizado
// (change comparacion-planificado-realizado). La identidad sale del token; un
// entrenador puede consultar a un corredor suyo con athlete_user_id.
type ActivityComparisonController interface {
	GetSessionComparison(c *gin.Context)
	ListComparisons(c *gin.Context)
}

// Los tipos de respuesta se nombran acá para que swag los resuelva.
var (
	_ *comparison.SessionComparisonResponse
	_ *comparison.ComparisonListResponse
)

type activityComparisonController struct {
	service services.ActivityComparisonServiceInterface
}

func NewActivityComparisonController(service services.ActivityComparisonServiceInterface) ActivityComparisonController {
	return &activityComparisonController{service: service}
}

// GetSessionComparison godoc
// @Summary      Comparación planificado vs realizado de una sesión
// @Description  Devuelve cada ejercicio planificado de la sesión con su objetivo y cada serie con lo que registró el atleta: estado, duración, distancia, ritmo, desviación y cumplimiento (por serie, con tope de 100%). Sin athlete_user_id compara al usuario autenticado; con athlete_user_id, el usuario tiene que ser dueño del equipo de la sesión.
// @Tags         activities
// @Produce      json
// @Param        sessionInstanceId  path   int  true   "ID de la sesión (session_instance)"
// @Param        athlete_user_id    query  int  false  "ID del atleta (default: el usuario autenticado)"
// @Success      200  {object}  comparison.SessionComparisonResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      404  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/activities/comparison/{sessionInstanceId} [get]
func (ac *activityComparisonController) GetSessionComparison(c *gin.Context) {
	userID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondUnauthorized(c)
		return
	}
	sessionInstanceID, err := parsePositivePathParam(c, "sessionInstanceId")
	if err != nil {
		respondInvalidQuery(c, err.Error())
		return
	}
	athleteUserID, err := parsePositiveQueryParam(c, "athlete_user_id")
	if err != nil {
		respondInvalidQuery(c, err.Error())
		return
	}

	resp, err := ac.service.GetSessionComparison(c, userID, sessionInstanceID, athleteUserID)
	if err != nil {
		respondComparisonError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListComparisons godoc
// @Summary      Cumplimiento por rango de fechas
// @Description  Devuelve las sesiones de entrenamiento del rango de los grupos donde el atleta era miembro en cada fecha (también las que no hizo, con 0%) más las que tienen feedback del atleta sin un día así, con sus totales, su cumplimiento y un resumen del período. El rango es obligatorio, de 366 días como máximo y no puede terminar después de hoy. Con athlete_user_id de otro usuario, team_id es obligatorio y el usuario tiene que ser dueño de ese equipo.
// @Tags         activities
// @Produce      json
// @Param        date_from        query  string  true   "Desde (YYYY-MM-DD)"
// @Param        date_to          query  string  true   "Hasta (YYYY-MM-DD)"
// @Param        team_id          query  int     false  "ID del equipo"
// @Param        group_id         query  int     false  "ID del grupo (requiere team_id)"
// @Param        kind             query  string  false  "Tipo de ejercicio (liviano, elongacion, ritmo_continuo, velocidad_tiempo, velocidad_distancia, ...)"
// @Param        athlete_user_id  query  int     false  "ID del atleta (default: el usuario autenticado)"
// @Success      200  {object}  comparison.ComparisonListResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      401  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/activities/comparison [get]
func (ac *activityComparisonController) ListComparisons(c *gin.Context) {
	userID, ok := utils.GetAuthUserID(c)
	if !ok {
		respondUnauthorized(c)
		return
	}
	query := comparison.ListQuery{
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
		Kind:     c.Query("kind"),
	}
	params := []struct {
		name   string
		target **int64
	}{
		{"team_id", &query.TeamID},
		{"group_id", &query.GroupID},
		{"athlete_user_id", &query.AthleteUserID},
	}
	var err error
	for _, p := range params {
		if *p.target, err = parsePositiveQueryParam(c, p.name); err != nil {
			respondInvalidQuery(c, err.Error())
			return
		}
	}

	resp, err := ac.service.ListComparisons(c, userID, query)
	if err != nil {
		respondComparisonError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// respondComparisonError mapea los errores del service a status HTTP. Un error
// sin sentinel es crudo de la DB: se responde con un mensaje fijo para no
// filtrar texto del driver.
func respondComparisonError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrComparisonInvalidQuery):
		respondInvalidQuery(c, err.Error())
	case errors.Is(err, services.ErrComparisonForbidden):
		c.JSON(http.StatusForbidden, apierror.APIError{StatusCode: http.StatusForbidden, Code: "Forbidden", Message: err.Error()})
	case errors.Is(err, services.ErrComparisonNotFound):
		c.JSON(http.StatusNotFound, apierror.APIError{StatusCode: http.StatusNotFound, Code: "Not Found", Message: err.Error()})
	default:
		customlogger.Error(c, "error building activity comparison", err)
		c.JSON(http.StatusInternalServerError, apierror.APIError{StatusCode: http.StatusInternalServerError, Code: "Internal Server Error", Message: "error interno"})
	}
}
