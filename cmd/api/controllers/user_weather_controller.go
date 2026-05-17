package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/delegates"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type UserWeatherController interface {
	GetUserWithWeather(c *gin.Context)
}

type userWeatherController struct {
	delegate delegates.UserWeatherDelegate
}

func NewUserWeatherController(delegate delegates.UserWeatherDelegate) UserWeatherController {
	return &userWeatherController{
		delegate: delegate,
	}
}

// GetUserWithWeather godoc
// @Summary      Get user with weather data
// @Description  Get a user by ID and combine with current weather at given coordinates
// @Tags         users,weather
// @Accept       json
// @Produce      json
// @Param        user_id    path  int     true  "User ID"
// @Param        latitude   query  number  true  "Latitude (-90 to 90)"
// @Param        longitude  query  number  true  "Longitude (-180 to 180)"
// @Success      200  {object}  delegates.UserWithWeatherResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /user/{user_id}/weather [get]
func (ctrl *userWeatherController) GetUserWithWeather(c *gin.Context) {
	customlogger.Info(c, "incoming request to get user with weather", customlogger.TagMethod("GetUserWithWeather"))

	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		customlogger.Warn(c, "invalid user_id", customlogger.Tag("value", userIDStr))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "user_id must be a valid number",
		})
		return
	}

	latStr := c.Query("latitude")
	lonStr := c.Query("longitude")

	if latStr == "" || lonStr == "" {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "latitude and longitude query params are required",
		})
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "latitude must be a valid number",
		})
		return
	}

	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "longitude must be a valid number",
		})
		return
	}

	result, err := ctrl.delegate.GetUserWithWeather(c, userID, lat, lon)
	if err != nil {
		customlogger.Error(c, "delegate failed", err)
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    fmt.Sprintf("failed to get user with weather: %s", err.Error()),
		})
		return
	}

	customlogger.Info(c, "user with weather response sent successfully")
	c.JSON(http.StatusOK, result)
}
