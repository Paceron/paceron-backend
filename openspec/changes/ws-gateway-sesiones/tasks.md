# Tasks: ws-gateway-sesiones

Global constraints (aplican a todas las tasks):

- Rama `feature/ws-gateway-sesiones` (ya creada desde `develop`), Conventional Commits, stagear solo rutas explícitas (nunca `git add -A`; nada bajo `.superpowers/` se commitea).
- Postgres real para tests de DB: `docker start paceron-test-db` (:5433), env `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Gate de coverage 85: verificar con `go clean -cache` + `make coverage-with-db` + analyzer `go run github.com/vladopajic/go-test-coverage/v2@latest --config ./.testcoverage.yml --profile ci/test_coverage/coverage.out` (bug de cache conocido: sin clean el número puede salir falso bajo).
- Comentarios de código mínimos (solo invariants/trampas no obvias).
- No push/merge (el usuario lo hace). Módulo Go: `simple-arq-golang`.
- La API HTTP existente no cambia comportamiento. El endpoint WS no va al Swagger.
- Reglas de sesión ya existen y NO se re-implementan: autorización de canal = `sessionInstanceDao.HasInstanceAccess` (Gap 14); broadcast hook usa el mapper `toWorkoutFeedbackResponse` existente del controller.

### Task 1: Hub y protocolo en `cmd/api/realtime`

**Dependencia:** agregar `github.com/gorilla/websocket` a `go.mod`.

- [x] 1.1 Crear `cmd/api/realtime`: tipos del protocolo (mensajes cliente→servidor y servidor→cliente según design D4), `Hub` (mapa canal→set de `*client` con `RWMutex`; `Subscribe`, `Unsubscribe`, `Broadcast(channel, frame, exclude)`, `Count`), `client` con userID y canal de salida bufferado (`chan []byte`, cap ~32), y interfaz `ChannelAuthorizer` (`(channel string, userID int64) (bool, error)`).
- [x] 1.2 `Broadcast` no-bloqueante: destinatario con buffer lleno → descarta el frame para ese cliente (y marca para cierre si persiste). Emisión a canal sin suscriptores = no-op.
- [x] 1.3 Tests unitarios del Hub con `-race`: suscripciones concurrentes de múltiples clientes, broadcast llega a todos los del canal menos al excluido, limpieza al desconectar, descarte por overflow, `Count`.

### Task 2: Handler del gateway en `/api/v1/ws`

- [x] 2.1 Handler de upgrade en el paquete realtime (o controller mínimo): token desde `c.Query("token")` con `utils.ParseAccessToken` → `401` JSON antes del upgrade si falta/inválido; `websocket.Upgrader` con `CheckOrigin` que permita `CORS_ALLOWED_ORIGINS` y requests sin header `Origin`.
- [x] 2.2 Read pump + write pump por conexión según D6: `SetReadLimit(4KB)`, read deadline 45s refrescada por cada mensaje leído, write deadline ~10s por frame, despacho de `subscribe`/`unsubscribe`/`presence`/`control`/`ping`, respuesta `pong` y `error` sin cortar, limpieza de suscripciones al desconectar. El authorizer inyectado decide autorización de canales (tope ~20 canales por conexión, resubscribe idempotente); el patrón `session:{id}` NO vive en el paquete realtime (se inyecta).
- [x] 2.3 Wiring: ruta pública `r.GET("/api/v1/ws", ...)` en `url_mappings.go` **antes** de `r.Use(AuthMiddleware())`; `Application` gana la dependencia con hub + `sessionInstanceDao` (authorizer de `session:{id}` delegando en `HasInstanceAccess`) + orígenes permitidos.
- [x] 2.4 Tests de integración con `httptest` + cliente WS real (gorilla): 401 sin token / token inválido; upgrade exitoso con token válido (y sin header Origin); subscribe autorizado → `subscribed`; canal ajeno → `error` con conexión viva; canal desconocido → `error`; `ping`→`pong`; presence reenviado a los demás excluyendo emisor; tope de canales. (`go test -race` para el paquete.)

### Task 3: Broadcast update:set_event en Create

- [x] 3.1 `realtime.Notifier` (`Emit(channel string, payload []byte)` — asíncrono, no-bloqueante, nil-safe) expuesto del paquete; inyección opcional en `WorkoutFeedbackController` (campo + wiring en `app.go`; nil en tests = sin cambios de comportamiento).
- [x] 3.2 En `WorkoutFeedbackController.Create`, tras construir `response := toWorkoutFeedbackResponse(feedback)` y solo si `notifier != nil`: emitir a `session:{feedback.AssignedSessionID}` el payload `{"type":"update:set_event","data": MutationResponse{Message: MsgFeedbackCreated, Data: &response}}` (mismo objeto que va en la `c.JSON(201, ...)`). No emiten `CreatePoints`/`Update`/`SoftDelete`.
- [x] 3.3 Tests: controller con notifier mock (canal correcto `session:{id}`, payload con `athlete_user_id` del ítem), con notifier nil (sin romper), y de punta a punta con hub real vía integración: crear feedback por DAO/service mientras hay una conexión suscripta al canal → recibe `update:set_event`; nadie suscripto → no-op.
- [x] 3.4 `go build ./...`, `go vet ./...`, suite verde.

### Task 4: Docs + verificación final

- [x] 4.1 `docs/REALTIME_WS.md` nueva: ruta y forma de conexión (`wss://host/api/v1/ws?token=`), protocolo completo (D4), autorización de canales (regla dual), límites y heartbeat, notas Render (deploy corta conexiones → reconexión; plan free con WS activo no spinea down; single instance → hub en memoria; escalar → Redis pub/sub como mejora futura).
- [x] 4.2 `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §11: contrato para el frontend (conexión, canales, eventos, `update:set_event` con body HTTP igual al de `POST /workout-feedback`, errores sin cortar conexión, heartbeat JSON).
- [x] 4.3 Verificación final: `openspec validate ws-gateway-sesiones --strict`; gofmt en archivos tocados; `go build ./...` + `go vet ./...`; suite completa `go test -count=1 ./...` con Postgres real → 0 FAIL; `go clean -cache` + `make coverage-with-db` + analyzer ≥ 85 (gate intacto, `.testcoverage.yml` sin diff vs develop); tildar todos los checkboxes del change.
