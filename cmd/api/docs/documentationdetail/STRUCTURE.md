# Estructura del Proyecto

```
cmd/api/
├── main.go                          # Punto de entrada
├── docs.go                          # Metadatos de Swagger
├── docs/                            # Documentación Swagger generada (no editar)
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
│
├── app/                             # Inicialización de la aplicación
│   ├── app.go                       # Inyección de dependencias, struct Application
│   ├── router.go                    # Arranque del servidor Gin
│   ├── url_mappings.go              # Definición de rutas
│   ├── url_mappings_test.go
│   ├── middleware.go                # Middleware de Request ID
│   └── middleware_test.go
│
├── config/                          # Configuración por entorno
│   ├── config.go                    # Variables de entorno, config DB, init()
│   ├── properties.go                # Cargador de .properties + RestClientConfig
│   └── properties/                  # Archivos .properties por entorno
│       ├── application-local.properties
│       ├── application-test.properties
│       ├── application-stage.properties
│       └── application-prod.properties
│
├── constants/                       # Constantes compartidas
│   ├── constants.go
│   └── metrics_constants.go
│
├── controllers/                     # Handlers HTTP (delgados, solo validación + delegación)
│   ├── ping_controller.go           # GET /ping
│   ├── ping_controller_test.go
│   ├── user_controller.go           # GET /user/:id, POST /user
│   ├── exampleweather_controller.go # GET /example/weather
│   └── user_weather_controller.go   # GET /user/:id/weather
│
├── delegates/                       # Orquestación entre servicios
│   └── user_weather_delegate.go     # Puente entre UserService + WeatherService
│
├── services/                        # Capa de lógica de negocio (interfaz + impl)
│   ├── user_service.go
│   ├── exampleweather_service.go
│   └── *_test.go
│
├── daos/                            # Capa de acceso a datos (GORM)
│   └── user_dao.go
│
├── restclients/                     # Clientes de APIs externas (específicos por dominio)
│   └── exampleweatherclient/
│       └── client.go
│
├── domains/                         # Modelos de dominio / DTOs
│   ├── apierror/                    # Respuesta de error API
│   ├── dbs/                         # Modelos GORM
│   ├── exampleweather/              # DTOs del clima (request/response)
│   └── user/                        # DTOs de usuario
│
├── infrastructure/                  # Componentes reutilizables transversales
│   ├── customlogger/                # Logger estructurado basado en Logrus
│   │   ├── customLogger.go          # Logger principal
│   │   └── adapter.go              # LoggerAdapter para httpclient
│   ├── httpclient/                  # Cliente HTTP genérico con reintento + circuit breaker
│   │   ├── client.go
│   │   ├── options.go
│   │   ├── errors.go
│   │   ├── logger.go
│   │   ├── telemetry.go
│   │   └── circuitbreaker.go
│   ├── httputils/                   # Utilidades HTTP
│   └── postgresdb/                  # Conexión a Postgres (GORM)
│
├── metrics/                         # Helpers de métricas
├── testutils/                       # Utilidades para pruebas
├── transformations/                 # (legado, migrar a utils/)
└── utils/                           # Utilidades generales
    ├── utils.go
    ├── transform_utils.go
    └── validation_utils.go
```
