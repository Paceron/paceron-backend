## ADDED Requirements

### Requirement: Listar mis invitaciones pendientes
El sistema SHALL aceptar solicitudes GET a `/api/v1/invitations?user_id=` y SHALL devolver las invitaciones pendientes (no vencidas) de ese usuario, sin importar el equipo.

#### Scenario: Listado exitoso
- **WHEN** se envía GET a `/api/v1/invitations?user_id=X` para un usuario con invitaciones pendientes en distintos equipos
- **THEN** el sistema responde HTTP 200 con todas ellas, incluyendo nombre del equipo

#### Scenario: user_id faltante
- **WHEN** se envía GET a `/api/v1/invitations` sin `user_id`
- **THEN** el sistema responde HTTP 400

### Requirement: Ver detalle de una invitación propia
El sistema SHALL aceptar solicitudes GET a `/api/v1/invitations/:id?user_id=` y SHALL devolver el detalle solo si `user_id` coincide con el invitado, sin restricción de estado.

#### Scenario: Detalle exitoso
- **WHEN** el usuario invitado consulta GET `/api/v1/invitations/:id?user_id=` con su propio ID
- **THEN** el sistema responde HTTP 200 con el detalle, sin importar si está pending/accepted/rejected

#### Scenario: Consulta de invitación ajena
- **WHEN** se consulta con un `user_id` distinto al `invitee_id` de la invitación
- **THEN** el sistema responde HTTP 403

### Requirement: Elegir grupo al invitar
El sistema SHALL aceptar un `group_id` opcional en `POST /api/v1/teams/:id/invite`, y SHALL validar que pertenezca al equipo.

#### Scenario: Invitación con grupo válido
- **WHEN** se invita con `group_id` de un grupo que pertenece al equipo
- **THEN** la invitación se crea con ese `group_id`

#### Scenario: Invitación con grupo de otro equipo
- **WHEN** se invita con `group_id` que no pertenece al equipo (o no existe)
- **THEN** el sistema responde HTTP 404 sin crear la invitación

### Requirement: Asignar grupo al aceptar
El sistema SHALL dar de alta al invitado en `group_users` al aceptar una invitación: en el grupo especificado en la invitación, o en el grupo principal (`is_main`) del equipo si no se especificó ninguno.

#### Scenario: Aceptar con grupo especificado
- **WHEN** se acepta una invitación que tiene `group_id` asignado
- **THEN** el sistema crea un `GroupUser` para ese grupo y usuario

#### Scenario: Aceptar sin grupo especificado, equipo con grupo principal
- **WHEN** se acepta una invitación sin `group_id`, y el equipo tiene un grupo con `is_main: true`
- **THEN** el sistema crea un `GroupUser` para ese grupo principal

#### Scenario: Aceptar sin grupo especificado ni grupo principal
- **WHEN** se acepta una invitación sin `group_id`, y el equipo no tiene ningún grupo con `is_main: true`
- **THEN** la invitación se acepta igual (queda `accepted`, el usuario queda en `team_users`), sin crear ningún `GroupUser`
