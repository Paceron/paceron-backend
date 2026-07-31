## Why

Probando el flujo de invitaciones con `group_id` (feature previa) se detectó que `POST /teams` podía crear equipos sin ningún grupo (si se omitía `create_default_group`), y que `POST /teams/:id/users` (alta directa sin invitación) nunca asignaba grupo — mismo hueco que tenía `AcceptInvitation` antes de la feature anterior. El modelo real del dominio es que un usuario pertenece a un grupo dentro de un equipo (el principal u otro más específico), no al equipo directamente — un equipo sin ningún grupo es un estado inconsistente.

## What Changes

- `POST /teams`: el grupo principal se crea **por default**. `create_default_group` pasa a ser "salteá la creación" — solo si se manda explícitamente `false` no se crea. Antes era al revés (opt-in, default `false`).
- `POST /teams/:id/users` (alta directa): ahora también da de alta en `group_users`, en el grupo principal del equipo — mismo criterio no bloqueante que `AcceptInvitation` (si falla o no hay grupo principal, el alta al equipo igual se completa).

## Capabilities

### Modified Capabilities
- `team-management`: cambia el comportamiento default de `create_default_group` (no el contrato — el campo sigue existiendo, mismo tipo).
- `team-user-management`: `AddUser` gana efecto secundario de asignación a grupo.

## Impact

- **Modificado**: `delegates/team_delegate.go` (lógica de creación de grupo default invertida), `services/team_user_service.go` (`AddUser` + `assignToMainGroup`), `app/app.go` (`teamUserService` gana `groupDao`/`groupUserDao`), `domains/team/team_request.go` (doc del campo actualizada).
- **Sin cambios de schema**: no hay columnas nuevas, es lógica de negocio.
- **Frontend**: **cambio de comportamiento a comunicar**. El frontend que ya manda `create_default_group: true` sigue funcionando igual (sin cambios). El frontend que omite el campo (o algún flujo que no lo mande) ahora SÍ obtiene el grupo default — antes no lo obtenía. Si algún flujo del frontend dependía deliberadamente de "no crear grupo" sin mandar `false` explícito, hay que revisarlo.

### Alcance

- Invariante: todo equipo activo tiene al menos un grupo principal, salvo que se pida explícitamente lo contrario.
- Alta directa de usuario a equipo también asigna grupo.

### No alcance

- Backfill de equipos ya creados sin grupo (deuda conocida, decisión explícita del usuario 2026-07-31 — se resuelve si/cuando haga falta).
- Impedir borrar el último grupo principal de un equipo (los endpoints de grupo no cambiaron) — fuera de alcance.

### Métrica de éxito

- Un equipo nuevo, creado sin especificar `create_default_group`, tiene un grupo principal.
- Un usuario agregado directamente a un equipo (sin invitación) queda en el grupo principal, verificable en `GET /groups/:id/users`.
