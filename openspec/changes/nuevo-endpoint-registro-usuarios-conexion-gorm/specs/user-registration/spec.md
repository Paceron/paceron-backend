## ADDED Requirements

### Requirement: DB Client inicialización en infrastructure/postgresdb
El sistema SHALL mejorar la inicialización de PostgreSQL en `cmd/api/infrastructure/postgresdb/postgres.go`, mejorando la función `ConfigDB(cfg config.DB) (*gorm.DB, error)` con configuración explícita de pool de conexiones, health check y auto-migrate de modelos.

#### Scenario: Conexión exitosa a PostgreSQL
- **WHEN** se invoca `ConfigDB` con una configuración DB válida
- **THEN** retorna un `*gorm.DB` conectado y funcional
- **AND** el pool de conexiones está configurado con `MaxIdleConns`, `MaxOpenConns` y `ConnMaxLifetime`

#### Scenario: Error de conexión
- **WHEN** se invoca `ConfigDB` con credenciales u host inválidos
- **THEN** retorna un error describiendo la causa de la falla de conexión

#### Scenario: Health check de conexión
- **WHEN** se completa la conexión exitosamente
- **THEN** se ejecuta `sqlDB.Ping()` para verificar conectividad activa

### Requirement: Auto-migrate de modelos GORM
El sistema SHALL ejecutar auto-migrate de los modelos GORM (`dbs.User`) al inicializar la DB, garantizando que las tablas existan con el esquema correcto.

#### Scenario: Auto-migrate en inicio
- **WHEN** `ConfigDB` establece la conexión exitosamente y se completa el ping
- **THEN** se ejecuta `db.AutoMigrate(&dbs.User{})` sin errores
- **AND** la tabla `users` existe con todas las columnas definidas en el modelo

### Requirement: Modelo User GORM completo
El modelo GORM `dbs.User` SHALL contener los siguientes campos con sus constraints:

- `id` — int64, primaryKey, autoincrement
- `name` — string, not null
- `surname` — string, not null
- `email` — string, unique, not null
- `phone` — string, nullable
- `phone_contact` — string, nullable
- `country` — string, nullable
- `city` — string, nullable
- `street` — string, nullable
- `number` — string, nullable
- `dni` — string, unique, not null
- `birth_date` — time.Time, not null
- `password` — string, not null (hash bcrypt)
- `created_at` — autoCreateTime
- `updated_at` — autoUpdateTime

#### Scenario: Columnas correctas en tabla
- **WHEN** se ejecuta auto-migrate
- **THEN** la tabla `users` contiene las columnas `id`, `name`, `surname`, `email`, `phone`, `phone_contact`, `country`, `city`, `street`, `number`, `dni`, `birth_date`, `password`, `created_at`, `updated_at`
- **AND** `email` y `dni` tienen constraint UNIQUE
- **AND** los campos not null están marcados como NOT NULL

### Requirement: Request DTO para registro
El sistema SHALL definir un struct `RegisterRequest` en `domains/auth/register_request.go` con los campos en camelCase para Go y tags JSON en under_score. La contraseña SHALL enviarse por header `X-Password` (no en el body) por seguridad, evitando que se loguee en bodies de request.

```go
type RegisterRequest struct {
    Name         string `json:"name" binding:"required"`
    Surname      string `json:"surname" binding:"required"`
    Email        string `json:"email" binding:"required"`
    Phone        string `json:"phone,omitempty"`
    PhoneContact string `json:"phone_contact,omitempty"`
    Country      string `json:"country,omitempty"`
    City         string `json:"city,omitempty"`
    Street       string `json:"street,omitempty"`
    Number       string `json:"number,omitempty"`
    Dni          string `json:"dni" binding:"required"`
    BirthDate    string `json:"birth_date" binding:"required"`
    // Password se obtiene del header X-Password, no del body
}
```

#### Scenario: Request sin password en body
- **WHEN** un cliente envía `POST /api/v1/auth/register`
- **THEN** la contraseña NO se recibe en el body JSON
- **AND** la contraseña se obtiene del header `X-Password`

### Requirement: Response DTO para registro
El sistema SHALL definir un struct `RegisterResponse` en `domains/auth/register_response.go` separado del request, usando under_score en JSON y excluyendo la contraseña.

```go
type RegisterResponse struct {
    UserID       int64  `json:"user_id"`
    Name         string `json:"name"`
    Surname      string `json:"surname"`
    Email        string `json:"email"`
    Phone        string `json:"phone,omitempty"`
    PhoneContact string `json:"phone_contact,omitempty"`
    Country      string `json:"country,omitempty"`
    City         string `json:"city,omitempty"`
    Street       string `json:"street,omitempty"`
    Number       string `json:"number,omitempty"`
    Dni          string `json:"dni"`
    BirthDate    string `json:"birth_date"`
}
```

#### Scenario: Response sin contraseña
- **WHEN** el registro es exitoso
- **THEN** el response `RegisterResponse` NO contiene el campo password bajo ninguna circunstancia

### Requirement: Parseo y transformación de datos
El sistema SHALL implementar transformers/parsers en `services/auth_service.go` o `utils/` para convertir entre capas:
- `ParseRegisterRequest` — convierte el request DTO a modelo DB (parsea `birth_date` de string `dd/mm/aaaa` a `time.Time`, limpia espacios en blanco)
- `ToRegisterResponse` — convierte modelo DB a response DTO (formatea `birth_date` a string `dd/mm/aaaa`)

#### Scenario: Parseo de birth_date
- **WHEN** se recibe un `birth_date` en formato `dd/mm/aaaa`
- **THEN** se parsea correctamente a `time.Time`
- **AND** si el formato es inválido, retorna error 400

#### Scenario: Limpieza de espacios
- **WHEN** los campos opcionales contienen espacios al inicio o final
- **THEN** se recortan (trim) antes de persistir
- **AND** campos vacíos se guardan como string vacío o NULL según corresponda

### Requirement: Validación de campos
El sistema SHALL validar cada campo según las siguientes reglas en la capa de controller/service:

| Campo | Regla | Código HTTP |
|-------|-------|-------------|
| `name` | Requerido, no vacío | 400 |
| `surname` | Requerido, no vacío | 400 |
| `email` | Requerido, formato email válido | 400 |
| `email` | Único en DB (no registrado previamente) | 409 |
| `phone` | Opcional, solo dígitos numéricos (regex: `^[0-9]+$`) | 400 |
| `phone_contact` | Opcional, solo dígitos numéricos | 400 |
| `country` | Opcional, solo letras y espacios (`^[a-zA-ZáéíóúÁÉÍÓÚñÑ ]+$`) | 400 |
| `city` | Opcional, solo letras, números y espacios | 400 |
| `street` | Opcional, solo letras, números y espacios | 400 |
| `number` | Opcional, solo letras, números y espacios | 400 |
| `dni` | Requerido, solo dígitos numéricos, único en DB | 400 / 409 |
| `birth_date` | Requerido, formato `dd/mm/aaaa`, fecha válida | 400 |
| `password` (header) | Requerido, mínimo 8 caracteres, no contiene caracteres sensibles a hacking (`<>"'&;%`) | 400 |

#### Scenario: Validación individual por campo
- **WHEN** un campo no cumple su regla de validación
- **THEN** el sistema retorna `400 Bad Request`
- **AND** el mensaje de error indica específicamente qué campo falló y por qué

#### Scenario: DNI duplicado
- **WHEN** se registra un usuario con un dni ya existente
- **THEN** el sistema retorna `409 Conflict`
- **AND** el mensaje indica que el dni ya está registrado

#### Scenario: Password con caracteres sensibles
- **WHEN** el password contiene caracteres como `<`, `>`, `"`, `'`, `&`, `;`, `%`
- **THEN** el sistema retorna `400 Bad Request`
- **AND** el mensaje indica que el password contiene caracteres no permitidos

### Requirement: Seguridad — contraseña por header
La contraseña SHALL enviarse exclusivamente por el header `X-Password` y NO debe incluirse en el body JSON, logs de request, ni en ninguna respuesta.

#### Scenario: Password no logueada
- **WHEN** ocurre un error durante el registro
- **THEN** el password NUNCA aparece en los logs del sistema
- **AND** el password NUNCA aparece en la respuesta HTTP

### Requirement: Logging con customlogger
El sistema SHALL usar el logger existente `infrastructure/customlogger` siguiendo el mismo patrón que el resto del repositorio:

- `customlogger.Info(ctx, "mensaje", customlogger.TagMethod("Register"))` en flujo exitoso
- `customlogger.Warn(ctx, "mensaje", customlogger.Tag("field", "email"))` en validaciones fallidas
- `customlogger.Error(ctx, "mensaje", err, customlogger.Tag("step", "create_user"))` en errores de sistema

#### Scenario: Log de registro exitoso
- **WHEN** un usuario se registra exitosamente
- **THEN** se loguea con `customlogger.Info` incluyendo el email (nunca el password)
- **AND** el log incluye el método `Register` via `TagMethod`

#### Scenario: Log de error de validación
- **WHEN** una validación falla
- **THEN** se loguea con `customlogger.Warn` indicando el campo y el motivo

### Requirement: Manejo de errores con APIError
El sistema SHALL usar `apierror.APIError` para todas las respuestas de error, respetando el formato existente.

| Escenario | Código | Code | Message |
|-----------|--------|------|---------|
| Campo inválido | 400 | `"Bad request"` | Descripción del error |
| Email duplicado | 409 | `"Conflict"` | "El email ya está registrado" |
| DNI duplicado | 409 | `"Conflict"` | "El DNI ya está registrado" |
| Password ausente | 400 | `"Bad request"` | "La contraseña es requerida (header X-Password)" |
| Error interno | 500 | `"Internal Server Error"` | "Error al registrar usuario" |

#### Scenario: Errores consistentes
- **WHEN** ocurre cualquier error
- **THEN** la respuesta HTTP contiene un JSON con estructura `apierror.APIError`
- **AND** el status code es el correcto según la tabla

### Requirement: Configuración de entorno — Supabase + Render
El sistema SHALL soportar dos modos de conexión a PostgreSQL:

**Modo 1 — `DATABASE_URL` (prioritario):** Conexión mediante connection string completo. Estándar usado por Supabase y Render.

**Modo 2 — Variables individuales (fallback):** `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`.

| Variable | Descripción | Prioridad |
|----------|-------------|-----------|
| `DATABASE_URL` | Connection string completo (`postgresql://user:pass@host:port/db`) | Alta (si existe, ignora las demás) |
| `DB_HOST` | Host de PostgreSQL | Baja |
| `DB_PORT` | Puerto de PostgreSQL | Baja |
| `DB_USER` | Usuario de PostgreSQL | Baja |
| `DB_PASSWORD` | Contraseña de PostgreSQL | Baja |
| `DB_NAME` | Nombre de la base de datos | Baja |
| `ENVIRONMENT` | Entorno (`local`, `test`, `stage`, `prod`) | Siempre |

#### Scenario: Conexión via DATABASE_URL
- **WHEN** la variable `DATABASE_URL` está definida
- **THEN** se usa como connection string para GORM
- **AND** se ignoran `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`

#### Scenario: Fallback a variables individuales
- **WHEN** `DATABASE_URL` NO está definida
- **THEN** se construye el DSN desde `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`

#### Scenario: .env.example documentado
- **WHEN** se genera `.env.example`
- **THEN** incluye `DATABASE_URL` como opción principal documentada para Supabase/Render
- **AND** incluye las variables individuales como alternativa
