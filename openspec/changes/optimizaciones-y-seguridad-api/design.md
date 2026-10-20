# Design: optimizaciones-y-seguridad-api

## Stage 1 — Optimizaciones

### D1 — Listings batch (P1/P2)

`session_service.go` `List` (~:236): hoy `toResponse` llama `sessionExerciseDao.FindBySession` por sesión. Fix: colectar IDs → `FindBySessions(ids)` (ya existe, `session_exercise_dao.go:38`) → agrupar en memoria → `toResponse` variante que recibe los links precargados. `training_plan_service.go` `List` (~:303): mismo patrón con nuevo `plan_day_dao.FindByPlanIDs` (patrón `FindBySessions`, early-return vacío, `Order("plan_id, sequence_no")`). Respuesta byte-idéntica (mismo orden, misma forma). Sin DB el path mock mantiene comportamiento.

### D2 — Mensajería: tope + batch de acceso (P3)

`session_message_service.go`:
- `validateSendMessage` (~:149): nuevo tope `maxRecipientsPerMessage = 100` — `recipient_user_ids` con más de 100 → 400 `ErrSessionMessageInvalid` con mensaje "los destinatarios no pueden superar 100". Aplica solo a `multiple` (en `all` y `direct` el costo es constante).
- `validateRecipients` (~:193): hoy 1 `HasInstanceAccess` por destinatario. Fix: variante batch `HasInstanceAccessBatch(ctx, instanceID, userIDs []int64) (map[int64]bool, error)` en `SessionInstanceDaoInterface` (misma regla del singular: día, feedback y competencia-route; hasta 3 queries totales con `IN`) → map de acceso. Los no autorizados → 400 igual que hoy (mismo sentinel y mensaje de detalle previos) (comportamiento pre-task intacto — constraint stage 1). `HasInstanceAccess` singular se mantiene (lo usan WS guard y otros callers).

### D3 — Pagos: cuota reutilizada (P5)

`payment_service.go` `ProcessPayment` (~:190): `validateOwnedInstallment` (~:555) pasa a devolver `(*dbs.Installment, error)` y `resolveTeamSplitConfig` (~:769) acepta la cuota ya cargada como parámetro (variante nueva o parámetro — el equipo decide la forma mínima, sin cambiar su comportamiento). Elimina el doble `installDao.FindByID`. Para non-split el ahorro es nulo pero el código queda uniforme.

### D4 — MP client timeouts (P6)

`restclients/mercadopagoclient/client.go`: los 4 endpoints con `&http.Client{}` crudos (GenerateCardToken :282, ExchangeCodeForToken :342, RefreshAccessToken :394, GetUserInfo :439) pasan a usar un `http.Client{Timeout: 10 * time.Second}` compartido del paquete (const/var package-level). **No** usar la infra `httpclient` (tiene retry — los POSTs de OAuth no son idempotentes y el retry correcto por endpoint es otro trabajo; el timeout simple elimina el riesgo de colgado). CreatePayment del SDK ya trae idempotency key — no tocar. Sin reintentos.

### D5 — Push batch (P7)

`services/push_notifier.go` (~:41): en vez de 1 HTTP call por token, agrupar todos los tokens del usuario en UN request a Expo (`to: [tokens...]`). Si `expopushclient` no soporta array, extenderlo (rest client interno, sin contrato externo). Best-effort igual (fallo logueado, no rompe).

### D6 — Roles batch + catálogo batch (P8/P9)

`auth_service.go` (~:350): roles del usuario con 1 query `WHERE id IN ?` (agregar `FindByIDs` a `RoleDaoInterface` si no existe) — corre en cada login/refresh. `session_service.go` `validateExercises` (~:82): batch con `exerciseDao.FindByIDs` (existe). `training_plan_service.go` (~:79): 1 query por día training del body — acotado a ≤31, batch con `FindByIDs` igual.

### D7 — 13 índices gorm (P4 + deuda propuesta)

Tags en los modelos (`cmd/api/domains/dbs/*.go`) + extensión de `daos_indexes_test.go` con verificación `pg_indexes` (patrón PR #97). Lista exacta:

| # | Tabla → índice | Motivo (queries, file:line) |
|---|---|---|
| 1 | `runner_session(competition_id)` parcial `WHERE competition_id IS NOT NULL` | ExistsAttemptByCompetitionAndAthlete `competition_dao.go:346`, ListAttemptSessions `:541` |
| 2 | `competition(route_session_instance_id)` UNIQUE parcial `WHERE route_session_instance_id IS NOT NULL` | FindByRouteSessionInstance `competition_dao.go:280` (cada intento tagueado) — único por diseño D1 del change ruta-intake |
| 3 | `user_roles(user_id)` | FindByUserID `user_role_dao.go:48` — hot path login/refresh |
| 4 | `installments(user_id)` | ListHistory `payment_history_dao.go:184` |
| 5 | `installments(team_id, user_id)` | FindPendingByUserTeam/FindNextByUserTeam `installment_dao.go:116,131` |
| 6 | `installments(subscription_id)` | FindPendingBySubscription/FindNext `installment_dao.go:75,90,107` |
| 7 | `payments(installment_id)` | JOINs de `payment_history_dao.go:114,179` |
| 8 | `session_exercises(session_id)` | FindBySession/FindBySessions `session_exercise_dao.go:31,43` |
| 9 | `plan_days(plan_id)` | FindByPlan `plan_day_dao.go:27` |
| 10 | `join_requests(team_id, status)` + `join_requests(runner_id)` | `join_request_dao.go:53,65,74,93` |
| 11 | `user_role_tier_subscriptions(user_id, role_id)` | `tier_subscription_dao.go:87,104,127` (los únicos índices hoy son los parciales de estado) |
| 12 | `sessions(owner_id)` + `exercises(owner_id)` + `training_plans(owner_id)` | Lists de catálogo `session_dao.go:64`, `exercise_dao.go:64`, `training_plan_dao.go:46` scanean tabla global |

Índices 1 y 2 (parciales): gorm no genera índices parciales via tags → crearlos via SQL post-migrate en `postgres.go` (patrón drops legacy existente) con `CREATE INDEX IF NOT EXISTS`. Los demás via tags `index` (gorm estándar). No tocar `daos_indexes_test.go` nombres de índices preexistentes — solo agregar.

## Stage 2 — Seguridad

### D8 — Webhook MP firma (S1)

`payment_controller.go` `HandleWebhook` (~:331): parsear `ts,v1=<hex>` de `X-Signature`; si `config.MyMP.WebhookSecret` **está configurado** → `mpClient.ValidateWebhookSignature(xSig, xRequestID, dataID, secret)` (ya existe, `client.go:221-223`) → inválida o headers faltantes → **401** (con `apierror.APIError`, sin leak). Si el secret **no está configurado** → comportamiento actual (aceptar) + `customlogger.Warn` una vez (rollout gradual: producción/testing activan el env cuando quieran, sin deploy ya que el código queda listo). Dejar de loguear el valor completo de `X-Signature` (`:306` — loguear solo presencia/longitud). La idempotencia y re-fetch del pago no cambian.

### D9 — OAuth state HMAC (S2)

`domains/mpconnect/state.go`: `BuildState(userID, target)` firma `fmt.Sprintf("%d-%d-%s", userID, tsNanos, target)` con HMAC-SHA256 usando el **secret de JWT del server** (ya existe en config para los tokens — no env nueva, funciona en los 3 stages) → state = `payload + "." + hex(hmac)`. `ParseState` verifica la firma **antes** de parsear payload (firma inválida → error, no diferencia de timing sensible necesaria — callback público, rechazo uniforme 400). `validateState` del service mantiene el chequeo de antigüedad. El callback de error de MP (state inválido) ya existe — reutilizar ese flujo 400. Sin cambio de shape HTTP.

### D10 — IDOR pagos (S5)

`payment_service.go`:
- `GetPaymentStatus` (~:323): tras `FindByID`, si `payment.UserID != callerID && payment.SellerUserID != callerID` → `404` (mismo sentinel/patrón de "not found" del paquete — evita enumeración con 404, no 403).
- `GetPaymentStatusFromMP` (~:354): antes de consultar MP, resolver el pago local por `external_reference` (ya lo hace) y aplicar el mismo guard de ownership — sin guard no devuelve shape (404). Los flows internos (webhook) no pasan por estos métodos — solo endpoints de usuario.
- Tests: ajeno → 404 sin tocar MP; payer/seller → 200.

### D11 — Revocación de sesiones (S6)

`refresh_token_dao.go`: nuevo `RevokeByUserID(ctx, userID) error` (UPDATE `revoked = true WHERE user_id = ? AND revoked = false` — o el campo que use el modelo; patrón de `RevokeBySessionID` existente). Llamado desde `user_service.ChangePassword` (~:371) y `password_reset_service.ResetPassword` (~:129) después del UPDATE de contraseña exitoso. El access token viejo sigue vivo hasta expirar (limitación conocida — JWT sin DB en middleware, documentado). Mensajes de respuesta sin cambio. Tests: refresh tras cambio de contraseña → token revocado rechazado con el error genérico existente.

### D12 — Webhook deauth firma (S7)

`mp_connect_controller.go` (~:192): mismo patrón D8 — si el secret está configurado, validar firma (los datos de MP deauth llegan con `data.id`/`user_id`; firmar/validar sobre lo disponible siguiendo el mismo mecanismo ts,v1); inválida → 401. Sin secret → comportamiento actual + warning. El shape del payload no cambia.

### D13 — MaxBytesReader global (S9)

Middleware nuevo en `cmd/api/app/middleware.go`: `r.Body = http.MaxBytesReader(w, r.Body, 10<<20)` (10 MB global — cubre multipart de fotos que ya limitan a su propio tope menor; JSON de 100MB muere antes de BindJSON con 413 que Gin devolverá como error de binding). Aplicar en `r.Use(...)` global, ANTES del routing. Los endpoints con `MaxBytesReader` propio (fotos/ícono) no cambian (su cap es menor y ya lee con límite). El 413 para JSON excedido llega como error de BindJSON — mapear si el binding no lo traduce bien (verificar en implementación; si Gin devuelve 400 con "request body too large" es aceptable, no nuevo contrato).

### D14 — Rate limiter público (S10)

Middleware en `cmd/api/app/middleware.go`: `IPRateLimiter` con `golang.org/x/time/rate` — `map[ip]*rate.Limiter` + mutex (limpiar entradas viejas periódicamente o con `rate.Limiter` de límite + TTL naive; single instance, en memoria). Límite: p. ej. 30 req/min por IP con burst 10 en las **8 rutas públicas** de `url_mappings.go:18-28` (login, register, refresh, forgot, reset, auth/user, webhook routes NO — MP necesita confiar en reintentos; ws no). Solo rutas de usuario público. Excedido → 429 con `apierror.APIError` genérico. Constantes hardcodeadas (sin env — se puede agregar después). Verificar que el webhook de payments y ws NO pasen por el limiter.

### D15 — Testing

- Stage 1: tests de batch (2 queries exactas con counting pool para P1/P2; tope 100 → 400; batch access map con autorizado/no autorizado; timeouts no testeados unitariamente — configs). Índices: extensión de `daos_indexes_test.go` + SQL parciales verificados con `pg_indexes`.
- Stage 2: S1/S7 — tests con firma válida/inválida/ausente (secret seteado y no seteado, `t.Setenv`). S2 — state forjado (firma inválida) rechazado; state legítimo viaja; antigüedad sigue. S5 — ajeno 404 (DAO no consultado para shape), payer/seller 200. S6 — refresh tras revocación rechazado. S9/S10 — request gigante rechazado; IP limitada recibe 429.
- Verificación transversal: suite completa con Postgres real, coverage gate 85 con `go clean -cache` + analyzer, `openspec validate --strict`, gofmt/build/vet, Swagger regenerado SOLO si cambia anotación (S1/S7 cambian `@Failure` a 401 → regenerar con comando canónico).

## No-goals / riesgos

- Sin guard en roles/tiers/fee (decisión de producto, arriba).
- Rate limiter en memoria: por réplica (single instance Render — mismo acople del WS hub, documentado).
- HMAC con JWT secret: si el secret rota, los states en vuelo mueren (ventana de minutos, aceptable).
- Revocación no mata access tokens ya emitidos (JWT sin DB — limitación estructural documentada).
