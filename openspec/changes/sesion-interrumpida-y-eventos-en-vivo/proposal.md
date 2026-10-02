# Proposal: sesion-interrumpida-y-eventos-en-vivo

## Why

Ronda de gaps del frontend (`BACKEND_API_GAPS.md` 19, 23, 25, 26, 27, 28) para completar el sub-proyecto de sesión en vivo. Los seis se resuelven en una sola rama, por etapas:

- **Gap 19** — `runner_session` necesita un tercer estado `interrupted`: el corredor cancela (termina temprano) sin deshacer su feedback de series ya hechas, y la fila no es rejugable. Hoy PATCH solo admite `finished`.
- **Gap 23** — `SearchResultItem` (compartido por `GET /users` search y `GET /users?ids=` batch lookup) no trae `photo_url`; el roster del frontend ya lo consume (`photoUrl: user.photo_url ?? null`).
- **Gap 25** — Bug confirmado con repro SQL: el roster de `GET /attendance/session/{id}` compara `group_users.date_start` (timestamptz, con hora) contra la fecha de la sesión (DATE a medianoche) → los corredores con membresía iniciada el MISMO día de la sesión quedan excluidos de la grilla (y el mismo helper rechazaría registrarles asistencia). Es un bug de datos activo, va primero.
- **Gap 26** — Sesión presencial controlada por el entrenador: su Play abre la sesión y su slide-to-finish la cierra (persistido en el servidor, no solo mensaje WS efímero). El corredor solo puede unirse entre apertura y cierre, y un corredor que abre el pre-start tarde consulta el estado sin depender de WS vivo.
- **Gap 27** — El relay WS no soporta mensajes dirigidos: se agrega respeto de `payload.to` (userId) en `presence`/`control` para entregar solo a las conexiones de ese usuario. La persistencia de mensajes críticos queda fuera (deuda).
- **Gap 28** — Sin evento WS para asistencia: se emite `update:attendance_event` al canal `session:{id}` en registro por QR, carga manual y borrado, con payload = fila exacta del roster.

## What Changes

- `runner_session`: PATCH admite `status: 'interrupted'` con transiciones `wip→interrupted`, `interrupted→finished` (explícita, re-setea `end_date=now`), `finished→X` nunca (400); `interrupted→interrupted` idempotente. Mismo shape de respuesta y misma matriz de autorización.
- `users`: `photo_url` nullable en `SearchResultItem` (search + batch lookup), armada con la misma URL pública `?v=` que ya usa el update response.
- `attendance`: fix de lectura en el helper de membresía (`date_start::date <= fecha sesión::date`, idem `date_end`) — cubre roster, `IsActiveGroupMember` y `MissingGroupMembers` con un solo cambio.
- `group_calendar_day`: columnas `presencial_opened_at` / `presencial_closed_at` (nullable). El owner del equipo del grupo abre al hacer POST `/runner` (su Play) y cierra al PATCH `finished`. El corredor no-owner no puede crear su estado de sesión en un día presencial si no está abierta (409 `session_not_opened`) o ya se cerró (409 `session_closed`). `GET /session-instances/:id` (Gap 14) suma `presencial_open` / `opened_at` / `closed_at` (omitempty, solo en ese endpoint). Evento WS `update:session_state` al abrir/cerrar.
- `realtime-gateway`: `presence`/`control` con `payload.to` numérico se entregan SOLO a las conexiones de ese usuario suscriptas al canal; `to: 'all'` o ausente mantiene el comportamiento actual.
- `attendance`: eventos WS `update:attendance_event` con payload = fila de roster exacta (`{user_id, status, source, registered_at, attendance_id}`) en Register (solo si creó), Bulk (1 evento por usuario) y Delete (fila en `not_confirmed` con nulls).

## Impact

- **Capabilities afectadas:** `runnersession` (nueva), `users` (nueva), `attendance` (nueva), `group-calendar` (existe), `realtime-gateway` (existe).
- **Breaking:** ninguno duro — todo es aditivo o fix de bug; los códigos 409 nuevos (`session_not_opened`/`session_closed`) y el evento nuevo se coordinan con el frontend, que ya confirmó los shapes.
- **Sin migración de datos:** AutoMigrate agrega las 2 columnas nullable nuevas; no hay backfill.
- **Fuera de alcance:** sumarización de historial, persistencia/replay de mensajes WS críticos (deuda), deprecar `GET /workout-feedback/search`, normalizar la escritura de `date_start`.

## Stages (orden de implementación)

1. **Gap 25** (fix del bug de roster — hot, con test de repro).
2. **Gap 19** (`interrupted`).
3. **Gap 23** (`photo_url`).
4. **Gap 26** (open/close + gate + detalle + WS).
5. **Gap 27** (relay dirigido).
6. **Gap 28** (evento de asistencia).
7. Docs + verificación final.
