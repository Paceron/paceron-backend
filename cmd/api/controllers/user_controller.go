package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/user"
	"simple-arq-golang/cmd/api/services"
)

type UserController interface {
	GetUser(c *gin.Context)
	CreateUser(c *gin.Context)
}

type userController struct {
	userService services.UserServiceInterface
}

func NewUserController(userService services.UserServiceInterface) UserController {
	return &userController{
		userService: userService,
	}
}

// GetUser godoc
// @Summary      Get user by ID
// @Description  Retrieve a user by their unique ID
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        user_id  path      int  true  "User ID"
// @Success      200  {object}  user.User
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /user/{user_id} [get]
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

// CreateUser godoc
// @Summary      Create a new user
// @Description  Create a new user with name and password
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        body  body  user.CreateUserRequest  true  "User data"
// @Success      201  {object}  user.User
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /user [post]
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
