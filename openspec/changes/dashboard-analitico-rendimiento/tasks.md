# Tasks

## 1. Índices

- [ ] 1.1 `cmd/api/infrastructure/postgresdb/postgres.go` — nuevo item 10 (después del item 9 de migración de `kind`): `idx_runner_session_athlete (athlete_user_id)` y `idx_feedback_session_date (assigned_session_id, session_date) WHERE deleted_at IS NULL`, ambos con `CREATE INDEX CONCURRENTLY IF NOT EXISTS` (design.md D15). **No envolver en `db.Transaction`** — `CONCURRENTLY` está prohibido ahí.
- [ ] 1.2 Correr la inicialización contra una DB real y verificar que el segundo arranque no falla (idempotencia).
- [ ] 1.3 `EXPLAIN ANALYZE` de la query core de `design.md` (Context) y adjuntar el plan al PR.
- [ ] 1.4 Runbook de rollback con los `DROP INDEX CONCURRENTLY IF EXISTS` correspondientes.

## 2. `SEEDING_ENABLED`

- [ ] 2.1 Leer `SEEDING_ENABLED` de la config del servidor (no de la request) en `cmd/api/config`.
- [ ] 2.1b **Decidir antes de tocar la firma** de `isCalendarDayClosed`: si recibe el flag como parámetro nuevo, dejan de compilar `calendar_service_task3_test.go:424-428` y `calendar_service_task5_test.go:321-323`, que la llaman con 2 argumentos — y ninguna task los actualiza. La opción que **no rompe nada** es leer `config.SeedingEnabled()` adentro de la función sin cambiar la firma: los tests siguen compilando y el caso de la bandera se cubre con un test propio que setea la variable. Si aun así se cambia la firma, agregar la actualización de los 3 archivos como parte de esta misma task.
- [ ] 2.2 `cmd/api/services/calendar_service.go` — `isCalendarDayClosed` recibe el flag y relaja **solo** la rama `date < hoy`; la presencial de hoy ya empezada y el async de hoy siguen cerrando igual (spec `group-calendar`, design.md D14).
- [ ] 2.3 **Gate de stage**: el seed solo es alcanzable con `config.IsLocalStage()` (`--stage=local`) **Y** `SEEDING_ENABLED=true`. **Solo** con `panic` en la rama `SEEDING_ENABLED=true && !IsLocalStage()`, nombrando el stage detectado. Con la bandera inerte (sin setear o en `false`) **no hay comprobación**: el proceso arranca siempre. Implementar la condición como «si falta cualquiera de las dos, no arrancar» deja el backend caído en todos los deploys. `IsProduction()` **no** alcanza como guarda: staging no contiene `"prod"` en la env var y es un entorno real que comparte el proyecto de Supabase de testing.
- [ ] 2.4 Log estructurado de cada uso: usuario, plan, fechas afectadas.
- [ ] 2.5 Tests: con la bandera activa estampa una fecha pasada; sin ella responde `422`; con la bandera activa la presencial de hoy ya empezada sigue rechazando; un `seeding_enabled` en el body se ignora.

## 3. Lógica de KPI (pura, sin HTTP ni DB)

- [ ] 3.1 `cmd/api/domains/dashboard/kpi.go` — `objectiveAxisForKind(kind)`, `targetFromExerciseInstance(ei)`, `speedFrom(distanceM, durationMs)` (derivada, nunca desde `speed_kph`).
- [ ] 3.2 `cmd/api/domains/dashboard/compliance.go` — `isCompliant(kind, targets, achieved, tolerance)` y `deltaPct(kind, targets, achieved)`.

**Un solo `target` escalar no alcanza** para `ritmo_continuo`, que es el tipo por defecto del catálogo: su objetivo son *tres* cosas a la vez —tiempo, distancia y la velocidad que implican— y su firma necesita las tres. Por eso el parámetro es un struct, no un número:

```go
type SeriesTargets struct {
    Minutes    *int     // objetivo de tiempo, congelado
    DistanceM  *int     // objetivo de distancia, congelado
    SpeedKph   *float64 // velocidad esperada, derivada de los dos anteriores
}
type SeriesAchieved struct {
    TimeMs     int64    // tiempo logrado (COALESCE active/duration)
    DistanceM  *float64 // distancia lograda
    SpeedKph   *float64 // velocidad lograda, derivada
}
```

`isCompliant` devuelve `true` cuando **se cumple cualquiera** de los criterios que el `kind` declara: `ritmo_continuo` exige tiempo **y** distancia, así que un `ritmo_continuo` que va más rápido pero se queda corto de distancia no cuenta como cumplido.

El `axis` es **obligatorio en las dos**: en el eje temporal el signo se invierte, y sin el parámetro `deltaPct` devuelve la diferencia aritmética cruda, que el spec prohíbe explícitamente (240 s objetivo / 230 s logrado debe dar **+4,2 %**, no −4,2 %). Es el bug más fácil de escribir y el más difícil de ver en un mock: el número aparece, el color sale rojo, y nadie nota que el corredor mejoró.
- [ ] 3.3 `cmd/api/domains/dashboard/validity.go` — `isValidSeries(kind, feedback)`, `timeQuality(fb)` (`alta`/`degraded`), `completionTaxonomy(fb)`.
- [ ] 3.4 `cmd/api/domains/dashboard/trend.go` — `trendPct(series)` con umbral de 3 datos y clasificación `improving`/`stable`/`regressing` en ±3 %.
- [ ] 3.4b `cmd/api/domains/dashboard/band.go` — `band(kind, targets, achieved, tolerance) string` y su agregador `bandCounts(series)`. Es la función pura que produce `band` por serie y `band_counts` por bucket, y hoy no tiene dónde aterrizar. El caso `unknown` (objetivo cargado pero el campo del eje de la serie viene `null`) se distingue del `null` de `has_target: false`: el primero cuenta serie, el segundo no cuenta banda.
- [ ] 3.4c `cmd/api/domains/dashboard/consistency.go` — `coefficientOfVariation(velocidades []float64) *float64`: σ(v)/μ(v) sobre la **velocidad** de las series válidas, excluyendo las que no tienen `distance_meters`. Devuelve `nil` con menos de 2 valores o con `μ = 0`. El front lo consume directo en el KPI y en el radar.
- [ ] 3.4d `cmd/api/domains/dashboard/quality.go` — los seis contadores de `data_quality`. `invalid_series`, `degraded_time` y `legacy_sessions` salen del mismo result set; **`open_sessions` exige una segunda consulta**, porque el `WHERE` que excluye las filas `wip` ya las sacó del primer result set y contarlas ahí daría siempre cero.
- [ ] 3.4e `cmd/api/domains/dashboard/units.go` — el bloque `units` como **constante**, no armado en cada serializer: las 7 claves son fijas, y duplicar el mapa en 6 responses es la forma de que una quede desactualizada. `dashboard.go` lo inyecta en los seis endpoints.
- [ ] 3.5 Tests unitarios **exhaustivos** de las fórmulas por los 5 tipos: eje correcto, cumplimiento en el borde de la tolerancia, `null` en vez de `0` cuando no calculable, `NaN`/negativos, decimales.
- [ ] 3.6 Test que verifica que un `kind` legacy (`walking`, `jogging`, …) no entra a ningún KPI: no produce estadísticas, no rompe.

## 4. Scope y autorización

- [ ] 4.1 `cmd/api/services/dashboard_scope.go` — `ResolveScope(callerID)` que devuelve los `group_id` permitidos reutilizando `groupDao.FindByOwnerID` y el criterio de `isGroupOwnerOrMember`. Se calcula **una vez** por request.
- [ ] 4.1b `cmd/api/domains/dashboard/normalize.go` + `normalize_test.go` — **en Go, no en JS**: este repo es Go puro, sin `package.json` ni toolchain JS. `normalizeExerciseName(string) string` baja a minúsculas, quita acentos, elimina puntuación y colapsa espacios, más el `__tests__` equivalente que acá es un `_test.go` en el mismo paquete. Resuelve las `ExerciseInstance` sin `source_exercise_id`, que son todas las anteriores al change de instancia-referencia-catálogo. Es el único mecanismo declarado para no perder ese histórico y no tenía dónde aterrizar.
   **El fallback por nombre SHALL acotarse al mismo `owner_id`.** `exercises` es catálogo por dueño (`exercises.owner_id`), así que dos entrenadores distintos pueden tener "Distancia 4 km" con objetivos distintos: sin acotar por dueño, el dashboard mostraría el objetivo de un entrenador evaluando las sesiones de otro. Si un nombre normalizado matchea más de un ejercicio del mismo dueño, la coincidencia SHALL resolverse por `exercises.id` ascendente y `legacy_sessions` SHALL contar el caso, para que el fallback sea visible en vez de silencioso. Resuelve las `ExerciseInstance` sin `source_exercise_id`, que son todas las anteriores al change de instancia-referencia-catálogo. Es el único mecanismo declarado para no perder ese histórico y no tenía dónde aterrizar: sin él el dashboard abre y el histórico aparece vacío.
- [ ] 4.2 `cmd/api/services/dashboard_service.go` — el scope se pasa ya resuelto a todos los DAOs. **Ningún DAO acepta un `group_id` crudo.**
- [ ] 4.3 `POST /comparisons` valida cada `athlete_user_id` por separado y responde `403` nombrando el que falla, sin datos de los válidos.
- [ ] 4.4 `404` para inexistente, `403` para ajeno.

## 5. DAO de métricas

- [ ] 5.1 `cmd/api/daos/dashboard_dao.go` — query de series del scope con los 4 joins de `design.md` (Context), **columnas explícitas, prohibido `SELECT *`**.
- [ ] 5.2 Agregaciones: por corredor, por bucket (`session_date`, granularidad `session`/`week`/`month`/`quarter`), y el leaderboard con `ORDER BY` por whitelist.
- [ ] 5.3 Whitelist de `sort`: `best_time_ms`, `mean_speed_kph`, `total_distance_m`, `compliance_rate`. Fuera de ella → `400` (rechazar **antes** de tocar la DB).
- [ ] 5.4 Límites: `from` obligatorio, rango máximo 2 años, `page_size` máximo 100, **`tolerance` dentro de `[0.01, 0.20]`**, **`granularity` dentro de `{session, week, month, quarter}`** y **`order` dentro de `{asc, desc}`**. Cada violación responde `400` con su `code` de `error_code.go`, no un `400` genérico, **`tolerance` dentro de `[0.01, 0.20]`**, **`granularity` dentro de `{session, week, month, quarter}`**, y **`order` dentro de `{asc, desc}`**. Cada violación responde `400` con su `code` de la tabla de `error_code.go`, no un `400` genérico.
- [ ] 5.5 Test que verifica que el filtro `sei.role = 'main'` está presente (regresión de D9).

## 6. Endpoints + swagger

- [ ] 6.0 **Formato de fecha de salida: `"2006-01-02"` (ISO)**, y solo ISO. El repo parsea `YYYY-MM-DD` (`parseSessionDate`, `workout_feedback_service.go:463`), así que emitirlo es lo consistente. **El front emite `DD/MM/YYYY`** en `DateField` desde el 2026-10-04 (`CLAUDE.md`, quirk de `DateField`), o sea que ambos lados hablan formatos distintos y hace falta el helper inverso a `toISODate` (`utils/date-field-format.js:10`), que **no existe**. Sin declararlo, `date_range.min` sembrado en un `DateField` muestra `01-09-2026` sobre un placeholder `DD/MM/AAAA`, y el picker nativo lee basura.
- [ ] 6.1 `cmd/api/domains/dashboard/` — DTOs de request/response, incluido el bloque `data_quality` presente aunque todos los contadores sean cero.
- [ ] 6.2c `date_range` (MIN/MAX de `session_date` dentro del scope) es una **query de apoyo** más, y no está asignada a ninguna task: es la que le dice al front si nunca hubo datos (`null`) o si el rango está acotado. Sin ella, `/filters` no cumple su propio contrato.
- [ ] 6.2b Constante `CURRENT_EXERCISE_KINDS` con los **5** tipos vigentes, en `constants/exercise_kind.go`, junto a `GetValidExerciseKinds()` que devuelve los **10** (5 vigentes + 5 legacy). Reusar la función existente devuelve 10 en `/filters.exercise_types` y en el tablist, que es un bug observable: el usuario ve tipos que ya no existen en el catálogo.
- [ ] 6.2 `cmd/api/controllers/dashboard_controller.go` + `GET /api/v1/dashboard/filters`.
  - **Definir cómo se determina que el llamador es entrenador**, porque de eso depende la forma de toda la respuesta: entrenador → equipos + grupos + corredores de su scope; corredor → solo su propia entrada en `athletes[]`. El criterio es **`teams.owner_id == callerID`** (`groupDao.FindByOwnerID` ya resuelve los grupos por dueño, `group_dao.go:95`), no un rol: el scope es "grupos que me pertenecen". Un `user_roles` sería otro concepto — poder ser entrenador sin tener grupos— y dejaría la pantalla vacía sin explicación.
  - La **membresía del corredor** entra por la ventana de `group_users.date_start` (`group_user_dao.go:96-106`): el roster del leaderboard debe respetar la fecha de la sesión, no la membresía actual.
- [ ] 6.3 `GET /metrics/summary` y `GET /metrics/timeseries`.
- [ ] 6.4 `POST /comparisons` y `POST /comparisons/periods` (delta de cumplimiento en **puntos porcentuales**).
- [ ] 6.5 `GET /leaderboard` (paginado, ordenable, empates deterministas) y `GET /exercises/{id}/series`.
  - **`position` e `is_self`:** `is_self` es un `WHERE athlete_user_id = $caller`; `position` es un `RANK() OVER (ORDER BY <columna de sort>, <mismo desempate>)` sobre **todas** las filas, no solo la página. El `RANK` tiene que usar **exactamente** la misma columna y el mismo desempate que el `ORDER BY` final, o el número que ve el corredor no coincide con la posición real. Sin esto, el requisito de "identificar la posición propia" queda sin implementar: hoy ninguna task lo nombra.
- [ ] 6.6 Rutas en `cmd/api/app/url_mappings.go`, detrás de `AuthMiddleware`, **y** el campo `dashboardController` en `cmd/api/app/app.go` (`app.go:26-60` tiene ~35 campos y `:119-333` ~35 constructores). Registrar la ruta sin cablear el controller en `Application` deja la app sin compilar.
- [ ] 6.7 Anotaciones `swag` en cada handler (`@Summary`/`@Param`/`@Success`/`@Router`), mismo estilo que los controllers existentes.
- [ ] 6.8 Correr `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs` y commitear `swagger.json`/`yaml`/`docs.go`.
- [ ] 6.9 Tests de integración por rol: propio → `200`, ajeno → `403`, inexistente → `404`.
- [ ] 6.9b Test de los cinco rechazos de parámetro con su `code`: `tolerance` fuera de `[0.01, 0.20]`, `granularity` fuera de `{session, week, month, quarter}`, `order` fuera de `{asc, desc}`, `page_size` > 100 y rango > 2 años. Los cinco son normativos en el spec y hoy solo `sort` tiene dónde implementarse.
- [ ] 6.10 Test de que un `sort` fuera de whitelist devuelve `400` sin ejecutar la query.
- [ ] 6.11 Test de que un rango de 12 años devuelve `400`.

## 7. Seed

- [ ] 7.1 `cmd/api/scripts/seed_dashboard/main.go` con `TEAM_ID = 29` y `GROUP_ID = 39` como constantes en el **top del archivo**, reusables para otro scope.
- [ ] 7.2 Catálogo de 12 ejercicios respetando la matriz de tipos vigente:
  - `liviano` — Sparring (10 min / 1.2 km) y Trote Vuelta a la Calma (15 min / 1.2 km).
  - `elongacion` — Elongación (10 min, sin distancia).
  - `ritmo_continuo` — Distancia 4/6/8/10/21 km. Necesitan distancia **y** tiempo, que es justo lo que tienen.
  - `velocidad_distancia` — Pasadas 1000 m (4:00/serie), Pasadas 2000 m (10:00/serie), **Velocidad Máxima 100 m y 200 m** (solo distancia, `minutes` en `null`).

  **Por qué las series de velocidad máxima van como `velocidad_distancia` y no como `velocidad_tiempo`:** `minutes` es `INT`, y un sprint de 100 m son ~12 s = **0,2 minutos**, que no es representable — redondeado da 0, que el validador de tiempo del front rechaza (`> 0`). Como `velocidad_distancia` tiene el **tiempo opcional**, 100 m se declara con `distance_m = 100` y `minutes` en `null`, que sí es exacto y además lo puede crear un entrenador desde el form. El KPI relevante —mejor tiempo y velocidad derivada— sigue disponible vía `best_time_ms` y `speed_kph`.

- [ ] 7.2b **Consecuencia para la validación:** el seed **no ejercita la paleta B** (sin objetivo cargado), porque los 12 ejercicios tienen objetivo. La paleta B queda como robustez del dashboard, para datos que lleguen sin objetivo —filas previas a la migración de tipos, o un ejercicio que el entrenador dejó incompleto—. El seed debe **incluir al menos un caso con `has_target: false`** para probar esa rama: agregar un ejercicio de `velocidad_distancia` con `distance_m` en `null` es inválido, así que el caso se cubre en los **tests** del DAO (fila de prueba, no del catálogo).
- [ ] 7.3 Nombres: **"Sparring"** y no "Calentamiento" — el rol lo define `session_exercises.role`, no el nombre. Un `liviano` puede ser calentamiento, recuperación o trote suelto.
- [ ] 7.4 Composición de sesión: siempre Sparring → Elongación → bloque principal → Vuelta a la calma. Si entra distancia ≥8 km, ese es el **único** bloque principal.
- [ ] 7.5 Planes de 14 días aplicados vía el mecanismo de stamp existente (con `SEEDING_ENABLED`), **nunca insertando a mano por saltarse la instanciación**. 2 sesiones presenciales por semana (martes y jueves) usando `exclude_dates` para saltear el resto.
- [ ] 7.6 `group_users` con `date_start` correcta: sin ella el calendario del corredor sale vacío aunque el `group_calendar_days` exista.
- [ ] 7.7 Series: `duration_ms`/`active_duration_ms`/`distance_meters`/`set_number` por corredor, ejercicio y día, con `runner_session` en `finished`.
- [ ] 7.8 Generador con arquetipos (velocista, fondista, resistente, regular, en progreso), 2–3 destacados por especialidad, ruido ±3–7 %, fatiga intra-ejercicio, progresión semanal, ~10 % de series fuera de objetivo.
- [ ] 7.9 Feedback contextual al ejercicio y al corredor. **Prohibido el genérico.**
- [ ] 7.10 `--dry-run` obligatorio, log de cantidades por tabla e IDs, y guard de scope que **aborta** si `GROUP_ID` no pertenece a `TEAM_ID`.
- [ ] 7.11 Idempotente: correrlo dos veces no duplica nada.

## 8. Limpieza

- [ ] 8.1 `cmd/api/scripts/seed_dashboard/cleanup.go` — borrado en orden inverso de FK: `runner_session` → `workout_feedback_points` → `workout_feedback` → `session_exercise_instances` → `exercise_instances` → `session_instances` → `group_calendar_days` → `plan_days` → `training_plans` → `sessions`/`session_exercises` → `exercises`

   **`runner_session` va primero, y es obligatorio**: el seed crea filas en `finished` (task 7.7), y borrarlas al final dejaría huérfanas apuntando a `session_instances` inexistentes. `group_users` NO se borra: el seed solo **ajusta** `date_start` (task 7.6) y restaurarlo sería más riesgoso que dejarlo — se documenta en el runbook que el seed mueve la membresía del grupo y que hay que revertirla a mano si se quiere el estado previo.
- [ ] 8.2 Restringido **estrictamente** a `TEAM_ID`, `GROUP_ID` y la ventana temporal. Prohibido tocar datos fuera de ese scope.
- [ ] 8.3 Conteo previo, `--dry-run`, confirmación explícita y transacción con rollback.
- [ ] 8.4 Test que verifica que la limpieza no deja residuos y no toca filas fuera del scope.

## 9. Cierre

- [ ] 9.1 `go build ./...` y `go vet` verdes.
- [ ] 9.1b Los tests que necesitan Postgres real (5.5, 6.9, 6.9b, 6.10, 6.11, 7.2b, 8.4) **se skipean en silencio** sin `TEST_DB_HOST` (`testutils/db.go:42-45`), y "suite verde" con skips no es verde. El repo tiene `make test-with-db` (`Makefile:112`): el cierre tiene que correrlo, no `go test ./...` a secas.
- [ ] 9.2 Suite completa verde, incluyendo los tests de permisos (`propio → 200`, `ajeno → 403`).
- [ ] 9.3 Guardar en el repo el dataset generado (o el generador con su semilla fija) para que el frontend pueda desarrollar contra mocks.
- [ ] 9.4 Documentar en el change las fórmulas de KPI por tipificación, para que el frontend no las reimplemente.
- [ ] 9.5 `openspec validate dashboard-analitico-rendimiento` en verde antes de archivar.
