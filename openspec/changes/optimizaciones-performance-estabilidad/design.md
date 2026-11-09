# Diseño — optimizaciones de performance y estabilidad

## D1. Contexto y logging seguro

- `sendPushToUser`: si `ctx == nil`, usar `context.Background()` con timeout de 10s para `SendBatch`; si no, conservar el contexto recibido. No cambiar la firma pública.
- `customlogger.addRequestIDToTags`: no tocar `ctx.Writer` si es nil. Leer el request-id desde `ctx.Get("X-Request-Id")` y, como fallback, desde `ctx.Request.Header`. El middleware de request-id debe hacer `c.Set("X-Request-Id", id)` además de escribir el header de respuesta.
- `realtime.HubNotifier.Emit`: agregar `defer recover()` en la goroutine de entrega para que un panic de un frame no tumbe el proceso.

## D2. Feedback: acceso y coherencia

- Inyectar `SessionInstanceDaoInterface` en `WorkoutFeedbackService` (segundo parámetro del constructor; actualizar `app.go` y tests).
- Agregar a `SessionInstanceDaoInterface`:
  - `TeamIDForSessionInstance(ctx, instanceID) (*int64, error)`: devuelve el `team_id` del día que referencia la instancia; nil si no hay día (instancia sintética/huérfana).
  - `ExerciseBelongsToSessionInstance(ctx, instanceID, exerciseInstanceID) (bool, error)`: existe un link `session_exercise_instances` para ese par.
- En `Create`:
  1. `HasInstanceAccess(ctx, assigned_session_id, authUserID)` → 403 si false.
  2. Si `athlete_user_id` es ajeno, además `HasInstanceAccess(ctx, assigned_session_id, athleteUserID)` → 403.
  3. Validar `assigned_exercise_id` contra la instancia → 400.
  4. Resolver `teamID` de la instancia; si el request trae `team_id` y no coincide (o la instancia no tiene equipo) → 400; si no trae, persistir el derivado. Para instancia sintética/route, el derivado es nil.
- En `GetSessionFeedback`, exigir `HasInstanceAccess(ctx, sessionInstanceID, authUserID)` antes de leer.
- No cambiar el shape de respuesta.

## D3. Runner sessions

- Inyectar `SessionInstanceDaoInterface` en `runnerSessionService` (o reutilizar el DAO ya disponible si existiera; hoy no).
- En `Create`, `Finish` y `Get`, después de resolver el atleta efectivo, exigir `HasInstanceAccess(ctx, sessionInstanceID, athleteUserID)`. Devolver un sentinel nuevo `ErrRunnerSessionForbidden` mapeado a 403 en el controller.
- Mantener el gate presencial, la idempotencia y la validación de competencia en el mismo orden. No cambiar el flujo de owner del team.

## D4. Pagos

- `ProcessPayment`: si `validatedInstallment != nil`, comparar `req.TransactionAmount` con `validatedInstallment.Amount` con tolerancia de 0.01; si difiere, devolver error de validación antes de crear el pago o llamar a MP.
- `CreatePreference`: si `req.InstallmentID != nil`, la suma de `req.Items` debe coincidir con el monto de la cuota; si no, 400. Si no hay cuota, conservar el comportamiento actual.
- `POST /payments` con `PreferenceID`: antes de leer/mutar, exigir `paymentOwnedBy(ctx, existing, authUserID)`; si no, 404 genérico sin mutar.
- Webhook: si el pago tiene `InstallmentID` y el resultado está `approved`, cargar la cuota y comparar `result.TransactionAmount` con `installment.Amount`; si el monto es menor, no aplicar la cuota y devolver error para que MP reintente/alerte. No marcar `paid`.
- No cambiar el flujo `concept=order` sin cuota.

## D5. Lecturas N+1

- **Permisos:** resolver roles con `FindByIDs`; una query batch de suscripciones activas por `(user_id, role_ids)`; tiers con `FindByIDs`; permisos por tier con una query batch (`tier_permissions` + `permissions`, o `ListPermissionNamesByTier` por tier único). Mantener el orden y los errores de `missingData`.
- **Join requests:** resolver `teamIDs` con `teamDao.FindByIDs` y `runnerIDs` con `userDao.FindByIDs` una vez; `toResponse` recibe nombres ya resueltos. Conservar nombres vacíos para referencias inexistentes.
- **Dashboard:** reutilizar el `openSessions` que ya devuelve `seriesAndQuality` en `Leaderboard` y `Periods`; resolver equipos del scope con `teamDao.FindByIDs`; validar atletas con una sola lectura (`dashboardDao.ScopeAthletes` o equivalente batch) en vez de `FindByGroupID` por grupo. Preservar el 403 por atleta fuera de scope.

## D6. Calendario y CPU

- `SessionInstanceDetail`: leer la fila una vez y reutilizarla en el builder (`sessionInstanceResponseFromRow` o equivalente); no duplicar `FindByID`.
- `UpsertDay`: usar `instantiateSessionsBatch` con un slice de 1 y `deleteSupersededInstances` para el borrado, preservando D10 y los mensajes de error.
- `Shift`: leer las fechas destino con `FindByGroupAndDates` y aplicar un único `UpdateDatesForShift` batch; conservar el orden descendente para evitar colisiones del unique y los guards actuales.
- `computePeakSpeedKPH`: no copiar ni reordenar si el caller ya entrega los puntos ordenados; agregar una variante explícita para puntos ordenados y usarla desde `BulkCreatePointsRecompute`.
- Regex de validación a variables de paquete; base pública de media URL resuelta una vez y cacheada en el cliente de storage.

## D7. Gamificación y push

- Unificar el post-finish: calcular la historia de rachas una vez y pasarla al trigger de rachas y al evaluador de logros. Mantener exactamente la semántica D3/D14, el upsert de `gamification_streak_state`, los hitos `10/20/50/100` y la idempotencia de logros.
- `Close` de competencia: recolectar tokens con `PushTokenDao.FindByUserIDs` y enviar en lotes de hasta 100 por request (límite Expo) fuera del camino crítico de la respuesta (goroutine best-effort con contexto independiente y recover). Mismo contenido de push.

## D8. DB/config

- Agregar tag `index` a `WorkoutFeedback.FeedbackOwnerUserID` y el índice al test `daos_indexes_test.go`; sin backfill.
- `config.LoadDBConfigDB`: leer `DB_MAX_OPEN_CONNS` y `DB_MAX_IDLE_CONNS` con defaults 5; aplicar en `postgresdb.ConfigDB`. No cambiar `ConnMaxLifetime` ni el pooler.

## D9. Verificación

`openspec validate --strict`, `gofmt`, `go build`, `go vet`, suite completa con Postgres real, coverage gate 85. Tests de query-count para N+1 y tests de regresión para los guards de seguridad. No ejecutar local/demo/fullseed. No push/merge.
