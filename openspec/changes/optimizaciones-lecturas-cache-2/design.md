# Design — optimizaciones de lecturas y cache 2

## D1. `early_N` en SQL

`achievementDao` gana una agregación que devuelve directamente el conteo de sesiones `finished` con `start_date` local (Córdoba) anterior a las 08:00. La consulta usa `COUNT(*) FILTER (WHERE ...)` o `SUM(CASE WHEN ...)` en SQL, con `AT TIME ZONE 'America/Argentina/Cordoba'`. Se conserva el contrato de `AchievementDaoInterface` para los evaluadores: se reemplaza el método que devuelve `[]time.Time` por uno que devuelve el conteo, o se agrega un método específico y se deja el anterior sin uso solo si otros callers lo necesitan. Tests de paridad contra el cálculo en Go con fechas de borde (07:59/08:00), TZ y sesiones no finished.

## D2. Historial de feedback

`WorkoutFeedbackHistoryRow` agrega `TotalCount int64` y `AthleteName string` (o `*string` consistente con el pool). `HistorySearch`:
- agrega `COUNT(*) OVER() AS total_count` al SELECT;
- agrega `LEFT JOIN users` para `athlete_name` (misma fuente que el pool actual);
- conserva joins, filtros, orden, paginación y semántica de huérfanos.

`buildHistoryResponse` usa el total de la primera fila y cae a `HistoryCount` solo si la página viene vacía (offset fuera de rango). `historyItems` deja de pedir `UsersByIDs` para atleta; mantiene `TeamsByIDs`/`GroupsByIDs`. Tests de paridad de shape, página vacía, total y nombres.

## D3. `UnlinkedExerciseInstances` acotado

`UnlinkedExerciseInstances` recibe el rango de fechas de la consulta (`From`/`To`) además de `groupIDs`. La SQL filtra las instancias legacy por el rango de días/sesiones que realmente se va a agregar. Se preserva la resolución por nombre y el log de ambigüedad. Tests con instancias fuera del rango que no deben aparecer y con el mismo resultado en el rango.

## D4. Nombres de calendario

`CalendarSummary` resuelve los grupos del usuario en una sola consulta con JOIN a `group_users` (o un método DAO equivalente), preservando orden y exclusión de grupos soft-deleted. Los calendarios agregados resuelven nombre de grupo y nombre de equipo en una sola consulta con JOIN `groups → teams`, manteniendo deduplicación y fallback. Sin cambios de DTOs ni de shapes.

## D5. Índices

- Eliminar el índice duplicado `runner_session(athlete_user_id)` (conservar uno, eliminar el otro con SQL idempotente).
- Eliminar los índices prefijo redundantes de `group_users`/`team_users` solo si el compuesto nuevo cubre el prefijo; mantener los índices por `user_id`.
- Agregar índice parcial `workout_feedback(assigned_exercise_id) WHERE deleted_at IS NULL` y `group_calendar_days(source_plan_id) WHERE source_plan_id IS NOT NULL` si el costo de escritura es aceptable.
- Extender `daos_indexes_test.go` para validar `indexdef` de los compuestos/parciales, no solo el nombre.
- Migraciones con `CREATE/DROP INDEX` idempotente; evaluar `CONCURRENTLY` fuera de transacción para tablas grandes.

## D6. Cobertura

Agregar tests en las áreas tocadas y en ramas internas de bajo costo, priorizando:
- errores de DAO y caminos de fallback de las lecturas modificadas;
- bordes de fecha/TZ de `early_N`;
- página vacía del historial y nombres huérfanos;
- `CalendarSummary` sin grupos y con grupos soft-deleted;
- índices y migraciones.

No se baja el umbral. La meta es mejorar el número actual (86.3%) y dejar documentado qué falta para 90%.

## Verificación

`go build ./...`, `go vet ./...`, suite completa con Postgres test, coverage con `go clean -cache && make coverage-with-db` + analyzer gate 85, `openspec validate --strict`, `gofmt` en archivos tocados y `git diff --check`.
