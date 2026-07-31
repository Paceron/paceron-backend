## ADDED Requirements

### Requirement: Grupo principal por default al crear equipo
El sistema SHALL crear un grupo principal (`is_main: true`) al crear un equipo, salvo que `create_default_group` se envíe explícitamente en `false`.

#### Scenario: Crear equipo sin especificar el flag
- **WHEN** se envía POST a `/api/v1/teams` sin `create_default_group` en el body
- **THEN** el sistema crea el equipo y su grupo principal

#### Scenario: Crear equipo con el flag en true
- **WHEN** se envía POST a `/api/v1/teams` con `"create_default_group": true`
- **THEN** el sistema crea el equipo y su grupo principal (igual que sin el flag)

#### Scenario: Crear equipo excluyendo el grupo default
- **WHEN** se envía POST a `/api/v1/teams` con `"create_default_group": false`
- **THEN** el sistema crea el equipo sin ningún grupo

### Requirement: Alta directa asigna grupo principal
El sistema SHALL dar de alta al usuario en el grupo principal del equipo al agregarlo vía `POST /api/v1/teams/:id/users`, con el mismo criterio no bloqueante que `AcceptInvitation`.

#### Scenario: Alta con grupo principal existente
- **WHEN** se agrega un usuario a un equipo que tiene grupo principal
- **THEN** el sistema crea el `TeamUser` y también un `GroupUser` en el grupo principal

#### Scenario: Alta sin grupo principal
- **WHEN** se agrega un usuario a un equipo sin ningún grupo principal
- **THEN** el sistema crea igual el `TeamUser`, sin crear ningún `GroupUser`, sin fallar la operación
