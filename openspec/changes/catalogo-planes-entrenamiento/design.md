## D1 — Modelo de datos

### `exercises`

| Campo | Tipo Go/GORM | Notas |
|---|---|---|
| `ID` | `int64` PK | |
| `OwnerID` | `int64` not null | FK lógica a `users.id` |
| `Name` | `string` not null | |
| `Description` | `*string` | opcional |
| `Kind` | `string` not null | enum app-level, ver D2 |
| `Intensity` | `*string` | enum app-level, ver D2 |
| `Minutes` | `*int` | |
| `DistanceM` | `*int` | |
| `SpeedKph` | `*float64` | `gorm:"type:numeric(4,1)"` |
| `MuscleGroup` | `*string` | enum app-level, ver D2 |
| `VideoURL` | `*string` | siempre `nil` al crear/editar, el endpoint no lo acepta en el body — reservado, spec frontend §6 |
| `DeletedAt` | `*time.Time` | soft-delete, decisión de este change (spec frontend §5 lo deja abierto) |
| `CreatedAt`/`UpdatedAt` | `time.Time` | |

**Sin CHECK constraints acoplando `Kind` a los demás campos** — la spec frontend §3.1 es explícita: cualquier combinación es válida, el backend no debe inventar reglas de negocio no pedidas.

### `sessions`

| Campo | Tipo | Notas |
|---|---|---|
| `ID` | `int64` PK | |
| `OwnerID` | `int64` not null | |
| `Name` | `string` not null | |
| `Description` | `*string` | |
| `DeletedAt` | `*time.Time` | soft-delete |
| `CreatedAt`/`UpdatedAt` | `time.Time` | |

### `session_exercises`

| Campo | Tipo | Notas |
|---|---|---|
| `ID` | `int64` PK | |
| `SessionID` | `int64` not null | borrado físico en cascada manual desde el service cuando se borra la `Session` (sin `ON DELETE CASCADE` a nivel DB — este repo no usa FKs físicas, ver `team_dao.go`/`join_request_dao.go` como precedente) |
| `ExerciseID` | `int64` not null | |
| `Role` | `string` not null | enum app-level `warmup`/`main`/`cooldown` |
| `RepeatCount` | `int` not null default 1 | |
| `RestMinutes` | `int` not null default 0 | |

Sin campo de orden explícito — el `id` autoincremental desempata el orden de inserción (spec frontend §3.3).

### `training_plans`

| Campo | Tipo | Notas |
|---|---|---|
| `ID` | `int64` PK | |
| `OwnerID` | `int64` not null | |
| `Name` | `string` not null | |
| `Description` | `*string` | |
| `CreatedAt`/`UpdatedAt` | `time.Time` | |

Sin `deleted_at` — un `TrainingPlan` no tiene borrado lógico, se borra físico (spec frontend §4: al borrar, los calendarios que lo estamparon en el pasado solo pierden la referencia informativa `source_plan_id`, no dependen de que el plan siga existiendo).

### `plan_days`

| Campo | Tipo | Notas |
|---|---|---|
| `ID` | `int64` PK | fila propia, no PK compuesta — más simple para el `PUT` de reemplazo total |
| `PlanID` | `int64` not null | |
| `SequenceNo` | `int` not null | `1..N` |
| `Kind` | `string` not null | enum app-level `rest`/`other`/`training` |
| `OtherName` | `*string` | solo si `kind=other` |
| `SessionID` | `*int64` | solo si `kind=training` |
| `DefaultPresencial` | `bool` not null default false | |
| `DefaultTime` | `*time.Time` | `gorm:"type:time"` — solo hora, sin fecha real (ver D3) |
| `DefaultLocation` | `*string` | `gorm:"type:jsonb"`, JSON de `{lat,lng,label?}` (ver D4) |

## D2 — Enums app-level (constants package, sin enum nativo de Postgres)

Mismo patrón que `TeamUserRole`/`InvitationStatus` (`cmd/api/domains/constants/`): tipo `string` + const + `GetValid...()` + `IsValid...()`.

- `ExerciseKind`: `walking`, `jogging`, `elongation`, `cruising`, `running`.
- `ExerciseIntensity`: `light`, `moderate`, `vigorous`.
- `MuscleGroup`: `cuadriceps`, `isquiotibiales`, `gemelos`, `gluteos`, `aductores`, `psoas`, `lumbares`, `core`.
- `SessionExerciseRole`: `warmup`, `main`, `cooldown`.
- `PlanDayKind`: `rest`, `other`, `training`.

## D3 — Campo `time` (solo hora, sin fecha)

No hay precedente en el repo de una columna Postgres `time` (todos los `time.Time` existentes son `timestamptz`). Se mapea `*time.Time` con `gorm:"type:time"` — el driver Postgres (`lib/pq`/`pgx` vía GORM) devuelve/acepta `time.Time` con fecha `0000-01-01` para columnas `time`, se ignora esa parte al leer/escribir (`.Format("15:04")` al serializar al frontend). Ida y vuelta simple: el frontend manda `"HH:MM"`, el binding de Gin lo parsea a `time.Time` con `time.Parse("15:04", ...)` antes de persistir.

## D4 — Shape de ubicación (`jsonb`)

Mismo patrón que `platform_settings.value` (`*string` con `type:jsonb`, serializado a mano). Domain struct:

```go
type Location struct {
    Lat   float64 `json:"lat"`
    Lng   float64 `json:"lng"`
    Label *string `json:"label,omitempty"`
}
```

El service serializa a JSON antes de guardar en `PlanDay.DefaultLocation` (`*string`), y deserializa al armar la respuesta. Nulo si `DefaultPresencial=false`.

## D5 — Validaciones de negocio (server-side, nunca confiar en el frontend)

**`Exercise`**: `owner_id`/`name`/`kind` obligatorios; `kind` debe estar en `GetValidExerciseKinds()`; `intensity`/`muscle_group` si vienen deben ser válidos: sin más reglas cruzadas (D1).

**`Session`**: `owner_id`/`name` obligatorios; `exercises` debe tener ≥1 ítem de cada uno de los 3 roles (`warmup`/`main`/`cooldown`) — validar contando por rol antes de persistir, sea `POST` o `PUT`. Cada `exercise_id` referenciado debe existir (`404`-equivalente en un `422` general del batch, no hace falta identificar cuál).

**`TrainingPlan`**: `owner_id`/`name` obligatorios; `days` entre 2 y 31 filas; `sequence_no` cubre `1..N` sin huecos ni repetidos (`N` = `len(days)`); por cada día: `kind=training` ⇒ `session_id` no nulo y `other_name` nulo; `kind=other` ⇒ `other_name` no nulo y `session_id` nulo; `kind=rest` ⇒ ambos nulos. `default_time`/`default_location` solo tienen sentido si `default_presencial=true` — si viene `false`, limpiar ambos a `nil` (mismo criterio de limpieza que la spec de calendario aplica a `is_presencial`, D5 de ese change).

## D6 — Endpoints y códigos de error

Convención de respuesta de error de este dominio (**distinta** de la convención SCREAMING_SNAKE del resto del backend): la spec de frontend (`BACKEND_TRAINING_PLANS_SPEC.md` §2) pide explícitamente `{"message": "..."}` + status code semántico, sin campo `code`. Se respeta tal cual está pedido — es un dominio nuevo, consumido por un frontend que ya espera ese shape exacto, no el `apierror.APIError` de SCREAMING_SNAKE del resto del backend.

```go
// respondCatalogError - helper compartido por los 3 controllers de este change
func respondCatalogError(c *gin.Context, status int, message string) {
    c.JSON(status, gin.H{"message": message})
}
```

| Método | Path | Body | Respuesta |
|---|---|---|---|
| `GET` | `/exercises?owner_id={id}` | — | `200` array `Exercise` |
| `GET` | `/exercises/{id}` | — | `200` `Exercise` \| `404` |
| `POST` | `/exercises` | `{owner_id,name,description?,kind,intensity?,minutes?,distance_m?,speed_kph?,muscle_group?}` | `201` \| `400` |
| `PUT` | `/exercises/{id}` | igual shape | `200` \| `404` \| `400` |
| `DELETE` | `/exercises/{id}` | — | `204` \| `404` — soft-delete |
| `POST` | `/exercises/{id}/clone` | — | `201` nuevo `Exercise`, sufijo `" (copia)"` |
| `GET` | `/sessions?owner_id={id}` | — | `200` array `Session` con `exercises` embebido |
| `GET` | `/sessions/{id}` | — | `200` \| `404` |
| `POST` | `/sessions` | `{owner_id,name,description?,exercises:[...]}` | `201` \| `400`/`422` (regla de roles) |
| `PUT` | `/sessions/{id}` | igual shape | `200` \| `404` \| `422` — reemplaza `exercises` entero |
| `DELETE` | `/sessions/{id}` | — | `204` \| `404` — soft-delete |
| `POST` | `/sessions/{id}/clone` | — | `201` nueva `Session`, copia profunda de `exercises` |
| `GET` | `/training-plans?owner_id={id}` | — | `200` array `TrainingPlan` con `days` embebido |
| `GET` | `/training-plans/{id}` | — | `200` \| `404` |
| `POST` | `/training-plans` | `{owner_id,name,description?,days:[...]}` | `201` \| `422` (D5) |
| `PUT` | `/training-plans/{id}` | parcial, `days` si viene debe ser el set completo válido | `200` \| `404` \| `422` |
| `DELETE` | `/training-plans/{id}` | — | `204` \| `404` — físico (D1) |
| `POST` | `/training-plans/{id}/clone` | — | `201` nuevo plan, copia profunda de `days` |

Nota sobre `PUT /sessions/{id}` y `exclude_group_ids`/`clone_name`/`clone_description`: esos 3 campos adicionales del body pertenecen al **segundo change** (`calendario-asignacion-grupos`, spec frontend de calendario §5) — este change implementa el `PUT` simple (reemplazo total de `exercises`), el segundo change lo extiende sin tocar la firma base.

## D7 — Capas y archivos (sin delegate)

Mismo criterio ya aplicado en `busqueda-equipos-solicitudes-ingreso`: delegate solo si una operación compone lógica de negocio de 2+ *services*. Acá cada entidad es autocontenida (su propio service con sus propias DAOs) — ni siquiera `Session`/`TrainingPlan` necesitan el service de la otra entidad, solo su DAO directo (para validar que un `exercise_id`/`session_id` referenciado existe), igual que `invitation_service.go` toca `teamDao`/`groupDao` directo sin pasar por sus services.

- `cmd/api/domains/dbs/{exercise,session,session_exercise,training_plan,plan_day}.go`
- `cmd/api/domains/constants/{exercise_kind,exercise_intensity,muscle_group,session_exercise_role,plan_day_kind}.go`
- `cmd/api/domains/exercise/{exercise_request.go,exercise_response.go}`
- `cmd/api/domains/session/{session_request.go,session_response.go}`
- `cmd/api/domains/trainingplan/{training_plan_request.go,training_plan_response.go,location.go}` (D4, `Location` compartido con el próximo change)
- `cmd/api/daos/{exercise,session,session_exercise,training_plan,plan_day}_dao.go`
- `cmd/api/services/{exercise,session,training_plan}_service.go`
- `cmd/api/controllers/{exercise,session,training_plan}_controller.go`
- `cmd/api/app/app.go` (wiring) + `cmd/api/app/url_mappings.go` (rutas)
- `cmd/api/infrastructure/postgresdb/postgres.go` (AutoMigrate)

## D9 — Autorización por dueño (adición de esta implementación, no está en la spec de frontend)

La spec de frontend no menciona un chequeo de propiedad — solo pide `Authorization: Bearer` genérico. Sin un chequeo adicional, cualquier usuario autenticado podría editar/borrar/clonar el catálogo de **otro** entrenador con solo conocer el `id`, o crear un recurso con un `owner_id` ajeno. Mismo tipo de gap que se encontró y corrigió hoy en `team_dao.SearchPublic`/`join_request_service.Create` (ver rama `fix/busqueda-equipos-owner-y-path-traversal`, ya mergeada a `develop`) — se agrega acá por el mismo criterio, sin esperar a que aparezca como bug:

- **`Create`**: `req.OwnerID` debe ser igual al usuario autenticado (`utils.GetAuthUserID(c)`) — `403` si no.
- **`Update`/`Delete`/`Clone`**: el `OwnerID` del recurso existente debe ser igual al usuario autenticado — `403` si no.
- **`Get`/`List`**: sin chequeo — la spec no pide que el catálogo sea privado entre entrenadores (a diferencia de equipos, acá no hay noción de "descubrir" catálogo ajeno, pero tampoco se pidió bloquear la lectura; se deja abierta como está especificado, `owner_id` es un filtro, no una ACL).

Nuevo sentinel compartido por los 3 services: `ErrCatalogForbidden = errors.New("no autorizado")`, mapeado a `403` en cada controller.

## D8 — Clone (las 3 entidades)

`POST /{entity}/{id}/clone`, sin body. Mismo patrón en los 3: leer original (404 si no existe o `deleted_at` no nulo), copiar todos los campos salvo `ID`/`CreatedAt`/`UpdatedAt`, `Name = original.Name + " (copia)"`, insertar. `Session.clone` copia profunda de `session_exercises` (nuevas filas apuntando al `exercise_id` original, no clona los ejercicios). `TrainingPlan.clone` copia profunda de `plan_days` (mismo `session_id` que el original, no clona sesiones).
