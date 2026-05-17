package delegates

import (
	"context"

	"github.com/gin-gonic/gin"
	"simple-arq-golang/cmd/api/domains/exampleweather"
	"simple-arq-golang/cmd/api/domains/user"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
	"simple-arq-golang/cmd/api/services"
)

type UserWeatherDelegate interface {
	GetUserWithWeather(ctx *gin.Context, userID int64, lat, lon float64) (*UserWithWeatherResponse, error)
}

type userWeatherDelegate struct {
	userSvc    services.UserServiceInterface
	weatherSvc services.ExampleWeatherServiceInterface
}

func NewUserWeatherDelegate(userSvc services.UserServiceInterface, weatherSvc services.ExampleWeatherServiceInterface) UserWeatherDelegate {
	return &userWeatherDelegate{
		userSvc:    userSvc,
		weatherSvc: weatherSvc,
	}
}

type UserWithWeatherResponse struct {
	User    user.User                     `json:"user"`
	Weather *exampleweather.WeatherResponse `json:"weather"`
}

func (d *userWeatherDelegate) GetUserWithWeather(ctx *gin.Context, userID int64, lat, lon float64) (*UserWithWeatherResponse, error) {
	customlogger.Info(ctx, "delegate: fetching user with weather", customlogger.TagMethod("GetUserWithWeather"))

	userResult, err := d.userSvc.GetUser(ctx, userID)
	if err != nil {
		customlogger.Error(ctx, "delegate: failed to get user", err)
		return nil, err
	}

	weatherReq := exampleweather.WeatherRequest{
		Latitude:       lat,
		Longitude:      lon,
		CurrentWeather: true,
	}

	weatherResult, err := d.weatherSvc.GetWeather(context.Background(), weatherReq)
	if err != nil {
		customlogger.Error(ctx, "delegate: failed to get weather", err)
		return nil, err
	}

	return &UserWithWeatherResponse{
		User:    userResult,
		Weather: weatherResult,
	}, nil
}
