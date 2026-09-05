# Design: Búsqueda de equipos y solicitudes de ingreso

## Context

Hoy la única vía para que un corredor entre a un equipo es la invitación: el entrenador lo busca (`GET /users/search`, ya existe) y le manda una invitación que el corredor acepta o rechaza. No hay forma de que el corredor descubra equipos por su cuenta.

Esta feature estuvo pausada (ver memoria de sesión, brainstorming 2026-09-03/04) porque el diseño necesitaba el mismo punto de extensión — creación de `team_user` — que `feature/suscripciones-tier-equipos` estaba tocando en simultáneo para meter el gate de pago por membresía. Esperar a que esa rama mergeara evitó un refactor-y-re-refactor y cualquier colisión de merge con un colaborador que no maneja git con fluidez.

Con `feature/suscripciones-tier-equipos` ya mergeado (PR #39, `6055b36`), `ApplyTeamMembershipGate` (`cmd/api/services/team_membership_gate.go`) es hoy el único punto donde se crea un `team_user`, consumido por `team_user_service.AddUser` e `invitation_service.AcceptInvitation`. Este change agrega un tercer consumidor.

## Goal

- Permitir a un corredor buscar equipos públicos (por nombre/nivel/ubicación) y pedir unirse, sin depender de que el entrenador lo invite primero.
- Que el entrenador vea y resuelva esas solicitudes desde el equipo, con un badge agregado de pendientes.
- Que aceptar una solicitud cree la membresía exactamente con las mismas reglas de pago/grupo que ya rigen para invitaciones y alta directa — sin lógica de negocio duplicada ni divergente.

## Non-Goals

Ver `proposal.md` — condición de carrera de cupo (se deja como está, a resolver en change aparte), elección de grupo al pedir unirse, `total` en paginación, resolución del badge client-side.

## Decisions

### D1. Modelo de datos

`teams` gana dos columnas booleanas, ambas `not null default true`:

| columna | notas |
|---|---|
| `visible` | si `false`, el equipo nunca aparece en `GET /teams/search`, sin importar el resto de filtros |
| `is_public` | si `false`, el equipo puede aparecer en la búsqueda (si `visible`) pero no acepta solicitudes — solo entra por invitación |

Default `true` en ambas: el proyecto sigue en testing/pre-lanzamiento, así que los equipos que ya existen quedan buscables/solicitables sin que el entrenador tenga que prender el flag a mano. Es una decisión reversible por equipo vía el `PUT /api/v1/teams/:id` ya existente (mismo patrón puntero-opcional que `ShowGroupsToRunners`).

`join_requests` es tabla nueva:

```go
type JoinRequest struct {
    ID        int64     `gorm:"column:id;primaryKey"`
    TeamID    int64     `gorm:"column:team_id;not null"`
    RunnerID  int64     `gorm:"column:runner_id;not null"`
    Status    string    `gorm:"column:status;not null;default:pending"`
    CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
    UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}
```

`Status` reusa los valores de `constants.InvitationStatus` (`pending`/`accepted`/`rejected`) — son exactamente los mismos 3 estados, no se crea un enum nuevo para lo mismo.

### D2. Búsqueda: paginación por página, sin `total`

No hay precedente de paginación en el repo (confirmado, cero uso de `limit`/`offset`/`page` en controllers/domains existentes salvo el dummy `exampleweather`). El frontend ya diseñó "Cargar más" como acción explícita del usuario que concatena resultados client-side (no `useInfiniteQuery`, no scroll infinito automático).

`GET /api/v1/teams/search?name=&level=&country=&province=&city=&page=1`:
- `page` 1-indexado, tamaño de página fijo `20` (constante, no configurable por query param).
- Respuesta: `{ teams: [...], has_more: bool }`. Se evita `total` — requeriría un `COUNT(*)` adicional por request solo para que el frontend sepa si mostrar el botón "Cargar más", cuando alcanza con pedir `page_size + 1` filas y devolver `has_more = len(rows) > page_size` (cortando la fila extra antes de responder).
- Filtra `visible = true` siempre; excluye equipos donde el caller ya es miembro (`team_users` activo); ignora equipos con `deleted_at` seteado (soft-delete existente).
- Cada resultado trae equipo + `owner_name` (join a `users` por `owner_id`) + conteo de miembros actuales (para el cupo mostrado en la card).

### D3. `join_requests`: ciclo de vida y autorización

| Endpoint | Quién | Efecto |
|---|---|---|
| `POST /api/v1/teams/:id/join-requests` | corredor autenticado | crea `pending`, valida `is_public`, cupo, no-ya-miembro, no-duplicado |
| `DELETE /api/v1/join-requests/:id` | dueño de la solicitud | cancela, solo si sigue `pending` |
| `GET /api/v1/join-requests/mine` | corredor autenticado | sus solicitudes, cualquier estado |
| `GET /api/v1/teams/:id/join-requests` | entrenador dueño del equipo | solicitudes `pending` de ese equipo |
| `GET /api/v1/join-requests/pending-count` | entrenador autenticado | conteo agregado de `pending` en todos sus equipos, para el badge |
| `POST /api/v1/join-requests/:id/accept` | entrenador dueño del equipo | ver D4 |
| `POST /api/v1/join-requests/:id/reject` | entrenador dueño del equipo | marca `rejected` |

### D4. `Accept`: reuso del gate de membresía, mismo patrón secuencial que `AcceptInvitation`

`Accept` replica exactamente la estructura de `invitation_service.AcceptInvitation` (no una transacción nueva): busca si el corredor ya es miembro (`teamUserDao.FindByTeamAndUser`); si no lo es, valida cupo (check-then-act, ver D6 — condición de carrera conocida y aceptada, no se resuelve acá), arma `dbs.TeamUser{TeamID, UserID: request.RunnerID, RoleInTeam: "corredor"}` y llama `ApplyTeamMembershipGate(ctx, s.db, s.teamUserDao, s.installDao, teamUser, teamDB.MembershipFee)` — el gate abre su propia transacción internamente cuando `s.db` no es nil, igual que ya hace para `AddUser`/`AcceptInvitation`, sin que `join_request_service` necesite envolver nada por fuera. Si el corredor ya era miembro (reintento tras un fallo parcial previo), se saltea directamente a los pasos siguientes sin volver a crear nada — mismo guard `if existingMember == nil` que ya usa `AcceptInvitation`.

Después, sin importar si vino del branch de alta o del de "ya era miembro", se llama `AssignToDefaultGroup(ctx, groupDao, groupUserDao, teamDB.ID, nil, jr.RunnerID)` (D5, siempre `groupID: nil`, best-effort, no bloquea) y se actualiza `join_request.Status = "accepted"` como paso final independiente. Igual que en `AcceptInvitation`, esto **no es atómico de punta a punta**: si `UpdateStatus` falla después de que el gate ya creó el `team_user`, la solicitud queda `pending` con la membresía ya creada — un reintento de `Accept` lo detecta vía el guard `existingMember != nil` y solo corrige el estado de la solicitud, sin duplicar nada. Mismo trade-off ya aceptado y documentado en el comentario de `AcceptInvitation` (`invitation_service.go:356-361`), no uno nuevo introducido por este change.

### D5. Asignación a grupo default: extracción de `AssignToDefaultGroup`

`invitation_service.go:474` (`assignInviteeToGroup`) ya resuelve "caer al grupo `IsMain` del equipo si no se especificó uno", pero es un método no exportado de `invitationService`, atado a `*dbs.Invitation`. Join-request necesita la misma lógica — el corredor nunca elige grupo al pedir unirse — pero desde otro service.

Mismo criterio que ya se aplicó para extraer `ApplyTeamMembershipGate` cuando dos call sites necesitaron la misma lógica de creación de membresía: se extrae a una función package-level en `services/team_group_assignment.go`:

```go
func AssignToDefaultGroup(
    ctx *gin.Context,
    groupDao daos.GroupDaoInterface,
    groupUserDao daos.GroupUserDaoInterface,
    teamID int64,
    groupID *int64,
    userID int64,
)
```

Mismo comportamiento best-effort que hoy (loguea y retorna sin bloquear al caller si falla — no tiene sentido revertir la membresía ya creada por un fallo de asignación a grupo, que se puede resolver a mano después). `invitation_service.go` pasa a llamar esta función en vez de tener su propio método (`inv.GroupID` se le pasa como `groupID`); sin cambio de comportamiento observable en `AcceptInvitation`. `join_request_service.Accept` la llama con `groupID: nil` siempre (el corredor nunca eligió uno).

### D6. Condición de carrera de cupo — mismo patrón existente, no se resuelve acá

`AddUser` y `AcceptInvitation` ya validan `MaxMembers` con un `count` seguido de un `insert` sin transacción que cubra ambos pasos — hay una ventana de carrera si dos altas concurrentes pasan el chequeo antes de que cualquiera inserte. `Accept` de join-request hereda el mismo patrón por consistencia.

Se evaluaron alternativas (`SELECT ... FOR UPDATE`, constraint a nivel DB) durante el brainstorming y se descartaron por ahora: parchear un solo call site sería inconsistente con los otros dos, y el impacto de colarse un miembro de más es autocorregible (el entrenador lo saca del roster) — no hay corrupción de datos ni de dinero en juego. Queda anotado como mejora futura a resolver en los 3 call sites (`AddUser`, `AcceptInvitation`, `Accept`) en el mismo change, no en este.

### D7. Códigos de error

| Endpoint | Código | HTTP |
|---|---|---|
| `POST .../join-requests` | `TEAM_NOT_FOUND` | 404 |
| | `TEAM_NOT_PUBLIC` | 403 |
| | `TEAM_FULL` | 409 |
| | `ALREADY_MEMBER` | 409 |
| | `JOIN_REQUEST_ALREADY_PENDING` | 409 |
| `DELETE /join-requests/:id` | `JOIN_REQUEST_NOT_FOUND` | 404 |
| | `FORBIDDEN` | 403 |
| | `JOIN_REQUEST_NOT_PENDING` | 409 |
| `POST .../accept` \| `.../reject` | `JOIN_REQUEST_NOT_FOUND` | 404 |
| | `FORBIDDEN` | 403 |
| | `JOIN_REQUEST_NOT_PENDING` | 409 |
| | `TEAM_FULL` (solo accept) | 409 |
| `GET /teams/search` | `INVALID_QUERY` (`page < 1`) | 400 |
| `GET .../join-requests` \| `pending-count` | `FORBIDDEN` | 403 |

Se elige la convención de códigos específicos en SCREAMING_SNAKE (la usada por la feature de fotos, `PHOTO_TOO_LARGE`/`STORAGE_UNAVAILABLE`) en vez de la convención genérica más vieja (`"Not Found"`/`"Conflict"`/`"Forbidden"` de `team_user_controller.go`) — el frontend necesita distinguir los 4 estados del botón "Solicitar unirse" (habilitado/cupo lleno/no público/ya enviada), y un código genérico no alcanza para eso.

### D8. Capas y archivos

Sigue `Controllers → Delegates → Services → DAOs` (`.agentics/CONVENTIONS.md`):

- **DAOs**: `daos/join_request_dao.go` (nuevo) — `Create`, `FindByID`, `FindPendingByTeamAndUser`, `FindPendingByTeam` (sin paginar — la lista de pendientes de un equipo no la pide paginada el frontend, a diferencia de la búsqueda), `FindByUser`, `UpdateStatus`, `Delete` (usado por `Cancel` — no hay un 4to valor `cancelled` en `constants.InvitationStatus`, D1 reusa deliberadamente solo `pending`/`accepted`/`rejected`, así que cancelar borra la fila en vez de cambiarle el estado), `CountPendingByOwner`. `daos/team_dao.go` extendido con `SearchPublic(filters, page, pageSize)`.
- **Services**: `services/join_request_service.go` (nuevo) — `Create`, `Cancel`, `Accept`, `Reject`, `ListMine`, `ListByTeam`, `PendingCount`. `services/team_service.go` extendido con `Search`. `services/team_group_assignment.go` (nuevo, D5).
- **Delegates**: `delegates/join_request_delegate.go` (nuevo), `delegates/team_delegate.go` extendido si el patrón de delegate-por-recurso lo pide para `Search`.
- **Controllers**: `controllers/join_request_controller.go` (nuevo), `controllers/team_controller.go` extendido (`Search`).
- **Rutas** (`cmd/api/app/url_mappings.go`): las 8 de D3 + `GET /api/v1/teams/search`, todas detrás de `AuthMiddleware()`.

## Risks / Trade-offs

- **Condición de carrera de cupo** (D6): aceptada, impacto autocorregible, anotada como deuda a resolver en los 3 call sites juntos.
- **`has_more` sin `total`** (D2): el frontend no puede mostrar "página 3 de 10", solo "hay más" — aceptado porque el diseño de UI ya es "Cargar más", no un paginador numerado.
- **Objetos huérfanos de `join_requests` rechazadas**: no se borran, quedan en `rejected` indefinidamente (igual que `invitations` hoy) — sin impacto de storage real, son filas chicas.

## Follow-up

- Unificar el fix de la condición de carrera de cupo en `AddUser`, `AcceptInvitation` y `Accept` (D6) en un change dedicado.
- Si en algún momento el corredor necesita elegir grupo al pedir unirse (hoy explícitamente fuera de alcance), `AssignToDefaultGroup` ya soporta `groupID` explícito — extender el `POST .../join-requests` con un campo opcional sería un cambio acotado.
