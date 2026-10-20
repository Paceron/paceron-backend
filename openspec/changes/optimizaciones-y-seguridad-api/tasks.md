# Tasks: optimizaciones-y-seguridad-api

## Global Constraints

- Rama `feature/optimizaciones-y-seguridad-api` ya creada desde develop. Conventional Commits.
- **Stage 1: cero cambios de API** — respuestas byte-idénticas (solo cantidad de queries cambia). Stage 2: cambios de comportamiento SOLO en guards/errores especificados en los deltas.
- Postgres real para tests DAO/service: `docker start paceron-test-db` (:5433), env `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Coverage gate 85: `go clean -cache` + `make coverage-with-db` + analyzer `go run github.com/vladopajic/go-test-coverage/v2@latest --config ./.testcoverage.yml --profile ci/test_coverage/coverage.out` (bug de cache conocido). `.testcoverage.yml` NO se toca.
- Comentarios de código mínimos (solo invariantes no obvios, brevísimos).
- Stage de commits: SOLO rutas explícitas — JAMÁS `git add -A`/`git add .`, nada de `.superpowers/` commiteado.
- NO push/merge — el usuario pushea y mergea.
- Swagger (si aplica): `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs` (comando canónico del repo).
- Módulo `simple-arq-golang` en imports internos.
- NO tocar: roles/permissions/tiers/platform-settings (esperan módulo admin), `/auth/user` shape público, `test-card-token`, `GET /groups` sin scope.

### Task 1: Listings de catálogo en batch (P1/P2)

- [x] 1.1 `GET /sessions`: colectar session IDs → `sessionExerciseDao.FindBySessions(ids)` (existe, `session_exercise_dao.go:38`) → agrupar por `session_id` → `toResponse` variante con links precargados. Respuesta idéntica (orden y forma). Conteo: 2 queries para N sesiones.
- [x] 1.2 `GET /training-plans`: nuevo `FindByPlanIDs(ids)` en `plan_day_dao.go` (patrón FindBySessions: early-return vacío, `Order("plan_id, sequence_no")`) + agrupado en memoria. Conteo: 2 queries.
- [x] 1.3 Tests: conteo de queries con counting pool (2 exactas para N>1), respuesta idéntica (orden/contenido), caso vacío. Regresión de los tests de List existentes sin tocar aserciones.
- [x] 1.4 `go build ./...` + `go vet` + suite services con Postgres real + commit `perf(catalogo): listings de sesiones y planes en batch (P1/P2)`.

### Task 2: Mensajería — tope de destinatarios + acceso batch (P3)

- [x] 2.1 `validateSendMessage`: const `maxRecipientsPerMessage = 100` → más de 100 en `multiple` → 400 `ErrSessionMessageInvalid` "los destinatarios no pueden superar 100". `all`/`direct` sin tope (costo constante).
- [x] 2.2 `SessionInstanceDaoInterface` gana `HasInstanceAccessBatch(ctx, instanceID, userIDs []int64) (map[int64]bool, error)` — misma regla de acceso del singular (día, feedback, competencia-route), hasta 3 queries totales con `IN`, map por userID. `HasInstanceAccess` singular intacto (WS y otros callers).
- [x] 2.3 `validateRecipients` usa el batch; no autorizados → 400 igual que hoy (ErrSessionMessageInvalid, mensaje y detalle previos). Excepción de DAO propaga.
- [x] 2.4 Tests: DAO batch real Postgres (mixto autorizado/ajeno, mapa completo), service (tope 100 → 400 sin DAO; batch no-autorizado → 403; happy multiple), mocks actualizados.
- [x] 2.5 `go build ./...` + `go vet` + suites con Postgres real + commit `feat(messages): tope de destinatarios y acceso en batch (P3)`.

### Task 3: Pagos y MP client (P5/P6)

- [x] 3.1 `validateOwnedInstallment` devuelve `(*dbs.Installment, error)`; `resolveTeamSplitConfig` recibe la cuota cargada (parámetro/variante mínima) — `ProcessPayment` sin doble `FindByID`. Comportamiento idéntico.
- [x] 3.2 `mercadopagoclient/client.go`: `http.Client{Timeout: 10s}` package-level usado por los 4 endpoints crudos (GenerateCardToken :282, ExchangeCodeForToken :342, RefreshAccessToken :394, GetUserInfo :439). CreatePayment del SDK no se toca.
- [x] 3.3 Tests: service sin doble lectura (pin con mock de conteo si aplica); timeout no se testea unitariamente (configs). Suite pagos verde.
- [x] 3.4 Commit `perf(payments): cuota reutilizada y timeouts en mp client (P5/P6)`.

### Task 4: Push batch + roles batch + catálogo batch (P7/P8/P9)

- [x] 4.1 `push_notifier.go`: 1 request Expo con `to: [tokens...]` por usuario (extender `expopushclient` si no soporta array). Best-effort igual, tests existentes adaptados sin perder cobertura.
- [x] 4.2 `auth_service.go` login/refresh: roles en 1 query — `RoleDaoInterface.FindByIDs` (nuevo si no existe, patrón UsersByIDs).
- [x] 4.3 `session_service.validateExercises` y `training_plan_service`: batch con `FindByIDs` existentes/nuevos. Comportamiento idéntico (404 del item faltante igual).
- [x] 4.4 Tests: conteos batch, regresión de login/refresh/create/clone.
- [x] 4.5 Commit `perf(misc): push batcheado y lecturas de roles/catalogo en batch (P7/P8/P9)`.

### Task 5: 13 índices (D7)

- [x] 5.1 Tags `index` en los 11 modelos estándar (lista exacta en design D7, filas 3-12). gofmt.
- [x] 5.2 Índices parciales 1-2 (runner_session.competition_id / competition.route_session_instance_id UNIQUE) via SQL `CREATE INDEX IF NOT EXISTS` post-migrate en `postgres.go` (patrón drops legacy).
- [x] 5.3 `daos_indexes_test.go`: extender con los 13 índices nuevos (los parciales verificados con `pg_indexes` tras corrida).
- [x] 5.4 Tests DAO verdes con Postgres real; `go build`/`go vet`/gofmt; commit `perf(daos): indices para pagos, catálogo, roles y competencias (P4)`.

### Task 6: Verificación stage 1

- [x] 6.1 Suite completa `go test -count=1 ./...` con Postgres real → 0 FAIL.
- [x] 6.2 Coverage: `go clean -cache` + `make coverage-with-db` + analyzer → gate 85 PASS.
- [x] 6.3 Ledger del stage 1 (queries antes/después por endpoint optimizado).

### Task 7: Webhooks de MP firmados (S1/S7)

- [x] 7.1 `payment_controller.HandleWebhook`: parsear `ts,v1` de X-Signature; secret configurado → `ValidateWebhookSignature(xSig, xRequestID, dataID, secret)` → inválida/faltante → 401 (apierror, sin leak); secret ausente → comportamiento actual + warning. Log de X-Signature: presencia/longitud, nunca el valor.
- [x] 7.2 `mp_connect_controller` deauth (~:192): mismo patrón con los datos disponibles de la notificación.
- [x] 7.3 Tests: firma válida/inválida/faltante × secret seteado (t.Setenv) / secret ausente — matriz completa; el 401 NO toca DB ni MP. Regresión: flujos actuales con secret ausente byte-idénticos.
- [x] 7.4 Swagger: `@Failure 401` en ambos webhooks + regeneración canónica (el shape de los 200 no cambia).
- [x] 7.5 Commit `feat(payments): firma HMAC en los webhooks de Mercado Pago (S1/S7)`.

### Task 8: State OAuth HMAC (S2)

- [x] 8.1 `domains/mpconnect/state.go`: `BuildState` firma `userID-tsNanos-target` con HMAC-SHA256 (secret JWT del server, `config` existente) → `payloadBase64.hmacHex` (o formato propio); `ParseState` verifica firma antes de parsear; inválida → error.
- [x] 8.2 `mp_connect_service.validateState` mantiene chequeo de antigüedad; state inválido → flujo de error existente del callback (400/redirect existente, sin nuevo shape).
- [x] 8.3 Tests: state forjado (payload válido, firma inventada) → rechazado sin vincular; state legítimo viaja completo; tampering del payload → rechazado; antigüedad sigue.
- [x] 8.4 Commit `feat(mpconnect): state de oauth firmado con HMAC (S2)`.

### Task 9: IDOR pagos (S5)

- [x] 9.1 `GetPaymentStatus`: tras FindByID, guard `payment.UserID == callerID || payment.SellerUserID == callerID` → otro caso 404 (sentinel/patrón not-found del paquete).
- [x] 9.2 `GetPaymentStatusFromMP`: resolver pago local + mismo guard ANTES de la llamada HTTP a MP (ajeno → 404, cero llamadas MP).
- [x] 9.3 Tests: ajeno 404 (DAO/MP no consultados con mocks), payer 200, seller 200; regresión de los flows existentes (webhook no pasa por estos métodos).
- [x] 9.4 Commit `feat(payments): ownership en la consulta de pagos (S5)`.

### Task 10: Revocación de refresh tokens (S6)

- [x] 10.1 `refresh_token_dao.go`: `RevokeByUserID(ctx, userID) error` (patrón RevokeBySessionID, condición `revoked = false`).
- [x] 10.2 Llamado tras UPDATE exitoso de contraseña en `ChangePassword` y `ResetPassword`. Error de revocación propaga (rollback/500 si falla — contraseña ya cambiada es el estado autoritativo).
- [x] 10.3 Tests DAO (revoca activos, idempotente) + service (refresh tras cambio → rechazado con error genérico; mensajes de respuesta sin cambio).
- [x] 10.4 Commit `feat(auth): revocar sesiones al cambiar la contrasena (S6)`.

### Task 11: Body limit + rate limiter (S9/S10)

- [x] 11.1 Middleware `MaxBodySize(10MB)` en `middleware.go`, aplicado en `r.Use` global antes del routing. Fotos con su límite propio menor intactos.
- [x] 11.2 Middleware `IPRateLimiter` con `golang.org/x/time/rate` (map+mutex, límite 30/min burst 10 hardcodeado) aplicado SOLO a las rutas públicas de usuario (login/register/refresh/forgot/reset/auth/user) — webhooks MP y ws EXCLUIDOS explícitamente.
- [x] 11.3 Tests: body gigante → rechazo temprano; IP sobre límite → 429 genérico; webhooks/ws sin limiter (verificado con test del wiring).
- [x] 11.4 `go get golang.org/x/time/rate`; commit `feat(app): limite de body y rate limiter en rutas publicas (S9/S10)`.

### Task 12: Docs + verificación final + review

- [x] 12.1 Docs: CATALOGO_Y_CALENDARIO §pagos (ownership + firma webhook), FRONTEND_IMPACTO sección nueva (solo 404s nuevos de pagos si el front puede chocarlos — pagos ajenos; rate limit 429 en públicos; body limit), DEUDA_TECNICA (roles/tiers/fee pendiente admin, S11 limpieza, rate limiter en memoria single-instance, access token sobrevive revocación).
- [x] 12.2 `openspec validate --strict` + gofmt/build/vet + suite completa Postgres real 0 FAIL + coverage gate 85 (clean cache).
- [x] 12.3 Swagger idempotente (sin churn) o regenerado si faltó.
- [x] 12.4 Review final de rama + fix wave si aplica + tildes finales.
