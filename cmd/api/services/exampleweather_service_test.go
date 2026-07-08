package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"simple-arq-golang/cmd/api/domains/exampleweather"
)

type mockWeatherClient struct {
	mockGetCurrentWeather func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error)
}

func (m mockWeatherClient) GetCurrentWeather(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error) {
	return m.mockGetCurrentWeather(ctx, req)
}

func TestGetWeather_Success(t *testing.T) {
	mockClient := mockWeatherClient{
		mockGetCurrentWeather: func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error) {
			return &exampleweather.OpenMeteoResponse{
				Latitude:  -31.42,
				Longitude: -64.18,
				CurrentWeather: &exampleweather.CurrentWeather{
					Temperature: 25.5,
				},
			}, nil
		},
	}

	svc := NewExampleWeatherService(mockClient)
	resp, err := svc.GetWeather(context.Background(), exampleweather.WeatherRequest{
		Latitude:  -31.42,
		Longitude: -64.18,
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, -31.42, resp.Latitude)
	assert.Equal(t, 25.5, resp.CurrentWeather.Temperature)
}

func TestGetWeather_ClientError(t *testing.T) {
	mockClient := mockWeatherClient{
		mockGetCurrentWeather: func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error) {
			return nil, errors.New("api error")
		},
	}

	svc := NewExampleWeatherService(mockClient)
	_, err := svc.GetWeather(context.Background(), exampleweather.WeatherRequest{
		Latitude:  -31.42,
		Longitude: -64.18,
	})

	assert.Error(t, err)
}

func TestGetWeather_EmptyResponse(t *testing.T) {
	mockClient := mockWeatherClient{
		mockGetCurrentWeather: func(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error) {
			return nil, nil
		},
	}

	svc := NewExampleWeatherService(mockClient)
	_, err := svc.GetWeather(context.Background(), exampleweather.WeatherRequest{
		Latitude:  -31.42,
		Longitude: -64.18,
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}
