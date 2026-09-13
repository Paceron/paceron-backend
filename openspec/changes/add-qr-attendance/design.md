# Diseño — Asistencia por QR

## Context

Paceron es una app de running con equipos: los equipos tienen un `owner` (el entrenador, `teams.owner_id`) y miembros (`team_users`). Hoy no existe ningún modelo de sesiones de entrenamiento ni de asistencias. Este change agrega el registro de asistencias vía QR: el entrenador genera un QR por sesión, el corredor lo escanea desde su app, el backend registra la asistencia de forma idempotente, y el entrenador puede buscar asistencias de su equipo.

Stack: Go 1.26 + Gin + GORM/PostgreSQL. Arquitectura en capas (Controllers → Services → DAOs → Infrastructure), DI manual en `app.go`, `AutoMigrate` centralizado en `cmd/api/infrastructure/postgresdb/postgres.go`, tests con `testify` y DAOs con Postgres real vía `testutils.SetupTestDB` (se skipean sin `TEST_DB_HOST`).

Constraints relevantes:
- El spec fija `UNIQUE (team_id, training_session_id, user_id)` como garantía de no-duplicados y los índices `(team_id, training_session_id)` y `(user_id, team_id)` para performance.
- La tabla `training_sessions` NO existe aún. Decisión acordada: `training_session_id` es una **FK opaca** (columna `BIGINT` > 0 como dato, sin constraint a una tabla inexistente). La constraint real se agrega en el change futuro que cree `training_sessions`.
- El QR debe ser **determinista** (mismos inputs → mismo PNG), o el frontend vería QRs que cambian entre refrescos.

## Goals / Non-Goals

**Goals:**
- Tabla `attendances` con índice único compuesto y los dos índices de búsqueda.
- 3 endpoints detrás del `AuthMiddleware`: generación de QR, registro idempotente y búsqueda con matriz de autorización.
- QR determinista y comprobable por tests.
- Autorización de búsqueda basada en la membresía real del sistema (`teams.owner_id` + `team_users`), no en el rol global del usuario.
- Manejo de errores uniforme vía `APIError` (400/401/403/404).

**Non-Goals:**
- Crear la tabla/CRUD de `training_sessions` (change futuro; acá solo se valida que `training_session_id > 0`).
- Edición/borrado/soft-delete de asistencias.
- Estadísticas, exportación o reportes.
- Push notifications al registrar.
- Validar que el usuario YA sea miembro del equipo al registrarse (el QR está pensado para corredores invitados/públicos; el requerimiento solo exige idempotencia por la UNIQUE).

## Decisions

### 1. `training_session_id` como FK opaca, sin `training_sessions`

`attendances.training_session_id` se modela como `int64` `not null`, sin `ForeignKey` en GORM. El único contrato es `> 0` (validado a nivel controller/service). Cuando el change de `training_sessions` llegue, se agrega la constraint `FOREIGN KEY (training_session_id) REFERENCES training_sessions(id)` en su migration.

**Alternativa descartada:** crear ya una tabla `training_sessions` mínima. Se descarta porque el modelo real la definirá otro change; una tabla precaria ahora dispararía una migración (y deuda) sin valor.

### 2. Librería de QR: `github.com/skip2/go-qrcode`

Generación de PNG con `qrcode.Encode(url, qrcode.Medium, 256)` (nivel de corrección y tamaño fijos, sin quiet zone adicional). Esta librería produce bytes idénticos para el mismo contenido + mismos parámetros — no inyecta timestamps ni metadatos. Se fuerza tamaño y corrección constantes para descartar variabilidad.

**Alternativas descartadas:** `boombuler/barcode` (determinista también, pero API más verbosa) y `rsc.io/qr` (menos mantenida). El contrato de determinismo se cubre con un test que genera dos veces y compara bytes.

### 3. URL base embebida en el QR

El QR codifica `<base>/api/v1/attendance/team/{team_id}/session/{training_session_id}`. `<base>` es la URL pública del backend desde el teléfono del corredor, que varía por ambiente. Se agrega una env var `ATTENDANCE_BASE_URL` (default `http://localhost:8080`) leída en `config.go` y seteada en `render.yaml` para producción/testing. Se documenta en `docs/ENVIRONMENTS.md`.

### 4. Registro idempotente: insert directo + captura de la UNIQUE

`attendance_dao.Create` ejecuta `db.Create(&attendance)`. Si GORM devuelve `gorm.ErrDuplicatedKey` (Postgres viola la PK/UNIQUE 23505), el service retorna el flujo "asistencia previamente registrada" (200). Sin `SELECT` previo — evita race conditions y un round-trip extra. Los tests de DAO contra Postgres real verifican el UNIQUE.

### 5. Matriz de autorización en el service, filtros de ownership en el DAO

La evaluación del árbol de decisiones (spec `attendance-qr`) vive en `attendance_service` (lógica de negocio pura, sin HTTP):

- **A — sin params o `user_id == auth_user_id`:** scope `self` → `WHERE user_id = auth_user_id`, aplicando filtros opcionales de `team_id`/`training_session_id` si vinieron.
- **B — solo `team_id`:** se verifica `teams.owner_id == auth_user_id` para ese team. Team inexistente → 404; no-owner → 403. Scope `team`.
- **C — `user_id != auth_user_id`:** se verifica que el `auth_user_id` sea owner de al menos un team al que pertenezca el usuario objetivo. Sin relación → 403. Con relación → scope del usuario objetivo (+ filtros opcionales).

El `attendance_dao` expone operaciones atómicas para resolver la pertenencia, operando sobre las tablas `teams`/`team_users` (tablas existentes):
- `TeamExists(teamID) (bool, error)`
- `IsTeamOwner(teamID, userID) (bool, error)`
- `UserInTeamOwnedBy(targetUserID, ownerUserID) (bool, error)` — `EXISTS(team_users tu JOIN teams t ON t.id = tu.team_id AND t.owner_id = owner AND tu.user_id = target AND tu.deleted_at IS NULL)`
- `Search(filters) ([]Attendance, error)` — WHERE dinámico por los filtros ya autorizados.

**Alternativa descartada:** reutilizar `team_dao` para el ownership. Se descarta para no acoplar el módulo de asistencias al de equipos vía DAO ajeno; la pertenencia que acá se consulta es un caso de uso específico (autorización de asistencias) y los 3 métodos son baratos y autónomos.

### 6. Orden de dependencias en capas

Controller → `attendance_service` → `attendance_dao` + config (base URL). Sin delegates necesarios: ningún endpoint combina dos servicios. El service recibe la base URL desde config en el wiring de `app.go`.

### 7. DTOs del dominio

`domains/attendance/` con structs planos:
- `QRResponse { QRCodeBase64 string; URLEncoded string }`
- `SearchResponse { Data []dbs.Attendance }`
- Mensajes de registro como constantes `"asistencia registrada"` / `"esta asistencia fue previamente registrada"`.

Reutiliza `dbs.Attendance` en la respuesta de búsqueda (misma forma que pide el spec: id, team_id, training_session_id, user_id, created_at).

### 8. Errores

400 → params faltantes o ≤ 0 (QR y search); 401 → el `AuthMiddleware` existente; 403 → violaciones de la matriz; 404 → team inexistente en el scope autorizado. Se usa la estructura `APIError` existente de `domains/apierror`.

## Risks / Trade-offs

- [FK opaca] Si alguien crea `training_sessions` sin recordar la constraint, las asistencias pueden referenciar sesiones inexistentes. → Mitigación: el change futuro que crea la tabla incluye la constraint + backfill; mientras tanto, solo valores `> 0` como contrato. Queda anotado en tasks y en CLAUDE.md como quirk.
- [Determinismo de librería de QR] Cambiar de versión de `skip2/go-qrcode` podría alterar bytes. → Mitigación: el test de determinismo compara exactamente los mismos parámetros; el upgrade queda cubierto por ese test.
- [Registro sin membresía] Un corredor puede registrarse a una sesión sin ser miembro del equipo. Aceptado a propósito (non-goal); se revisa cuando exista `training_sessions`.
- [Owner de equipo] El criterio de autorización B/C es `teams.owner_id` (único owner por equipo), consistente con el resto del sistema. Si en el futuro hay co-owners, se revisa la matriz.
- [Búsqueda sin límites] `Search` no pagina; con muchas asistencias podría degradarse. → Los índices del spec cubren el costo; paginado queda como mejora futura si aparece.

## Migration Plan

- Deploy: commit → CI verde → merge a `develop` → deploy de staging. La tabla se crea sola vía `AutoMigrate` (agregar `dbs.Attendance{}` a la lista en `postgres.go`) y el `UNIQUE`/índices se definen en las tags GORM (GORM los genera en Postgres).
- Rollback: revertir el commit; como la tabla es nueva y solo la usa este módulo, no hay datos que preservar. Si hubiera filas, se conservan (la tabla no se dropea automáticamente).
- Config: setear `ATTENDANCE_BASE_URL` en `render.yaml` (prod/testing) antes del deploy; default local cubre desarrollo.

## Open Questions

- ¿El frontend espera la imagen PNG directa (`image/png`) o el base64? Hoy se asume base64 en JSON (`qr_code_base64`) según el spec; es trivial servir `image/png` raw si el frontend lo prefiere.
- ¿Se valida que `user_id == auth_user_id` en el POST cuando el acceso es por QR de otro usuario? No: el QR está pensado para que cada corredor escanee con SU sesión; el `user_id` del POST siempre sale del token, nunca de un parámetro, así que no hay superficie de spoofing.