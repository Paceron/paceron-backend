## Why

El equipo de frontend documentó (`paceron-frontend/docs/BACKEND_API_GAPS.md`, gaps 5 y 6) que `POST /api/v1/teams/:id/invite` solo envía un email — no persiste nada en base de datos. No hay forma de listar invitaciones pendientes de un equipo (bloquea la sección "Solicitudes pendientes" del frontend, mockeada indefinidamente) ni de aceptar/rechazar una invitación (el flujo hoy es "se manda un email y nada más", el propio template de mail ya dice "ingresá a la app y aceptá la notificación pendiente", pero esa notificación no existe). Ambos gaps dependen de la misma pieza faltante (persistencia de la invitación), por eso se resuelven juntos.

## What Changes

- Nuevo modelo `dbs.Invitation` (team_id, inviter_id, invitee_id, status, expires_at, responded_at) — flujo in-app, sin token secreto ni magic-link en el email.
- `InviteRunner` (`POST /api/v1/teams/:id/invite`) ahora persiste la invitación antes de mandar el mail, y valida que el usuario no sea ya miembro del equipo ni tenga una invitación pendiente duplicada.
- Nuevo `GET /api/v1/teams/:id/invitations`: lista invitaciones pendientes (no vencidas) de un equipo.
- Nuevo `POST /api/v1/invitations/:id/accept`: el invitado acepta, queda dado de alta en `team_users` como "corredor".
- Nuevo `POST /api/v1/invitations/:id/reject`: el invitado rechaza, sin afectar `team_users`.
- `InviterID` se resuelve como `team.OwnerID` — no se modifica el contrato actual de `InviteRunnerRequest` (solo `email`).

## Capabilities

### New Capabilities
- `team-invitations`: persistencia de invitaciones de equipo, listado de pendientes, aceptar/rechazar.

### Modified Capabilities
<!-- No aplica: InviteRunner es la misma capability, se extiende su comportamiento interno (persistencia), no cambia su contrato HTTP -->

## Impact

- **Nuevo**: `domains/dbs/invitation.go`, `domains/constants/invitation_status.go`, `daos/invitation_dao.go` (+test), 3 endpoints nuevos en `controllers/invitation_controller.go` (+tests), 3 métodos nuevos en `services/invitation_service.go` (+tests)
- **Modificado**: `InviteRunner` en `services/invitation_service.go` (persistencia + validaciones nuevas), `app/app.go` (wiring de `InvitationDao`), `app/url_mappings.go` (3 rutas nuevas)
- **Sin cambios**: contrato HTTP de `POST /api/v1/teams/:id/invite` (mismo request/response)
- **Swagger**: 3 endpoints nuevos, regenerar docs

### Alcance

- Modelo, DAO, service, controller y rutas para persistir invitaciones y responderlas.
- Anti-duplicado: no se puede invitar dos veces al mismo usuario al mismo equipo mientras haya una invitación pendiente.
- Expiración informativa (15 días), chequeo perezoso sin job de limpieza.

### No alcance

- Magic-link o token secreto por email — el flujo es siempre in-app, el usuario ve sus invitaciones logueado.
- Invitar a alguien que no es usuario registrado todavía (`InviteRunner` sigue requiriendo que el email exista como usuario, sin cambios sobre esa limitación preexistente).
- Notificaciones push de "tenés una invitación nueva" — fuera de este cambio.
- Rol distinto de "corredor" al aceptar — hoy solo se puede invitar/aceptar como corredor, igual que `AddUser`.

### Métrica de éxito

- Un entrenador puede ver la lista de invitaciones pendientes de su equipo sin acceso directo a la base de datos.
- Un usuario invitado puede aceptar o rechazar una invitación desde la API, y al aceptar queda efectivamente como miembro del equipo (`team_users`).
- No se puede duplicar una invitación pendiente para el mismo usuario en el mismo equipo.
