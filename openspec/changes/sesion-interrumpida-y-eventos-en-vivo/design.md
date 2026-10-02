# Design: sesion-interrumpida-y-eventos-en-vivo

> **Idioma:** documentación en español.

Decisiones por gap. Los errores de negocio siguen la convención del repo: sentinels en el service, mapeo a HTTP en el controller.

---

## Gap 25 — fix de comparación fecha/timestamp (etapa 1)

**D1 (fix de lectura).** `activeGroupMemberWhere` (`cmd/api/daos/group_user_dao.go`) cambia a comparación por fecha:

```sql
date_start::date <= ?::date
AND (date_end IS NULL OR date_end::date >= ?::date)
```

- El parámetro sigue siendo el `time.Time` de la fecha de sesión; el cast `?::date` lo trunca a día. Un solo helper cubre roster (`FindGroupRosterWithAttendance`), `IsActiveGroupMember` y `MissingGroupMembers`.
- NO se normaliza la escritura de `date_start` (sigue `time.Now()`): el fix de lectura cubre filas existentes y nuevas. Menor costo de índice no relevante (tabla chica, sin índice sobre date_start).
- Test de repro: fixture con grupo cuya membresía de un corredor arranca hoy con hora > 00:00 → DEBE aparecer en el roster y NO ser rechazado por `IsActiveGroupMember`/`MissingGroupMembers`. Con el código viejo, ese caso falla (es el escenario del gap).

---

## Gap 19 — estado `interrupted` (etapa 2)

**D2 (transiciones).** `RunnerStatusRequest.Status` admite `finished` e `interrupted`. Máquina:

```
wip ──► finished     (ya existe, idempotente)
wip ──► interrupted  (nuevo)
interrupted ──► finished  (explícita; re-setea end_date = now del server)
finished ──► X       NUNCA (400 ErrRunnerSessionInvalid)
interrupted ──► interrupted  idempotente (200 sin cambios)
fila inexistente → 404 (igual que hoy)
```

- `end_date` la setea el servidor en AMBOS destinos (interrupted y finished), incluida la transición `interrupted→finished` (momento real del finish explícito).
- Matriz de autorización sin cambios: self por default, `athlete_user_id` ajeno solo si el auth es owner de un equipo al que pertenece el atleta (misma `resolveAthlete`).
- Mensajes: constante nueva `MsgRunnerSessionInterrupted = "sesión marcada como interrumpida"`; `MsgRunnerSessionFinished` queda igual. `finished→interrupted` devuelve 400 con mensaje "no se puede interrumpir una sesión ya finalizada".
- Sin migración: `status` es string sin check constraint. Sin constantes de estado nuevas en `domains/constants` (el módulo usa literales `"wip"`/`"finished"`/`"interrupted"` — se extraen a const locales del paquete si el review lo pide, sin cambio de comportamiento).

**D3 (DAO generalizado).** `Finish(ctx, r)` (guard `WHERE status='wip'`) se reemplaza por una transición genérica:

```go
// UpdateStatus marca la fila con el nuevo status y end_date del server,
// solo si el status actual está en fromStatuses (guard anti-transición
// ilegal a nivel SQL, mismo rol que el WHERE status='wip' de hoy).
UpdateStatus(ctx, runnerSessionID int64, to string, fromStatuses []string, endDate time.Time) error
```

- El service resuelve la fila (`GetBySessionAndAthlete`) igual que hoy y decide: idempotencia (status destino == actual → devolver sin cambios), transición legal (llamar UpdateStatus), ilegal (400). El re-GET después del update también queda como hoy.
- No rejugable: el POST `/runner` idempotente de una fila `interrupted` devuelve la fila existente sin reabrir (igual que con finished hoy).

---

## Gap 23 — photo_url (etapa 3)

**D4 (DTO).** `SearchResultItem` suma `PhotoURL *string json:"photo_url"` (nullable: `null` si no hay foto). Mapeo con `buildMediaURL(u.PhotoKey, u.PhotoUpdatedAt)` (misma URL pública con `?v=` que ya usa el update response) en los DOS callers: `Search` y `BatchLookup`. `SearchActive` y `FindByIDs` ya cargan el modelo completo — no toca DAO.

---

## Gap 26 — sesión abierta/cerrada (etapa 4)

**D5 (modelo).** `GroupCalendarDay` suma 2 columnas nullable (AutoMigrate las agrega, sin backfill):

```go
PresencialOpenedAt *time.Time `gorm:"column:presencial_opened_at"`
PresencialClosedAt *time.Time `gorm:"column:presencial_closed_at"`
```

- Estado abierto = `opened_at != NULL AND closed_at == NULL`. Cerrado = `closed_at != NULL` (final, NO hay reopen). No presencial / sin apertura = no abierto.
- El estado muere con el día: si el día se reasigna, la instancia vieja se borra (D10 existente) y el día nuevo arranca sin apertura.

**D6 (lookup).** Método nuevo en `GroupCalendarDayDAOInterface`:

```go
// FindBySessionInstanceID devuelve el día de calendario que referencia
// esa instancia (relación 1:1 por diseño; puede ser nil si la instancia
// quedó huérfana de feedback sin día, o si el id no existe).
FindBySessionInstanceID(ctx *gin.Context, sessionInstanceID int64) (*dbs.GroupCalendarDay, error)
```

- Lo consumen: el detalle (D8), los hooks de apertura/cierre (D7) y el gate (D9).

**D7 (hooks de apertura/cierre).** En el controller de runner session (es donde vive el hook, el service de runner session NO conoce calendario):

- **Apertura:** al crear estado (`POST /runner`), si el auth es **owner del team** del grupo del día (mismo criterio que Gap 10/next-presencial-session — no existe "staff entrenador" separado) Y el día es `kind=training` + `is_presencial=true` → setear `presencial_opened_at = now` si es NULL (idempotente; si el owner ya tenía fila wip y re-Play, solo setea si NULL).
- **Cierre:** al PATCH `finished` del owner sobre día presencial → setear `presencial_closed_at = now` si es NULL.
- **`interrupted` del owner NO cierra** la sesión (solo `finished` la cierra).
- Ambos seteos van dentro de la MISMA transacción del negocio correspondiente o, si el write de runner session no es transaccional, como update aparte inmediato después (el write de runner session es un Updates directo, no tx — el update del día va después del éxito del primero; si el update del día falla, el error sube y el estado de runner session ya quedó — se acepta: es consistente con que el Play del corredor ya creó su fila).
- El corredor NO-owner no abre ni cierra nada (solo el owner dispara los hooks; el gate D9 es el único efecto sobre corredores).
- Al abrir/cerrar se emite el evento WS de D10.

**D8 (detalle).** `GET /session-instances/:id` (Gap 14) suma 3 campos al response de detalle, con `omitempty` para NO contaminar las respuestas de calendario que comparten `SessionInstanceResponse`:

```go
PresencialOpen  *bool      `json:"presencial_open,omitempty"`   // solo detalle
PresencialOpenedAt *time.Time `json:"opened_at,omitempty"`
PresencialClosedAt *time.Time `json:"closed_at,omitempty"`
```

- `presencial_open` = `opened_at != NULL && closed_at == NULL` (bool real, `false` si cerrada/no abierta). En día no presencial o instancia huérfana (sin día): los 3 campos quedan fuera del JSON (nil).
- El detalle los resuelve vía D6 (día por instancia). Los paths de calendario (`GetRange`/`NextSession`/member-calendar) NO los setean.

**D9 (gate del corredor).** En `Create` (POST `/runner`) del corredor NO-owner sobre día `training`+`presencial`:

- `opened_at == NULL` → error 409, slug `session_not_opened`.
- `closed_at != NULL` → error 409, slug `session_closed`.
- Slugs viajan en el campo `Code` del `APIError` (patrón mp-connect del frontend: nunca se muestra el slug crudo, se mapea a copy).
- Owner exento (su Create ES la apertura). Sesiones no presenciales o días no training: sin gate (comportamiento actual).
- El orden de checks: 404 instancia inexistente → gate de apertura → 403 autorización atleta. (El 403 de resolveAthlete corre antes si el atleta target es ajeno — decide el orden natural del service: existe sesión → resolveAthlete → gate presencial.)

**D10 (WS update:session_state).** Al abrir y al cerrar se emite al canal `session:{id}` (id = session instance id, mismo canal del Gap 18):

```json
{"type":"update:session_state","data":{"presencial_open":true,"opened_at":"...","closed_at":null}}
```

- Sin exclusión del emisor (consistente con `update:set_event`). `presencial_open` refleja el estado POST-write. Wiring: notifier en runner session controller (patrón workout-feedback Create: notifier nil-safe, en tests nil).

---

## Gap 27 — relay dirigido (etapa 5)

**D11 (to en payload).** El relay de `presence`/`control` inspecciona el payload (ya exigido objeto JSON):

- `payload.to` numérico (userId) → el frame va SOLO a las conexiones de ESE usuario suscriptas al canal (todas sus conexiones — puede haber 2 dispositivos). Mismo frame `{type, from, payload}` completo (incluye el `to`).
- `payload.to == "all"` o ausente → comportamiento actual (todos los suscriptores menos el emisor).
- `payload.to` de otro tipo (string no-"all", null explícito) → se ignora el campo y va a todos (tolerante; el payload sigue opaco).
- Autorización: sin cambio — el emisor debe estar suscripto al canal (ya garantizado). Un user puede dirigir a cualquiera de la sala: son participantes de la misma sesión.
- Implementación: el hub expone targeting por userID (método tipo `BroadcastToUser(channel, frame, targetUserID)` que reutiliza la mecánica de Broadcast no-bloqueante); el connection.go decide entre broadcast-dirigido y broadcast-normal según `payload.to`. Sin persistencia: fire-and-forget igual que hoy (deuda registrada en DEUDA_TECNICA_Y_PENDIENTES.md).

---

## Gap 28 — evento de asistencia (etapa 6)

**D12 (frame).** `update:attendance_event` al canal `session:{sessionInstanceId}`. Payload = EXACTO la fila del roster (`SessionAttendanceRow` menos name/email):

```json
{"type":"update:attendance_event","data":{"user_id":7,"status":"attended","source":"qr","registered_at":"...","attendance_id":42}}
```

- Delete: `{"user_id":7,"status":"not_confirmed","source":null,"registered_at":null,"attendance_id":null}` (el status infiere la baja; sin flag `removed`).
- Sin exclusión del emisor.

**D13 (emisión por operación).**

- **Register (QR):** SOLO si `created=true` (el 200 idempotente no emite: no hay cambio de estado). `source:'qr'`.
- **Bulk (manual):** 1 evento por usuario afectado (created o updated — cada uno patchea su fila), `source:'manual'`, en el orden del lote.
- **Delete:** siempre que el borrado tenga éxito.
- El wiring del notifier va en el attendance controller (patrón workout-feedback Create; notifier nil-safe para tests).

**D14 (datos para el payload).** Para armar la fila exacta:

- `Register`: el service pasa a devolver la fila creada (o al menos `attendance.ID` + `CreatedAt`) — hoy devuelve solo `created bool + sessionCtx`.
- `BulkUpsertManual`: extiende su `RETURNING` (hoy `(xmax=0) AS inserted`) a `user_id, id, created_at, inserted` y el DAO devuelve las filas afectadas con su flag inserted/updated (los counts se derivan en Go, mismo resultado para `BulkSaveResult`).
- `DeleteAttendance`: el service lee la fila por id (ya la necesita para autorizar o la lee antes del borrado) y expone al controller el `user_id` + `training_session_id` (canal) de la fila borrada.

---

## Verificación (transversal, etapa 7)

- Postgres real (`docker start paceron-test-db` :5433, env `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`).
- `openspec validate sesion-interrumpida-y-eventos-en-vivo --strict` → valid.
- Suite completa `go test -count=1 ./...` → 0 FAIL.
- Coverage: `go clean -cache` + `make coverage-with-db` + analyzer (`go run github.com/vladopajic/go-test-coverage/v2@latest --config ./.testcoverage.yml --profile ci/test_coverage/coverage.out`) → gate 85 PASS, `.testcoverage.yml` intacto. Bug de cache conocido: sin clean el número puede salir falso ~70%.
- gofmt/build/vet limpios en archivos tocados. Swagger regenerado con `swag init -g cmd/api/main.go -d . -o cmd/api/docs --parseDependency --parseInternal` donde aplique.
- Comentarios mínimos; Conventional Commits; stagear solo rutas explícitas (nada de `.superpowers/`).
