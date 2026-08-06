package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/auth"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type AuthController interface {
	Register(c *gin.Context)
	Login(c *gin.Context)
	GetUser(c *gin.Context)
	Refresh(c *gin.Context)
	Logout(c *gin.Context)
}

type authController struct {
	authService services.AuthServiceInterface
}

func NewAuthController(authService services.AuthServiceInterface) AuthController {
	return &authController{
		authService: authService,
	}
}

// Register godoc
// @Summary      Register a new user
// @Description  Creates a new user account. Password is sent in the request body (min 8 chars).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      auth.RegisterRequest  true  "User registration data (password required)"
// @Success      201   {object}  auth.UserResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      409   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/auth/register [post]
func (ac *authController) Register(c *gin.Context) {
	var req auth.RegisterRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if msg := services.ValidatePassword(req.Password); msg != "" {
		customlogger.Warn(c, "password validation failed",
			customlogger.Tag("field", "password"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    msg,
		})
		return
	}

	if msg := services.ValidateRegisterRequest(&req); msg != "" {
		customlogger.Warn(c, "validation failed",
			customlogger.Tag("field", msg))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    msg,
		})
		return
	}

	response, err := ac.authService.Register(c, &req, req.Password)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "el email ya está registrado" || errMsg == "el DNI ya está registrado" {
			statusCode = http.StatusConflict
			code = "Conflict"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    errMsg,
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// Login godoc
// @Summary      Authenticate user and get tokens
// @Description  Login with email and password. Returns access and refresh JWT tokens.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      auth.LoginRequest  true  "User credentials"
// @Success      200   {object}  auth.LoginResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      401   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/auth/login [post]
func (ac *authController) Login(c *gin.Context) {
	var req auth.LoginRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := ac.authService.Login(c, req.Email, req.Password)
	if err != nil {
		errMsg := err.Error()
		customlogger.Warn(c, "login failed",
			customlogger.Tag("email", req.Email),
			customlogger.Tag("reason", errMsg))

		if errMsg == "No se pudo autenticar" {
			c.JSON(http.StatusUnauthorized, apierror.APIError{
				StatusCode: http.StatusUnauthorized,
				Code:       "Unauthorized",
				Message:    errMsg,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    "Error interno del servidor",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Refresh godoc
// @Summary      Renovar sesión
// @Description  Rota un refresh token activo: lo revoca y emite un access + refresh nuevos
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      auth.RefreshRequest  true  "Refresh token vigente"
// @Success      200   {object}  auth.RefreshResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      401   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/auth/refresh [post]
func (ac *authController) Refresh(c *gin.Context) {
	var req auth.RefreshRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	response, err := ac.authService.Refresh(c, req.RefreshToken)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "refresh token inválido o expirado" {
			statusCode = http.StatusUnauthorized
			code = "Unauthorized"
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

// Logout godoc
// @Summary      Cerrar sesión
// @Description  Revoca el refresh token indicado. El access token sigue válido hasta su expiración natural
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      auth.LogoutRequest  true  "Refresh token a revocar"
// @Success      200   {object}  auth.LogoutResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/auth/logout [post]
func (ac *authController) Logout(c *gin.Context) {
	var req auth.LogoutRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if err := ac.authService.Logout(c, req.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, auth.LogoutResponse{Message: "Sesión cerrada correctamente"})
}

// GetUser godoc
// @Summary      Get user by ID or email
// @Description  Retrieve a user by providing either id or email as query parameter (not both).
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        id     query     int     false  "User ID"
// @Param        email  query     string  false  "User email"
// @Success      200    {object}  auth.UserResponse
// @Failure      400    {object}  apierror.APIError
// @Failure      404    {object}  apierror.APIError
// @Failure      500    {object}  apierror.APIError
// @Router       /api/v1/auth/user [get]
func (ac *authController) GetUser(c *gin.Context) {
	idStr := c.Query("id")
	email := c.Query("email")

	if idStr == "" && email == "" {
		customlogger.Warn(c, "missing id or email query param",
			customlogger.Tag("field", "query"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "debe proporcionar id o email como query param",
		})
		return
	}

	var id int64
	if idStr != "" {
		var err error
		id, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			customlogger.Warn(c, "invalid id format",
				customlogger.Tag("field", "id"))
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    "id debe ser un número válido",
			})
			return
		}
	}

	response, err := ac.authService.GetUser(c, id, email)
	if err != nil {
		errMsg := err.Error()
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"

		if errMsg == "usuario no encontrado" {
			statusCode = http.StatusNotFound
			code = "Not Found"
		} else if errMsg == "debe proporcionar solo id o email, no ambos" || errMsg == "debe proporcionar id o email" {
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
