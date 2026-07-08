## ADDED Requirements

### Requirement: Constantes de estados de usuario
El sistema SHALL definir una lista de estados válidos para usuarios como constantes tipadas en `domains/constants/user_status.go`, incluyendo: `active`, `inactive`, `pause`, `blocked`, `suspended`.

#### Scenario: Constantes definidas
- **WHEN** se importa el paquete `constants`
- **THEN** existen constantes exportadas `UserStatusActive`, `UserStatusInactive`, `UserStatusPause`, `UserStatusBlocked`, `UserStatusSuspended` de tipo `string`
- **AND** cada constante tiene su valor en minúsculas (ej: `UserStatusActive = "active"`)

#### Scenario: Lista de estados válidos accesible
- **WHEN** se llama a `GetValidUserStatuses()`
- **THEN** retorna un slice con los 5 estados válidos en orden: `["active", "inactive", "pause", "blocked", "suspended"]`

#### Scenario: Validación de estado
- **WHEN** se llama a `IsValidUserStatus(status string)` con un estado válido
- **THEN** retorna `true`
- **WHEN** se llama con un estado inválido
- **THEN** retorna `false`

### Requirement: Valor por defecto de status en registro
El sistema SHALL establecer `status = "active"` por defecto al crear un nuevo usuario mediante el endpoint de registro.

#### Scenario: Usuario registrado con status active
- **WHEN** se registra un usuario exitosamente via `POST /api/v1/auth/register`
- **THEN** el campo `status` en la BD se guarda como `"active"`
- **AND** el response `RegisterResponse` incluye el campo `status` con valor `"active"`