package services

import (
	"context"
	"fmt"

	"simple-arq-golang/cmd/api/domains/exampleweather"
	"simple-arq-golang/cmd/api/restclients/exampleweatherclient"
	"simple-arq-golang/cmd/api/infrastructure/customlogger"
)

type ExampleWeatherServiceInterface interface {
	GetWeather(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.WeatherResponse, error)
}

type exampleWeatherService struct {
	client exampleweatherclient.ExampleWeatherClientInterface
}

func NewExampleWeatherService(client exampleweatherclient.ExampleWeatherClientInterface) ExampleWeatherServiceInterface {
	return &exampleWeatherService{
		client: client,
	}
}

func (s *exampleWeatherService) GetWeather(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.WeatherResponse, error) {
	customlogger.Info(nil, "calling open-meteo API", customlogger.Tag("latitude", fmt.Sprintf("%f", req.Latitude)), customlogger.Tag("longitude", fmt.Sprintf("%f", req.Longitude)))

	openMeteoResp, err := s.client.GetCurrentWeather(ctx, req)
	if err != nil {
		customlogger.Error(nil, "open-meteo API call failed", err)
		return nil, err
	}

	if openMeteoResp == nil {
		customlogger.Error(nil, "open-meteo returned empty response", nil)
		return nil, fmt.Errorf("empty response from weather provider")
	}

	weatherResp := &exampleweather.WeatherResponse{
		Latitude:       openMeteoResp.Latitude,
		Longitude:      openMeteoResp.Longitude,
		Elevation:      openMeteoResp.Elevation,
		CurrentWeather: openMeteoResp.CurrentWeather,
	}

	customlogger.Info(nil, "weather data retrieved successfully", customlogger.Tag("latitude", fmt.Sprintf("%f", weatherResp.Latitude)), customlogger.Tag("longitude", fmt.Sprintf("%f", weatherResp.Longitude)))

	return weatherResp, nil
}
