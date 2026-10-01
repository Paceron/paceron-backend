# Diseño: ws-gateway-sesiones

## D1 — Librería y paquete

- `github.com/gorilla/websocket` (estándar de facto, mantenida, MIT). Nueva dependencia en `go.mod`.
- Nuevo paquete `cmd/api/realtime`: `Hub`, `client`, tipos de mensaje del protocolo. Sin lógica de dominio — el gateway no sabe qué es una sesión; solo aplica un registro de patrones de autorización.

## D2 — Hub

- `map[string]map[*client]struct{}` con `RWMutex`. Operaciones: `Subscribe(channel, client)`, `Unsubscribe(channel, client)`, `Broadcast(channel, frame []byte, exclude *client)`, `Count(channel)`.
- Envío por conexión: cada `client` tiene un canal de salida bufferado (`chan []byte`, cap ~32). `Broadcast` es no-bloqueante: si el buffer del destinatario está lleno, **descarta el frame** para ese cliente (feed live best-effort; la HTTP es la fuente de verdad). Con buffer lleno sostenido (pump de escritura no alcanza), se cierra la conexión.
- Los eventos emitidos con hub vacío (nadie suscripto al canal) son no-ops baratos.

## D3 — Conexión y upgrade

- Handler `GET /api/v1/ws` registrado **en el bloque público** de `url_mappings.go` (antes del `r.Use(AuthMiddleware())`) — valida su propio token.
- Upgrade: `websocket.Upgrader{CheckOrigin}` — permite orígenes de `CORS_ALLOWED_ORIGINS` y requests sin header `Origin` (clientes nativos/React Native no lo mandan).
- Token: `c.Query("token")` → `utils.ParseAccessToken`. Falla → `401` JSON (antes del upgrade, respuesta HTTP normal). Éxito → extrae `auth_user_id` y deja de usar el header/query.
- Invariants: un `client` = una conexión WS autenticada con `userID`; ninguna ruta de escritura pasa por WS (solo `presence`/`control` efímeros).

## D4 — Protocolo (JSON, un objeto por frame)

Cliente → servidor:

```json
{"type":"subscribe", "channel":"session:123"}
{"type":"unsubscribe", "channel":"session:123"}
{"type":"presence", "payload":{...}}
{"type":"control", "payload":{...}}
{"type":"ping"}
```

Servidor → cliente:

```json
{"type":"subscribed", "channel":"session:123"}
{"type":"error", "message":"..."}
{"type":"pong"}
{"type":"presence", "from":12, "payload":{...}}
{"type":"control", "from":12, "payload":{...}}
{"type":"update:set_event", "data":{"message":"...", "data":{...WorkoutFeedbackResponse...}}}
```

- `payload` de presence/control viaja opaco (objeto JSON válido, sin validación semántica). Se reenvía a **todos los demás suscriptores** del canal (excluye al emisor) con `from` = userID del emisor.
- `update:set_event` es server-originado: la forma del wrapper `{message, data}` replica exactamente el body HTTP de `POST /workout-feedback` para que el frontend reutilice su normalizador.

## D5 — Autorización de suscripción

- Registro de patrones (`realtime.ChannelAuthorizer`): función `(channel string, userID int64) (bool, error)`. El gateway valida el patrón; el patrón de `session:{id}` vive en el wire-up (app), no en el paquete realtime:
  - Parsear `session:` → id int64 > 0 (canal inválido → `error`, no desconexión).
  - Autorización: `sessionInstanceDao.HasInstanceAccess(ctx, instanceID, userID)` — misma regla dual que `GET /session-instances/{id}` (Gap 14). Reutiliza el DAO existente; la authorization se evalúa **una vez, en el subscribe** (no se re-chequea en el medio de la sesión).
  - Canal sin patrón registrado → `error` (canal desconocido).
- El cliente puede suscribirse a varios canales (tope ~20 por conexión); resubscribe al mismo canal es idempotente.

## D6 — Pumps, límites y heartbeat

- Por conexión: goroutine **read pump** (SetReadLimit 4KB, SetReadDeadline 45s refrescada en cada mensaje leído, despacha por `type`) y goroutine **write pump** (drena el canal de salida, write deadline ~10s por frame).
- `ping` (app-level JSON) → responder `pong` y refrescar deadline. El resto del tráfico también refresca la deadline — el heartbeat del frontend (20-30s) sobra.
- Close del socket limpia sus suscripciones (limpieza por defer en el read pump).
- Mensajes con `type` desconocido → `error` sin cortar.

## D7 — Broadcast del feedback

- Interfaz pequeña en el wire-up: `realtime.Notifier` con `Emit(channel string, payload []byte)` (asíncrono, no-bloqueante, nil-safe).
- Hook en `WorkoutFeedbackController.Create`: tras persistir y construir `response := toWorkoutFeedbackResponse(feedback)`, si `notifier != nil` → emite a `session:{feedback.AssignedSessionID}` el payload `{"type":"update:set_event","data": MutationResponse{Message: MsgFeedbackCreated, Data: &response}}` — el mismo objeto que va en la `c.JSON(201, ...)`.
- El mapper del DTO vive en el paquete controllers — por eso el hook va en el controller, no en el service (evita duplicar el mapeo o moverlo de capa). Inyección opcional en `Application` wiring; `nil` en tests/mocks = comportamiento actual intacto.
- No emiten: `POST .../points`, `PUT /workout-feedback/:id`, `DELETE`.

## D8 — Rutas y wiring

- `r.GET("/api/v1/ws", app.realtimeController.Upgrade)` en el bloque público (antes de `AuthMiddleware`).
- `app` gana `realtimeController` (o equivalente) con dependencias: hub, `sessionInstanceDao` (para el authorizer), config de orígenes. El resto del wiring no cambia.

## D9 — Documentación

- `docs/REALTIME_WS.md` nueva: protocolo completo, autenticación, autorización, límites, notas Render (deploy corta conexiones → reconexión del cliente; plan free con WS activo no spinea down; single instance).
- `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §11: contrato para el frontend (ruta `wss://.../api/v1/ws?token=`, canales, eventos, códigos).
- El endpoint no entra al Swagger (no es OpenAPI-representable) — decisión consciente, doc aparte.

## D10 — Testing

- Unit: Hub con `-race` (suscripciones concurrentes, broadcast a múltiples, descarte por overflow, limpieza al desconectar).
- Integración con `httptest` + cliente WS real: 401 sin/`token` inválido; `subscribe` autorizado; canal ajeno → `error` (403-equivalente), conexión viva; `presence`/`control` reenviados excluyendo emisor; `update:set_event` llega tras emitir; `ping`→`pong`; límite de canales.
- Controller: `Create` con notifier mock (canal + payload correctos) y con notifier nil (sin romper).
- Suite completa + coverage gate 85 como siempre (`go clean -cache` si el número sale raro — bug conocido).

## Riesgos asumidos

- Token en query param: aceptado (estándar para WS desde JS); mitigado con access token corto + no loguear query strings.
- Hub en memoria acoplado a 1 instancia: OK en Render free (single instance); escalar → Redis pub/sub (deuda futura documentada).
- Deploys/mantenimiento cortan conexiones: el cliente debe reconectar (ya previsto en su spec).
