# Tasks

## 1. Dominio: período calendario

- [ ] 1.1 Agregar `RunnerPeriod` y `ResolveRunnerPeriod(period string, reference time.Time) (RunnerPeriod, error)` en `cmd/api/domains/dashboard/`, con `from`, `to`, `daysConsidered` y `referenceDate` normalizada.
- [ ] 1.2 Implementar `month` con el último día real del mes, `week` con lunes ISO a domingo y `day` con un único día.
- [ ] 1.3 Agregar tests de tabla para octubre (31), febrero bisiesto (29), febrero común (28), semana que cruza de mes y de año, y día.
- [ ] 1.4 Validar que una fecha imposible (`2026-02-30`) no se normalice en silencio y que `from <= to` siempre.

## 2. Dominio: fórmulas del Home

- [ ] 2.1 Agregar `RunnerKpiTab` con los cuatro valores (`general`, `ritmo_continuo`, `velocidad_tiempo`, `velocidad_distancia`) y `IsValidRunnerKpiTab`.
- [ ] 2.2 Agregar `RunnerKpisMetrics` con todas las claves del contrato y `BuildRunnerKpis(rows []ScopedSeries, tab RunnerKpiTab) RunnerKpisMetrics`.
- [ ] 2.3 Reutilizar `achievedFromSeries`, `speedFrom` e `isValidSeries`; no duplicar la derivación de tiempo ni velocidad.
- [ ] 2.4 Calcular `total_distance_m`, `best_day_distance_m`/`best_day_date`, `average_speed_kph`, `best_speed_kph`, `best_time_ms` y `best_distance_m` con las reglas del spec.
- [ ] 2.4.1 En `general`, calcular `average_speed_kph` solo con las series `ritmo_continuo`; si no hay, devolver `null` sin afectar `has_data` ni la distancia total.
- [ ] 2.5 Devolver `null` en métricas no calculables o sin datos; devolver `0` solo cuando el cero sea un resultado real.
- [ ] 2.6 Resolver empates de mejor día por fecha más antigua.
- [ ] 2.7 Tests de tabla: cada pestaña, serie inválida, sin objetivo, sin distancia, sin tiempo, `wip` ya filtrada, empate y ponderación 21 km a 5 km/h + 4 km a 15 km/h.
- [ ] 2.7.1 Test de `general` con series de varios tipos: la distancia suma todas y `average_speed_kph` solo pondera `ritmo_continuo`; sin `ritmo_continuo`, el promedio es `null` y `has_data` sigue `true`.
- [ ] 2.8 Test explícito de que `best_speed_kph` es el máximo de las velocidades promedio por serie, que en `general` no excluye tipos y que no se lee `speed_kph` de la instancia.

## 3. DAO: series propias

- [ ] 3.1 Agregar `FindRunnerSeries(ctx, athleteUserID int64, from, to time.Time) ([]DashboardSeriesRow, error)` a `DashboardDaoInterface`.
- [ ] 3.2 Implementar la query desde `workout_feedback`, con `JOIN exercise_instances` y `LEFT JOIN runner_session`; sin `group_calendar_days`, sin `session_exercise_instances` y sin filtro de rol.
- [ ] 3.3 Filtrar por `athlete_user_id`, rango inclusivo de `session_date`, `deleted_at IS NULL` y estado `finished`/`interrupted` o sin fila de `runner_session`.
- [ ] 3.4 Ordenar por `session_date, id` para que el empate de mejor día sea determinista.
- [ ] 3.5 Test de DAO con Postgres real: series en dos equipos, serie de grupo dejado, serie `wip`, serie borrada y serie de otro corredor.
- [ ] 3.6 Verificar con `EXPLAIN` que la query usa `idx_feedback_athlete_date`; no agregar índice si ya lo usa.

## 4. Service: validación y orquestación

- [ ] 4.1 Agregar `RunnerKpis(ctx, callerID int64, query RunnerKpisQuery)` a `DashboardServiceInterface`.
- [ ] 4.2 Definir `RunnerKpisQuery` con `Period`, `ReferenceDate` y `Tab`; aplicar defaults `month` y `general`.
- [ ] 4.3 Validar `period` (`INVALID_PERIOD`), `reference_date` (`INVALID_DATE`) y `tab` (`INVALID_TAB`) antes de tocar la base.
- [ ] 4.4 Resolver el período, llamar a `FindRunnerSeries`, convertir filas a `ScopedSeries` con el helper existente y llamar a `BuildRunnerKpis`.
- [ ] 4.5 Contar `series_count`, `valid_series` y `sessions` distintos; setear `has_data`.
- [ ] 4.6 Tests: defaults, cada code de error, corredor sin datos, corredor con datos en dos grupos y que el service no acepte un `athlete_user_id` del request.

## 5. Controller y ruta

- [ ] 5.1 Agregar `RunnerKpis(c *gin.Context)` al `DashboardController` y a la interfaz.
- [ ] 5.2 Leer `period`, `reference_date` y `tab` del query string; responder `401` sin token y delegar el resto al service.
- [ ] 5.3 Agregar anotaciones `swag` con parámetros, respuesta `200`, y errores `400`/`401`/`500`.
- [ ] 5.4 Registrar `GET /api/v1/dashboard/runner-kpis` en `url_mappings.go`, sin tocar las rutas existentes.
- [ ] 5.5 Tests de controller: query válida, default, `400` con code, `401` y respuesta con todas las claves.

## 6. Contrato y regresión

- [ ] 6.1 Agregar `RunnerKpisResponse` al DTO y declararlo en el controller para que `swag` lo resuelva.
- [ ] 6.2 Agregar los codes `INVALID_PERIOD`, `INVALID_DATE` e `INVALID_TAB` a `domains/constants/error_code.go`.
- [ ] 6.3 Test de contrato: todas las claves de `metrics` presentes, `units` completo y `null` serializado como `null`.
- [ ] 6.4 Ejecutar la suite existente del dashboard y confirmar que no cambió ninguna expectativa.
- [ ] 6.5 Ejecutar `go test ./...` y `go vet ./...`.
- [ ] 6.6 Verificar manualmente con `curl` un corredor con datos en dos equipos y otro sin datos.
