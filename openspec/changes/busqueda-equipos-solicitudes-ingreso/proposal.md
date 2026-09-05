## Why

Hoy un corredor solo puede sumarse a un equipo si el entrenador lo invita (buscándolo por `GET /users/search`). No hay forma de que un corredor descubra equipos públicos y pida unirse por su cuenta. El frontend ya tiene esta pantalla diseñada contra mocks (`paceron-frontend/docs/superpowers/specs/2026-09-03-team-search-join-requests-design.md`), pendiente de contrato real de backend.

Esto quedó deliberadamente pausado hasta que `feature/suscripciones-tier-equipos` (pagos por equipo) mergeara a `develop`, porque esa rama extraía `ApplyTeamMembershipGate` — el único punto donde se crea una membresía de equipo, ahora gateada por `membership_fee`. Ya mergeó (PR #39, commit `6055b36`) y el helper quedó consolidado como el único call site usado por `team_user_service.AddUser` e `invitation_service.AcceptInvitation` — terreno limpio para que join-requests lo use desde el día uno, sin refactor previo.

## What Changes

- **Columnas nuevas en `teams`**: `visible` (bool, default `true`) — aparece o no en resultados de búsqueda; `is_public` (bool, default `true`) — habilita o no el botón "Solicitar unirse" (un equipo puede ser visible pero no público: se ve en la búsqueda, pero solo se entra por invitación). Default `true` en ambas: el proyecto sigue en etapa de testing/pre-lanzamiento, conviene que los equipos ya cargados aparezcan buscables sin que el entrenador tenga que activarlo a mano equipo por equipo.
- **Nuevo endpoint de búsqueda** `GET /api/v1/teams/search`: filtros opcionales `name`, `level`, `country`, `province`, `city`; solo equipos `visible = true`; excluye equipos donde el caller ya es miembro; paginación por `page` (1-indexado, tamaño fijo 20, sin precedente de paginación en el repo hoy), respuesta con `has_more` (no `total`, evita un `COUNT(*)` extra).
- **Nueva entidad `join_requests`**: solicitud de un corredor a un equipo (`pending`/`accepted`/`rejected`, reusa `constants.InvitationStatus` — mismos 3 valores que ya existen para invitaciones). A diferencia de la invitación, el corredor no elige grupo: al aceptar, siempre cae al grupo default del equipo (`IsMain`).
- **Endpoints de solicitudes**: crear (`POST /teams/:id/join-requests`), cancelar (`DELETE /join-requests/:id`), listar propias (`GET /join-requests/mine`), listar por equipo (`GET /teams/:id/join-requests`, dueño), aceptar/rechazar (`POST /join-requests/:id/accept|reject`, dueño), y conteo agregado (`GET /join-requests/pending-count`, dueño, para el badge del entrenador — simétrico al patrón ya existente de `myInvitationsCount`).
- **`Accept` reusa `ApplyTeamMembershipGate`**: misma gate de pago por membresía que ya usan `AddUser`/`AcceptInvitation`, llamada con la misma estructura secuencial que `AcceptInvitation` (guard `existingMember == nil`, gate, asignación a grupo, y por último el cambio de estado del `join_request` a `accepted` como paso independiente) — sin transacción propia nueva, mismo trade-off de no-atomicidad de punta a punta ya aceptado y documentado en `AcceptInvitation`.
- **Refactor targeted**: se extrae `assignInviteeToGroup` (hoy método no exportado de `invitationService`, atado a `*dbs.Invitation`) a una función package-level `AssignToDefaultGroup` — mismo criterio que ya se usó para extraer `ApplyTeamMembershipGate`, un único lugar para la lógica de "caer al grupo default" consumido por `AcceptInvitation` y por el nuevo `Accept` de join-request.

## Capabilities

### New Capabilities

- `team-search`: búsqueda paginada de equipos visibles, con filtros de nombre/nivel/ubicación, excluyendo equipos donde el caller ya es miembro.
- `team-join-requests`: ciclo de vida completo de una solicitud de ingreso de un corredor a un equipo público (crear, cancelar, listar, aceptar, rechazar, contar pendientes), incluyendo el gate de pago por membresía y la asignación al grupo default.

### Modified Capabilities

- Ninguna formalmente en OpenSpec, pero `AcceptInvitation` cambia internamente: pasa a llamar `AssignToDefaultGroup` en vez de tener su propia copia de la lógica (sin cambio de comportamiento observable).

## Non-Goals

- **Arreglar la condición de carrera de cupo** (`MaxMembers` chequeado sin lock, check-then-act) de forma unificada en `AddUser`/`AcceptInvitation`/`Accept` de join-request: se identificó durante el diseño, pero arreglarla en un solo call site sería inconsistente y en los 3 sería una feature aparte. Este change mantiene el mismo patrón ya existente en el repo (ver memoria de sesión, a resolver más adelante en un change dedicado que toque los 3 a la vez).
- **Elegir grupo al pedir unirse**: a diferencia de la invitación, el corredor no conoce la estructura interna de grupos del equipo — siempre cae al grupo default. El entrenador puede reasignarlo después con la acción "mover" que ya existe en el roster.
- **`total` en la paginación de búsqueda**: alcanza con `has_more` para el patrón "Cargar más" del frontend, sin el costo de un `COUNT(*)` adicional por request.
- **Resolver el badge de pendientes con N requests client-side**: se decidió el endpoint dedicado `GET /join-requests/pending-count` desde el arranque, no como iteración futura.

## Impact

- **Schema/DB**: `teams.visible` (bool, default `true`), `teams.is_public` (bool, default `true`); tabla nueva `join_requests` (`team_id`, `runner_id`, `status`, timestamps). Aditivo, sin romper nada existente. AutoMigrate en `cmd/api/infrastructure/postgresdb/postgres.go`.
- **Modelos**: `cmd/api/domains/dbs/team.go` (+2 campos), nuevo `cmd/api/domains/dbs/join_request.go`.
- **Dominios/DTOs**: `domains/team/team_update_request.go`/`team_response.go` (+`Visible`/`IsPublic`), nuevo `domains/joinrequest/` (request/response), nuevo `domains/team/team_search_request.go` (o equivalente).
- **DAOs**: `daos/team_dao.go` (+`SearchPublic`), nuevo `daos/join_request_dao.go`.
- **Servicios**: `services/team_service.go` (+`Search`), nuevo `services/join_request_service.go`, nuevo `services/team_group_assignment.go` (extracción de `AssignToDefaultGroup`), `services/invitation_service.go` (ajustado para usar la función extraída en vez de su método propio).
- **Controllers/rutas**: nuevo `controllers/join_request_controller.go` (sin delegate, llama al service directo — igual que `invitation_controller.go`, ninguna operación de esta feature compone dos services entre sí); `controllers/team_controller.go` extendido (`Search`, sin delegate); 8 rutas nuevas en `cmd/api/app/url_mappings.go`.
- **Swagger**: regenerar `cmd/api/docs` con los endpoints nuevos.
- **Tests**: DAOs con Postgres real (`testutils.SetupTestDB`); services/controllers con mocks, siguiendo el molde de `team_user_service_test.go`/`team_user_controller_test.go`; caso de condición de carrera de cupo documentado como comportamiento conocido (no bloqueante), no como bug a corregir en este change.
