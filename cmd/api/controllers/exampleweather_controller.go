package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/exampleweather"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type ExampleWeatherController interface {
	GetWeather(c *gin.Context)
}

type exampleWeatherController struct {
	service services.ExampleWeatherServiceInterface
}

func NewExampleWeatherController(service services.ExampleWeatherServiceInterface) ExampleWeatherController {
	return &exampleWeatherController{
		service: service,
	}
}

// GetWeather godoc
// @Summary      Get current weather
// @Description  Get current weather data from Open-Meteo for given coordinates
// @Tags         weather
// @Accept       json
// @Produce      json
// @Param        latitude         query  number  true   "Latitude (-90 to 90)"
// @Param        longitude        query  number  true   "Longitude (-180 to 180)"
// @Param        current_weather  query  boolean false  "Include current weather (default: true)"
// @Success      200  {object}  exampleweather.WeatherResponse
// @Failure      400  {object}  apierror.APIError
// @Failure      500  {object}  apierror.APIError
// @Router       /example/weather [get]
func (ctrl *exampleWeatherController) GetWeather(c *gin.Context) {
	customlogger.Info(c, "incoming request to get weather", customlogger.TagMethod("GetWeather"))

	latStr := c.Query("latitude")
	lonStr := c.Query("longitude")
	cwStr := c.Query("current_weather")

	if latStr == "" {
		customlogger.Warn(c, "missing latitude parameter")
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "latitude query parameter is required",
		})
		return
	}

	if lonStr == "" {
		customlogger.Warn(c, "missing longitude parameter")
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "longitude query parameter is required",
		})
		return
	}

	latitude, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		customlogger.Warn(c, "invalid latitude", customlogger.Tag("value", latStr))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "latitude must be a valid number",
		})
		return
	}

	longitude, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		customlogger.Warn(c, "invalid longitude", customlogger.Tag("value", lonStr))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "longitude must be a valid number",
		})
		return
	}

	if latitude < -90 || latitude > 90 {
		customlogger.Warn(c, "latitude out of range", customlogger.Tag("latitude", latStr))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "latitude must be between -90 and 90",
		})
		return
	}

	if longitude < -180 || longitude > 180 {
		customlogger.Warn(c, "longitude out of range", customlogger.Tag("longitude", lonStr))
		c.JSON(http.StatusBadRequest, apierror.APIError{
			StatusCode: http.StatusBadRequest,
			Code:       "Bad Request",
			Message:    "longitude must be between -180 and 180",
		})
		return
	}

	currentWeather := true
	if cwStr != "" {
		currentWeather, err = strconv.ParseBool(cwStr)
		if err != nil {
			customlogger.Warn(c, "invalid current_weather parameter", customlogger.Tag("value", cwStr))
			c.JSON(http.StatusBadRequest, apierror.APIError{
				StatusCode: http.StatusBadRequest,
				Code:       "Bad Request",
				Message:    "current_weather must be a boolean (true/false)",
			})
			return
		}
	}

	req := exampleweather.WeatherRequest{
		Latitude:       latitude,
		Longitude:      longitude,
		CurrentWeather: currentWeather,
	}

	weatherResp, err := ctrl.service.GetWeather(c.Request.Context(), req)
	if err != nil {
		customlogger.Error(c, "error getting weather", err)
		c.JSON(http.StatusInternalServerError, apierror.APIError{
			StatusCode: http.StatusInternalServerError,
			Code:       "Internal Server Error",
			Message:    fmt.Sprintf("failed to retrieve weather data: %s", err.Error()),
		})
		return
	}

	customlogger.Info(c, "weather response sent successfully")
	c.JSON(http.StatusOK, weatherResp)
}
