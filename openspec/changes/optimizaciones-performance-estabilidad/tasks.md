# Tasks — performance y estabilidad

## Global Constraints

- Rama `feature/gap-42-y-performance`; no cambiar de rama, no push/merge.
- Sin cambios de contrato HTTP/DTO/autenticación; solo los errores de autorización/monto documentados en `design.md`.
- Tests con `paceron-test-db:5433` y `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`. Nunca `local-*`, `demo-*` ni `fullseed-*`.
- Comentarios mínimos; stagear solo rutas explícitas; nunca `.superpowers/`.
- Conventional Commits. Coverage gate 85.

### Task 1: Panics de push y logging async

**Files:**
- Modify: `cmd/api/services/push_notifier.go`
- Modify: `cmd/api/infrastructure/customlogger/customLogger.go`
- Modify: `cmd/api/app/middleware.go` (request-id set)
- Modify: `cmd/api/app/router.go` si hace falta
- Modify: `cmd/api/realtime/notifier.go`
- Test: `cmd/api/services/push_notifier_test.go` (crear si no existe)
- Test: `cmd/api/infrastructure/customlogger/customLogger_test.go`

- [x] 1.1 `sendPushToUser(nil, ...)`: usar `context.Background()` con timeout 10s para `SendBatch`; nunca pasar un `*gin.Context` nil tipado como `context.Context`.
- [x] 1.2 `addRequestIDToTags`: tolerar `ctx.Writer == nil`; leer request-id de `ctx.Get("X-Request-Id")` y `ctx.Request.Header` como fallback.
- [x] 1.3 El middleware de request-id debe setear `c.Set("X-Request-Id", id)` además del header de respuesta.
- [x] 1.4 `HubNotifier.Emit`: recuperar panics en la goroutine de entrega y loguear sin volver a paniquear.
- [x] 1.5 Tests: push nil no paniquea; `ctx.Copy()` + `customlogger.Error` no paniquea; entrega WS con panic simulado no tumba el proceso.
- [x] 1.6 build/vet/race en paquetes tocados y commit `fix(stability): evitar panics en push y logs async`.

### Task 2: Acceso y coherencia de workout feedback

**Files:**
- Modify: `cmd/api/daos/session_instance_dao.go`
- Modify: `cmd/api/services/workout_feedback_service.go`
- Modify: `cmd/api/app/app.go`
- Test: `cmd/api/daos/session_instance_dao_access_test.go`
- Test: `cmd/api/services/workout_feedback_service_test.go`
- Test: `cmd/api/services/security_task6_test.go`
- Test: `cmd/api/scripts/seed_dashboard/*` si el constructor cambia (solo llamadas)

**Interfaces:**
- Produce: `SessionInstanceDao.TeamIDForSessionInstance(ctx, instanceID) (*int64, error)` y `ExerciseBelongsToSessionInstance(ctx, instanceID, exerciseInstanceID) (bool, error)`.
- Consume: `HasInstanceAccess` existente.

- [x] 2.1 Agregar los métodos DAO e implementarlos con SQL parametrizado; actualizar mocks/interfaces.
- [x] 2.2 Inyectar `SessionInstanceDaoInterface` en `WorkoutFeedbackService` y actualizar `app.go` + tests.
- [x] 2.3 En `Create`, exigir acceso del caller y del atleta efectivo a la instancia; validar el ejercicio; derivar el team de la instancia y rechazar `team_id` incoherente o persistir el derivado.
- [x] 2.4 En `GetSessionFeedback`, exigir acceso del caller a la instancia antes de leer.
- [x] 2.5 Tests DAO reales y de service: cross-tenant 403, ejercicio ajeno 400, team incoherente 400, flujo feliz sin cambio de shape, huérfana/sintética sin team.
- [x] 2.6 build/vet/suite de `daos`+`services` y commit `fix(security): validar acceso a la instancia en workout feedback`.

### Task 3: Acceso a instancia en runner session

**Files:**
- Modify: `cmd/api/services/runner_session_service.go`
- Modify: `cmd/api/controllers/runner_session_controller.go`
- Modify: `cmd/api/app/app.go`
- Test: `cmd/api/services/runner_session_service_test.go`
- Test: `cmd/api/controllers/runner_session_controller_test.go`

- [x] 3.1 Inyectar `SessionInstanceDaoInterface` en `runnerSessionService`; mantener constructor compatible con los call sites existentes.
- [x] 3.2 En `Create`, `Finish` y `Get`, exigir `HasInstanceAccess` para el atleta efectivo después de `resolveAthlete` y antes de escribir/leer; sentinel `ErrRunnerSessionForbidden`.
- [x] 3.3 Mapear el sentinel a 403 en el controller; conservar 404/400 existentes y el orden de guards.
- [x] 3.4 Tests: cross-tenant 403 en create/finish/get, owner y miembro siguen OK, offline/idempotencia sin regresión, competencia sigue OK.
- [x] 3.5 build/vet/suite y commit `fix(security): validar acceso a instancia en runner sessions`.

### Task 4: Ownership y monto en pagos

**Files:**
- Modify: `cmd/api/services/payment_service.go`
- Modify: `cmd/api/controllers/payment_controller.go` si hace falta mapeo
- Test: `cmd/api/services/payment_service_test.go`
- Test: `cmd/api/controllers/payment_controller_test.go`

- [x] 4.1 En `POST /payments` con `PreferenceID`, aplicar `paymentOwnedBy` antes de leer/mutar; 404 genérico y sin cambios si es ajeno.
- [x] 4.2 `ProcessPayment` con cuota: exigir `req.TransactionAmount` igual a `installment.Amount` (tolerancia 0.01) antes de crear pago/llamar MP.
- [x] 4.3 `CreatePreference` con `InstallmentID`: exigir que la suma de items coincida con el monto de la cuota; si no, 400.
- [x] 4.4 Webhook: si aprueba un pago con `InstallmentID` y el monto es menor al de la cuota, no aplicar la cuota ni marcarla `paid`; devolver error para retry/alerta.
- [x] 4.5 Tests: preference ajena 404, cuota con monto menor 400, monto exacto OK, webhook con underpayment no marca paid, flujo sin cuota intacto.
- [x] 4.6 build/vet/suite y commit `fix(payments): ownership de preferencia y validacion de montos`.

### Task 5: Lecturas N+1

**Files:**
- Modify: `cmd/api/services/permissions_query_service.go`
- Modify: `cmd/api/daos/tier_subscription_dao.go`
- Modify: `cmd/api/daos/tier_dao.go`
- Modify: `cmd/api/daos/tier_permission_dao.go`
- Modify: `cmd/api/services/join_request_service.go`
- Modify: `cmd/api/services/dashboard_service.go`
- Modify: `cmd/api/services/dashboard_scope.go`
- Test: `cmd/api/services/permissions_query_service_test.go`
- Test: `cmd/api/services/join_request_service_test.go`
- Test: `cmd/api/services/dashboard_service_test.go`
- Test: `cmd/api/daos/*` para batch methods

- [x] 5.1 Permisos: reemplazar la cadena por rol/permiso por lecturas batch; preservar orden, `Tier`, `Permissions`, `missingData` y errores.
- [x] 5.2 Join requests: resolver teams y runners con `FindByIDs` antes del loop; preservar nombres vacíos y orden de filas.
- [x] 5.3 Dashboard: reutilizar `openSessions` de `seriesAndQuality` en `Leaderboard` y `Periods`.
- [x] 5.4 Dashboard: resolver equipos del scope con `FindByIDs` y validar atletas con una lectura batch; preservar 403 por atleta fuera de scope.
- [x] 5.5 Tests de query-count constantes (2 roles/N permisos, join requests con N, dashboard con 2+ equipos) y regresión funcional.
- [x] 5.6 build/vet/suite y commit `perf(reads): eliminar N+1 en permisos, join requests y dashboard`.

### Task 6: Calendario y CPU

**Files:**
- Modify: `cmd/api/services/calendar_service.go`
- Modify: `cmd/api/daos/group_calendar_day_dao.go` si hace falta
- Modify: `cmd/api/daos/workout_feedback_dao.go`
- Modify: `cmd/api/daos/workout_feedback_metrics.go`
- Modify: `cmd/api/services/auth_service.go`
- Modify: `cmd/api/services/user_service.go`
- Modify: `cmd/api/services/media_url.go`
- Modify: `cmd/api/restclients/storageclient/*`
- Test: `cmd/api/services/calendar_service_test.go`
- Test: `cmd/api/services/workout_feedback_service_test.go`
- Test: `cmd/api/daos/workout_feedback_dao_test.go`

- [x] 6.1 `SessionInstanceDetail`: eliminar el doble `FindByID` reutilizando la fila leída.
- [x] 6.2 `UpsertDay`: usar `instantiateSessionsBatch` (slice de 1) y `deleteSupersededInstances`; preservar D10, errores y rollback.
- [x] 6.3 `Shift`: lectura batch de destinos y actualización batch, conservando orden descendente, colisiones y atomicidad.
- [x] 6.4 `computePeakSpeedKPH`: variante para puntos ya ordenados sin copia/sort; usar en `BulkCreatePointsRecompute`.
- [x] 6.5 Mover regex de validación a variables de paquete y cachear la base pública de media URL.
- [x] 6.6 Tests de query-count para detalle/UpsertDay/Shift y regresión de valores/errores.
- [x] 6.7 build/vet/suite y commit `perf(calendar): reducir round-trips y recomputes`.

### Task 7: Gamificación y push del cierre

**Files:**
- Modify: `cmd/api/controllers/runner_session_controller.go`
- Modify: `cmd/api/services/gamification_streak_trigger.go`
- Modify: `cmd/api/services/achievements_service.go`
- Modify: `cmd/api/services/streak_service.go`
- Modify: `cmd/api/services/competition_service.go`
- Modify: `cmd/api/services/push_notifier.go`
- Modify: `cmd/api/daos/push_token_dao.go`
- Test: `cmd/api/services/*` correspondientes

- [x] 7.1 Calcular la historia de rachas una vez por finish y pasarla al trigger de rachas y al evaluador de logros; conservar D3/D14, hitos, upsert e idempotencia.
- [x] 7.2 Agregar lectura batch de push tokens (`FindByUserIDs`) y envío en lotes de hasta 100 tokens por request.
- [x] 7.3 En `Close`, recolectar tokens de todos los participantes y enviar el fan-out fuera del camino de respuesta, best-effort y con recover; contenido del push sin cambios.
- [x] 7.4 Tests de idempotencia de rachas/logros, conteo de queries del finish (sin doble cálculo) y push batch (1 query + ceil(tokens/100) requests), sin bloquear la respuesta.
- [x] 7.5 build/vet/race y commit `perf(gamification): unificar fin de sesion y batchear push de cierre`.

### Task 8: DB y configuración

**Files:**
- Modify: `cmd/api/domains/dbs/workout_feedback.go`
- Modify: `cmd/api/config/config.go`
- Modify: `cmd/api/infrastructure/postgresdb/postgres.go`
- Test: `cmd/api/daos/daos_indexes_test.go`
- Test: `cmd/api/config/config_test.go`

- [x] 8.1 Agregar tag `index` a `FeedbackOwnerUserID` y el índice al test `daos_indexes_test.go`; sin backfill.
- [x] 8.2 Leer `DB_MAX_OPEN_CONNS` y `DB_MAX_IDLE_CONNS` con defaults 5; aplicar en `postgresdb.ConfigDB`; tests de defaults y override.
- [x] 8.3 Verificar `pg_indexes` y que el pool configurado no cambie otros parámetros.
- [x] 8.4 build/vet/suite y commit `perf(db): indexar feedback owner y configurar pool`.

### Task 9: Documentación y verificación final

**Files:**
- Modify: `docs/DEUDA_TECNICA_Y_PENDIENTES.md`
- Modify: `docs/CATALOGO_Y_CALENDARIO.md` si aplica
- Modify: `docs/FRONTEND_IMPACTO_INSTANCIACION.md` si hay cambios observables
- Modify: `openspec/changes/optimizaciones-performance-estabilidad/tasks.md`

- [x] 9.1 Registrar en deuda lo que queda fuera: búsquedas sin tope, gzip/PureJSON, `WithContext` masivo, email async, lock de `CreateAttemptSession`, visibilidad de equipos/PII.
- [x] 9.2 Documentar cambios observables de autorización/monto y los nuevos errores 403/400/404.
- [x] 9.3 `openspec validate optimizaciones-performance-estabilidad --strict`, `gofmt`, `go build ./...`, `go vet ./...`, suite completa con Postgres real y coverage gate 85.
- [x] 9.4 Tildar tasks y reportar; sin push/merge.
