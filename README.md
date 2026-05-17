# simple-arq-golang

> Desarrollado por **sintex-dev** © 2026
> Contacto: [sintex.dev@gmail.com](mailto:sintex.dev@gmail.com)

Base scaffolding for Go APIs with Gin framework.

## Architecture

```mermaid
graph TB
    Client[Client] --> |HTTP| Router[Gin Router]
    Router --> |/ping| PingController
    Router --> |/user/*| UserController
    Router --> |/example/weather| WeatherController
    Router --> |/user/*/weather| UserWeatherController
    Router --> |/swagger/*| SwaggerUI

    subgraph Controllers
        PingController[pingController]
        UserController[userController]
        WeatherController[exampleWeatherController]
        UserWeatherController[userWeatherController]
    end

    subgraph Delegates
        UserWeatherDelegate[userWeatherDelegate]
    end

    subgraph Services
        UserService[userService]
        WeatherService[exampleWeatherService]
    end

    subgraph DAOs
        UserDAO[userDao]
    end

    subgraph RestClients
        WeatherClient[exampleWeatherClient]
    end

    subgraph Infrastructure
        HTTPClient[httpclient.Client]
        Logger[customlogger]
        DB[postgresdb]
    end

    UserWeatherController --> UserWeatherDelegate
    UserWeatherDelegate --> UserService
    UserWeatherDelegate --> WeatherService
    UserController --> UserService
    WeatherController --> WeatherService
    UserService --> UserDAO
    UserDAO --> DB
    WeatherService --> WeatherClient
    WeatherClient --> HTTPClient
    HTTPClient --> |Open-Meteo API| ExternalAPI[api.open-meteo.com]
```

## Layer structure

```mermaid
graph LR
    subgraph cmd/api
        direction TB
        Controllers --> Delegates
        Controllers --> Services
        Delegates --> Services
        Services --> DAOs
        Services --> RestClients
        DAOs --> Infrastructure
        RestClients --> Infrastructure
    end
```

## Request flow

### Example: `GET /example/weather?latitude=-31.42&longitude=-64.18`

```mermaid
sequenceDiagram
    participant C as Client
    participant CTRL as exampleWeatherController
    participant SVC as exampleWeatherService
    participant CLI as exampleWeatherClient
    participant HTTP as httpclient.Client
    participant EXT as Open-Meteo API

    C->>CTRL: GET /example/weather
    CTRL->>CTRL: validate params
    CTRL->>SVC: GetWeather(lat, lon)
    SVC->>CLI: GetCurrentWeather(lat, lon)
    CLI->>HTTP: GET /v1/forecast
    HTTP->>EXT: HTTP Request
    EXT-->>HTTP: JSON Response
    HTTP-->>CLI: parsed response
    CLI-->>SVC: OpenMeteoResponse
    SVC-->>CTRL: WeatherResponse
    CTRL-->>C: 200 JSON
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/ping` | Health check |
| GET | `/user/:user_id` | Get user by ID |
| POST | `/user` | Create user |
| GET | `/example/weather` | Get weather from Open-Meteo |
| GET | `/user/:user_id/weather` | Get user with weather data |
| GET | `/swagger/*any` | Swagger UI |

## Run

```bash
go run cmd/api/main.go
```

## Swagger

After running, open http://localhost:8080/swagger/index.html

## Test

```bash
go test ./...
```
