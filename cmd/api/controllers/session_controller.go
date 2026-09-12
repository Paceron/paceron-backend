package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/session"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type SessionController interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Clone(c *gin.Context)
}

type sessionController struct {
	sessionService services.SessionServiceInterface
}

func NewSessionController(sessionService services.SessionServiceInterface) SessionController {
	return &sessionController{sessionService: sessionService}
}

func mapSessionError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrSessionNotFound):
		return http.StatusNotFound, "sesión no encontrada"
	case errors.Is(err, services.ErrSessionMissingRole):
		return http.StatusUnprocessableEntity, "la sesión debe tener al menos un ejercicio de cada rol"
	case errors.Is(err, services.ErrSessionInvalidRole):
		return http.StatusUnprocessableEntity, "role inválido"
	case errors.Is(err, services.ErrSessionExerciseNotFound):
		return http.StatusUnprocessableEntity, "ejercicio referenciado no encontrado"
	case errors.Is(err, services.ErrCatalogForbidden):
		return http.StatusForbidden, "no autorizado"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondSessionError(c *gin.Context, err error) {
	status, message := mapSessionError(err)
	respondCatalogError(c, status, message)
}

func (sc *sessionController) Create(c *gin.Context) {
	var req session.SessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := sc.sessionService.Create(c, callerID, req)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (sc *sessionController) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	resp, err := sc.sessionService.Get(c, id)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (sc *sessionController) List(c *gin.Context) {
	ownerID, err := strconv.ParseInt(c.Query("owner_id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "owner_id debe ser un número válido")
		return
	}
	resp, err := sc.sessionService.List(c, ownerID)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (sc *sessionController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req session.SessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := sc.sessionService.Update(c, id, callerID, req)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (sc *sessionController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := sc.sessionService.Delete(c, id, callerID); err != nil {
		respondSessionError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (sc *sessionController) Clone(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := sc.sessionService.Clone(c, id, callerID)
	if err != nil {
		respondSessionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}
