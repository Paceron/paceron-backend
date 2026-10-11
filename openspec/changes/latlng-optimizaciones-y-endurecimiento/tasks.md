# Tareas: latlng-optimizaciones-y-endurecimiento

## Restricciones globales

- **Cero cambios de contrato** fuera de: los campos `lat`/`lng` nuevos
  (aditivos) y los mensajes de charset actualizados para listar los
  caracteres admitidos. Las respuestas del calendario (stamp/bulk/shift con
  wrapper `/days`,`same_team_warnings`, individual plano, 409/422/400)
  deben quedar byte-idénticas; acotado: no tocar DTOs ni tags json de
  calendario ni respuestas de BulkSaveResult.
- `UpsertDay` de fila única NO se toca (mantiene upsert + `sameNullableInt`
  + hooks del controller). Los paths mock `s.db == nil` de los recorridos
  migrados NO se tocan (sin N+1, comportamiento igual).
- Cada partida de tests debe correr con `-count=1` contra Postgres real
  (`docker start paceron-test-db` :5433, env
  `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres
  TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`).
- Branch actual: `feature/latlng-optimizaciones-y-endurecimiento`.
- Commits Conventional Commits; stagear SOLO rutas explícitas del task
  (jamás `git add -A`/`.` ni `.superpowers/`); comentarios de código
  mínimos; NO pushear ni mergear (lo hace el usuario).
- Swagger: regenerar con `swag init --parseDependency -g cmd/api/docs.go
  --output cmd/api/docs` (comando canónico; evita churn de metadata).
- Coverage gate 85: al final, `go clean -cache` + `make coverage-with-db` +
  analyzer `go run github.com/vladopajic/go-test-coverage/v2@latest
  --config ./.testcoverage.yml --profile ci/test_coverage/coverage.out`;
  `.testcoverage.yml` intacto.
- Imports internos con módulo `simple-arq-golang`.

### Task 1: Gap 29 — lat/lng del usuario + charset

**Files:**
- Modify: `cmd/api/domains/dbs/user.go` (Lat/Lng *float64 columnas),
  `cmd/api/domains/auth/*.go` (RegisterRequest/RegisterResponse/response
  de auth user), `cmd/api/domains/user/*.go` (UpdateUserRequest/response),
  `cmd/api/services/auth_service.go` (validación rango + charset regex +
  seteo en el create), `cmd/api/services/user_service.go` (ídem en update
  y al mapear response), `cmd/api/controllers` (solo si un handler completa
  el DTO en algún lugar — sincronizar mapeo).
- Test: `cmd/api/services/auth_service_test.go`,
  `cmd/api/services/user_service_test.go`, `cmd/api/services/exercise_service_test.go`
  (solo si toca el paquete), `cmd/api/services` según alcance.
- Swagger: regenerar los 3 artefactos tras las anotaciones.

**Steps:**

- [x] **1.1** DB: columnas `lat`/`lng` en el modelo `User` (nullable,
  sin backfill). Verificar `AutoMigrate` con Node real vía `SetupTestDB`.
- [x] **1.2** DTOs: aceptar `lat`/`lng` en registro y update de usuario;
  devolverlos en las respuestas correspondientes (nullable, sin omitempty
  si la convención que use hoy la dirección dice sobre campos nulos de dirección: nullable sin omitempty).
- [x] **1.3** Validación: rango lat/lng (D2); charset relajado en los 5
  regex de dirección del update y en los 4 del registro (D3), con mensajes
  actualizados y asertados textualmente.
- [x] **1.4** Tests: rango (2 casos, lat/lng), charset (nombres con
  guion/punto/apóstrofe/coma EN pasan y se guardan; payload SQLi en country
  sigue 400 con el mensaje nuevo — pin de doble uso).
- [x] **1.5** Swagger anotado + regenerado; `go build/vet`, suite del
  paquete con DB real; commit
  `feat(users): ubicacion con lat/lng y charset de direccion (Gap 29)`.

### Task 2: Gap 29 — lat/lng del equipo

**Files:**
- Modify: `cmd/api/domains/dbs/team.go` (Lat/Lng),
  `cmd/api/domains/team/create_team_request.go`,
  `update_address_request.go`, `team_response.go`,
  `cmd/api/services/team_service.go` (Create/CreateInternal/update
  address/response mapeo + validación rango).
- Test: `cmd/api/services/team_service_test.go` (y el de invitation si un
  mapeo se reutiliza — NO cambia el shape de invitaciones).
- Swagger: regenerar.

**Steps:**

- [x] **2.1** DB: columnas de `Team` (ídem 1.1).
- [x] **2.2** DTOs de team lat/lng request/response en create + update
  address + respuestas.
- [x] **2.3** Validación de rango (ídem 1.3 — solo rango; teams NO
  aprovecha charset hoy y no se toca).
- [x] **2.4** Tests: create con punto, update address sin punto no toca
  lat/ng, fuera de rango 400, response nullable.
- [x] **2.5** Swagger + build/vet/suite; commit
  `feat(teams): ubicacion con lat/lng del equipo (Gap 29)`.

### Task 3: Rendimiento — upsert multi-fila y batch de instanciación

**Files:**
- Modify: `cmd/api/daos/group_calendar_day_dao.go` (`UpsertMany` con
  RETURNING — D5); `cmd/api/daos/session_instance_dao.go` /
  `exercise_instance_dao.go` si falta batch insert con RETURNING de los
  datos que necesita el caller; `cmd/api/services/calendar_service.go`
  (Stamp $ Bulk write path — D6; Shift usa el upsert batch si aplica).
- Test: los de service/persistence ya existentes se corren SIN ajustar
  aserciones; tests nuevos de `UpsertMany` (conflictí/limpieza presencial
  con Postgres real o mocks como hoy).

**Steps:**

- [x] **3.1** `UpsertMany` + limpieza presencial en SQL (D5), con test de
  mapeo orden→id y del CASE de `presencial_opened_at`/`closed_at`
  (activación/cambio de instancia limpia, re-activación de la misma
  instancia conserva).
- [x] **3.2** Instanciación batch por día (D6): sesiones/links/ejercicios
  de catálogo con `IN`, inserts multi-fila con RETURNING y map índice→id;
  `ErrCalendarSessionNotFound` y demás errores idénticos al path de hoy.
- [x] **3.3** Migrar los write paths de Stamp + Bulk (+ Shift si su loop
  hoy usa `Upsert` per-day) al batch; `UpsertDay` intacto.
- [x] **3.4** Tests: extend del test existente `TestCalendarService_GetRange_
  BatchDetalleQueries` con un test de conteo para stamp de 4 días
  (queries esperadas exactas); regresión: assembling/rollback/keep-con-
  feedback/cerradas 422 sin cambiar aserciones.
- [x] **3.5** Commit `perf(calendar): stamp y bulk con escritura batch`.

### Task 4: Rendimiento — borrado batch de instancias y bulk-clear

**Files:**
- Modify: `cmd/api/services/calendar_service.go` (`deleteSupervisedInstances`
  batch D6c reutilizado por Stamp/Bulk/Shift/bulk-clear),
  `cmd/api/daos/workout_feedback_dao.go` y los DAOs de instancia si falta
  un IN para el feedback check (D6c),
  `cmd/api/daos/group_calendar_day_dao.go` (finder de fechas IN para
  bulk-clear D7) y `cmd/api/services/calendar_service.go` (bulk-clear).
- Test: supersede batch (protección por feedback de sesión y por ejercicio,
  con ID real de BBDD), bulk-clear con guards en memoria (cerrada/422 por
  fechas idéntico al de hoy) y conteo de queries.

**Steps:**

- [x] **4.1** DAOs para feedback/instancia con `IN` (D6c) + finder de
  fechas (D7); tests Postgres real.
- [x] **4.2** `deleteSupersededInstances(batch)` con semántica D10 exacta
  (protege Instancia si feedback de sesión o de alguno de sus ejercicios);
  migrar stamp/bulk/shift/bulk-clear.
- [x] **4.3** Bulk-clear con 1 read `IN`, guards en memoria (mismo
  orden/errores del 422), delete único y supersede batch.
- [x] **4.4** Tests de conteo y regresión (aserciones existentes intactas);
  commit `perf(calendar): bulk-clear y borrado de instancias en lote`.

### Task 5: Rendimiento — administered-calendar e invitaciones en lote

**Files:**
- Modify: `cmd/api/services/calendar_service.go` (AdministeredCalendar —
  1 query de todas las fechas del rango + clasificación en memoria D8),
  `cmd/api/services/invitation_service.go` (list invitations — D9) y los
  DAOs batch que faltaren tomando los `FindByIDs` existentes.
- Test: mock/servicio + Postgres real de las rutas afectadas (invitados de
  equipo borrado, inviter inexistente, clasificación cross/same idénticas).

**Steps:**

- [x] **5.1** AdministeredCalendar sin el loop per-fecha de
  `findPresencialCollisions` (D8 — 1 query + memoria; helper del guard
  intacto); respuestas byte-idénticas.
- [x] **5.2** Invitaciones: 3 queries totales con maps (D9), misma
  tolerancia ante team/user no resuelto que el `FindByID` actual.
- [x] **5.3** Tests con conteo (rango del calendario administrado y
  las listas de invitaciones) + regresión SIN cambiar aserciones; commit
  `perf(reads): colisiones del calendario administrado e invitaciones en lote`.

### Task 6: Seguridad — pins anti-inyección, no-leak 500 y audit de authz

**Files:**
- Test: `cmd/api/controllers/*_test.go` y `cmd/api/services/*_test.go`
  (pines nuevos; `FailingDB` en testutils ya existe).
- Modify: solo si el audit detecta un guard sin test explícito (agregar
  el test; si el guard falta de verdad → bloquear y reportar).

**Steps:**

- [x] **6.1** SQLi pins (D10.1): `/users/search` q, `/teams/search`
  country/province/city, filtros de texto del historial; sort/order fuera
  de whitelist → fallback; payload matcheado literal (200, sin error SQL).
- [x] **6.2** No-leak 500s (D10.2): con `FailingDB`, asertar que los
  mensajes genéricos de error NUNCA incluyen el detalle del driver
  (SELECT/sysntax/SQLSTATE); endpoints calientes sin mock completo.
- [x] **6.3** Audit authz (D10.3): tabla endpoint → guard → test en el
  report; implementar los 403/404 faltantes (mock del paquete).
- [x] **6.4** Commit `test(security): pins anti-injection, no-leak 500 y audit authz`.

### Task 7: Docs + verificación final

**Files:**
- Modify: `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (§ nuevo Gap 29: aditivo,
  lat/lng nullable en user/team, charset veraz, sin backfill),
  `docs/DEUDA_TECNICA_Y_PENDIENTES.md` (§Resueltos: stamp/bulk/shift/bulk-
  clear batch, administered/invitations batch), nota en
  `docs/CATALOGO_Y_CALENDARIO.md` si las escrituras cambian algo (no
  debería;a menos que el review detecte un detalle visible).
- `openspec/changes/.../tasks.md`: tildes.

**Steps:**

- [x] **7.1** Docs con shapes verificados contra código; `openspec
  validate latlng-optimizaciones-y-endurecimiento --strict` → valid.
- [x] **7.2** Suite completa con DB real `go test -count=1 ./...` → 0 FAIL.
- [x] **7.3** Coverage con cache limpio → gate 85 PASS,
  `.testcoverage.yml` sin diff; gofmt/build/vet limpios; swagger final
  sync revisado.
- [x] **7.4** Commit `docs(gap29-perf): cerrar latlng y optimizaciones en docs`.
