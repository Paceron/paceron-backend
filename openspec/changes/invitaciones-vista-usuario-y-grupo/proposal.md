## Why

Al integrar la Etapa 3 (Invitaciones) del frontend contra `GET /teams/{id}/invitations` y `POST /invitations/{id}/accept|reject`, aparecieron 2 gaps nuevos documentados en `paceron-frontend/docs/BACKEND_API_GAPS.md` (gaps 8 y 9, 2026-07-31): el invitado no tiene forma de ver sus propias invitaciones pendientes (solo existe la vista del lado dueño-de-equipo), y no hay forma de elegir a qué grupo va un corredor invitado — hoy toda invitación cae en un grupo default que ni siquiera se asigna realmente al aceptar.

## What Changes

- Nuevo `GET /api/v1/invitations?user_id=`: lista las invitaciones pendientes de un usuario, sin importar el equipo.
- Nuevo `GET /api/v1/invitations/:id?user_id=`: detalle de una invitación puntual, valida que pertenezca al usuario que consulta (403 si no).
- `InviteRunnerRequest` gana `group_id` opcional — si se especifica, se valida que el grupo pertenezca al equipo (404 si no).
- `AcceptInvitation` ahora da de alta al invitado en `group_users`: usa el grupo elegido al invitar, o el grupo principal (`is_main`) del equipo si no se especificó ninguno. Antes de este cambio, aceptar una invitación nunca asignaba grupo (solo `team_users`).
- `InvitationResponse` gana `team_name` y `group_id`.

## Capabilities

### Modified Capabilities
- `team-invitations`: se amplía con vista propia del invitado y elección de grupo al invitar/aceptar.

## Impact

- **Modificado**: `domains/dbs/invitation.go` (columna aditiva `group_id`), `domains/invitation/{invitation_request,invitation_response}.go`, `daos/invitation_dao.go` (+1 método), `services/invitation_service.go` (2 métodos nuevos + cambios en `InviteRunner`/`AcceptInvitation`), `controllers/invitation_controller.go` (2 endpoints nuevos), `app/app.go` (invitationService gana `groupDao`/`groupUserDao`), `app/url_mappings.go` (2 rutas nuevas).
- **Sin cambios**: contrato de `POST /teams/:id/invite` (campo nuevo opcional, no rompe clientes existentes), `POST /invitations/:id/accept|reject` (mismo contrato, comportamiento interno ampliado).
- **Swagger**: 2 endpoints nuevos, campos nuevos en request/response existentes.

### Alcance

- Listado y detalle de invitaciones del lado del invitado.
- `group_id` opcional al invitar, con fallback al grupo principal del equipo al aceptar.

### No alcance

- Rol distinto de "corredor" al aceptar — sigue igual que antes.
- Notificaciones push de "nueva invitación" — fuera de este cambio.
- Reasignar el grupo de una invitación ya enviada (editar `group_id` post-creación) — no hay caso de uso pedido, se puede cancelar/reinvitar si hace falta cambiarlo.

### Métrica de éxito

- Un usuario puede ver sus invitaciones pendientes y el detalle de una sin pasar por el equipo.
- Un entrenador puede elegir el grupo al invitar, y el corredor queda efectivamente en ese grupo (o en el principal) al aceptar.
