# paceron-backend

API backend con arquitectura en capas (Controllers → Services → DAOs/RestClients → Infrastructure) usando Go + Gin.

## Documentación

| Archivo | Descripción |
|---------|-------------|
| [`SETUP.md`](SETUP.md) | Setup local para nuevos desarrolladores |
| [`STRUCTURE_PACKAGE.md`](STRUCTURE_PACKAGE.md) | Estructura de carpetas con descripciones |
| [`STRUCTURE_FOLDERS.md`](STRUCTURE_FOLDERS.md) | Mapa arquitectónico solo con carpetas |
| [`cmd/api/docs/documentationdetail/STRUCTURE.md`](cmd/api/docs/documentationdetail/STRUCTURE.md) | Árbol del proyecto en español |
| [`cmd/api/docs/documentationdetail/CONVENTIONS.md`](cmd/api/docs/documentationdetail/CONVENTIONS.md) | Convenciones de código en español |
| [`cmd/api/docs/documentationdetail/WORKFLOW.md`](cmd/api/docs/documentationdetail/WORKFLOW.md) | Flujo de trabajo en español |
| [`cmd/api/docs/documentationdetail/GLOSSARY.md`](cmd/api/docs/documentationdetail/GLOSSARY.md) | Glosario detallado en español |
| [`.agentics/`](.agentics/) | Documentación técnica para asistentes de IA (inglés) |
| [`openspec/`](openspec/) | Configuración y cambios de OpenSpec (spec-driven development) |
| [`.opencode/`](.opencode/) | Skills y comandos para OpenCode |
| [`.qwen/`](.qwen/) | Skills y comandos para Qwen Code |

## Project root

```
ci/               Artefactos de CI/CD (cobertura, scripts)
.agentics/        Documentación para asistentes de IA
cmd/api/          Código fuente de la API
```

## Architecture

```mermaid
graph TB
    Client[Client] --> |HTTP| Router[Gin Router]

    subgraph Controllers
        PingController[pingController]
        AuthController[authController]
        UserController[userController]
        WeatherController[exampleWeatherController]
        UserWeatherController[userWeatherController]
    end

    subgraph Delegates
        UserWeatherDelegate[userWeatherDelegate]
    end

    subgraph Services
        AuthService[authService]
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

    Router --> |/ping| PingController
    Router --> |/api/v1/auth/*| AuthController
    Router --> |/user/*| UserController
    Router --> |/api/v1/users/*| UserController
    Router --> |/example/weather| WeatherController
    Router --> |/user/*/weather| UserWeatherController
    Router --> |/swagger/*| SwaggerUI

    UserWeatherController --> UserWeatherDelegate
    UserWeatherDelegate --> UserService
    UserWeatherDelegate --> WeatherService
    AuthController --> AuthService
    UserController --> UserService
    WeatherController --> WeatherService
    AuthService --> UserDAO
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
| POST | `/api/v1/auth/register` | Register new user (name, surname, email, dni, birth_date, password + optional fields) |
| POST | `/api/v1/auth/login` | Login with email/password, returns JWT access + refresh tokens |
| GET | `/api/v1/auth/user?id=&email=` | Get user by ID or email |
| GET | `/user/:user_id` | Get user by ID (legacy) |
| POST | `/user` | Create user (legacy) |
| PUT | `/api/v1/users/:id` | Update user attributes (email change requires X-Current-Password header) |
| PATCH | `/api/v1/users/:id/status` | Change user status (active/inactive/pause/blocked/suspended) |
| GET | `/example/weather` | Get weather from Open-Meteo |
| GET | `/user/:user_id/weather` | Get user with weather data |
| GET | `/swagger` | Swagger UI |

## Run

```bash
go run cmd/api/main.go
```

> Para setup completo ver [`SETUP.md`](SETUP.md).

## Swagger

After running, open http://localhost:8080/swagger/index.html

## Test

```bash
go test ./...
```

---

*Arquitectura basada en **simple-arq-golang** diseñada por [sintex-dev](https://github.com/sintex-dev/simple-arq-golang) © 2026 — [sintex.dev@gmail.com](mailto:sintex.dev@gmail.com)*

**© 2026 Paceron. Todos los derechos reservados.**  
Todo el contenido, código y documentación de este repositorio son propiedad exclusiva de **Paceron** y sus miembros. Queda prohibida su reproducción, distribución o uso sin autorización expresa.
