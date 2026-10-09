# Design: permisos-tier-fees-y-mensajeria-sesion

Decisiones confirmadas con el frontend (ronda de dudas 2026-10-09). Todas las rutas nuevas viven bajo `/api/v1/` autenticado, salvo indicación.

## D1 — Gap 5: GET /tiers/:id/permissions

- Ruta: `GET /api/v1/tiers/:id/permissions` → `TierPermissionController.List`.
- Autorización: cualquier usuario autenticado (mismo criterio que `GET /tiers/:id` — el front lo usa sobre tiers ajenos a propósito).
- Service: usa `FindByTierID` existente (filtra `deleted_at IS NULL`) + `permissionDao` para resolver nombres. 404 si el tier no existe (alineado con GetByID); 200 `[]` si el tier existe sin permisos.
- Shape respuesta: `{"permissions": [{"permission_id": <int>, "permission_name": "<string>"}]}` ordenado por `permission_id` ASC (determinista).
- No filtra por jerarquía ni estado del usuario — es un listado descriptivo.

## D2 — Gap 15: membership_fee en search e invitaciones (fee vigente)

- `TeamSearchResult`: suma `membership_fee float64 json:"membership_fee"` (el DAO `SearchPublic` ya trae el modelo completo del team — el campo existe en la columna `teams.membership_fee`).
- `InvitationResponse`: suma `membership_fee` — el service ya carga el team para `team_name` (builder `invitation_service.go:343`), fee tomado de esa misma fila. Es el fee VIGENTE del equipo al consultar (no el frozen de la invitación; el único congelado real es `init_amount` del pago).
- Todos los DTOs que exponen `team_name` de invitación comparten la shape (una sola clave, sin omitempty).

## D3 — Gap 17: can_receive_payments (capacidad, no consentimiento)

- Definición: `seller_connections` del owner del team con `status = authorized` Y `public_key != ''` — el mismo criterio que `resolveTeamSplitConfig` usa para decidir que un pago puede crearse.
- Derivado en service (NUNCA columna nueva): 1 lookup por owner.
- Endpoints: `TeamSearchResult` (batch: 1 query `seller_connections WHERE user_id IN (owners) AND client_id = config.MyMP.ClientID?` — cuidado: `FindByUserAndClient` usa `client_id`; si hay varias conexiones por owner, authorized en cualquiera alcanza) + `TeamResponse` (detalle, single) + `InvitationResponse` (single, mismo owner del team de la invitación).
- Nombre confirmado por frontend: `can_receive_payments bool` (nullable no; bool plano, default false).
- Al fallar el lookup de seller_connections, flag = false (capacidad "desconocida" se trata como false; el 500 real no puede depender de un flag informativo).

## D4 — Gap 16: mapeo de errores de CreatePreference

- Sentinels nuevos en `services/payment_service.go`:
  - `ErrPaymentSellerNotConnected` (409 + `code:"SELLER_NOT_CONNECTED"` — constante ya existe en `constants/error_code.go`, mismo patrón que mp-connect).
  - `ErrPaymentInstallmentNotFound` / `ErrPaymentTeamNotFound` (404).
  - `ErrPaymentInvalid` (400 — ej. `installment_id requerido para team_subscription`, `la cuota no pertenece a un equipo`).
- Body de error: `apierror.APIError` con `status_code`/`code`/`message` (el frontend confirma que su `services/api.js` ya expone `error.data` completo).
- Errores restantes (upstream MP, DAO): siguen 500 con mensaje genérico "Error al crear la preferencia" (sin 502 — confirmado con frontend).
- Los mensajes actuales del service se preservan como `message` del 409/404 (el front muestra causa específica cuando hay code).

## D5 — Gap 27: modelo session_messages

- Tabla `session_messages`: `id` PK, `session_instance_id` (int64, no FK física — patrón opaque del repo), `sender_user_id` int64, `sender_role` string (`trainer`|`runner`), `type` string (`info`|`aviso`|`alerta`), `recipient_mode` string (`all`|`multiple`|`direct`), `body` text, `reply_to_message_id` int64 nullable, `created_at`.
- Tabla join `session_message_recipients`: `message_id` int64 + `user_id` int64 (PK compuesta; vacía si `recipient_mode = 'all'`). Decisión interna confirmada con el front (no cambia su contrato: ve `recipient_user_ids: number[]`).
- `sender_role` derivado al persistir: `'trainer'` si el emisor es owner del team del grupo del día de la instancia; `'runner'` en cualquier otro caso.
- AutoMigrate de ambas tablas, sin backfill (tabla nueva).

## D6 — Gap 27: autorización y validaciones

- POST y GET: emisor/consultante debe tener acceso a la instancia vía `HasInstanceAccess` (dual: miembro activo del grupo del día u owner del equipo, O feedback activo como atleta/reportante/owner) — la misma regla que ya protege el resto de los endpoints de la sesión y el authorizer del canal WS. 403 en otro caso; 404 si la instancia no existe.
- Validaciones POST (400):
  - `type` ∈ {info, aviso, alerta}; `recipient_mode` ∈ {all, multiple, direct}.
  - `recipient_mode = all` → `recipient_user_ids` vacío/ausente.
  - `recipient_mode = direct` → exactamente 1 id; `multiple` → ≥ 2 ids.
  - Cada destinatario debe pasar `HasInstanceAccess` (participante de la sesión).
  - `reply_to_message_id` (si viene): mensaje existente, de la MISMA sesión, y visible para el emisor según la regla de visibilidad.
- Cuerpo `body` no vacío (trim), tope razonable (ej. 2000 chars) para no convertir el chat en blob.

## D7 — Gap 27: visibilidad del GET

- `GET /session-instances/:id/messages?since=<id>`: mensajes con `id > since` (omitido/0 = todo el historial) que el consultante ve: es el emisor, O `recipient_mode = 'all'`, O su userId está en recipients.
- Orden `id ASC` (cronológico), sin paginación (solo cursor `since`).
- Respuesta: `{"messages": [ <mensaje serializado> ]}` — el front no re-filtra nada.

## D8 — Gap 27: shape del mensaje (confirmado con el front)

```json
{
  "id": 9, "session_instance_id": 85, "sender_user_id": 37, "sender_role": "trainer",
  "type": "aviso", "recipient_mode": "multiple", "recipient_user_ids": [7, 12],
  "body": "Falten 5 minutos, agrupense en el puente",
  "reply_to_message_id": null, "created_at": "2026-10-09T..."
}
```
- En la respuesta del POST (mensaje creado) y en cada ítem del GET.
- `recipient_user_ids: []` cuando `all`.

## D9 — Gap 27: broadcast WS

- Tras POST exitoso: `Notifier.Emit(sessionChannel(sessionInstanceID), MarshalControlMessageCreated(messageID))` — nil-safe, async, sin exclusión de emisor (patrón exacto de `update:set_event` en workout_feedback Create).
- Frame exacto confirmado por el front: `{"type":"control:message_created","channel":"session:{id}","payload":{"sessionMessageId":<id>}}` — un solo string literal (no el relay cliente-a-cliente).
- El POST responde 201 con el mensaje creado (igual que cualquier POST del resto de la app); la emisión no cambia la respuesta HTTP.
- Notifier compartido de app.go, wiring en el controller nuevo de mensajes.

## D10 — testing y verificación

- DAO session_messages contra Postgres real (fixture con la misma base de las demás suites: user/team/group/day/instancia).
- Service: autorización (403/404), derivación de sender_role, validaciones, visibilidad (emisor/all/recipients), reply inválido.
- Controllers: mocks, codes 200/201/400/403/404; Gap 16: mapeo de sentinels; Gap 5: shapes.
- Verificación final: `openspec validate --strict`, build/vet/gofmt, suite completa con Postgres real, coverage gate 85 con `go clean -cache` + analyzer.
- Sin Swagger nuevo para el WS frame (no va al Swagger — patrón wsUpgrade/update:session_state).
