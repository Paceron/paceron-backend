## ADDED Requirements

### Requirement: Filtrar equipos por owner o miembro
El sistema SHALL aceptar los query params opcionales `owner_id` y `member_id` en `GET /api/v1/teams`, y SHALL filtrar el listado en consecuencia. Sin ninguno de los dos, SHALL devolver todos los equipos activos (comportamiento actual, sin cambios).

#### Scenario: Sin filtros
- **WHEN** se envía GET a `/api/v1/teams` sin query params
- **THEN** el sistema responde con todos los equipos activos, igual que hoy

#### Scenario: Filtro por owner_id
- **WHEN** se envía GET a `/api/v1/teams?owner_id=X`
- **THEN** el sistema responde solo con los equipos activos donde `owner_id` de la fila coincide con `X`

#### Scenario: Filtro por member_id
- **WHEN** se envía GET a `/api/v1/teams?member_id=Y`
- **THEN** el sistema responde solo con los equipos activos donde el usuario `Y` tiene un `TeamUser` activo, sin importar el rol

#### Scenario: Filtro combinado
- **WHEN** se envía GET a `/api/v1/teams?owner_id=X&member_id=Y`
- **THEN** el sistema responde solo con los equipos administrados por `X` donde además `Y` es miembro activo (AND)

#### Scenario: ID inválido
- **WHEN** se envía GET a `/api/v1/teams?owner_id=abc` (no numérico)
- **THEN** el sistema responde HTTP 400
