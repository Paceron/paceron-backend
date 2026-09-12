package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/exercise"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type ExerciseController interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Clone(c *gin.Context)
}

type exerciseController struct {
	exerciseService services.ExerciseServiceInterface
}

func NewExerciseController(exerciseService services.ExerciseServiceInterface) ExerciseController {
	return &exerciseController{exerciseService: exerciseService}
}

func mapExerciseError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrExerciseNotFound):
		return http.StatusNotFound, "ejercicio no encontrado"
	case errors.Is(err, services.ErrExerciseInvalidKind):
		return http.StatusBadRequest, "kind inválido"
	case errors.Is(err, services.ErrExerciseInvalidIntensity):
		return http.StatusBadRequest, "intensity inválido"
	case errors.Is(err, services.ErrExerciseInvalidMuscleGroup):
		return http.StatusBadRequest, "muscle_group inválido"
	case errors.Is(err, services.ErrCatalogForbidden):
		return http.StatusForbidden, "no autorizado"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondExerciseError(c *gin.Context, err error) {
	status, message := mapExerciseError(err)
	respondCatalogError(c, status, message)
}

func (ec *exerciseController) Create(c *gin.Context) {
	var req exercise.ExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := ec.exerciseService.Create(c, callerID, req)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (ec *exerciseController) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	resp, err := ec.exerciseService.Get(c, id)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (ec *exerciseController) List(c *gin.Context) {
	ownerID, err := strconv.ParseInt(c.Query("owner_id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "owner_id debe ser un número válido")
		return
	}
	resp, err := ec.exerciseService.List(c, ownerID)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (ec *exerciseController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req exercise.ExerciseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := ec.exerciseService.Update(c, id, callerID, req)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (ec *exerciseController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := ec.exerciseService.Delete(c, id, callerID); err != nil {
		respondExerciseError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (ec *exerciseController) Clone(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := ec.exerciseService.Clone(c, id, callerID)
	if err != nil {
		respondExerciseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}
