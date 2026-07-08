## ADDED Requirements

### Requirement: Modelo User con campo status
El modelo GORM `dbs.User` SHALL incluir el campo `status` con constraint NOT NULL y valor por defecto `active`.

#### Scenario: Columna status en tabla users
- **WHEN** se ejecuta auto-migrate
- **THEN** la tabla `users` contiene la columna `status` tipo string NOT NULL DEFAULT 'active'

### Requirement: Endpoint PUT /api/v1/users/{id} para actualizar usuario
El sistema SHALL proveer endpoint `PUT /api/v1/users/{id}` que permita actualizar cualquier atributo del usuario excepto `id` y `created_at`.

#### Scenario: Actualización exitosa de campos permitidos
- **WHEN** se envía `PUT /api/v1/users/{id}` con JSON válido conteniendo campos a actualizar
- **THEN** retorna `200 OK` con `UserUpdateResponse` con los datos actualizados
- **AND** los campos actualizados se persisten en BD

#### Scenario: No permitir actualización de ID
- **WHEN** el request incluye `user_id` en el body
- **THEN** el sistema ignora el campo `user_id` y no lo actualiza
- **AND** retorna `200 OK` con los demás campos actualizados

#### Scenario: Usuario no encontrado
- **WHEN** el `id` en la URL no existe en BD
- **THEN** retorna `404 Not Found` con `APIError` code `"Not Found"`

#### Scenario: Validación de campos (reutiliza reglas de registro)
- **WHEN** un campo no cumple su regla de validación (email formato, phone solo dígitos, etc.)
- **THEN** retorna `400 Bad Request` con mensaje específico del campo

### Requirement: Cambio de email requiere autenticación de password actual
El sistema SHALL requerir el header `X-Current-Password` cuando el request intente cambiar el campo `email`.

#### Scenario: Cambio de email con password correcto
- **WHEN** se envía `PUT /api/v1/users/{id}` con `email` nuevo y header `X-Current-Password` válido
- **THEN** actualiza el email y retorna `200 OK`

#### Scenario: Cambio de email sin password
- **WHEN** se envía `PUT /api/v1/users/{id}` con `email` nuevo SIN header `X-Current-Password`
- **THEN** retorna `400 Bad Request` con mensaje "Para cambiar el email debe proporcionar la contraseña actual (header X-Current-Password)"

#### Scenario: Cambio de email con password incorrecto
- **WHEN** se envía `PUT /api/v1/users/{id}` con `email` nuevo y header `X-Current-Password` inválido
- **THEN** retorna `401 Unauthorized` con mensaje "Contraseña actual incorrecta"

#### Scenario: Cambio de email a uno ya existente
- **WHEN** se intenta cambiar email a uno ya registrado por otro usuario
- **THEN** retorna `409 Conflict` con mensaje "El email ya está registrado"

### Requirement: DTOs para actualización de usuario
El sistema SHALL definir `UserUpdateRequest` y `UserUpdateResponse` en `domains/user/` con JSON under_score.

#### Scenario: Request sin password en body
- **WHEN** se envía request de actualización
- **THEN** el password NO se recibe en body (igual que en registro, por header si se necesita cambiar)

#### Scenario: Response excluye password
- **WHEN** la actualización es exitosa
- **THEN** `UserUpdateResponse` NO contiene campo password