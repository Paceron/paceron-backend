# Especificación delta — workout-history

## ADDED Requirements

### Requirement: El historial expone si la sesión fue presencial

El sistema DEBE incluir (MUST) `is_presencial` (boolean) en cada ítem de `GET /users/{id}/workout-feedback-history` y de `GET /users/{id}/administered-workout-feedback-history`, calculado a partir del día de calendario asociado a la sesión del feedback. Este campo extiende el contrato del historial definido para el Gap 13.

#### Scenario: Sesión presencial

- **WHEN** el feedback pertenece a una sesión con un día de calendario marcado `is_presencial=true`
- **THEN** el ítem devuelve `is_presencial: true`

#### Scenario: Sesión asíncrona

- **WHEN** el feedback pertenece a una sesión con un día de calendario marcado `is_presencial=false`
- **THEN** el ítem devuelve `is_presencial: false`

#### Scenario: Feedback huérfano

- **WHEN** el feedback no tiene un día de calendario asociado (día borrado o instancia sin día)
- **THEN** el ítem devuelve `is_presencial: false` y conserva `group_id`/`group_name` en `null`

#### Scenario: Compatibilidad

- **WHEN** un cliente existente consume la respuesta sin leer el campo nuevo
- **THEN** el resto de los campos, filtros, orden y paginación se mantienen sin cambios
