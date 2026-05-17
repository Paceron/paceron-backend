package exampleweather

type WeatherRequest struct {
	Latitude       float64
	Longitude      float64
	CurrentWeather bool
}

type OpenMeteoResponse struct {
	Latitude             float64                `json:"latitude"`
	Longitude            float64                `json:"longitude"`
	GenerationTimeMs     float64                `json:"generationtime_ms"`
	UTCOffsetSeconds     int                    `json:"utc_offset_seconds"`
	Timezone             string                 `json:"timezone"`
	TimezoneAbbreviation string                 `json:"timezone_abbreviation"`
	Elevation            float64                `json:"elevation"`
	CurrentWeatherUnits  *CurrentWeatherUnits    `json:"current_weather_units"`
	CurrentWeather       *CurrentWeather         `json:"current_weather"`
}

type CurrentWeatherUnits struct {
	Time           string `json:"time"`
	Interval       string `json:"interval"`
	Temperature    string `json:"temperature"`
	WindSpeed      string `json:"windspeed"`
	WindDirection  string `json:"winddirection"`
	IsDay          string `json:"is_day"`
	WeatherCode    string `json:"weathercode"`
}

type CurrentWeather struct {
	Time           string  `json:"time"`
	Interval       int     `json:"interval"`
	Temperature    float64 `json:"temperature"`
	WindSpeed      float64 `json:"windspeed"`
	WindDirection  int     `json:"winddirection"`
	IsDay          int     `json:"is_day"`
	WeatherCode    int     `json:"weathercode"`
}

type WeatherResponse struct {
	Latitude             float64             `json:"latitude"`
	Longitude            float64             `json:"longitude"`
	Elevation            float64             `json:"elevation"`
	CurrentWeather       *CurrentWeather     `json:"current_weather,omitempty"`
}
