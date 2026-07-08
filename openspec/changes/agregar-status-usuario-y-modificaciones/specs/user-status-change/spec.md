## ADDED Requirements

### Requirement: Endpoint PATCH /api/v1/users/{id}/status
El sistema SHALL proveer endpoint `PATCH /api/v1/users/{id}/status` para cambiar exclusivamente el estado de un usuario. Solo se permite actualizar el campo `status` mediante este endpoint.

#### Scenario: Cambio de estado exitoso
- **WHEN** se envía `PATCH /api/v1/users/{id}/status` con JSON `{"status": "pause"}`
- **THEN** retorna `200 OK` con `UserUpdateResponse` mostrando el nuevo `status`
- **AND** el cambio se persiste en BD

#### Scenario: Usuario no encontrado
- **WHEN** el `id` en la URL no existe en BD
- **THEN** retorna `404 Not Found` con `APIError`

#### Scenario: Estado inválido
- **WHEN** se envía un `status` que no está en la lista de estados válidos
- **THEN** retorna `400 Bad Request` con mensaje específico indicando los estados permitidos

#### Scenario: Mismo estado actual
- **WHEN** se envía el mismo `status` que ya tiene el usuario
- **THEN** retorna `200 OK` sin cambios (idempotente)

### Requirement: DTO para cambio de estado
El sistema SHALL definir `StatusChangeRequest` en `domains/user/` con el campo `status` requerido y validación contra constantes.

#### Scenario: Campo status requerido
- **WHEN** se envía request sin campo `status` o con `status` vacío
- **THEN** retorna `400 Bad Request` indicando que el campo es requerido

#### Scenario: Validación contra constantes
- **WHEN** se recibe un `status` en el request
- **THEN** el sistema valida contra `IsValidUserStatus()` antes de persistir