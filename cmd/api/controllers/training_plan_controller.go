package controllers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/trainingplan"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type TrainingPlanController interface {
	Create(c *gin.Context)
	Get(c *gin.Context)
	List(c *gin.Context)
	Update(c *gin.Context)
	Delete(c *gin.Context)
	Clone(c *gin.Context)
}

type trainingPlanController struct {
	trainingPlanService services.TrainingPlanServiceInterface
}

func NewTrainingPlanController(trainingPlanService services.TrainingPlanServiceInterface) TrainingPlanController {
	return &trainingPlanController{trainingPlanService: trainingPlanService}
}

func mapTrainingPlanError(err error) (int, string) {
	switch {
	case errors.Is(err, services.ErrPlanNotFound):
		return http.StatusNotFound, "plan no encontrado"
	case errors.Is(err, services.ErrPlanInvalidDayCount):
		return http.StatusUnprocessableEntity, "el plan debe tener entre 2 y 31 días"
	case errors.Is(err, services.ErrPlanInvalidSequence):
		return http.StatusUnprocessableEntity, "sequence_no debe cubrir 1..N sin huecos ni repetidos"
	case errors.Is(err, services.ErrPlanInvalidDayKind):
		return http.StatusUnprocessableEntity, "kind de día inválido"
	case errors.Is(err, services.ErrPlanDayFieldMismatch):
		return http.StatusUnprocessableEntity, "combinación de campos inválida para el kind del día"
	case errors.Is(err, services.ErrPlanSessionNotFound):
		return http.StatusUnprocessableEntity, "session_id referenciado no encontrado"
	case errors.Is(err, services.ErrPlanInvalidTimeFormat):
		return http.StatusUnprocessableEntity, "default_time debe tener formato HH:MM"
	case errors.Is(err, services.ErrCatalogForbidden):
		return http.StatusForbidden, "no autorizado"
	default:
		return http.StatusInternalServerError, "error interno"
	}
}

func respondTrainingPlanError(c *gin.Context, err error) {
	status, message := mapTrainingPlanError(err)
	respondCatalogError(c, status, message)
}

// Create godoc
// @Summary      Crear plan de entrenamiento
// @Tags         training-plans
// @Accept       json
// @Produce      json
// @Param        body  body  trainingplan.TrainingPlanRequest  true  "Datos del plan"
// @Success      201  {object}  trainingplan.TrainingPlanResponse
// @Failure      400
// @Failure      403
// @Failure      422
// @Router       /api/v1/training-plans [post]
func (tc *trainingPlanController) Create(c *gin.Context) {
	var req trainingplan.TrainingPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := tc.trainingPlanService.Create(c, callerID, req)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// Get godoc
// @Summary      Obtener plan de entrenamiento por ID
// @Tags         training-plans
// @Produce      json
// @Param        id  path  int  true  "Training Plan ID"
// @Success      200  {object}  trainingplan.TrainingPlanResponse
// @Failure      400
// @Failure      403
// @Failure      404
// @Router       /api/v1/training-plans/{id} [get]
func (tc *trainingPlanController) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	resp, err := tc.trainingPlanService.Get(c, id)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// List godoc
// @Summary      Listar planes de entrenamiento por propietario
// @Tags         training-plans
// @Produce      json
// @Param        owner_id  query  int  true  "Owner ID"
// @Success      200  {array}  trainingplan.TrainingPlanResponse
// @Failure      400
// @Failure      403
// @Router       /api/v1/training-plans [get]
func (tc *trainingPlanController) List(c *gin.Context) {
	ownerID, err := strconv.ParseInt(c.Query("owner_id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "owner_id debe ser un número válido")
		return
	}
	resp, err := tc.trainingPlanService.List(c, ownerID)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Update godoc
// @Summary      Actualizar plan de entrenamiento
// @Tags         training-plans
// @Accept       json
// @Produce      json
// @Param        id    path  int  true  "Training Plan ID"
// @Param        body  body  trainingplan.TrainingPlanUpdateRequest  true  "Datos del plan"
// @Success      200  {object}  trainingplan.TrainingPlanResponse
// @Failure      400
// @Failure      403
// @Failure      404
// @Failure      422
// @Router       /api/v1/training-plans/{id} [put]
func (tc *trainingPlanController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	var req trainingplan.TrainingPlanUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondCatalogError(c, http.StatusBadRequest, "payload inválido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := tc.trainingPlanService.Update(c, id, callerID, req)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete godoc
// @Summary      Eliminar plan de entrenamiento
// @Tags         training-plans
// @Param        id  path  int  true  "Training Plan ID"
// @Success      204
// @Failure      403
// @Failure      404
// @Router       /api/v1/training-plans/{id} [delete]
func (tc *trainingPlanController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	if err := tc.trainingPlanService.Delete(c, id, callerID); err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Clone godoc
// @Summary      Clonar plan de entrenamiento
// @Tags         training-plans
// @Produce      json
// @Param        id  path  int  true  "Training Plan ID"
// @Success      201  {object}  trainingplan.TrainingPlanResponse
// @Failure      403
// @Failure      404
// @Router       /api/v1/training-plans/{id}/clone [post]
func (tc *trainingPlanController) Clone(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		respondCatalogError(c, http.StatusBadRequest, "id debe ser un número válido")
		return
	}
	callerID, _ := utils.GetAuthUserID(c)
	resp, err := tc.trainingPlanService.Clone(c, id, callerID)
	if err != nil {
		respondTrainingPlanError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}
