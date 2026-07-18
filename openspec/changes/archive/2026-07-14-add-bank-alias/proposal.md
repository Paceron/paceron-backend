## Why

Se necesita agregar un alias bancario (`bank_alias`) al perfil de usuario para permitir la identificación de cuentas bancarias de forma personalizada. Este campo es opcional y nullable, ya que no todos los usuarios cuentan con información bancaria asociada al momento de registrarse o actualizar su perfil.

## What Changes

- Se agrega el campo `bank_alias` al modelo de usuario en la base de datos (columna nullable).
- Se agrega `bank_alias` al DTO de request de actualización (`UserUpdateRequest`) como campo opcional.
- Se agrega `bank_alias` al DTO de response de actualización (`UserUpdateResponse`).
- Se implementa validación del campo en la capa de servicios:
  - Longitud: entre 6 y 20 caracteres.
  - Caracteres permitidos: letras (minúsculas/mayúsculas), números, puntos (.) y guiones (-).
- Se aplica `TrimSpace` al valor recibido antes de persistir.
- Se incluye el campo en la respuesta del endpoint de actualización.

## Capabilities

### New Capabilities

- `user-bank-alias`: Capacidad de gestión del alias bancario en el perfil de usuario, incluyendo validación de formato y persistencia en la base de datos.

### Modified Capabilities

<!-- No existen specs previos que modifiquen. -->

## Impact

- **Archivos afectados**:
  - `cmd/api/domains/dbs/user.go` — Nuevo campo `BankAlias` en el struct del modelo.
  - `cmd/api/domains/user/update_request.go` — Nuevo campo `BankAlias *string` en el request DTO.
  - `cmd/api/domains/user/update_response.go` — Nuevo campo `BankAlias string` en el response DTO.
  - `cmd/api/services/user_service.go` — Validación del campo y lógica de actualización.
- **API**: El campo `bank_alias` se agrega al body JSON del PUT `/api/v1/users/:id` y a la respuesta. Es retrocompatible (campo opcional).
- **Base de datos**: Nueva columna `bank_alias` (nullable) en la tabla `users`. GORM AutoMigrate la agregará automáticamente.
- **Dependencias**: Sin cambios.

### Objetivo

Agregar un campo opcional `bank_alias` al perfil de usuario que permita identificar cuentas bancarias con un alias personalizado, validando su formato (6-20 caracteres, solo letras, números, puntos y guiones).

### Alcance

- Endpoint `PUT /api/v1/users/:id`
- Modelo DB, DTOs de request/response, validación y lógica de actualización en el servicio.

### No alcance

- Endpoints de consulta de usuario (GET) — no se modifica la respuesta de otros endpoints.
- Autenticación o permisos — el campo no afecta el flujo de autenticación.
- Migraciones manuales — AutoMigrate de GORM se encarga.

### Métrica de éxito

- El endpoint PUT `/api/v1/users/:id` acepta y valida correctamente el campo `bank_alias`.
- El valor se persiste y se retorna en la respuesta de actualización.
- La validación rechaza valores fuera de las reglas definidas (longitud o caracteres no permitidos).
