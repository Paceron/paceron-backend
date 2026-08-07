## Why

`feature/auth-session-infrastructure` dejó lista la infraestructura de sesión (JWT de acceso, refresh opaco, `AuthMiddleware()`) pero **sin aplicarla a ninguna ruta** — decisión explícita para no mezclar infraestructura con migración. Hoy, pese a tener todo el mecanismo, el backend sigue sin protección real: cualquier endpoint confía en un `user_id` que manda el propio cliente. Investigando el código se confirmó además un hueco sistemático: casi ninguna operación de escritura en teams/groups/team_users/group_users/invitations tenía chequeo de quién llama (`RemoveUser`, `Update`, `Create`, `AddUser`, `InviteRunner` — sin excepción).

## What Changes

- `AuthMiddleware()` se aplica a todas las rutas salvo 5 públicas (register, login, forgot/reset-password, `GET /auth/user`) + refresh/logout (credencial propia) + rutas demo/legacy (`/user/:user_id`, `POST /user`, weather, swagger).
- Cada controller que antes leía `user_id` de query/body/path lo reemplaza por la identidad resuelta del access token (`utils.GetAuthUserID(c)`).
- Se agrega autorización real en la capa de servicio, distribuida (no un middleware genérico — ABAC necesita el recurso ya cargado):
  - **Solo entrenador del equipo**: `team.Update/UpdateAddress/Create`, `group.Create/Update`, `team_user.AddUser`, `group_user.AddUser`, `invitation.InviteRunner`, `invitation.ListPendingInvitations` (confirmado sin ningún chequeo antes — cualquier logueado veía invitados de cualquier equipo).
  - **Self o entrenador delegado**: `team_user.RemoveUser`, `group_user.RemoveUser` (antes sin ningún chequeo — confirmado leyendo el código).
  - **Self-only**: `user.Update/ChangeStatus/ChangePassword`.
  - **Requiere login, sin chequeo adicional** (limitación documentada, no se resuelve acá — no existe concepto de "admin"): catálogo de roles/tiers/permisos.
- `team.Create` deja de aceptar `owner_id` en el body — el owner es siempre quien está autenticado.
- `POST /api/v1/invitations/:id/accept|reject` dejan de requerir body — el usuario que responde sale del token.
- Parámetros `user_id` que antes eran query/body obligatorios en varios GETs de invitaciones y grupos se eliminan, resueltos del token.

## Capabilities

### New Capabilities
- `endpoint-authorization`: aplicación del middleware de autenticación a las rutas protegidas, y las reglas de autorización por patrón (self-only, self-o-delegado, solo-entrenador) en la capa de servicio.

### Modified Capabilities
- (ninguna formalizada en OpenSpec previamente — team/group/team-user/group-user/invitation no tenían spec propia; el detalle de qué cambió por servicio está en `design.md` y en `docs/AUTH_MIGRATION.md`, orientado al consumidor externo del contrato).

## Impact

- **Modificado**: firmas de `TeamServiceInterface`, `GroupServiceInterface`, `TeamUserServiceInterface`, `GroupUserServiceInterface`, `InvitationServiceInterface` (agregan `callerID`/`ownerID` explícito); `TeamDelegate.CreateTeam`; todos los controllers de teams/groups/team-users/group-users/invitations/users.
- **Nuevo**: `cmd/api/utils/authcontext.go` (claves de contexto compartidas entre `app` y `controllers`, evita import cycle).
- **Eliminado**: `team.CreateTeamRequest.OwnerID`, `invitation.RespondInvitationRequest` (ya no se necesita, accept/reject no llevan body).
- **Tests**: reescritos todos los tests de service/controller/delegate afectados, más casos nuevos de autorización (forbidden self-vs-delegado, forbidden no-entrenador) — suite completa verde.
- **Swagger**: regenerado.
- **Docs**: `README.md` (tabla de endpoints con marca 🔓 en públicos + notas de autorización), `docs/AUTH_MIGRATION.md` nuevo (contrato para consumir desde el frontend, en otra iniciativa separada).
- **Frontend**: sin cambios en este repo — `docs/AUTH_MIGRATION.md` es el hand-off para que el equipo de frontend actualice el cliente por separado.
