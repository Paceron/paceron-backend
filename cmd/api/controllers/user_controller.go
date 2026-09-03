package controllers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/constants"
	"simple-arq-golang/cmd/api/domains/user"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
	"simple-arq-golang/cmd/api/utils"
)

type UserController interface {
	Update(c *gin.Context)
	ChangeStatus(c *gin.Context)
	ChangePassword(c *gin.Context)
	Search(c *gin.Context)
	BatchLookup(c *gin.Context)
	UploadPhoto(c *gin.Context)
	DeletePhoto(c *gin.Context)
}

type userController struct {
	userService services.UserServiceInterface
}

func NewUserController(userService services.UserServiceInterface) UserController {
	return &userController{
		userService: userService,
	}
}

// Update godoc
// @Summary      Update user attributes
// @Description  Update user attributes (all fields except id, status). Email change requires X-Current-Password header. Only the user itself can update its own data.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id                 path      int                      true  "User ID"
// @Param        X-Current-Password header    string                   false  "Current password (required for email change)"
// @Param        body               body      user.UserUpdateRequest   true  "User data to update"
// @Success      200                {object}  user.UserUpdateResponse
// @Failure      400                {object}  apierror.APIError
// @Failure      401                {object}  apierror.APIError
// @Failure      403                {object}  apierror.APIError
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

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		c.JSON(http.StatusForbidden, apierror.APIError{
			StatusCode: http.StatusForbidden,
			Code:       "Forbidden",
			Message:    "solo podés modificar tu propio usuario",
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
// @Description  Change the status of a user. Valid statuses: active, inactive, pause, blocked, suspended. Only the user itself can change its own status.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                         true  "User ID"
// @Param        body  body      user.StatusChangeRequest     true  "New status"
// @Success      200   {object}  user.UserUpdateResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
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

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		c.JSON(http.StatusForbidden, apierror.APIError{
			StatusCode: http.StatusForbidden,
			Code:       "Forbidden",
			Message:    "solo podés modificar tu propio usuario",
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

// ChangePassword godoc
// @Summary      Change password while authenticated
// @Description  Changes the user's password, verifying the current one. Distinct from the forgot/reset-password OTP flow. Only the user itself can change its own password.
// @Tags         users
// @Accept       json
// @Produce      json
// @Param        id    path      int                          true  "User ID"
// @Param        body  body      user.ChangePasswordRequest    true  "Current and new password"
// @Success      200   {object}  user.ChangePasswordResponse
// @Failure      400   {object}  apierror.APIError
// @Failure      401   {object}  apierror.APIError
// @Failure      403   {object}  apierror.APIError
// @Failure      404   {object}  apierror.APIError
// @Failure      500   {object}  apierror.APIError
// @Router       /api/v1/users/{id}/password [patch]
func (u *userController) ChangePassword(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "ID de usuario inválido",
		})
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		c.JSON(http.StatusForbidden, apierror.APIError{
			StatusCode: http.StatusForbidden,
			Code:       "Forbidden",
			Message:    "solo podés modificar tu propio usuario",
		})
		return
	}

	var req user.ChangePasswordRequest
	if err := c.BindJSON(&req); err != nil {
		customlogger.Warn(c, "invalid change password request body",
			customlogger.Tag("field", "body"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "Cuerpo de solicitud inválido",
		})
		return
	}

	if req.NewPassword != req.ConfirmPassword {
		customlogger.Warn(c, "password confirmation mismatch",
			customlogger.Tag("field", "confirm_password"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "las contraseñas no coinciden",
		})
		return
	}

	if msg := services.ValidatePassword(req.NewPassword); msg != "" {
		customlogger.Warn(c, "password validation failed",
			customlogger.Tag("field", "new_password"))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    msg,
		})
		return
	}

	err = u.userService.ChangePassword(c, userID, req.CurrentPassword, req.NewPassword)
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
		case "la nueva contraseña debe ser distinta a la actual":
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

	c.JSON(http.StatusOK, user.ChangePasswordResponse{Message: "Contraseña actualizada correctamente"})
}

// Search godoc
// @Summary      Search users
// @Description  Busca usuarios activos por coincidencia parcial de nombre, apellido o email (autocompletar al invitar). Requiere login, sin restricción adicional de rol. Mínimo 3 caracteres, hasta 5 resultados.
// @Tags         users
// @Produce      json
// @Param        q  query     string  true  "Texto de búsqueda (mínimo 3 caracteres)"
// @Success      200  {object}  user.SearchResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/users/search [get]
func (u *userController) Search(c *gin.Context) {
	query := c.Query("q")

	result, err := u.userService.Search(c, query)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		if strings.Contains(err.Error(), "la búsqueda requiere al menos") {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// BatchLookup godoc
// @Summary      Batch user lookup
// @Description  Resuelve nombre/apellido/email para varios user_id de una sola consulta (evita el fan-out N+1 al mostrar un roster de equipo/grupo). Requiere login, sin restricción adicional de rol. Hasta 50 ids.
// @Tags         users
// @Produce      json
// @Param        ids  query     string  true  "Ids de usuario separados por coma, ej. 1,2,3"
// @Success      200  {object}  user.BatchLookupResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/users [get]
// UploadPhoto godoc
// @Summary      Upload profile photo
// @Description  Uploads or replaces the authenticated user's own profile photo (self only). Max 5MB, JPEG/PNG/WEBP only (validated by content, not filename)
// @Tags         users
// @Accept       multipart/form-data
// @Produce      json
// @Param        id     path      int   true  "User ID"
// @Param        photo  formData  file  true  "Photo file"
// @Success      200    {object}  map[string]string
// @Failure      400    {object}  apierror.APIError
// @Failure      403    {object}  apierror.APIError
// @Failure      500    {object}  apierror.APIError
// @Router       /api/v1/users/{id}/photo [put]
func (u *userController) UploadPhoto(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "ID de usuario inválido",
		})
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		c.JSON(http.StatusForbidden, apierror.APIError{
			StatusCode: http.StatusForbidden,
			Code:       "Forbidden",
			Message:    "solo podés modificar tu propio usuario",
		})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, services.MaxPhotoSizeBytes+1024)
	fileHeader, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "PHOTO_TOO_LARGE",
			Message:    "Archivo inválido o demasiado grande (máximo 5MB)",
		})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "No se pudo leer el archivo",
		})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "No se pudo leer el archivo",
		})
		return
	}

	photoURL, err := u.userService.UploadPhoto(c, userID, content)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		if errors.Is(err, services.ErrPhotoTooLarge) {
			statusCode = http.StatusBadRequest
			code = "PHOTO_TOO_LARGE"
		} else if errors.Is(err, services.ErrPhotoInvalidType) {
			statusCode = http.StatusBadRequest
			code = "PHOTO_INVALID_TYPE"
		} else if errors.Is(err, services.ErrStorageUnavailable) {
			code = "STORAGE_UNAVAILABLE"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"photo_url": photoURL})
}

// DeletePhoto godoc
// @Summary      Delete profile photo
// @Description  Deletes the authenticated user's own profile photo (self only). Idempotent
// @Tags         users
// @Param        id  path  int  true  "User ID"
// @Success      204
// @Failure      400  {object}  apierror.APIError
// @Failure      403  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /api/v1/users/{id}/photo [delete]
func (u *userController) DeletePhoto(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "ID de usuario inválido",
		})
		return
	}

	if authUserID, _ := utils.GetAuthUserID(c); authUserID != userID {
		c.JSON(http.StatusForbidden, apierror.APIError{
			StatusCode: http.StatusForbidden,
			Code:       "Forbidden",
			Message:    "solo podés modificar tu propio usuario",
		})
		return
	}

	if err := u.userService.DeletePhoto(c, userID); err != nil {
		code := "Internal Server Error"
		if errors.Is(err, services.ErrStorageUnavailable) {
			code = "STORAGE_UNAVAILABLE"
		}
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func (u *userController) BatchLookup(c *gin.Context) {
	idsParam := c.Query("ids")
	if strings.TrimSpace(idsParam) == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad request",
			Message:    "se requiere el parámetro ids",
		})
		return
	}

	rawIDs := strings.Split(idsParam, ",")
	userIDs := make([]int64, 0, len(rawIDs))
	for _, raw := range rawIDs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		id, err := strconv.ParseInt(trimmed, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad request",
				Message:    fmt.Sprintf("id inválido: %s", trimmed),
			})
			return
		}
		userIDs = append(userIDs, id)
	}

	result, err := u.userService.BatchLookup(c, userIDs)
	if err != nil {
		statusCode := http.StatusInternalServerError
		code := "Internal Server Error"
		if strings.Contains(err.Error(), "se requiere al menos") || strings.Contains(err.Error(), "no se pueden consultar más de") {
			statusCode = http.StatusBadRequest
			code = "Bad request"
		}

		c.JSON(statusCode, apierror.APIError{
			StatusCode: statusCode,
			Code:       code,
			Message:    err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}
