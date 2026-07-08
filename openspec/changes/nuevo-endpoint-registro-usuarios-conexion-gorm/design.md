## Context

Actualmente el proyecto ya tiene GORM conectado via `infrastructure/postgresdb/postgres.go` y un endpoint básico `POST /user` con modelo `dbs.User` de solo 3 campos (id, name, password). No existe auto-migrate, las contraseñas se guardan en texto plano, y no hay validación de email único.

Este diseño implementa:
- Auto-migrate de modelos GORM en `infrastructure/postgresdb/postgres.go`
- Modelo `dbs.User` completo con todos los campos solicitados
- Endpoint `POST /api/v1/auth/register` con validaciones y seguridad

## Goals / Non-Goals

**Goals:**
- Mejorar la inicialización de PostgreSQL en `infrastructure/postgresdb/postgres.go` con auto-migrate
- Ejecutar auto-migrate de `dbs.User` al iniciar la aplicación
- Proveer endpoint de registro con request/response DTOs desacoplados
- Validar cada campo con reglas específicas (regex, formato, unicidad)
- Hashear contraseñas con bcrypt
- Seguridad: password por header `X-Password`, nunca en body/logs/response
- Configuración externalizada via env vars + `.env.example`
- Logging con customlogger y errores con apierror.APIError
- JSON en under_score, Go en camelCase

**Non-Goals:**
- Login / autenticación (JWT, sesiones)
- Recuperación de contraseña
- Confirmación de email
- Rate limiting
- Roles y permisos

## Decisions

### 1. Cliente DB en `infrastructure/postgresdb/postgres.go`
- **Qué**: Mejorar la función `ConfigDB()` existente en `infrastructure/postgresdb/postgres.go` agregando auto-migrate y mejor configuración de pool
- **Por qué**: Los clientes externos se inicializan en `infrastructure/`, que es la capa base de la pirámide de dependencias. La DB es un servicio de infraestructura, no un dominio de negocio.

### 2. Auto-migrate en `ConfigDB`
- **Qué**: Ejecutar `db.AutoMigrate(&dbs.User{})` dentro de `ConfigDB` después del ping exitoso
- **Por qué**: Garantiza que la tabla exista sin necesidad de migraciones manuales ni scripts SQL externos. Apto para etapas tempranas del proyecto.
- **Alternativa**: Migraciones SQL manuales con golang-migrate — más control pero sobreingeniería para el estado actual.

### 3. Password por header `X-Password`
- **Qué**: La contraseña se recibe en `c.GetHeader("X-Password")`, no en el body JSON
- **Por qué**: Los bodies de request suelen loguearse por proxies, balanceadores y middlewares. Los headers personalizados no se loguean por defecto. El password nunca debe aparecer en logs.
- **Riesgo**: Headers pueden ser logueados si se configura logging de headers. Mitigación: sanitizar explícitamente en el logger.

### 4. Request/Response DTOs separados (`domains/auth/`)
- **Qué**: `RegisterRequest` (input) y `RegisterResponse` (output) en packages separados dentro de `domains/auth/`
- **Por qué**: Desacopla la firma del contrato público de la representación interna. Permite evolucionar cada uno independientemente.
- **Convención**: Tags JSON en under_score (`json:"birth_date"`), campos Go en camelCase (`BirthDate`).

### 5. Validación en dos capas
- **Qué**: `binding:"required"` + validaciones custom en el controller para formato, y validación de unicidad (email/dni) en el service
- **Por qué**: Las validaciones de formato son síncronas y atómicas (rápidas, sin IO), ideales para el controller. Las de unicidad requieren consulta a DB, pertenecen al service.
- **Alternativa**: Todo en una capa — mezcla responsabilidades y dificulta el testing.

### 6. Transformers en el service
- **Qué**: `service.toDBModel(req, passwordHash) *dbs.User` y `service.toResponse(userDB) *auth.RegisterResponse`
- **Por qué**: Mantiene la lógica de transformación cerca de la capa que la necesita. El controller no sabe nada del modelo DB.
- **Parseo**: `birth_date` string → `time.Time` con formato `dd/mm/aaaa` usando `time.Parse("02/01/2006", ...)`. Si falla → 400.

### 7. Bcrypt para password
- **Qué**: `golang.org/x/crypto/bcrypt` con costo 10 (`bcrypt.GenerateFromPassword`)
- **Por qué**: Estándar de la industria, resistente a ataques de fuerza bruta, soportado por Go.

### 8. Soporte de DATABASE_URL para Supabase/Render
- **Qué**: La configuración de DB acepta `DATABASE_URL` (prioritario) además de las variables individuales (`DB_HOST`, etc.)
- **Por qué**: Supabase entrega un connection string único. Render también soporta `DATABASE_URL` como estándar. Tener ambos modos permite usar Supabase en stage y variables separadas en otros entornos.
- **Implementación**: En `config.LoadValues()`, si `DATABASE_URL` existe, se parsea y se extraen host, port, user, password, dbname para mantener compatibilidad con el struct `config.DB` existente.

### 9. .env.example para Render
- **Qué**: Archivo `.env.example` en raíz con todas las variables documentadas, incluyendo `DATABASE_URL` como opción principal
- **Por qué**: Facilita el setup local y el deploy en Render. Las variables se configuran en el panel de Render.

## Architecture Flow

```
POST /api/v1/auth/register
  │
  ├─► router.go: auth.POST("/register", authController.Register)
  │
  ├─► authController.Register(c *gin.Context)
  │     ├─ Obtener X-Password del header
  │     ├─ BindJSON → auth.RegisterRequest
  │     ├─ Validar formato de campos (regex)
  │     ├─ Llamar authService.Register(ctx, req, password)
  │     └─ Responder 201 con auth.RegisterResponse
  │
  ├─► authService.Register(ctx, req, password) → (*auth.RegisterResponse, error)
  │     ├─ Validar email único (authDao.FindByEmail)
  │     ├─ Validar dni único (authDao.FindByDNI)
  │     ├─ Hashear password con bcrypt
  │     ├─ Transformar req → dbs.User (parsear birth_date)
  │     ├─ Llamar authDao.Create(ctx, userDB)
  │     ├─ Transformar dbs.User → auth.RegisterResponse
  │     └─ Log con customlogger.Info
  │
  └─► authDao (implements AuthDaoInterface)
        ├─ FindByEmail(ctx, email) → (*dbs.User, error)
        ├─ FindByDNI(ctx, dni) → (*dbs.User, error)
        └─ Create(ctx, user *dbs.User) → (*dbs.User, error)
```

## New files structure

```
cmd/api/
├── domains/
│   └── auth/
│       ├── register_request.go    ← DTO de entrada (under_score JSON)
│       └── register_response.go   ← DTO de salida (under_score JSON)
├── controllers/
│   └── auth_controller.go         ← Handler POST /api/v1/auth/register
├── services/
│   └── auth_service.go            ← Lógica de registro + transformers
├── daos/
│   └── auth_dao.go                ← Acceso a DB para auth
├── config/
│   └── config.go                  ← + DATABASE_URL support
└── app/
    ├── app.go                     ← DI (postgresdb.ConfigDB + auto-migrate)
    └── url_mappings.go            ← Nueva ruta auth

raíz/
└── .env.example                   ← Variables documentadas (DATABASE_URL + individuales)
```

## Risks / Trade-offs

- **Auto-migrate en producción**: GORM AutoMigrate solo agrega columnas, nunca elimina. Bajo riesgo, pero para proyectos maduros conviene migraciones versionadas.
- **Header X-Password**: No es estándar. Algunos API gateways pueden bloquear headers custom. Mitigación: documentar en Swagger que el password va por header.
- **bcrypt costo 10**: Seguro pero lento (~100ms por hash). Para registro es aceptable. Para login frecuente, evaluar costo menor o caching.
- **Sin confirmación de email**: El registro es inmediato. Para producción real se debería agregar verificación de email.

## Open Questions

- ¿El endpoint debe ser versionado como `/api/v1/auth/register` o `/auth/register`? Se opta por `/api/v1/auth/register` por claridad.
- Ninguna por el momento.
