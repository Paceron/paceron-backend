## ADDED Requirements

### Requirement: Crear solicitud de ingreso a un equipo público

El sistema SHALL permitir a un corredor autenticado crear una solicitud de ingreso (`POST /api/v1/teams/:id/join-requests`) a un equipo con `is_public = true`. El sistema SHALL rechazar la creación si el equipo no existe, no es público, está al cupo máximo, el caller ya es miembro, o ya existe una solicitud `pending` del mismo caller a ese equipo.

#### Scenario: Solicitud válida
- **WHEN** un corredor autenticado solicita unirse a un equipo público con cupo disponible, sin solicitud previa pendiente ni membresía existente
- **THEN** el sistema crea la solicitud con estado `pending`

#### Scenario: Equipo no existe
- **WHEN** el `:id` del equipo no corresponde a ningún equipo existente
- **THEN** el sistema responde 404 con el código `TEAM_NOT_FOUND`

#### Scenario: Equipo no público
- **WHEN** el equipo tiene `is_public = false`
- **THEN** el sistema responde 403 con el código `TEAM_NOT_PUBLIC`, sin crear la solicitud

#### Scenario: Equipo al cupo máximo
- **WHEN** el equipo ya tiene `max_members` miembros activos
- **THEN** el sistema responde 409 con el código `TEAM_FULL`

#### Scenario: Caller ya es miembro
- **WHEN** el corredor ya pertenece al equipo
- **THEN** el sistema responde 409 con el código `ALREADY_MEMBER`

#### Scenario: Solicitud duplicada
- **WHEN** el corredor ya tiene una solicitud `pending` a ese mismo equipo
- **THEN** el sistema responde 409 con el código `JOIN_REQUEST_ALREADY_PENDING`

### Requirement: Cancelar solicitud propia

El sistema SHALL permitir al corredor dueño de una solicitud cancelarla (`DELETE /api/v1/join-requests/:id`) mientras su estado sea `pending`.

#### Scenario: Cancelación válida
- **WHEN** el dueño de una solicitud `pending` la cancela
- **THEN** el sistema la elimina o marca como no vigente, dejando de contar para el badge del entrenador

#### Scenario: Solicitud no encontrada
- **WHEN** el `:id` no corresponde a ninguna solicitud existente
- **THEN** el sistema responde 404 con el código `JOIN_REQUEST_NOT_FOUND`

#### Scenario: No es el dueño de la solicitud
- **WHEN** un usuario que no creó la solicitud intenta cancelarla
- **THEN** el sistema responde 403 con el código `FORBIDDEN`

#### Scenario: Solicitud ya resuelta
- **WHEN** la solicitud ya está `accepted` o `rejected`
- **THEN** el sistema responde 409 con el código `JOIN_REQUEST_NOT_PENDING`, sin modificarla

### Requirement: Listar solicitudes propias y por equipo

El sistema SHALL exponer `GET /api/v1/join-requests/mine` (solicitudes del corredor autenticado, cualquier estado) y `GET /api/v1/teams/:id/join-requests` (solicitudes `pending` de un equipo, solo para el entrenador dueño).

#### Scenario: Corredor lista sus solicitudes
- **WHEN** un corredor autenticado llama `GET /api/v1/join-requests/mine`
- **THEN** el sistema devuelve todas sus solicitudes, en cualquier estado

#### Scenario: Entrenador no dueño intenta listar
- **WHEN** un usuario que no es el entrenador dueño del equipo llama `GET /api/v1/teams/:id/join-requests`
- **THEN** el sistema responde 403 con el código `FORBIDDEN`

### Requirement: Conteo agregado de solicitudes pendientes

El sistema SHALL exponer `GET /api/v1/join-requests/pending-count`, que devuelve la suma de solicitudes `pending` en todos los equipos que administra el entrenador autenticado, para alimentar un badge de novedades.

#### Scenario: Entrenador con solicitudes pendientes en varios equipos
- **WHEN** el entrenador autenticado tiene solicitudes `pending` repartidas en 2 equipos que administra
- **THEN** el sistema devuelve la suma total, sin desglosar por equipo

### Requirement: Aceptar solicitud crea la membresía gateada por pago

El sistema SHALL permitir al entrenador dueño del equipo aceptar una solicitud `pending` (`POST /api/v1/join-requests/:id/accept`). Al aceptar, el sistema SHALL crear el `team_user` del corredor aplicando la misma gate de pago por membresía (`membership_fee`) que ya rige para alta directa y aceptación de invitación, asignarlo al grupo default del equipo, y marcar la solicitud como `accepted` — todo en una única operación atómica.

#### Scenario: Aceptar solicitud a equipo gratuito
- **WHEN** el entrenador dueño acepta una solicitud `pending` a un equipo con `membership_fee = 0`
- **THEN** el sistema crea el `team_user` con estado de suscripción activo, lo asigna al grupo default, y marca la solicitud `accepted`

#### Scenario: Aceptar solicitud a equipo con mensualidad
- **WHEN** el entrenador dueño acepta una solicitud `pending` a un equipo con `membership_fee > 0`
- **THEN** el sistema crea el `team_user` en estado de primer pago pendiente y genera la cuota #1, igual que al aceptar una invitación al mismo equipo

#### Scenario: Falla cualquier paso de la aceptación
- **WHEN** la creación del `team_user`, la asignación de grupo, o la actualización de estado de la solicitud fallan
- **THEN** el sistema revierte toda la operación — no queda un `team_user` creado con la solicitud todavía `pending`, ni una solicitud `accepted` sin membresía real

#### Scenario: Solicitud ya resuelta
- **WHEN** se intenta aceptar una solicitud que ya está `accepted` o `rejected`
- **THEN** el sistema responde 409 con el código `JOIN_REQUEST_NOT_PENDING`

#### Scenario: Equipo se llenó mientras la solicitud estaba pendiente
- **WHEN** al momento de aceptar, el equipo ya alcanzó `max_members` con otras altas ocurridas después de crearse la solicitud
- **THEN** el sistema responde 409 con el código `TEAM_FULL`, sin crear la membresía

#### Scenario: No es el dueño del equipo
- **WHEN** un usuario que no es el entrenador dueño del equipo intenta aceptar
- **THEN** el sistema responde 403 con el código `FORBIDDEN`

### Requirement: Rechazar solicitud

El sistema SHALL permitir al entrenador dueño del equipo rechazar una solicitud `pending` (`POST /api/v1/join-requests/:id/reject`), marcándola como `rejected` sin crear ninguna membresía.

#### Scenario: Rechazo válido
- **WHEN** el entrenador dueño rechaza una solicitud `pending`
- **THEN** el sistema la marca `rejected` y no crea ningún `team_user`

#### Scenario: Solicitud ya resuelta
- **WHEN** se intenta rechazar una solicitud que ya no está `pending`
- **THEN** el sistema responde 409 con el código `JOIN_REQUEST_NOT_PENDING`
