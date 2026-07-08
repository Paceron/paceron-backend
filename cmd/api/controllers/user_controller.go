package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/user"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type UserController interface {
	GetUser(c *gin.Context)
	CreateUser(c *gin.Context)
	Update(c *gin.Context)
	ChangeStatus(c *gin.Context)
}

type userController struct {
	userService services.UserServiceInterface
}

func NewUserController(userService services.UserServiceInterface) UserController {
	return &userController{
		userService: userService,
	}
}

func (u *userController) GetUser(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Invalid user ID",
		})
		return
	}

	userResult, err := u.userService.GetUser(c, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, userResult)
}

func (u *userController) CreateUser(c *gin.Context) {
	var req user.CreateUserRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Invalid request body",
		})
		return
	}

	createdUser, err := u.userService.CreateUser(c, req.Name, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, createdUser)
}

// Update godoc
// @Summary      Update user attributes
// @Description  Update user attributes (all fields except id, status). Email change requires X-Current-Password header.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id                 path      int                      true  "User ID"
// @Param        X-Current-Password header    string                   false  "Current password (required for email change)"
// @Param        body               body      user.UserUpdateRequest   true  "User data to update"
// @Success      200                {object}  user.UserUpdateResponse
// @Failure      400                {object}  apierror.APIError
// @Failure      401                {object}  apierror.APIError
// @Failure      404                {object}  apierror.APIError
// @Failure      409                {object}  apierror.APIError
// @Failure      500                {object}  apierror.APIError
// @Router       /api/v1/users/{id} [put]
func (u *userController) Update(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "ID de usuario inválido",
		})
		return
	}

	currentPassword := c.GetHeader("X-Current-Password")

	var req user.UserUpdateRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid update request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if msg := services.ValidateUserUpdateRequest(&req); msg != "" {
		customlogger.Warn(c, "update validation failed",
			customlogger.Tag("field", msg))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    msg,
		})
		return
	}

	response, err := u.userService.Update(c, userID, &req, currentPassword)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		switch errMsg {
		case "usuario no encontrado":
			statusCode = http.StatusNotFound
			code = "Not Found"
		case "contraseña actual incorrecta":
			statusCode = http.StatusUnauthorized
			code = "Unauthorized"
		case "el email ya está registrado":
			statusCode = http.StatusConflict
			code = "Conflict"
		case "para cambiar el email debe proporcionar la contraseña actual (header X-Current-Password)":
			statusCode = http.StatusBadRequest
			code = "Bad request"
		case "birth_date debe tener formato dd/mm/aaaa":
			statusCode = http.StatusBadRequest
			code = "Bad request"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ChangeStatus godoc
// @Summary      Change user status
// @Description  Change the status of a user. Valid statuses: active, inactive, pause, blocked, suspended.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                         true  "User ID"
// @Param        body  body      user.StatusChangeRequest     true  "New status"
// @Success      200   {object}  user.UserUpdateResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/users/{id}/status [patch]
func (u *userController) ChangeStatus(c *gin.Context) {
	userIDStr := c.Param("id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "ID de usuario inválido",
		})
		return
	}

	var req user.StatusChangeRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid status change request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido. status es requerido",
		})
		return
	}

	if !constants.IsValidUserStatus(req.Status) {
		customlogger.Warn(c, "invalid status value",
			customlogger.Tag("field", "status"),
			customlogger.Tag("value", req.Status))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    fmt.Sprintf("Estado inválido: %s. Estados permitidos: %v", req.Status, constants.GetValidUserStatuses()),
		})
		return
	}

	response, err := u.userService.ChangeStatus(c, userID, req.Status)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "usuario no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusOK, response)
}
