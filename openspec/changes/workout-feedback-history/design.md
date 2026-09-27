# Design: workout-feedback-history

## D1 — Derivación de grupo y nombres (joins en DAO)

`workout_feedback` no tiene `group_id`: se deriva con

```
LEFT JOIN group_calendar_day gcd ON gcd.session_instance_id = wf.assigned_session_id
LEFT JOIN exercise_instances  ei  ON ei.id  = wf.assigned_exercise_id
```

- `group_id = gcd.group_id` (nullable). NOTA: `assigned_session_id` guarda **session_instance id** (confirmado: `GetBySession` filtra por ese valor), y `session_instance_id` es la FK de `group_calendar_days` desde `asignacion-por-instanciacion` — el join es 1:1 por diseño (una instancia por día).
- `session_name` viene de `session_instances` (LEFT JOIN), `exercise_name` de `exercise_instances`, `catalog_exercise_id = ei.source_exercise_id` (nullable en instancias viejas).
- `athlete_name`/`team_name`/`group_name` NO van en el SQL del listado: se resuelven **en batch en el service** (mismo patrón que member-calendar: 1 query por entidad con `IN`). Motivo: sort whitelisted incluye `exercise_name`, que sí requiere estar en el SQL — para eso alcanza el join con `exercise_instances`; los demás nombres no se ordenan.

Los huérfanos quedan dentro: LEFT JOINs no filtran; los nulls comunican la razón (D-decisión del usuario).

## D2 — Fila enriquecida del DAO

Nueva struct `dbs.WorkoutFeedbackHistoryRow` (o en `domains/workoutfeedback`, DTO puro): campos del feedback que expone el ítem (ID, AthleteUserID, TeamID, GroupID, SessionDate, SetNumber, CompletionStatus, DurationMs, ActiveDurationMs, DistanceMeters, StartedAt, EndedAt) + `ExerciseID` (= assigned_exercise_id), `ExerciseName`, `CatalogExerciseID *int64`, `SessionName *string`. El DAO hace el SELECT con joins y devuelve filas crudas; el service agrega los nombres restantes y arma el response DTO (`domains/workoutfeedback`, paquete excluido del coverage — DTOs puros).

## D3 — Autorización (matriz)

| Endpoint | Guard |
|---|---|
| AthleteHistory | `targetID == callerID` sino `403` (sentinel propio, patrón `ErrCalendarForbidden` de member-calendar) |
| AdministeredHistory | `targetID == callerID` sino `403`; `team_id` requerido sino `400`; equipo no existe `404` (`ErrTeamNotFound`); caller no es owner `403` |

Reutiliza helpers existentes del DAO (`TeamExists`/`IsTeamOwner`).

## D4 — Validaciones de query (service, previas al DAO)

- `date_from`/`date_to`: o ambos o ninguno (400 si uno solo); `from > to` → 400; iguales = un día.
- `group_id` sin `team_id` → 400 (ambos endpoints).
- `page` ≥ 1, `page_size` ∈ [1,100] (defaults 1/20; fuera de rango → 400).
- `sort` fuera de whitelist → 400. `order` fuera de {asc,desc} → 400.

## D5 — DAO: 3 queries, filtros de primer y segundo nivel

Estructura de filtros `WorkoutFeedbackHistoryFilters`:

- **Primer nivel** (aplican a todo): `AthleteScope` (para corredor: `athlete_user_id = self`; para entrenador: `team_id = X`), `TeamID`, `GroupID`, `DateFrom`, `DateTo`.
- **Segundo nivel** (ítems y total, NO pools): `ExerciseInstanceID`, `SetNumber`, `AthleteFilterUserID` (solo endpoint entrenador).

Métodos nuevos en `WorkoutFeedbackDAO` interface:

1. `HistorySearch(ctx, filters, sortCol, order, limit, offset) ([]WorkoutFeedbackHistoryRow, error)` — WHERE `deleted_at IS NULL` + filtros de 1er y 2do nivel + joins (D1), ORDER BY whitelisted.
2. `HistoryCount(ctx, filters) (int64, error)` — COUNT con los mismos filtros (todos los niveles), sin joins que no afecten count (group/calendar join sí: un feedback puede matchear sin día; LEFT JOIN no multiplica filas porque `session_instance_id` es único por día, pero por seguridad DISTINCT COUNT del id).
3. `HistoryAvailableAthletes(ctx, firstLevelFilters) ([]IDName, error)` y `HistoryAvailableExercises(idem)` — DISTINCT con solo primer nivel; exercises via join `exercise_instances` (id = instancia, name = nombre de instancia); atletas via join `users`.

`sort` se mapea por whitelist a columnas SQL: `feedback_date → wf.session_date`, `set_number → wf.set_number`, `exercise_name → ei.name`. Desempate determinista: `, wf.id DESC/ASC` según order.

## D6 — Response DTO y pools

```
WorkoutFeedbackHistoryResponse {
  items []WorkoutFeedbackHistoryItem  // + AthleteName, TeamName, GroupName (+ CatalogExerciseID)
  total int64; page, page_size int
  available_athletes  []IDName  // {id, name}
  available_exercises []IDName
}
```

Ítem con `group_id`/`group_name`/`team_id`/`team_name` nullable (omitempty o punteros, según convención del repo — se define en implementación mirando DTOs existentes de workoutfeedback).

## D7 — Capas y rutas

- Rutas en `url_mappings.go`: `GET /api/v1/users/:id/workout-feedback-history` y `GET /api/v1/users/:id/administered-workout-feedback-history` (misma convención que member-calendar/administered-calendar).
- Controller `WorkoutFeedbackController` suma 2 handlers con parsing de query (patrón `parseSearchFilters`/member-calendar) + anotaciones Swagger + regeneración (`swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal`).
- Wiring en `app.go` (los DAOs necesarios ya existen en el service de feedback).

## D8 — Testing

- **DAO (Postgres real, `testutils.SetupTestDB`)**: fixture con 2 equipos/grupos + día con instancia + feedbacks (normal, huérfano sin día, sin team); asserts de filtros (team/group/rango/ejercicio/set), sort whitelist + order + desempate por id, paginación+offset, count, pools con/segundo nivel ignorado, `deleted_at` excluido.
- **Service**: matriz de autorización (403 self, 403 owner, 404 team, 400 team requerido, 400 group sin team, 400 fechas, 400 page/sort), armado de response con nombres batch, pools.
- **Controller (mock)**: parsing de query, 200 con JSON esperado, códigos de error mapeados, Swagger regenerado.
- Meta: mantener analyzer ≥85% (gate actual). Verificación SIEMPRE con `go clean -cache` + analyzer (bug cache -coverpkg conocido).

## D9 — No-goals

- Sumarización server-side (agregados por ejercicio/período).
- Deprecar/modificar `GET /workout-feedback/search`.
- Migración de DB: ninguna (solo lecturas; `assigned_session_id`/`assigned_exercise_id`/`team_id` ya persisten lo necesario).
- Filtro por `catalog_exercise_id` (se expone para agrupar del lado frontend; filtrar por familia queda para cuando lo pida la UI).
