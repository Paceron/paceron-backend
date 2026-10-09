# Proposal: permisos-tier-fees-y-mensajeria-sesion

## Por qué

Cinco gaps del frontend (docs `BACKEND_API_GAPS.md` del repo frontend), confirmados contra el código de develop en una sola ronda de análisis:

- **Gap 5**: no hay `GET /tiers/{id}/permissions` — la pantalla "Mejorar tier" no puede listar los permisos/beneficios de un tier al que el usuario todavía no accedió (hoy usa el `description` del tier como único contenido).
- **Gap 15**: `membership_fee` falta en `GET /teams/search` y en las invitaciones — la pantalla de pago necesita fan-out de hasta 20 requests extra por página de búsqueda para resolver el precio.
- **Gap 16**: `POST /payments/preference` colapsa TODO error a 500 genérico — el frontend no puede explicarle al corredor por qué falló un pago (ej. 409 `SELLER_NOT_CONNECTED` nunca llega).
- **Gap 17**: no hay forma de saber si el entrenador de un equipo ajeno tiene Mercado Pago conectado (capacidad de cobrar) antes de unirse — solo se entera cuando falla el pago.
- **Gap 27**: mensajería en sesión presencial en vivo — persistencia de mensajes + 2 endpoints REST + aviso WS sin contenido. Spec de frontend ya escrita (`2026-10-09-live-session-messaging-design.md` del repo frontend), todas las dudas de contrato ya respondidas por el frontend.

## Qué cambia

1. **Gap 5**: endpoint `GET /api/v1/tiers/:id/permissions` (lista de permisos de un tier, con nombre del permiso; cualquier autenticado, igual que los otros GET de tiers).
2. **Gap 15+17**: `membership_fee` (vigente del equipo, no congelado) y `can_receive_payments` (bool derivado de seller_connections: conexión authorized con public key) en `TeamSearchResult` (search) y `InvitationResponse`; `can_receive_payments` también en `TeamResponse` (detalle). Fee vigente: el único congelado real es `init_amount` al momento del pago (fuera de alcance).
3. **Gap 16**: sentinels en el service de payments + mapeo en el controller: 409 con `code:"SELLER_NOT_CONNECTED"`, 404 cuota/equipo inexistente, 400 lo ya validado, resto 500 genérico (sin 502; frontend confirma que 500/502 muestran el mismo mensaje vago).
4. **Gap 27**: tabla `session_messages` + tabla join `session_message_recipients`; `POST /session-instances/:id/messages` (autorización dual existente vía `HasInstanceAccess`; `sender_role` derivado del owner del team; validación de type/recipient_mode/destinatarios/reply) y `GET /session-instances/:id/messages?since=` (visibilidad filtrada server-side, orden cronológico, sin paginación). Al persistir un POST exitoso, broadcast WS `control:message_created` (frame `{type:"control:message_created", channel:"session:{id}", payload:{sessionMessageId}}`, sin contenido — la privacidad la aplica el filtro del GET).

## Impacto

- Aditivo en todos los casos salvo Gap 16 (que corrige códigos de error — comportamiento nuevo, no breaking de shapes existentes: el 500 sigue existiendo para causas no identificables).
- Nueva tabla + tabla join (AutoMigrate, sin backfill).
- Contratos ya confirmados con el frontend en la ronda de dudas (literal del frame, shape de mensaje, nombres de campos, fee vigente, códigos de error).

## Fuera de alcance

- Edición/borrado de mensajes ya enviados (spec frontend lo excluye).
- Push nativo si el destinatario no está conectado (extensión futura posible).
- `init_amount` congelado en invitaciones (el fee mostrado es el vigente).
- Persistencia de frames del bus WS ni replay por reconexión (el catch-up es el `GET ?since=` del frontend).
- Cambios al relay genérico del gateway (el aviso `control:message_created` es emisión server-originated, mismo patrón que `update:set_event`).
