## ADDED Requirements

### Requirement: Registro de token de push por dispositivo
El sistema SHALL permitir que un usuario autenticado registre o actualice el token de push de su dispositivo, con upsert por el valor del token (no por usuario).

#### Scenario: Registro exitoso
- **WHEN** un usuario autenticado envía `POST /api/v1/push-tokens` con un `token` y `platform` válidos
- **THEN** el sistema SHALL persistir el token asociado a ese usuario y retornar HTTP 200

#### Scenario: Mismo dispositivo cambia de cuenta
- **WHEN** un `token` ya registrado para un usuario se vuelve a registrar con un `user_id` distinto (otra sesión logueada en el mismo dispositivo)
- **THEN** el sistema SHALL reasignar el dueño del token al nuevo usuario, sin requerir una acción explícita de "desvincular" del usuario anterior

#### Scenario: Platform inválida
- **WHEN** se envía un `platform` fuera de los valores soportados (`android`, `web`)
- **THEN** el sistema SHALL retornar HTTP 400 sin persistir el token

### Requirement: Notificación push en eventos que le pasan a otro usuario
El sistema SHALL disparar una notificación push, vía la API de Expo, a todos los dispositivos registrados del usuario afectado, para cada uno de los 5 triggers acordados con el frontend.

#### Scenario: Invitación recibida
- **WHEN** un entrenador invita a un corredor a un equipo
- **THEN** el sistema SHALL enviar un push al invitado con `data.type = "invitation_received"` y `data.route = "/invitations"`

#### Scenario: Respuesta a invitación
- **WHEN** un corredor acepta o rechaza una invitación
- **THEN** el sistema SHALL enviar un push al entrenador que invitó con `data.type = "invitation_response"` y `data.route` apuntando al equipo

#### Scenario: Expulsión de equipo
- **WHEN** un entrenador quita a un corredor de su equipo
- **THEN** el sistema SHALL enviar un push al corredor removido con `data.type = "team_removed"` y `data.route = "/teams"`

#### Scenario: Un corredor deja el equipo por su cuenta
- **WHEN** un corredor se remueve a sí mismo de un equipo
- **THEN** el sistema SHALL enviar un push al entrenador del equipo con `data.type = "team_member_left"` y `data.route` apuntando al equipo

#### Scenario: Cambio de contraseña exitoso
- **WHEN** un usuario cambia su propia contraseña exitosamente
- **THEN** el sistema SHALL enviar un push informativo al propio usuario con `data.type = "password_changed"`, sin `data.route`

#### Scenario: Fallo de envío no bloquea la operación principal
- **WHEN** el envío de un push (o del mail equivalente) falla por cualquier motivo (token inválido, Expo o Resend no disponible)
- **THEN** el sistema SHALL loguear el error y completar igual la operación principal que disparó la notificación

### Requirement: Mail nuevo alineado con los triggers de push
El sistema SHALL enviar un correo, además del push, para los triggers que no tenían mail equivalente antes de esta iniciativa.

#### Scenario: Cobertura de mail para los 4 triggers nuevos
- **WHEN** ocurre respuesta a invitación, expulsión de equipo, salida voluntaria de un corredor, o cambio de contraseña
- **THEN** el sistema SHALL enviar también un mail (`EmailTypeInvitationResponse`, `EmailTypeTeamRemoved`, `EmailTypeTeamMemberLeft`, o `EmailTypePasswordChanged` respectivamente) al mismo destinatario que recibe el push
