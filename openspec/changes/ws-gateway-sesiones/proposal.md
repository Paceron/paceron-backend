# Propuesta: gateway WebSocket genérico + broadcast de sesión (Gap 18)

## Por qué

El módulo de registro en vivo (spec `presencial-live-session-transport-runner` del frontend) necesita transporte en tiempo real: el entrenador debe ver casi-en-vivo los registros que el corredor persiste durante una sesión presencial, y ambos lados deben poder intercambiar señales de presencia/control. Hoy el backend es 100% HTTP request/response — no hay forma de empujar un evento a otro cliente.

## Qué cambia

1. **Gateway WebSocket genérico** en `GET /api/v1/ws`:
   - Autenticación por JWT en query param `?token=` (la API JS de WebSocket no permite headers), validada en el upgrade; `401` antes del upgrade si el token falta o es inválido.
   - Protocolo JSON: el cliente puede `subscribe`/`unsubscribe` a canales string arbitrarios, y enviar `presence`/`control` que el gateway **reenvía** a los demás suscriptores del mismo canal (excluyendo al emisor). Es un transporte genérico: no interpreta los payloads, solo los reenvía.
   - Autorización por canal al momento del `subscribe`: patrón `session:{id}` → regla dual de `GET /session-instances/{id}` (Gap 14, `HasInstanceAccess`): miembro activo del grupo del día u owner del equipo, O feedback activo como atleta/reportante/owner. Canal no autorizado o patrón desconocido → mensaje `error`, la conexión sigue viva.
   - Heartbeat app-level: cliente manda `{"type":"ping"}` (la API JS no expone frames de protocolo), servidor responde `{"type":"pong"}`; read deadline ~45s refrescada por cualquier mensaje entrante.
   - Límites: ~20 canales por conexión, mensaje entrante máx ~4KB.
2. **Broadcast `update:set_event`**: al crear un feedback vía `POST /workout-feedback`, el backend emite al canal `session:{sessionInstanceId}` un evento con el mismo body que devuelve la HTTP (`{message, data}` — `data` es el `WorkoutFeedbackResponse` que ya consume el frontend, con `athlete_user_id` incluido). Best-effort y asíncrono: nunca bloquea ni altera la respuesta HTTP. `POST /workout-feedback/:id/points` y `PUT /workout-feedback/:id` no broadcastean.
3. **Sin persistencia**: el hub es 100% en memoria — los eventos solo llegan a conexiones suscriptas en ese momento; el cliente obtiene el estado inicial vía REST (snapshot) y usa WS solo para el feed live. Sin migración de DB.

## Impacto

- Nuevo paquete `cmd/api/realtime` (hub + conexión + protocolo) y nueva dependencia `github.com/gorilla/websocket`.
- Nueva ruta pública (antes del `AuthMiddleware`, valida su propio token) + hook de broadcast en `WorkoutFeedbackController.Create` vía notifier opcional (nil-safe).
- Resto de la API sin cambios. El endpoint WS no va a Swagger (no representable en OpenAPI) — se documenta aparte.
- Render: soporta WS nativamente sin configuración; una sola instancia hace suficiente el hub en memoria (multi-instancia requeriría Redis pub/sub — mejora futura, no aplica hoy). El plan free se mantiene despierto mientras entren mensajes WS.

Spec: `openspec/changes/ws-gateway-sesiones/` — capability nueva `realtime-gateway`.
