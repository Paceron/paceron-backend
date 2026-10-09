# Gateway WebSocket en tiempo real (`/api/v1/ws`)

Documento de referencia del gateway WebSocket (`cmd/api/realtime` + wiring en `cmd/api/app/realtime.go`). Cubre conexión, protocolo, autorización de canales, límites, el broadcast de feedback, los eventos en vivo de sesión (`update:session_state`/`update:attendance_event`) y el relay dirigido por `to`. Fuente de verdad: el código en `cmd/api/realtime/` (`hub.go`, `protocol.go`, `connection.go`, `notifier.go`) — este doc lo resume y explica, no lo reemplaza.

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
{"type":"update:set_event", "channel":"session:123", "data":{...}}
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
| `update:set_event` | s→c | Evento server-originado: se creó feedback de la sesión (ver §5). El frame incluye `channel`. |
| `update:session_state` | s→c | Evento server-originado: el owner abrió o cerró (`finished`) la sesión presencial (ver §6). |
| `update:attendance_event` | s→c | Evento server-originado: fila del roster de asistencia afectada (ver §6). El frame incluye `channel`. |
| `control:message_created` | s→c | Evento server-originado: un mensaje nuevo en el chat de la sesión (ver §6.3). Payload mínimo, sin contenido; el contenido se recupera por REST con `since`. El frame incluye `channel`. |

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
  "channel": "session:123",
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

## 6. Eventos en vivo de sesión (`update:session_state`, `update:attendance_event`)

Dos eventos server-originados nuevos (change `sesion-interrumpida-y-eventos-en-vivo`), al canal `session:{id}` con `{id}` = session instance id, best-effort asíncrono (mismo patrón de `update:set_event`: no bloquean ni alteran la HTTP; nadie suscripto = no-op; buffer lleno = frame descartado para ese receptor). **Sin exclusión de emisor** — el propio actor recibe su evento si está suscripto.

### 6.1 `update:session_state` — apertura/cierre presencial (Gap 26 D10)

Se emite solo cuando el frame de escritura **muta** el día presencial (no en re-play idempotente ni en `interrupted` del entrenador, que no cierra):

- `POST /session-instances/:id/runner` del owner del team del grupo → abre (setea `presencial_opened_at`; solo si estaba `NULL`).
- `PATCH .../runner` con `{"status":"finished"}` del owner → cierra (setea `presencial_closed_at`; solo si estaba `NULL` — el cierre es final, no hay reopen). `interrupted` del owner NO emite.

```json
{"type":"update:session_state","channel":"session:88","data":{"presencial_open":true,"opened_at":"2026-10-01T18:02:11Z","closed_at":null}}
```

`data` sin `omitempty`: los `null` explícitos (`opened_at`/`closed_at`) son parte del contrato; `presencial_open` es bool real (`opened != NULL && closed == NULL`).

### 6.2 `update:attendance_event` — fila del roster de asistencia (Gap 28 D13)

```json
{"type":"update:attendance_event","channel":"session:88","data":{"user_id":5,"status":"attended","source":"qr","registered_at":"2026-10-01T18:05:00Z","attendance_id":42}}
```

`data` es exactamente **la fila afectada del roster** (igual que en la grilla de asistencia, sin name/email), sin `omitempty` — el borrado viaja con nulls. Alcance por operador:

| Operación | Eventos emitidos |
|---|---|
| Registro QR del corredor | `{"user_id":N,"status":"attended","source":"qr","registered_at":<ts>,"attendance_id":N}` — **solo si el alta es real** (`201`); el `200` idempotente no emite |
| Asistencia manual por lote (bulk) | **un evento por corredor creado O actualizado** (todas las filas del import), `source:"manual"` |
| `DELETE /attendances/:id` | SIEMPRE emite: `{"user_id":N,"status":"not_confirmed","source":null,"registered_at":null,"attendance_id":null}` |

Nota: el `training_session_id` del attendance **es el session instance id** de la FK opaca con el calendario — por eso el canal es el mismo que WS y feedback.

### 6.3 `control:message_created` — mensaje nuevo en el chat de sesión (Gap 27, change `permisos-tier-fees-y-mensajeria-sesion`)

Al crear un mensaje vía `POST /session-instances/:id/messages`, el backend emite a todos los suscriptos de `session:{id}` (sin exclusión de emisor — el propio actor recibe su aviso):

```json
{"type":"control:message_created","channel":"session:88","payload":{"sessionMessageId":42}}
```

- **Payload mínimo {sessionMessageId}, sin contenido:** el frame es un "algo llegó" — el contenido del mensaje (y cualquier otro que se haya perdido offline) se recupera por REST con `GET /session-instances/:id/messages?since=<último id>` (cursado por `id`; el historial completo y su visibilidad por usuario están en `docs/CATALOGO_Y_CALENDARIO.md` §8.12 y `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §13.4).
- **Best-effort, asíncrono** (mismo patrón de los `update:*`): la emisión sale del controller vía `realtime.Notifier` después de persistir y no bloquea ni altera la respuesta HTTP; nadie suscripto → no-op; buffer lleno → frame descartado para ese receptor. No hay replay ni cola.
- Server originado: no lleva `from` ni `to` (regla del §7 aplica solo a presence/control de usuarios).
- Helper: `realtime.MarshalControlMessageCreated` (`realtime/notifier.go`) — el único productor del frame.

## 7. Relay dirigido por `to` en presence/control (Gap 27 D11)

El payload de `presence`/`control` sigue siendo opaco para el backend, pero el campo `to` adentro tiene semántica de entrega:

```json
{"type":"control","payload":{"action":"pace-alert","value":12.5,"to":5}}
```

- **`to` numérico entero en `[1, MaxInt64]`** → entrega **solo a las conexiones de ese user en el canal** (todas sus conexiones; si no está suscripto al canal, nadie lo recibe y la conexión del emisor sigue viva). `to` = propio userID → el emisor **sí se recibe a sí mismo con eco** (única vía de auto-eco: sin `to` el emisor siempre queda excluido).
- **`to` ausente o `"all"`** → comportamiento actual: todos los suscriptos del canal **menos el emisor**.
- **Cualquier otro valor** (`"to":"5"` string, decimal `5.5`, valor > `MaxInt64`, arreglo, booleano…) → broadcast normal (menos emisor) — el parser es estricto: solo el JSON number entero positivo dirige.
- El frame entregado lleva el **payload completo sin mutar**, con `to` adentro — el receptor puede leer a quién apuntaba el mensaje.

Aplica solo a presence/control; los frames `update:*` server-originados no llevan `to`. Cambio no-breaking: sin `to`, el protocolo es idéntico al previo.

## 8. Notas Render / operación

- **Deploy corta conexiones WS:** cada deploy de Render mata las conexiones abiertas → el cliente debe reconectar (con backoff) y re-suscribirse. Las suscripciones no sobreviven a la desconexión.
- **Plan free y spin-down:** un service con mensajes WS activos no spinea down (el tráfico cuenta como actividad). El cold-start habitual post-inactividad (~20-25 s, ver `AGENTS.md` §6) aplica igual: si nadie conectó por un rato, la primera conexión espera al arranque.
- **Single instance → hub en memoria:** el `Hub` (`map canal → set de conexiones`) vive en el proceso de la única instancia; no hay estado compartido externo. Esto acopla el gateway a 1 instancia — asumido a propósito para el plan free.
- **Escalar a múltiples instancias → Redis pub/sub** como mejora futura (deuda documentada en `design.md`, riesgos asumidos): el hub por proceso dejaría de ver los broadcasts originados en otra instancia.
