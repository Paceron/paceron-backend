package controllers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/delegates"
	"simple-arq-golang/cmd/api/domains/apierror"
	"simple-arq-golang/cmd/api/domains/exampleweather"
	"simple-arq-golang/cmd/api/domains/user"
)

type mockUserWeatherDelegate struct {
	getUserWithWeatherFn func(ctx *gin.Context, userID int64, lat, lon float64) (*delegates.UserWithWeatherResponse, error)
}

func (m *mockUserWeatherDelegate) GetUserWithWeather(ctx *gin.Context, userID int64, lat, lon float64) (*delegates.UserWithWeatherResponse, error) {
	if m.getUserWithWeatherFn != nil {
		return m.getUserWithWeatherFn(ctx, userID, lat, lon)
	}
	return nil, nil
}

func TestUserWeatherController_GetUserWithWeather_Success(t *testing.T) {
	mockDelegate := &mockUserWeatherDelegate{
		getUserWithWeatherFn: func(ctx *gin.Context, userID int64, lat, lon float64) (*delegates.UserWithWeatherResponse, error) {
			return &delegates.UserWithWeatherResponse{
				User: user.User{ID: userID, Name: "John"},
				Weather: &exampleweather.WeatherResponse{
					Latitude:  lat,
					Longitude: lon,
				},
			}, nil
		},
	}

	controller := NewUserWeatherController(mockDelegate)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/weather?latitude=40.7128&longitude=-74.0060", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusOK, response.Code)

	var result delegates.UserWithWeatherResponse
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, int64(1), result.User.ID)
}

func TestUserWeatherController_GetUserWithWeather_InvalidUserID(t *testing.T) {
	controller := NewUserWeatherController(&mockUserWeatherDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/abc/weather?latitude=40.7128&longitude=-74.0060", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "abc"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "user_id must be a valid number", result.Message)
}

func TestUserWeatherController_GetUserWithWeather_MissingLatLon(t *testing.T) {
	controller := NewUserWeatherController(&mockUserWeatherDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/weather", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "latitude and longitude query params are required", result.Message)
}

func TestUserWeatherController_GetUserWithWeather_MissingLongitude(t *testing.T) {
	controller := NewUserWeatherController(&mockUserWeatherDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/weather?latitude=40.7128", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserWeatherController_GetUserWithWeather_MissingLatitude(t *testing.T) {
	controller := NewUserWeatherController(&mockUserWeatherDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/weather?longitude=-74.0060", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserWeatherController_GetUserWithWeather_InvalidLatitude(t *testing.T) {
	controller := NewUserWeatherController(&mockUserWeatherDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/weather?latitude=abc&longitude=-74.0060", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "latitude must be a valid number", result.Message)
}

func TestUserWeatherController_GetUserWithWeather_InvalidLongitude(t *testing.T) {
	controller := NewUserWeatherController(&mockUserWeatherDelegate{})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/1/weather?latitude=40.7128&longitude=abc", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "1"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusBadRequest, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "longitude must be a valid number", result.Message)
}

func TestUserWeatherController_GetUserWithWeather_DelegateError(t *testing.T) {
	mockDelegate := &mockUserWeatherDelegate{
		getUserWithWeatherFn: func(ctx *gin.Context, userID int64, lat, lon float64) (*delegates.UserWithWeatherResponse, error) {
			return nil, errors.New("user not found")
		},
	}

	controller := NewUserWeatherController(mockDelegate)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/users/999/weather?latitude=40.7128&longitude=-74.0060", nil)
	c.Params = []gin.Param{{Key: "user_id", Value: "999"}}

	controller.GetUserWithWeather(c)

	assert.Equal(t, http.StatusInternalServerError, response.Code)

	var result apierror.APIError
	json.Unmarshal(response.Body.Bytes(), &result)
	assert.Equal(t, "Internal Server Error", result.Code)
}
