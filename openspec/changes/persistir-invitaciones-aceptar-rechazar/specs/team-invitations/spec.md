## ADDED Requirements

### Requirement: Persistir la invitación al invitar
El sistema SHALL persistir una invitación (`Invitation`, estado `pending`) al enviar una invitación vía `POST /api/v1/teams/:id/invite`, además de mandar el email, sin cambiar el request/response existente.

#### Scenario: Invitación exitosa
- **WHEN** se envía una solicitud POST a `/api/v1/teams/:id/invite` con el email de un usuario existente que no pertenece al equipo ni tiene invitación pendiente
- **THEN** el sistema crea una `Invitation` con estado `pending`, envía el email y responde HTTP 200

#### Scenario: Usuario ya es miembro del equipo
- **WHEN** se invita a un email cuyo usuario ya pertenece al equipo (tiene `TeamUser` activo)
- **THEN** el sistema responde HTTP 409 sin crear la invitación ni enviar el email

#### Scenario: Invitación pendiente duplicada
- **WHEN** se invita a un email que ya tiene una `Invitation` con estado `pending` para el mismo equipo
- **THEN** el sistema responde HTTP 409 sin crear una segunda invitación

### Requirement: Listar invitaciones pendientes de un equipo
El sistema SHALL aceptar solicitudes GET a `/api/v1/teams/:id/invitations` y SHALL devolver las invitaciones con estado `pending` que no hayan vencido.

#### Scenario: Listado exitoso
- **WHEN** se envía GET a `/api/v1/teams/:id/invitations` para un equipo con invitaciones pendientes activas
- **THEN** el sistema responde HTTP 200 con la lista de invitaciones, incluyendo nombre y email del invitado

#### Scenario: Invitaciones vencidas no aparecen
- **WHEN** un equipo tiene una invitación con estado `pending` cuyo `expires_at` ya pasó
- **THEN** esa invitación no aparece en el listado de `GET /api/v1/teams/:id/invitations`

#### Scenario: Equipo inexistente
- **WHEN** se envía GET a `/api/v1/teams/:id/invitations` para un `id` de equipo que no existe
- **THEN** el sistema responde HTTP 404

### Requirement: Aceptar una invitación
El sistema SHALL aceptar solicitudes POST a `/api/v1/invitations/:id/accept` con `{user_id}` en el body, y SHALL dar de alta al invitado como corredor del equipo si la invitación es válida.

#### Scenario: Aceptación exitosa
- **WHEN** el usuario invitado envía POST a `/api/v1/invitations/:id/accept` para una invitación propia, pendiente y no vencida
- **THEN** el sistema crea un `TeamUser` con rol `corredor` para ese usuario y equipo, marca la invitación como `accepted`, y responde HTTP 200

#### Scenario: Invitación de otro usuario
- **WHEN** se envía `user_id` distinto al `invitee_id` de la invitación
- **THEN** el sistema responde HTTP 403 sin modificar la invitación ni `team_users`

#### Scenario: Invitación ya respondida
- **WHEN** se intenta aceptar una invitación con estado `accepted` o `rejected`
- **THEN** el sistema responde HTTP 409

#### Scenario: Invitación vencida
- **WHEN** se intenta aceptar una invitación pendiente cuyo `expires_at` ya pasó
- **THEN** el sistema responde HTTP 409 sin dar de alta al usuario

#### Scenario: Aceptar una invitación cuando ya se es miembro del equipo
- **WHEN** el usuario invitado ya tiene un `TeamUser` activo en ese equipo al momento de aceptar (por ejemplo, se agregó por otra vía mientras la invitación seguía pendiente)
- **THEN** el sistema marca la invitación como `accepted` sin crear un segundo `TeamUser`, y responde HTTP 200

### Requirement: Rechazar una invitación
El sistema SHALL aceptar solicitudes POST a `/api/v1/invitations/:id/reject` con `{user_id}` en el body, y SHALL marcar la invitación como rechazada sin afectar `team_users`.

#### Scenario: Rechazo exitoso
- **WHEN** el usuario invitado envía POST a `/api/v1/invitations/:id/reject` para una invitación propia, pendiente y no vencida
- **THEN** el sistema marca la invitación como `rejected`, no crea ningún `TeamUser`, y responde HTTP 200

#### Scenario: Rechazo de invitación de otro usuario
- **WHEN** se envía `user_id` distinto al `invitee_id` de la invitación
- **THEN** el sistema responde HTTP 403
