package controllers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/calendar"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type CalendarController interface {
	GetRange(c *gin.Context)
	PutDay(c *gin.Context)
	DeleteDay(c *gin.Context)
	Stamp(c *gin.Context)
	Bulk(c *gin.Context)
	BulkClear(c *gin.Context)
	Shift(c *gin.Context)
	NextSession(c *gin.Context)
	CalendarSummary(c *gin.Context)
}

type calendarController struct {
	calendarService services.CalendarServiceInterface
}

func NewCalendarController(calendarService services.CalendarServiceInterface) CalendarController {
	return &calendarController{calendarService: calendarService}
}

func mapCalendarError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrCalendarGroupNotFound):
		return http.StatusNotFound, "grupo no encontrado"
	case errors.Is(err, services.ErrCalendarPlanNotFound):
		return http.StatusNotFound, "plan no encontrado"
	case errors.Is(err, services.ErrCalendarForbidden):
		return http.StatusForbidden, "no autorizado"
	case errors.Is(err, services.ErrCalendarPlanForbidden):
		return http.StatusForbidden, "el plan no pertenece al entrenador dueño del grupo"
	case errors.Is(err, services.ErrCalendarInvalidKind):
		return http.StatusUnprocessableEntity, "kind inválido"
	case errors.Is(err, services.ErrCalendarFieldMismatch):
		return http.StatusUnprocessableEntity, "combinación de campos inválida"
	case errors.Is(err, services.ErrCalendarSessionNotFound):
		return http.StatusUnprocessableEntity, "sesión referenciada no encontrada"
	case errors.Is(err, services.ErrSessionExerciseNotFound):
		return http.StatusUnprocessableEntity, "ejercicio referenciado no encontrado"
	case errors.Is(err, services.ErrCalendarInvalidCancelTransition):
		return http.StatusUnprocessableEntity, "solo se puede cancelar un día en training"
	case errors.Is(err, services.ErrCalendarInvalidTimeFormat):
		return http.StatusUnprocessableEntity, "presencial_time_from/presencial_time_to deben tener formato HH:MM"
	case errors.Is(err, services.ErrCalendarInvalidTimeRange):
		return http.StatusUnprocessableEntity, "presencial_time_to debe ser posterior a presencial_time_from"
	case errors.Is(err, services.ErrCalendarDayClosed):
		return http.StatusUnprocessableEntity, err.Error()
	case errors.Is(err, services.ErrCalendarStampConflict):
		return http.StatusConflict, err.Error()
	case errors.Is(err, services.ErrCalendarShiftCollision):
		return http.StatusConflict, "el corrimiento haría chocar dos fechas"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondCalendarError(c *gin.Context, err error) {
	status, message := mapCalendarError(err)
	respondCatalogError(c, status, message)
}

// GetRange godoc
// @Summary      Calendario de un grupo en un rango de fechas
// @Tags         calendar
// @Produce      json
// @Param        id    path   int     true   "Group ID"
// @Param        from  query  string  true   "Fecha desde (YYYY-MM-DD)"
// @Param        to    query  string  true   "Fecha hasta (YYYY-MM-DD)"
// @Success      200  {array}  calendar.CalendarDayResponse
// @Failure      400
// @Failure      403
// @Router       /api/v1/groups/{id}/calendar [get]
func (cc *calendarController) GetRange(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	fromStr, toStr := c.Query("from"), c.Query("to")
	if fromStr == "" || toStr == "" {
		respondCatalogError(c, http.StatusBadRequest, "from y to son obligatorios")
		return
	}
	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "from debe tener formato YYYY-MM-DD")
		return
	}
	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "to debe tener formato YYYY-MM-DD")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.GetRange(c, groupID, callerID, from, to)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PutDay godoc
// @Summary      Crear o actualizar un día en el calendario
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Param        id    path   int                       true   "Group ID"
// @Param        date  path   string                    true   "Fecha (YYYY-MM-DD)"
// @Param        body  body   calendar.CalendarDayRequest  true   "Datos del día"
// @Success      200  {object}  calendar.CalendarDayResponse
// @Failure      400
// @Failure      403
// @Failure      422
// @Router       /api/v1/groups/{id}/calendar/{date} [put]
func (cc *calendarController) PutDay(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	date, err := time.Parse("2006-01-02", c.Param("date"))
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "date debe tener formato YYYY-MM-DD")
		return
	}
	var req calendar.CalendarDayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.UpsertDay(c, groupID, callerID, date, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteDay godoc
// @Summary      Eliminar un día del calendario
// @Tags         calendar
// @Param        id    path  int     true  "Group ID"
// @Param        date  path  string  true  "Fecha (YYYY-MM-DD)"
// @Success      204
// @Failure      400
// @Failure      403
// @Router       /api/v1/groups/{id}/calendar/{date} [delete]
func (cc *calendarController) DeleteDay(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	date, err := time.Parse("2006-01-02", c.Param("date"))
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "date debe tener formato YYYY-MM-DD")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := cc.calendarService.DeleteDay(c, groupID, callerID, date); err != nil {
		respondCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Stamp godoc
// @Summary      Copiar plan al calendario del grupo
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Param        id    path  int                    true  "Group ID"
// @Param        body  body  calendar.StampRequest  true  "Datos del stamp"
// @Success      201  {array}  calendar.CalendarDayResponse
// @Failure      400
// @Failure      403
// @Failure      409
// @Router       /api/v1/groups/{id}/calendar/stamp [post]
func (cc *calendarController) Stamp(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.StampRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.Stamp(c, groupID, callerID, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// Bulk godoc
// @Summary      Operación masiva de días en el calendario
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Param        id    path  int                   true  "Group ID"
// @Param        body  body  calendar.BulkRequest  true  "Datos de la operación"
// @Success      200  {array}  calendar.CalendarDayResponse
// @Failure      400
// @Failure      403
// @Failure      422
// @Router       /api/v1/groups/{id}/calendar/bulk [post]
func (cc *calendarController) Bulk(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.BulkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.Bulk(c, groupID, callerID, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// BulkClear godoc
// @Summary      Limpiar múltiples días del calendario
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Param        id    path  int                       true  "Group ID"
// @Param        body  body  calendar.BulkClearRequest  true  "Datos de limpiar"
// @Success      204
// @Failure      400
// @Failure      403
// @Router       /api/v1/groups/{id}/calendar/bulk-clear [post]
func (cc *calendarController) BulkClear(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.BulkClearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := cc.calendarService.BulkClear(c, groupID, callerID, req); err != nil {
		respondCalendarError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Shift godoc
// @Summary      Desplazar días del calendario
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Param        id    path  int                   true  "Group ID"
// @Param        body  body  calendar.ShiftRequest  true  "Datos del desplazamiento"
// @Success      200  {array}  calendar.CalendarDayResponse
// @Failure      400
// @Failure      403
// @Failure      409
// @Router       /api/v1/groups/{id}/calendar/shift [post]
func (cc *calendarController) Shift(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req calendar.ShiftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := cc.calendarService.Shift(c, groupID, callerID, req)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// NextSession godoc
// @Summary      Próxima sesión del usuario
// @Tags         calendar
// @Produce      json
// @Param        id  path  int  true  "User ID"
// @Success      200  {object}  calendar.NextSessionResponse
// @Success      204
// @Failure      400
// @Failure      403
// @Router       /api/v1/users/{id}/next-session [get]
func (cc *calendarController) NextSession(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if userID != callerID {
		respondCatalogError(c, http.StatusForbidden, "no podés consultar los datos de otro usuario")
		return
	}
	resp, err := cc.calendarService.NextSession(c, userID)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	if resp == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CalendarSummary godoc
// @Summary      Resumen de calendario del usuario
// @Tags         calendar
// @Produce      json
// @Param        id  path  int  true  "User ID"
// @Success      200  {array}  calendar.CalendarSummaryItem
// @Failure      400
// @Failure      403
// @Router       /api/v1/users/{id}/calendar-summary [get]
func (cc *calendarController) CalendarSummary(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if userID != callerID {
		respondCatalogError(c, http.StatusForbidden, "no podés consultar los datos de otro usuario")
		return
	}
	resp, err := cc.calendarService.CalendarSummary(c, userID)
	if err != nil {
		respondCalendarError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
