# Tasks — optimizaciones-lecturas-cache-2

## Global Constraints

- Rama `feature/optimizaciones-lecturas-cache-2`; no push/merge.
- Sin cambios de endpoints, DTOs, shapes, autorización ni mecanismos del frontend.
- Tests con Postgres real `paceron-test-db:5433` (`TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`); prohibido local/demo/fullseed.
- Comentarios mínimos; stagear solo rutas explícitas; nunca `git add -A` ni `.superpowers/`.
- Coverage gate 85; no tocar `.testcoverage.yml`. Meta de esta ronda: mejorar el 86.3% actual y dejar documentado lo que falta para 90%.
- Conventional Commits por task.

### Task 1: `early_N` en SQL

**Files:** `cmd/api/daos/achievement_dao.go`, `cmd/api/services/achievements_service.go`, mocks y tests de `daos`/`services`.
**Commit:** `perf(gamification): contar early_N en SQL`

- [ ] 1.1 Reemplazar la lectura de todo el histórico de `start_date` por una agregación SQL con timezone Córdoba, conservando el conteo exacto de sesiones `finished` antes de las 08:00.
- [ ] 1.2 Adaptar interface/mocks y `valuesFor` sin cambiar logros ni shapes.
- [ ] 1.3 Tests de paridad con 07:59, 08:00, sesiones no finished y fechas en otra TZ; test de query-count que demuestre que no se trae el histórico.
- [ ] 1.4 Build/vet, tests focales con DB y commit.

### Task 2: Historial de feedback en menos lecturas

**Files:** `cmd/api/daos/workout_feedback_dao.go`, `cmd/api/domains/dbs/workout_feedback_history.go`, `cmd/api/services/workout_feedback_service.go`, tests.
**Commit:** `perf(workout-feedback): total y atleta en la consulta del historial`

- [ ] 2.1 Agregar `TotalCount` y `AthleteName` a la fila del historial; `COUNT(*) OVER()` y JOIN de usuarios en `HistorySearch`, preservando filtros, orden, paginación y huérfanos.
- [ ] 2.2 Usar el total de la fila y el nombre embebido en `buildHistoryResponse`/`historyItems`; fallback a `HistoryCount` solo con página vacía; mantener pools y nombres de equipo/grupo.
- [ ] 2.3 Tests de paridad de shape, primera página, página fuera de rango, total, atleta no resuelto y nombres de equipo/grupo; query-count 7→5/6.
- [ ] 2.4 Build/vet, tests focales con DB y commit.

### Task 3: Acotar instancias legacy del dashboard

**Files:** `cmd/api/daos/dashboard_dao.go`, `cmd/api/services/dashboard_service.go`, tests de `daos`/`services`/`dashboard`.
**Commit:** `perf(dashboard): acotar instancias legacy al rango consultado`

- [ ] 3.1 Agregar `From`/`To` a `UnlinkedExerciseInstances` y filtrar en SQL el rango de sesiones del dashboard.
- [ ] 3.2 Propagar el rango desde `resolveLegacyInstances` (`ExerciseSeries` y `SessionComparison`) sin cambiar la resolución por nombre ni el log de ambigüedad.
- [ ] 3.3 Tests: instancias fuera del rango no aparecen, mismas coincidencias dentro del rango, rango vacío y error de DB.
- [ ] 3.4 Build/vet, tests focales con DB y commit.

### Task 4: Nombres de calendario en una consulta

**Files:** `cmd/api/services/calendar_service.go`, `cmd/api/daos/group_dao.go` o `group_user_dao.go`, `cmd/api/daos/team_dao.go`, tests.
**Commit:** `perf(calendar): resolver nombres de resumen y equipos en una consulta`

- [ ] 4.1 Resolver `CalendarSummary` con una sola consulta (memberships + grupos) preservando orden, exclusión de soft-deleted y comportamiento ante error.
- [ ] 4.2 Resolver nombre de grupo y equipo de member/admin calendar en una sola consulta con JOIN, manteniendo deduplicación y fallback.
- [ ] 4.3 Tests de paridad y query-count para ambos caminos, incluidos sin grupos y grupo soft-deleted.
- [ ] 4.4 Build/vet, tests focales con DB y commit.

### Task 5: Índices y lecturas ligeras

**Files:** `cmd/api/domains/dbs/runner_session.go`, `group_user.go`, `team_user.go`, `workout_feedback.go`, `group_calendar_day.go`, `cmd/api/infrastructure/postgresdb/postgres.go`, `cmd/api/daos/daos_indexes_test.go`, DAOs/servicios de nombres.
**Commit:** `perf(db): limpiar indices y lecturas de nombres`

- [ ] 5.1 Eliminar el índice duplicado `runner_session(athlete_user_id)` y los prefijos redundantes de `group_users`/`team_users` con SQL idempotente, conservando los índices por `user_id`.
- [ ] 5.2 Agregar índice parcial de `workout_feedback(assigned_exercise_id) WHERE deleted_at IS NULL` y `group_calendar_days(source_plan_id) WHERE source_plan_id IS NOT NULL`, verificando costo/uso en test.
- [ ] 5.3 Extender `daos_indexes_test.go` para validar `indexdef`/composición/parcialidad y que el duplicado ya no exista.
- [ ] 5.4 Agregar métodos de lectura ligera (`id`, `name`, datos mínimos) para los batches de nombres y usarlos solo en resoluciones de nombres, sin cambiar el shape.
- [ ] 5.5 Build/vet, tests DAO/servicios con DB y commit.

### Task 6: Cobertura y verificación final

**Files:** tests de las áreas tocadas, `docs/DEUDA_TECNICA_Y_PENDIENTES.md`, `tasks.md`.
**Commit:** `test(performance): cubrir lecturas optimizadas y acercar cobertura`

- [ ] 6.1 Agregar tests para errores de DAO, fallbacks, bordes de TZ/fecha y caminos sin datos de las lecturas modificadas.
- [ ] 6.2 Revisar el perfil de coverage y sumar tests de bajo costo en paquetes con ramas internas no cubiertas, sin tocar comportamiento.
- [ ] 6.3 Actualizar `DEUDA_TECNICA_Y_PENDIENTES.md` con lo resuelto, lo que queda para 90% y cualquier riesgo de índices.
- [ ] 6.4 `openspec validate --strict`, gofmt, build, vet, suite completa con DB, `go clean -cache && make coverage-with-db` + analyzer gate 85, `git diff --check`.
- [ ] 6.5 Tildar tasks y commit.
