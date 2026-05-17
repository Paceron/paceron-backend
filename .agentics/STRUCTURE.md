# Project Structure

```
cmd/api/
├── main.go                          # Entry point
├── docs.go                          # Swagger API metadata
├── docs/                            # Generated swagger docs (do not edit)
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
│
├── app/                             # Application bootstrap
│   ├── app.go                       # Dependency injection, Application struct
│   ├── router.go                    # Gin server startup
│   ├── url_mappings.go              # Route definitions
│   ├── url_mappings_test.go
│   ├── middleware.go                # Request ID middleware
│   └── middleware_test.go
│
├── config/                          # Environment configuration
│   ├── config.go                    # Env vars, DB config, init()
│   ├── properties.go                # .properties loader + RestClientConfig
│   └── properties/                  # Per-environment .properties files
│       ├── application-local.properties
│       ├── application-test.properties
│       ├── application-stage.properties
│       └── application-prod.properties
│
├── constants/                       # Shared constants
│   ├── constants.go
│   └── metrics_constants.go
│
├── controllers/                     # HTTP handlers (thin, only validation + delegation)
│   ├── ping_controller.go           # GET /ping
│   ├── ping_controller_test.go
│   ├── user_controller.go           # GET /user/:id, POST /user
│   ├── exampleweather_controller.go # GET /example/weather
│   └── user_weather_controller.go   # GET /user/:id/weather
│
├── delegates/                       # Cross-service orchestration
│   └── user_weather_delegate.go     # Bridges UserService + WeatherService
│
├── services/                        # Business logic layer (interfaces + impl)
│   ├── user_service.go
│   ├── exampleweather_service.go
│   └── *_test.go
│
├── daos/                            # Data access layer (GORM)
│   └── user_dao.go
│
├── restclients/                     # External API clients (specific per domain)
│   └── exampleweatherclient/
│       └── client.go
│
├── domains/                         # Domain models / DTOs
│   ├── apierror/                    # API error response
│   ├── dbs/                         # GORM models
│   ├── exampleweather/              # Weather DTOs (request/response)
│   └── user/                        # User DTOs
│
├── infrastructure/                  # Reusable cross-cutting concerns
│   ├── customlogger/                # Logrus-based structured logger
│   │   ├── customLogger.go          # Core logger
│   │   └── adapter.go              # LoggerAdapter for httpclient
│   ├── httpclient/                  # Generic HTTP client with retry + circuit breaker
│   │   ├── client.go
│   │   ├── options.go
│   │   ├── errors.go
│   │   ├── logger.go
│   │   ├── telemetry.go
│   │   └── circuitbreaker.go
│   ├── httputils/                   # HTTP utilities
│   └── postgresdb/                  # Postgres connection (GORM)
│
├── metrics/                         # Metrics helpers
├── testutils/                       # Test utilities
├── transformations/                 # (legacy, migrate to utils/)
└── utils/                           # General utilities
    ├── utils.go
    ├── transform_utils.go
    └── validation_utils.go
```
