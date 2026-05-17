# Code Conventions

## General

- **Language**: Go 1.26
- **Framework**: Gin (github.com/gin-gonic/gin)
- **ORM**: GORM (gorm.io/gorm) with PostgreSQL driver
- **Logger**: logrus via infrastructure/customlogger
- **DI**: Manual dependency injection in app.go (no frameworks)
- **Tests**: testify (github.com/stretchr/testify)
- **Swagger**: swaggo/swag annotations → gin-swagger UI

## Architecture rules

### Layer dependency direction
```
Controllers → Delegates → Services → DAOs / RestClients → Infrastructure
Controllers → Services → DAOs / RestClients → Infrastructure
```

### NEVER
- A service imports another service directly
- A DAO imports a RestClient or vice versa
- A controller contains business logic (only validation + delegation)
- `fmt.Println` for logging (use customlogger always)
- Bypass layers (e.g. controller calling DAO directly)

### Layer responsibilities

**Controller**: Parse request, validate params, call service/delegate, return response
**Delegate**: Orchestrate multiple services, never contains business logic
**Service**: Business logic, calls DAOs/RestClients
**DAO**: Data access (GORM queries)
**RestClient**: External API calls via httpclient
**Domain**: Pure DTOs/structs, no logic

## Naming

### Files and directories
- `snake_case` for files: `user_controller.go`, `exampleweather_service.go`
- `camelCase` for unexported: `userService`, `pingController`
- `PascalCase` for exported: `UserService`, `PingController`
- Package names match directory name

### Project-specific
- `exampleweather` prefix for weather domain
- `user` prefix for user domain
- Interfaces end with `Interface` suffix (e.g. `UserServiceInterface`)
- Implementations are lowercased (e.g. `userService`)

## Error handling

- Always return `apierror.APIError` JSON for error responses
- Map HTTP status codes correctly (400 validation, 404 not found, 500 internal)
- Log errors with `customlogger.Error(ctx, msg, err)`
- Log validation failures with `customlogger.Warn(ctx, msg, tags...)`

## Logging

Use customlogger package at all times:

```go
customlogger.Info(ctx, "message", customlogger.TagMethod("MethodName"))
customlogger.Warn(ctx, "message", customlogger.Tag("key", "value"))
customlogger.Error(ctx, "message", err, customlogger.Tag("key", "value"))
customlogger.Debug(ctx, "message")
```

## Configuration

- Env vars loaded in `config/config.go` via `init()` → `LoadValues()`
- `.properties` files in `config/properties/` loaded by environment scope
- Use `config.LoadRestClientConfig()` for HTTP client configuration
- Scope determined by env var `environment` (local/test/stage/prod)

## Tests

- Use `testify/assert` for assertions
- Mock interfaces at each layer (mockUserDao, mockUserService, etc.)
- Test files alongside implementation: `user_service.go` + `opt_service_test.go`

## Swagger

When adding/editing endpoints, update swagger annotations:

```go
// MethodName godoc
// @Summary      Short description
// @Description  Longer description
// @Tags         tag1,tag2
// @Accept       json
// @Produce      json
// @Param        param_name  {type}  {position}  {required}  "Description"
// @Success      200  {object}  package.Type
// @Failure      400  {object}  apierror.APIError
// @Router       /path/{param} [method]
```

Then regenerate:
```bash
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs
```
