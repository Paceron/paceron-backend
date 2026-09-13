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
	case errors.Is(err, services.ErrCalendarInvalidCancelTransition):
		return http.StatusUnprocessableEntity, "solo se puede cancelar un día en training"
	case errors.Is(err, services.ErrCalendarStampConflict):
		return http.StatusConflict, "hay fechas con contenido existente"
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
