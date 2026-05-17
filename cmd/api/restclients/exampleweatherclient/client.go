package exampleweatherclient

import (
	"context"
	"fmt"
	"net/url"

	"simple-arq-golang/cmd/api/domains/exampleweather"
	"simple-arq-golang/cmd/api/infrastructure/httpclient"
)

type ExampleWeatherClientInterface interface {
	GetCurrentWeather(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error)
}

type exampleWeatherClient struct {
	httpClient *httpclient.Client
}

func New(httpClient *httpclient.Client) ExampleWeatherClientInterface {
	return &exampleWeatherClient{
		httpClient: httpClient,
	}
}

func (c *exampleWeatherClient) GetCurrentWeather(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error) {
	params := url.Values{}
	params.Set("latitude", fmt.Sprintf("%f", req.Latitude))
	params.Set("longitude", fmt.Sprintf("%f", req.Longitude))
	params.Set("current_weather", fmt.Sprintf("%t", req.CurrentWeather))

	path := "/v1/forecast?" + params.Encode()

	var response exampleweather.OpenMeteoResponse
	if err := c.httpClient.Get(ctx, path, &response); err != nil {
		return nil, err
	}

	return &response, nil
}
