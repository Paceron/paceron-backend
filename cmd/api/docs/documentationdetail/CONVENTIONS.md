# Convenciones de Código

## General

- **Lenguaje**: Go 1.26
- **Framework**: Gin (github.com/gin-gonic/gin)
- **ORM**: GORM (gorm.io/gorm) con driver PostgreSQL
- **Logger**: logrus vía infrastructure/customlogger
- **DI**: Inyección de dependencias manual en app.go (sin frameworks)
- **Tests**: testify (github.com/stretchr/testify)
- **Swagger**: anotaciones swaggo/swag → gin-swagger UI

## Reglas de arquitectura

### Dirección de dependencias entre capas
```
Controllers → Delegates → Services → DAOs / RestClients → Infrastructure
Controllers → Services → DAOs / RestClients → Infrastructure
```

### NUNCA
- Un service importa otro service directamente
- Un DAO importa un RestClient ni viceversa
- Un controller contiene lógica de negocio (solo validación + delegación)
- Usar `fmt.Println` para loguear (usar customlogger siempre)
- Saltarse capas (ej. controller llamando a DAO directamente)

### Responsabilidades de cada capa

**Controller**: Parsear request, validar params, llamar al service/delegate, devolver respuesta
**Delegate**: Orquestar múltiples servicios, nunca contiene lógica de negocio
**Service**: Lógica de negocio, llama a DAOs/RestClients
**DAO**: Acceso a datos (consultas GORM)
**RestClient**: Llamadas a APIs externas vía httpclient
**Domain**: DTOs/structs puras, sin lógica

## Nombres

### Archivos y directorios
- `snake_case` para archivos: `user_controller.go`, `exampleweather_service.go`
- `camelCase` para no exportados: `userService`, `pingController`
- `PascalCase` para exportados: `UserService`, `PingController`
- Los nombres de package coinciden con el nombre del directorio

### Específicos del proyecto
- `exampleweather` prefijo para el dominio del clima
- `user` prefijo para el dominio de usuario
- Las interfaces terminan con sufijo `Interface` (ej. `UserServiceInterface`)
- Las implementaciones van en minúscula (ej. `userService`)

## Manejo de errores

- Siempre devolver `apierror.APIError` en JSON para respuestas de error
- Mapear códigos HTTP correctamente (400 validación, 404 no encontrado, 500 interno)
- Loguear errores con `customlogger.Error(ctx, msg, err)`
- Loguear fallos de validación con `customlogger.Warn(ctx, msg, tags...)`

## Logging

Usar el package customlogger en todo momento:

```go
customlogger.Info(ctx, "mensaje", customlogger.TagMethod("NombreMetodo"))
customlogger.Warn(ctx, "mensaje", customlogger.Tag("clave", "valor"))
customlogger.Error(ctx, "mensaje", err, customlogger.Tag("clave", "valor"))
customlogger.Debug(ctx, "mensaje")
```

## Configuración

- Variables de entorno cargadas en `config/config.go` vía `init()` → `LoadValues()`
- Archivos `.properties` en `config/properties/` cargados según el entorno
- Usar `config.LoadRestClientConfig()` para configuración del cliente HTTP
- El entorno se determina por la variable de entorno `environment` (local/test/stage/prod)

## Tests

- Usar `testify/assert` para aserciones
- Mockear interfaces en cada capa (mockUserDao, mockUserService, etc.)
- Archivos de test junto a la implementación: `user_service.go` + `user_service_test.go`

## Swagger

Al agregar o editar endpoints, actualizar las anotaciones Swagger:

```go
// NombreMetodo godoc
// @Summary      Descripción corta
// @Description  Descripción más larga
// @Tags         tag1,tag2
// @Accept       json
// @Produce      json
// @Param        nombre_param  {tipo}  {posición}  {requerido}  "Descripción"
// @Success      200  {object}  package.Tipo
// @Failure      400  {object}  apierror.APIError
// @Router       /ruta/{param} [method]
```

Luego regenerar:
```bash
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs
```
