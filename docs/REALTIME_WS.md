# Gateway WebSocket en tiempo real (`/api/v1/ws`)

Documento de referencia del gateway WebSocket (`cmd/api/realtime` + wiring en `cmd/api/app/realtime.go`). Cubre conexión, protocolo, autorización de canales, límites y el broadcast de feedback. Fuente de verdad: el código en `cmd/api/realtime/` (`hub.go`, `protocol.go`, `connection.go`, `notifier.go`) — este doc lo resume y explica, no lo reemplaza.

Origen de la spec: `openspec/changes/ws-gateway-sesiones/` (`design.md` tiene el detalle de decisión con D1-D10; acá va la síntesis operativa).

**El endpoint NO está en el Swagger** (por diseño: el handshake WS no es OpenAPI-representable) — esta doc es su contrato.

## 1. Conexión

- **Ruta:** `GET /api/v1/ws` — pública a propósito (registrada antes de `AuthMiddleware`, `cmd/api/app/url_mappings.go`): el handler valida su propio token, no hay header `Authorization` en un handshake WS.
- **URL:** `wss://host/api/v1/ws?token=<access_token>` (en local: `ws://localhost:8080/api/v1/ws?token=...`).
- **Token:** query param `token` con el access token JWT (mismo que usa el resto de la API). Validación **antes del upgrade** — falla → `401` JSON con el mismo contrato de error que `AuthMiddleware` (`apierror.APIError {status_code, code, message}`):

| Caso | Body |
|---|---|
| Falta `token` | `401 {"status_code":401,"code":"unauthorized","message":"falta el query param token"}` |
| Token inválido | `401 {"status_code":401,"code":"unauthorized","message":"token inválido"}` |
| Token expirado | `401 {"status_code":401,"code":"token_expired","message":"el access token expiró"}` |

- **Origin:** se aceptan los orígenes de `CORS_ALLOWED_ORIGINS` (misma lista que el middleware CORS, ver `AGENTS.md` §5) y requests **sin header `Origin`** (clientes nativos/React Native no lo envían). Origen no permitido → el upgrade se rechaza con respuesta HTTP de error (no JSON del contrato de arriba).
- Una conexión establecida queda asociada a **un único usuario** (el del token) por toda su vida. La identidad viaja en cada frame servidor→cliente como `from` en presence/control.

## 2. Protocolo — JSON, un objeto por frame

Cliente → servidor:

```json
{"type":"subscribe",   "channel":"session:123"}
{"type":"unsubscribe", "channel":"session:123"}
{"type":"presence",    "payload":{...}}
{"type":"control",     "payload":{...}}
{"type":"ping"}
```

Servidor → cliente:

```json
{"type":"subscribed", "channel":"session:123"}
{"type":"error", "message":"..."}
{"type":"pong"}
{"type":"presence", "from":12, "payload":{...}}
{"type":"control", "from":12, "payload":{...}}
{"type":"update:set_event", "data":{...}}
```

Semántica por tipo:

| Tipo | Dirección | Uso |
|---|---|---|
| `subscribe` | c→s | Suscribirse a `channel`. Responde `subscribed` (con el mismo `channel`). Requiere `channel`. |
| `unsubscribe` | c→s | Dejar de recibir de `channel`. Sin respuesta explícita — a partir de ahí no llegan más frames de ese canal. Requiere `channel`. |
| `presence` | c→s | Estado efímero ("estoy viendo la serie"). Requiere `payload` **objeto JSON** (no vacío, no `null`, no arreglo). |
| `control` | c→s | Señal efímera (ej. "marcá el set en tu UI"). Requiere `payload` **objeto JSON** (no vacío, no `null`, no arreglo). |
| `ping` | c→s | Heartbeat app-level → responde `pong`. |
| `subscribed` | s→c | Confirmación de suscripción (con `channel`). |
| `error` | s→c | Rechazo de algo puntual. **Nunca corta la conexión** (ver §4). |
| `pong` | s→c | Respuesta al `ping`. |
| `presence` / `control` | s→c | Reenvío del frame de otro usuario: `from` = userID del emisor, `payload` intacto (opaco — el backend no valida ni modifica su contenido). |
| `update:set_event` | s→c | Evento server-originado: se creó feedback de la sesión (ver §5). |

Reglas comunes:

- **presence/control no traen channel en el formato base** — el destino se resuelve: si el frame trae `channel` explícito, debe ser un canal ya suscripto; si no trae, se usa la **única** suscripción activa de la conexión (multi-canal sin `channel` → `error`). Se reenvía a **todos los demás suscriptores** del canal, excluyendo al emisor (el emisor no recibe su propio frame). Único suscriptor del canal → nadie recibe nada, sin error.
- **Resubscribe idempotente:** suscribirse dos veces al mismo canal es un no-op que responde `subscribed` de nuevo (el Hub deduplica por set).
- `payload` de presence/control es opaco pero debe ser un **objeto JSON** (`{...}`, no vacío, no `null`, no arreglo — si no, `error`): cualquier esquema adentro es cosa del frontend.

## 3. Autorización de canales

El único patrón registrado hoy: **`session:{id}`**, con `{id}` = ID de **session instance** (la copia congelada del calendario, ver `docs/CATALOGO_Y_CALENDARIO.md` §8).

- **ID canónico estricto:** se acepta solo el decimal canónico (`session:123`). Alias que parten la sala en strings distintos son rechazados con `error`: `session:07`, `session:+7`, `session:7abc`, `session: 7`, `session:0`, `session:-3`, `session:abc` — y cualquier canal sin patrón (`team:5`, etc.).
- **Regla de acceso** (la misma dual de `GET /session-instances/{id}`, Gap 14 — delega en `sessionInstanceDao.HasInstanceAccess`):
  1. Miembro activo del grupo del día con esa instancia, **u** owner del equipo del grupo; **o**
  2. Feedback activo sobre la instancia, como atleta (`athlete_user_id`), reportante (`feedback_owner_user_id`) u owner del equipo.
- Canal no reconocido/inválido → sin acceso, **ni toca la DB**.
- La autorización se evalúa **una vez, en el `subscribe`** — no se re-chequea durante la vida de la suscripción (si el acceso se revoca, la suscripción ya establecida sigue hasta reconectar; la HTTP sigue siendo la fuente de verdad para datos sensibles).
- Fallo del authorizer (error de DB) → `error` ("error verificando acceso al canal"), conexión viva.

## 4. Límites, heartbeat y errores

| Límite | Valor | Comportamiento al exceder |
|---|---|---|
| Tamaño de frame cliente→servidor | 4 KB (`SetReadLimit(4096)`) | **Corta la conexión** — única excepción a "error no corta" |
| Read deadline | 45 s, refrescada por cada mensaje recibido (cualquiera, incluido `ping`) | Sin mensajes en 45 s → el servidor cierra |
| Write deadline | 10 s por frame saliente | Write falla → conexión cerrada |
| Canales por conexión | 20 | Subscribe adicional → `error` ("tope de canales por conexión alcanzado"), conexión viva, suscripciones existentes intactas |
| Buffer de salida por cliente | 32 frames | Frame descartado para ese cliente (feed best-effort); 8 descartes consecutivos → conexión cerrada |

- **`error` NUNCA corta la conexión:** tope de canales, canal ajeno, canal desconocido, JSON inválido, `type` desconocido, presence/control sin canal resoluble o con payload que no es objeto JSON, fallo del authorizer. Todo responde `error` y el loop sigue; al corte se llega solo por tres caminos: error de read/write del socket, frame over-size, o drop por overflow sostenido del buffer de salida (ver tabla de arriba).
- **Heartbeat:** cualquier mensaje del cliente refresca el deadline — un `ping` cada 20-30 s sobra. No hace falta ping a nivel TCP.
- Al caer la conexión (cualquier motivo) se limpian todas sus suscripciones — no hay estado zombie de canales.

## 5. Broadcast `update:set_event` — feedback de sesión

El único evento server-originado: al crear feedback vía `POST /api/v1/workout-feedback`, el backend emite al canal `session:{assigned_session_id}`:

```json
{
  "type": "update:set_event",
  "data": {
    "message": "feedback registrado",
    "data": { "id": 10, "assigned_session_id": 123, "athlete_user_id": 5, "...": "..." }
  }
}
```

- **`data` es exactamente el body HTTP 201** de `POST /workout-feedback` (`MutationResponse`): mismo `{message, data}` con el `WorkoutFeedbackResponse` completo (incluye `athlete_user_id`). El frontend reutiliza su normalizador HTTP tal cual.
- **Solo `Create` emite.** `POST .../:id/points`, `PUT /workout-feedback/:id` y `DELETE` no generan eventos.
- **Best-effort, asíncrono:** la emisión sale del controller vía `realtime.Notifier` (`HubNotifier.Emit`) después de persistir y no bloquea ni altera la respuesta HTTP. Nadie suscripto al canal → no-op; buffer del receptor lleno → el frame se descarta para ese receptor (la HTTP es la fuente de verdad: el cliente siempre puede refetchear).
- **Quién recibe qué:** cualquier conexión suscripta a `session:{assigned_session_id}` (atleta viendo su sesión, entrenador revisando, otro dispositivo del mismo usuario — todos los suscriptos al canal, sin exclusión).

## 6. Notas Render / operación

- **Deploy corta conexiones WS:** cada deploy de Render mata las conexiones abiertas → el cliente debe reconectar (con backoff) y re-suscribirse. Las suscripciones no sobreviven a la desconexión.
- **Plan free y spin-down:** un service con mensajes WS activos no spinea down (el tráfico cuenta como actividad). El cold-start habitual post-inactividad (~20-25 s, ver `AGENTS.md` §6) aplica igual: si nadie conectó por un rato, la primera conexión espera al arranque.
- **Single instance → hub en memoria:** el `Hub` (`map canal → set de conexiones`) vive en el proceso de la única instancia; no hay estado compartido externo. Esto acopla el gateway a 1 instancia — asumido a propósito para el plan free.
- **Escalar a múltiples instancias → Redis pub/sub** como mejora futura (deuda documentada en `design.md`, riesgos asumidos): el hub por proceso dejaría de ver los broadcasts originados en otra instancia.
