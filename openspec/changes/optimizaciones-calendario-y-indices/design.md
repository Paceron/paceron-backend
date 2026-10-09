# Design: optimizaciones-calendario-y-indices

## Contexto

Constraint del usuario (2026-10-09): la performance y la optimización deben mejorar, **pero no se
pueden cambiar endpoints, contratos de respuesta ni comportamiento observable**. Todo lo que sigue
es invisible para el frontend.

## D1 — Builder batch de respuestas de calendario

Hoy: `toCalendarDayResponse` (calendar_service.go:877) llama `sessionInstanceResponse` (:555) que hace
**3 queries por día**: `SessionInstanceDao.FindByID` + `SessionExerciseInstanceDao.FindBySessionInstance`
+ `ExerciseInstanceDao.FindByIDs`. `toCalendarDayResponse` corre en loop en:

- GetRange (:691), Stamp (respuestas :1109-1115, post-commit), Bulk (respuestas loop),
  Shift (respuestas loop), BulkClear (respuestas loop), MemberCalendar (:1761),
  AdministeredCalendar (:1906).
- NO se tocan: UpsertDay (:734/:812, fila única, dentro o inmediata al write) y la respuesta
  post-write de Stamp/UpsertDay correspondientes a filas individuales — perf igual que hoy.
- Importante: todas las respuestas se construyen **después del commit** de la tx con `s.db` (los
  loops de Stamp/Bulk/Shift/BulkClear están fuera del `Transaction` callback) — el batch no cambia
  esa semántica.

Diseño:

1. `SessionInstanceDaoInterface` + impl ganan `FindByIDs(ctx, ids []int64) ([]dbs.SessionInstance, error)`
   — patrón exacto de `ExerciseInstanceDao.FindByIDs` / `UserDao.FindByIDs`: `id IN ?`,
   `Order("id")`, early-return de slice vacío sin query si `len(ids) == 0`.
2. `SessionExerciseInstanceDaoInterface` + impl ganan
   `FindBySessionInstances(ctx, instanceIDs []int64) ([]dbs.SessionExerciseInstance, error)` —
   `session_instance_id IN ?`, `Order("id")`, early-return si vacía.
3. Servicio nuevo `sessionInstanceResponsesByIds(ctx, ids []int64)
   (map[int64]*instance.SessionInstanceResponse, error)`:
   - Filtra ids positivos y dedupea; si no queda ninguno → `map` vacío sin queries.
   - 3 queries: instancias batch, links batch, ejercicios batch (reusa `ExerciseInstanceDao.FindByIDs`).
   - `instance.NewSessionResponse` por instancia para armar el map.
   - **Semántica de errores idéntica a hoy**: instancia pedida que no existe en la tabla →
     `fmt.Errorf("sesión instancia %d no encontrada", id)` (el mismo texto de
     `sessionInstanceResponse`); link sin ejercicio instancia →
     el error de `NewSessionResponse` con los mismos ids. El behavior huérfana-rompe-GetRange
     (test `TestCobertura_GetRangeInstanciaHuerfanaRompe`) NO cambia.
4. Refactor de `toCalendarDayResponse` en dos partes: lookup de instancia (1 teléfono para el caso
   único) + builder puro `buildDayResponse(d dbs.GroupCalendarDay, inst *instance.SessionInstanceResponse)
   calendar.CalendarDayResponse` con el mapping existente INVARIADO (mismos campos, same_team_warnings,
   presencial_* se siguen NO seteando aquí — siguen solo en el detalle D8).
5. Nuevo `toCalendarDayResponses(ctx, days []dbs.GroupCalendarDay) ([]calendar.CalendarDayResponse, error)`:
   recolecta los `SessionInstanceID` no-nulos, 1 consulta batch de map, loop de `buildDayResponse`.
   Primer error corta y devuelve, igual que hoy.
6. Los 8 loops de respuestas listados arriba pasan a llamar `toCalendarDayResponses` (o padecen el
   helper batch directo si construyen su propio slice de filas). Los callers de un solo día
   **NO se cambian**.
7. Path `s.db == nil`: `toCalendarDayResponses` devuelve los días sin detalle de instancia (map vacío),
   igual que `toCalendarDayResponse` con database nil hoy — tests de mocks siguen pasando.

## D2 — Búsqueda de equipos batch

Hoy (team_service.go Search): por cada equipo del resultado `userDao.FindByID(owner)` +
`teamUserDao.CountActiveByTeam` (2N+1). El flag `can_receive_payments` ya es batch (dedupe owners).

Diseño: owners de TODOS los resultados con un solo `UserDao.FindByIDs` (existe) + conteos con
`TeamUserDaoInterface.CountActiveByTeams(ctx, teamIDs []int64) ([]dbs.TeamMemberCount, error)` nuevo
(`SELECT team_id, COUNT(*) GROUP BY team_id WHERE deleted_at IS NULL`, struct mínima en `dbs/`,
early-return sin query si vacía). Falla del batch → `false`/count 0 igual que hoy por equipo.
Respuesta byte-idéntica.

## D3 — CalendarSummary y otros grupos en batch

`CalendarSummary` (calendar_service.go:1923-1936): hoy `groupDao.FindByID` por membresía. Fix: un solo
`GroupDao.FindByIDs` (existe, filtra activos) + map. Nombres ausentes resuelven igual que hoy.

## D4 — Índices (tags gorm + AutoMigrate, sin backfill)

- `GroupUser`: `index:idx_group_users_group_id` sobre `GroupID`,
  `index:idx_group_users_user_id` sobre `UserID`.
- `TeamUser`: `index:idx_team_users_team_id` sobre `TeamID`, `index:idx_team_users_user_id`
  sobre `UserID`.
- `SessionExerciseInstance`: `index:idx_sei_session_instance` sobre `SessionInstanceID`,
  `index:idx_sei_exercise_instance` sobre `ExerciseInstanceID`.
- `WorkoutFeedbackPoint`: `index:idx_wfp_feedback_id` sobre `FeedbackID` — reevaluación del
  comentario "no lleva tag index" del modelo (hoy hay queries `IN` por feedback y el volumen crece
  con el uso).
- Convención de tags: los índices del repo se declaran por tag (patrón PR #93 — indicación
  `index:...` + test que verifica `pg_indexes`). Sin columnas nuevas ni backfill.

## D5 — Testing

- Tests DAO de los métodos batch nuevos (Postgres real): roundtrip, vacío sin query, orden, filtro
  activo de CountActiveByTeams.
- Tests de service: para cada superficie refactorizada existen ya suites verdes (`task3/task5`,
  presencial,exclude, colisiones, banners) — la regla es **regresión total sin cambio de expectativa**:
  si un test necesita ajuste por el refactor (mocks por interfaces nuevas), el ajuste no altera
  aserciones de shape/orden/errores.
- Un test nuevo que pincele la reducción de queries del builder batch de calendario (instrumentación
  con mock DAO que cuenta llamadas: N días → exactamente 3 `...IN` queries + 0 FindByID), para que la
  optimización no se regrese a N+1 en el futuro.
- Índices: tests que consultan `pg_indexes` (patrón PR #93) para las 7 índices.

## D6 — Verificación final

- Suite completa `go test -count=1 ./...` con Postgres real → 0 FAIL.
- Coverage: `go clean -cache` + `make coverage-with-db` + analyzer → gate 85 PASS, `.testcoverage.yml`
  sin diff (bug de cache conocido: número falso ~70% sin clean).
- `openspec validate --strict`, gofmt/build/vet, Swagger SIN regenerar (no cambió nada de anotaciones).
- Docs: nota corta en `docs/CATALOGO_Y_CALENDARIO.md` (sección rendimiento interna) y tildado;
  actualizar `docs/DEUDA_TECNICA_Y_PENDIENTES.md` — quitar/marcar resuelto el ítem "GetRange N+1" y
  la nota de `workout_feedback_point` sin índice.

## Riesgos

- Interfaces nuevas → mocks de tests a actualizar (ajustes mecánicos).
- El builder batch cambia el ORDER de queries (1 query post-commit en vez de 3 por día); nadie
  depende del orden entre esos reads (mismo snapshot de tx ya commiteada).
- AutoMigrate crea índices al arrancar: primer arranque tras merge tarda unos segundos más en esa
  tabla (D1 spike del arranque: 97% del tiempo era introspección WAN — en local irrelevante).
