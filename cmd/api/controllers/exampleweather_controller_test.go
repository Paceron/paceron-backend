package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/exampleweather"
)

type mockExampleWeatherService struct {
	getWeatherFn func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.WeatherResponse, error)
}

func (m *mockExampleWeatherService) GetWeather(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.WeatherResponse, error) {
	if m.getWeatherFn != nil {
		return m.getWeatherFn(ctx, req)
	}
	return nil, nil
}

func TestExampleWeatherController_GetWeather_Success(t *testing.T) {
	mockSvc := &mockExampleWeatherService{
		getWeatherFn: func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.WeatherResponse, error) {
			return &exampleweather.WeatherResponse{
				Latitude:  req.Latitude,
				Longitude: req.Longitude,
			}, nil
		},
	}

	controller := NewExampleWeatherController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=40.7128&longitude=-74.0060", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result exampleweather.WeatherResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, 40.7128, result.Latitude)
}

func TestExampleWeatherController_GetWeather_MissingLatitude(t *testing.T) {
	controller := NewExampleWeatherController(&mockExampleWeatherService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?longitude=-74.0060", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "latitude query parameter is required", result.Message)
}

func TestExampleWeatherController_GetWeather_MissingLongitude(t *testing.T) {
	controller := NewExampleWeatherController(&mockExampleWeatherService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=40.7128", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "longitude query parameter is required", result.Message)
}

func TestExampleWeatherController_GetWeather_InvalidLatitude(t *testing.T) {
	controller := NewExampleWeatherController(&mockExampleWeatherService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=abc&longitude=-74.0060", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "latitude must be a valid number", result.Message)
}

func TestExampleWeatherController_GetWeather_InvalidLongitude(t *testing.T) {
	controller := NewExampleWeatherController(&mockExampleWeatherService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=40.7128&longitude=abc", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "longitude must be a valid number", result.Message)
}

func TestExampleWeatherController_GetWeather_LatitudeOutOfRange(t *testing.T) {
	controller := NewExampleWeatherController(&mockExampleWeatherService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=91&longitude=-74.0060", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "latitude must be between -90 and 90", result.Message)
}

func TestExampleWeatherController_GetWeather_LongitudeOutOfRange(t *testing.T) {
	controller := NewExampleWeatherController(&mockExampleWeatherService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=40.7128&longitude=181", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "longitude must be between -180 and 180", result.Message)
}

func TestExampleWeatherController_GetWeather_InvalidCurrentWeather(t *testing.T) {
	controller := NewExampleWeatherController(&mockExampleWeatherService{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=40.7128&longitude=-74.0060&current_weather=invalid", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "current_weather must be a boolean (true/false)", result.Message)
}

func TestExampleWeatherController_GetWeather_ServiceError(t *testing.T) {
	mockSvc := &mockExampleWeatherService{
		getWeatherFn: func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.WeatherResponse, error) {
			return nil, errors.New("service unavailable")
		},
	}

	controller := NewExampleWeatherController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=40.7128&longitude=-74.0060", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestExampleWeatherController_GetWeather_CurrentWeatherFalse(t *testing.T) {
	mockSvc := &mockExampleWeatherService{
		getWeatherFn: func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.WeatherResponse, error) {
			return &exampleweather.WeatherResponse{
				Latitude:  req.Latitude,
				Longitude: req.Longitude,
			}, nil
		},
	}

	controller := NewExampleWeatherController(mockSvc)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/weather?latitude=40.7128&longitude=-74.0060&current_weather=false", nil)

	controller.GetWeather(c)

	assert.Equal(t, http.StatusOK, response.Code)
}
