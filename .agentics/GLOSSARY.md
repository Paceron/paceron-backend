# Glosario del Scaffolding

## 📁 Estructura de directorios

### `cmd/api/`
Raíz del ejecutable. Todo lo que está adentro compila para formar un solo binario.

### `cmd/api/main.go`
Punto de entrada de la aplicación. Solo importa el package `app` y ejecuta `app.StartApp()`. No hace nada más. Es intencionalmente mínimo.

### `cmd/api/docs.go`
Contiene los metadatos generales de la API para Swagger (título, versión, contacto, licencia). Swaggo lee esto para generar la documentación.

---

## 📁 `app/` — Bootstrap de la aplicación

### `app.go`
**Corazón de la inyección de dependencias.** Aquí se construyen todas las piezas del proyecto:

1. Se crean los DAOs
2. Se crean los Services (inyectándoles los DAOs)
3. Se crean los Delegates (inyectándoles Services)
4. Se crean los Controllers (inyectándoles Services o Delegates)
5. Se devuelve un `Application` struct con todos los controllers listos

```go
type Application struct {
    pingController          controllers.PingController
    userController          controllers.UserController
    exampleWeatherController controllers.ExampleWeatherController
    userWeatherController    controllers.UserWeatherController
}
```

**Regla de oro:** ningún package sabe cómo construir sus dependencias. Todo se construye acá y se inyecta. Esto se llama **Inversión de Control**.

### `router.go`
Crea el server Gin, configura el logger, llama a `NewApplication()`, registra las rutas con `mapUrls()` y arranca el servidor en `:8080`.

### `middleware.go`
Funciones que se ejecutan en **cada request** antes de llegar al controller:
- **SetRequestID**: Si el request no trae un `X-Request-Id`, genera uno con UUID. Lo agrega al context y al response header.

### `url_mappings.go`
El **mapa de rutas** del proyecto. Cada línea conecta un método HTTP + path con un método de un controller:
```go
r.GET("/ping", app.pingController.Ping)
r.GET("/user/:user_id", app.userController.GetUser)
```

---

## 📁 `config/` — Configuración

### `config.go`
Carga configuración desde **variables de entorno**. Se ejecuta automáticamente al iniciar la app gracias al `init()`.

Define:
- `DB` struct: datos de conexión a Postgres
- `MyDB` variable global con la config de DB
- Funciones para detectar el environment (production, test, development)

**Cómo fluye:** `init()` → `LoadValues()` → según el environment llama a `initLocal()`, `initProd()` o `initTest()` → cada una lee las env vars y carga `MyDB`.

### `properties.go`
Carga configuración desde **archivos .properties** para el RestClient.

Funciones clave:
- `GetScope()`: devuelve "local", "test", "stage" o "prod" según la env var `environment`
- `loadPropertiesFile(scope)`: lee el archivo `application-{scope}.properties` y lo parsea como `map[string]string`
- `LoadRestClientConfig()`: devuelve un `RestClientConfig` struct con baseURL, timeout, reintentos

### `properties/`
Archivos de configuración por environment:
```
application-local.properties   → desarrollo local
application-test.properties    → testing
application-stage.properties   → staging
application-prod.properties    → producción
```

Cada archivo define:
```properties
restclient.base.url=https://api.open-meteo.com
restclient.timeout.seconds=30
restclient.max.retries=3
restclient.retry.delay.ms=1000
```

---

## 📁 `controllers/` — Handlers HTTP

**Responsabilidad única:** recibir el request, validar los parámetros, llamar al service/delegate, devolver la respuesta.

Un controller NUNCA debe:
- Tener lógica de negocio
- Llamar a DAOs o RestClients directamente
- Usar `fmt.Println`

Ejemplo de flujo:
```
Request → Controller valida → llama a Service → Service devuelve datos
                                                      ↓
Controller arma la response JSON ← ← ← ← ← ← ← ← ← 
```

Controladores actuales:
| Controller | Endpoint | Qué hace |
|---|---|---|
| `pingController` | `GET /ping` | Devuelve "pong" (health check) |
| `userController` | `GET /user/:id` | Busca usuario por ID |
| `userController` | `POST /user` | Crea un usuario nuevo |
| `exampleWeatherController` | `GET /example/weather` | Obtiene clima de Open-Meteo |
| `userWeatherController` | `GET /user/:id/weather` | Usuario + clima combinados |

---

## 📁 `delegates/` — Orquestación entre servicios

**Problema que resuelve:** Si el `UserService` necesita datos del `WeatherService`, la tentación es importar un service dentro del otro. Eso está mal porque crea **acoplamiento horizontal**.

**Solución:** el `Delegate` conoce ambos servicios y los orquesta:

```go
type userWeatherDelegate struct {
    userSvc    services.UserServiceInterface      ← inyectado
    weatherSvc services.ExampleWeatherServiceInterface  ← inyectado
}
```

El controller inyecta el Delegate, no los servicios por separado. Así los servicios siguen sin conocerse.

**Cuándo crear un Delegate:** cuando un endpoint necesita combinar datos de 2 o más servicios.

---

## 📁 `services/` — Lógica de negocio

Capa más importante del proyecto. Acá vive la lógica de negocio.

Cada service:
1. Tiene una **interfaz** (ej: `UserServiceInterface`) que define sus métodos
2. Tiene una **implementación** privada (ej: `userService`) que la implementa
3. Recibe sus dependencias por constructor (DAO o RestClient)
4. No conoce nada de HTTP (no importa gin, no recibe *gin.Context a menos que sea necesario para logging)

Ejemplo:
```go
type userService struct {
    userDao daos.UserDaoInterface   ← única dependencia
}

func (s *userService) GetUser(ctx context.Context, userID int64) (user.User, error) {
    userDB, err := s.userDao.GetByID(ctx, userID)
    // lógica de negocio acá
    return user.User{...}, nil
}
```

---

## 📁 `daos/` — Data Access Objects

Capa de acceso a base de datos usando GORM.

Un DAO:
- Tiene una interfaz (ej: `UserDaoInterface`)
- Tiene una implementación concreta que usa `*gorm.DB`
- Solo se inyecta en Services
- Nunca es llamado por Controllers

```go
type userDao struct {
    DB *gorm.DB
}

func (ud *userDao) GetByID(ctx context.Context, userID int64) (*dbs.User, error) {
    var user dbs.User
    err := ud.DB.First(&user, userID).Error
    return &user, err
}
```

---

## 📁 `restclients/` — Clientes de APIs externas

Contiene clientes específicos para cada API externa que consumimos.

Cada RestClient:
- Usa el `httpclient.Client` genérico de infraestructura
- Sabe cómo armar la request específica (path, headers, query params)
- Sabe cómo parsear la response del proveedor
- Expone una interfaz para que los Services la consuman

```go
type ExampleWeatherClientInterface interface {
    GetCurrentWeather(ctx context.Context, req exampleweather.WeatherRequest) (*exampleweather.OpenMeteoResponse, error)
}
```

**Diferencia con DAOs:** DAOs hablan con DB, RestClients hablan con APIs HTTP externas.

---

## 📁 `domains/` — Modelos de dominio (DTOs)

**Data Transfer Objects.** Structs que representan datos, sin lógica.

| Package | Contiene |
|---|---|
| `apierror/` | `APIError` — estructura uniforme para errores HTTP |
| `dbs/` | `User` — modelo de GORM que mapea a la tabla en DB |
| `user/` | `User`, `CreateUserRequest` — DTOs de la API de usuarios |
| `exampleweather/` | `WeatherRequest`, `WeatherResponse`, `CurrentWeather` — DTOs del clima |

---

## 📁 `infrastructure/` — Capa transversal reutilizable

Cosas que cualquier capa podría necesitar y que no pertenecen a un dominio específico.

### `customlogger/`
Logger estructurado basado en logrus.

**Por qué existe:** Para tener logs consistentes con formato de tags, sin depender de un singleton global. Reemplaza a `fmt.Println`.

Modos de uso:
```go
customlogger.Info(ctx, "mensaje", customlogger.Tag("key", "value"))
customlogger.Error(ctx, "mensaje", err, customlogger.TagMethod("Metodo"))
customlogger.Warn(ctx, "mensaje")
customlogger.Debug(ctx, "mensaje")
```

**Adapter:** `customlogger/adapter.go` implementa la interfaz `httpclient.Logger` para que el httpclient pueda loguear usando el mismo logger.

### `httpclient/`
Cliente HTTP genérico con:
- Timeout configurable
- Retry automático con backoff
- Circuit breaker
- Telemetry
- Logger inyectable

**No se usa directamente en controllers/services.** Se inyecta en RestClients que lo envuelven con la lógica específica.

Opciones disponibles:
```go
httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithTimeout(30 * time.Second),
    httpclient.WithRetry(3, 1 * time.Second),
    httpclient.WithLogger(myLogger),
    httpclient.WithCircuitBreaker(cb),
)
```

### `postgresdb/`
Conexión a PostgreSQL usando GORM. Toma la config de `config.MyDB` y devuelve `*gorm.DB`.

### `httputils/`
Utilidades HTTP genéricas (ej: convertir string status a código HTTP).

---

## 📁 `docs/` — Documentación Swagger generada

**NO EDITAR MANUALMENTE.** Se genera automáticamente con:
```bash
swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs
```

Contiene:
- `docs.go`: define la variable `SwaggerInfo` con todos los endpoints
- `swagger.json`: especificación OpenAPI en JSON
- `swagger.yaml`: especificación OpenAPI en YAML

Se sirve en `GET /swagger/index.html`.

---

## 📁 `constants/` — Constantes compartidas

Valores que se usan en múltiples lugares del proyecto (nombres de métricas, steps, status codes).

---

## 📁 `metrics/` — Sistema de métricas

Helper para enviar métricas. Actualmente tiene una implementación simple que imprime por consola, pero está diseñado para ser reemplazado por un sistema real (Datadog, NewRelic, Prometheus).

---

## 📁 `testutils/` — Utilidades para tests

Helpers reutilizables para escribir tests:
- Crear contextos Gin falsos
- Mockear requests HTTP
- Parsear JSON de archivos mock

---

## 📁 `utils/` — Utilidades generales

Funciones genéricas que no pertenecen a un dominio específico:
- `Contains()` — verificar si un elemento está en un slice
- `StringToInt64()` / `Int64ToString()` — conversiones
- `IsPositiveInteger()` / `ParseInt64()` — validaciones

---

## 🧠 Conceptos clave

### Inversión de Dependencias (Dependency Injection)
Las dependencias no se crean adentro de cada struct, se reciben por constructor desde afuera (`app.go`). Esto permite:
- Cambiar implementaciones sin modificar el código que las usa
- Hacer tests con mocks fácilmente
- Tener un lugar único para ver cómo se conecta todo

### Programación por Interfaces
Cada capa expone una interfaz, no una implementación concreta:
```go
type UserServiceInterface interface {
    GetUser(ctx *gin.Context, userID int64) (user.User, error)
    CreateUser(ctx *gin.Context, name, password string) (user.User, error)
}
```

La implementación concreta (`userService`) es privada (minúscula). Nadie afuera del package sabe que existe.

### Separación por Capas
```
Controllers → reciben requests HTTP
Delegates → orquestan varios servicios
Services → lógica de negocio
DAOs → base de datos
RestClients → APIs externas
Infrastructure → herramientas transversales
```

Cada capa solo puede hablar con la capa de abajo, nunca con la de arriba ni con otra del mismo nivel.

### Inmutabilidad de packages
Un package NO debe conocer la existencia de otros packages del mismo nivel. Ejemplo: `services/user` no debe importar `services/weather`. Si necesita hacerlo, se crea un `delegate`.

### Propósito de los properties
Los archivos `.properties` separan la configuración del código. Permiten tener distintos valores para local, test, stage y prod sin modificar el código fuente.
