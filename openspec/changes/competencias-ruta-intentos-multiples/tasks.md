# Tasks

> Proyecto: competencias-ruta-intentos-multiples (Gap 36). Decisiones cerradas
> con el frontend, 2026-10-22. Fuente de verdad: `design.md` (D1-D8). Base:
> `develop` con `competencias-ruta-intake` mergeado. Implementación en orden
> Task 1 → Task 5; cada Task es un commit Conventional con el subject exacto y
> deja la rama verde.

## Global Constraints (aplicable a TODAS las tasks)

- Rama `feature/competencias-ruta-intentos-multiples` ya creada desde develop;
  los artefactos OpenSpec viven en
  `openspec/changes/competencias-ruta-intentos-multiples/` y se commitean
  ÚNICAMENTE en Task 5.
- Commits Conventional con el subject exacto indicado en cada task. Stage SOLO
  rutas explícitas del task — JAMÁS `git add -A` / `git add .`; nada de
  `.superpowers/` (artefactos del workspace SDD auto-ignoreados).
- DAOs testeados contra Postgres real (levantar con `docker start
  paceron-test-db`; env `TEST_DB_HOST=localhost TEST_DB_PORT=5433
  TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`).
  Services/controllers con mocks. La columna nueva `max_attempts` entra por
  AutoMigrate (BIGINT nullable, sin backfill). Si el Postgres de test no
  arranca, los DAO tests se skipean solos; no es excusa para bajar coverage.
- Coverage: `go clean -cache` + `make coverage-with-db` (mismo comando de
  ci.yml) con el analyzer go-test-coverage; gate 85 MÍNIMO que no se baja, y
  apuntando a más que lo mínimo (meta gradual 90) — tests exhaustivos en cada
  task, no relleno.
- Verificación por task: suite completa verde (vet, build), gofmt SOLO sobre
  archivos tocados, swagger regenerado con el comando
  canónico SOLO en tasks que tocan endpoints:
  `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`
  (revisar el diff que no suba churn de metadata ajena a competencias).
- FailingDB pins no-leak: cualquier 500 de endpoint nuevo devuelve body
  genérico sin filtrar detalles internos de DB (assert en test de controller).
- Guards de 403/400/404 con assert de que el DAO de escritura no se tocó
  (patrón estándar del repo).
- NO tocar: el finish del intento (`PATCH /session-instances/:id/runner`),
  feedback/syncRun, `Delete`/`deleteSyntheticRouteInstance` (limpieza de
  cancel), frames de protocolo WS, close/cancel/reject (solo lo que las tasks
  indican).
- No-goals duros: no borrar/editar intentos, no tope superior de
  `max_attempts`, no schedule de `can_attempt`, no migración destructiva ni
  backfill. Comentarios en código: mínimos (solo invariantes/trampas no
  obvias para IA, en español).
- Repo layout: `cmd/api/{controllers,services,daos,delegates,dbs,domains,
  constants,realtime,restclients,testutils}`; convenciones de capas en
  `.agentics/CONVENTIONS.md` (DAO→service único punto de DB; controllers sin
  DAOs).

## Tasks

### Task 1: max_attempts en create (DTO + columna + validación) — Tier: mecánica

Commit: `feat(competencias): max_attempts en el create de competencias de ruta`

Files/interfaces:
- `cmd/api/domains/competition/competition.go` (campo `MaxAttempts *int64` en
  `CreateCompetitionRequest`).
- `cmd/api/domains/dbs/competition.go` (campo `MaxAttempts *int64`, column
  `max_attempts` BIGINT nullable sin FK, entra por AutoMigrate).
- `cmd/api/services/competition_service.go` (armado del struct de competencia +
  `validateCreateRequest`: branch route y branch metric).
- `cmd/api/services/competition_service_test.go` (suite de validación) +
  `cmd/api/daos/competition_dao_test.go` (round-trip Postgres real).
- `cmd/api/docs/` (regeneración swagger de create).

Steps:
- [ ] 1.1 Campo `MaxAttempts *int64` en `CreateCompetitionRequest` y en
      `dbs.Competition` (column `max_attempts`, BIGINT nullable sin
      constraint); el service lo copia al struct de competencia en el create
      (insertado con la fila — cero código de DAO extra).
- [ ] 1.2 Validación en `validateCreateRequest`: branch route → max_attempts
      opcional; si viene debe ser entero ≥ 1 (400 "max_attempts debe ser un
      entero mayor o igual a 1" si 0/negativo; no-entero → 400 de binding);
      SIN tope superior; si no viene → NULL = sin límite. Branch metric → 400
      "type=metric no admite max_attempts" si viene.
- [ ] 1.3 Inmutable post-create: es un no-agregar (no se crea ninguna vía de
      edición de competencia); nota en aclaración de tests.
- [ ] 1.4 Tests de validación: route sin max_attempts (persistida null), con
      1, con 3, con 0 → 400, con negativo → 400, no-entero → 400, metric con
      max_attempts → 400, metric sin max_attempts sigue pasando (regresión).
- [ ] 1.5 DAO round-trip Postgres real: create route con y sin max_attempts
      persiste/lee el valor (y null) íntegro; competencia pre-existente (sin
      columna seteada) se lee null.
- [ ] 1.6 Regenerar swagger (create) + revisar diff limpio.
- [ ] 1.7 `go clean -cache` + `make coverage-with-db` + vet/gofmt/build en
      verde antes de tildar.

### Task 2: CreateAttempt rework — límite de intentos e instancia por intento — Tier: juicio

Commit: `feat(competencias): intentos múltiples en competencias de ruta`

Files/interfaces:
- `cmd/api/daos/competition_dao.go` (refactor firma de
  `createSyntheticRouteInstance` por nombre; método tx `CreateAttemptSession`
  con lock + count + instancia nueva + insert) + `cmd/api/daos/
  competition_dao_attempt_test.go` (Postgres real).
- `cmd/api/services/competition_service.go` (`CreateAttempt`: guard de límite
  reemplaza al doble-intento; delega la tx en el DAO) + tests de service con
  mocks.
- `cmd/api/controllers/competition_controller.go` + test (sin cambio de shape
  ni de mapping — regresión).
- `cmd/api/docs/` (regeneración de POST attempts por cambio de reglas de
  guard).

Steps:
- [ ] 2.1 `createSyntheticRouteInstance` refactorizado para recibir el nombre
      parametrizado; las llamadas existentes del create de competencia siguen
      pasando "Competencia ruta %d" (sin cambio de comportamiento, test de
      regresión del create con instancia compartida).
- [ ] 2.2 DAO `CreateAttemptSession(ctx, comp, athleteUserID, maxAttempts)`:
      transacción única con (a) SELECT FOR UPDATE de la fila de competencia
      (serializa concurrentes — el unique (session_instance_id, athlete_user_id)
      YA NO arbitra el límite porque los intentos 2+ usan instancias distintas),
      (b) COUNT de runner_sessions por competition_id + athlete (cualquier
      status), (c) 400-equivalente (error de límite) si count >= maxAttempts
      (cuando maxAttempts != nil), (d) intento 1 (count == 0) inserta sobre la
      instancia compartida con la semántica actual (ON CONFLICT DO NOTHING
      incluido), (e) intento 2+ crea instancia NUEVA con nombre
      "Competencia ruta N (intento M)" (M = count + 1) + ejercicio "Corrida"
      (velocidad_distancia, link main/repeat 1/rest 0) + insert de runner
      session, (f) cualquier fallo → rollback total.
- [ ] 2.3 Service `CreateAttempt`: mantengo orden de guards 404 → 400 type →
      400 status → 403 participante → guard instancia compartida existente;
      el guard "existe intento" se reemplaza por el límite del DAO. Respuesta
      201 con shape runner-session sin cambios (controller/regresión).
- [ ] 2.4 Tests DAO Postgres real: intento 1 reusa compartida (nombre sin
      "(intento)"); intento 2 crea instancia nueva con nombre exacto
      "Competencia ruta N (intento 2)"; intento 3 otra instancia más
      ("(intento 3)"); max_attempts = 2 con 2 intentos → error de límite y
      cero filas nuevas; max_attempts = null → N intentos sin rechazo;
      rollback (fallo forzado post-instancia) no deja sesión huérfana.
- [ ] 2.5 Test de carrera concurrente Postgres real (dos tx en paralelo sobre
      la misma competencia/atleta con límite para 1 intento más): una crea,
      otra recibe el error de límite y no supera max_attempts.
- [ ] 2.6 Tests de service con mocks: paths de error (404/type/status/403/
      límite) sin tocar DAO de escritura; y la vía feliz delegando en el DAO
      con los parámetros correctos.
- [ ] 2.7 Regenerar swagger; verificación gate (coverage/vet/gofmt/build) en
      verde antes de tildar.

### Task 3: WS 3ª rama extendida + GET /competitions/{id}/attempts — Tier: juicio

Commit: `feat(competencias): endpoint GET attempts y autorización WS de instancias de intento`

Files/interfaces:
- `cmd/api/daos/session_instance_dao.go` (extensión de la 3ª rama de
  `HasInstanceAccess` con el OR EXISTS por runner_session×competition×
  participants; espejo `hasAccessBatchCompetitionSQL`) + `cmd/api/daos/
  session_instance_dao_route_access_test.go` (Postgres real; extender el
  existente).
- `cmd/api/domains/competition/competition.go` (DTO
  `CompetitionAttemptsResponse {Data []CompetitionAttemptItem}` + ítem con los
  8 campos y punteros nullables).
- `cmd/api/daos/competition_dao.go` (`ListAttemptsByCompetitionAndAthlete`
  ORDER BY id ASC; `SumFeedbackDistanceByInstances` GROUP BY
  assigned_session_id con feedback activo) + test Postgres real.
- `cmd/api/services/competition_service.go` (`ListAttempts`: guard 404 →
  miembro-u-owner 403; default athlete = caller; atleta no-participante → [];
  compliance_ratio on-demand reusando `routeSamples` + `ListAttemptPoints` +
  `MatchPercent`; duration_ms calculado server-side) + tests con mocks.
- `cmd/api/controllers/competition_controller.go` (handler `ListAttempts`:
  200 `{data: [...]}` sin has_unseen; parseo de query athlete_user_id positivo
  400 si inválido; mapping 500 genérico sin leak) + test + `cmd/api/app/
  url_mappings.go` (ruta `GET /api/v1/competitions/:id/attempts`).
- `cmd/api/docs/` (regeneración por endpoint nuevo).

Steps:
- [ ] 3.1 Extensión de la 3ª rama `HasInstanceAccess`: OR EXISTS
      (runner_session rs JOIN competition c2 rs.competition_id = c2.id AND
      c2.type='route' WHERE rs.session_instance_id = ? AND caller participante
      de c2) SIN filtro de status del intento. La rama original
      (route_session_instance_id) queda intacta.
- [ ] 3.2 Mirror idéntico en `hasAccessBatchCompetitionSQL` (espejo batch,
      mismo OR con u.user_id como caller) — singular y batch no pueden
      divergir.
- [ ] 3.3 Tests DAO Postgres real de la rama nueva (ambos singular y batch):
      participante accede a la instancia de su intento 2 con solo el
      runner_session wip (sin día ni feedback); usuario ajeno no; intento
      finished también autoriza; instancia regular de calendario regida por
      las 2 ramas duales intactas (regresión).
- [ ] 3.4 DAO helpers `ListAttemptsByCompetitionAndAthlete` + 
      `SumFeedbackDistanceByInstances` con sus tests Postgres real (orden id
      ASC; sum distance null cuando no hay feedback; feedback soft-deleted no
      suma).
- [ ] 3.5 Service `ListAttempts` con la lógica de null-rules: distance_meters
      de feedback (null si no hay), compliance_ratio solo con intento finished
      (0 si finished sin GPS, on-demand matching), duration_ms server-side
      end-start (null si wip). Atleta pedido no-participante → slice vacío
      (sin error). Tests con mocks.
- [ ] 3.6 Controller + ruta + swagger: 404 antes de 403; guard
      miembro-u-owner; SIN athlete_user_id → caller; con query de otro (o
      incluso del caller) → intentos de ese atleta. Tests de controller
      (incluye FailingDB no-leak y query inválida → 400).
- [ ] 3.7 Verificación gate (coverage/vet/gofmt/build) en verde antes de
      tildar.

### Task 4: progress route shape + computeWinners por mejor intento + campos de CompetitionResponse — Tier: juicio

Commit: `feat(competencias): progreso y winners por mejor intento y can_attempt`

Files/interfaces:
- `cmd/api/domains/competition/competition.go` (`CompetitionProgressItem`
  se parte en `CompetitionMetricProgressItem` y `CompetitionRouteProgressItem`
  con interfaz común para el campo `Progress` de `CompetitionResponse`;
  `CompetitionResponse` + `MaxAttempts`/`AttemptsUsed`/`CanAttempt`).
- `cmd/api/services/competition_service.go` (helper compartido de mejor intento
  por atleta; rework de `computeRouteProgress` y del branch route de
  `computeWinners`; `fillAttemptFields` + variante batch con el DAO nuevo
  `CountAttemptsByCompetitionAndAthlete` en ListByTeam/Create/detalle/
  mutaciones) + `cmd/api/daos/competition_dao.go` (batch count) + tests de
  ambos con mocks y Postgres real.
- `cmd/api/controllers/competition_controller_test.go` (regresión de shapes de
  respuesta de listado/detalle si corresponde).
- `cmd/api/docs/` (regeneración por campos nuevos en response).

Steps:
- [ ] 4.1 DTOs: `CompetitionMetricProgressItem {user_id, value}` (JSON
      byte-idéntico al actual) y `CompetitionRouteProgressItem {user_id,
      compliance_ratio, duration_ms, session_instance_id}` (sin `value`);
      campo `Progress` de `CompetitionResponse` tipado con la interfaz común
      (serializa según struct dinámico).
- [ ] 4.2 Helper compartido `bestAttemptByAthlete(attempts)`: por atleta,
      max pct → tie-break menor duration_ms (misma regla para progress y
      winners — una sola fuente de verdad).
- [ ] 4.3 `computeRouteProgress` route: usa el helper y emite los ítems en el
      shape nuevo, ordenados por user_id ASC — participante sin intentos
      aparece igual que hoy con los valores null (compliance_ratio/duration_ms/
      session_instance_id null); metric byte-idéntico (regresión).
- [ ] 4.4 `computeWinners` branch route: reducción por atleta primero, luego
      comparación entre atletas con empate EXACTO en (pct, duración) → todos
      ganan; un atleta nunca aparece duplicado. Tests de service: el edge del
      atleta con 2 intentos en el mejor pct global (hoy duplicaría), empate
      intra-atleta por duración, empate exacto entre atletas, metric
      byte-idéntico (regresión).
- [ ] 4.5 DAO `CountAttemptsByCompetitionAndAthlete(batch)` → map
      competition_id → count, UNA query para el listado (test Postgres real).
- [ ] 4.6 `fillAttemptFields` (single) + variante batch: llena
      max_attempts/attempts_used/can_attempt en detalle, listado, Create y
      mutaciones, con la regla de null/false de D7. Tests: participante route
      active sin intentos → 0/true; límite cumplido → N/false; caller no
      participante o metric → null/false con bool siempre presente; listado
      batch coherente con el detalle.
- [ ] 4.7 Regenerar swagger; verificación gate (coverage/vet/gofmt/build) en
      verde antes de tildar.

### Task 5: Docs + tildes + validación openspec + verificación final — Tier: mecánica

Commit: `docs(openspec): propuesta competencias-ruta-intentos-multiples (gap 36)`

Files/interfaces:
- `docs/CATALOGO_Y_CALENDARIO.md` (§8.14: subsección nueva de intentos
  múltiples + corrección del texto "value null para route" que quedaba
  desmentido por el código).
- `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (sección nueva numerada siguiente:
  endpoint GET attempts, shape de progress route, campos de
  CompetitionResponse, reglas de la instancia por intento, relación con los
  canales WS por intento).
- `docs/DEUDA_TECNICA_Y_PENDIENTES.md` (si aplica: intentos wip/interrupted no
  se borran ni reabren; leaderboard live mid-attempt ya marcada — ligarla).
- `openspec/changes/competencias-ruta-intentos-multiples/**` (este commit
  ÚNICO de artefactos OpenSpec).

Steps:
- [ ] 5.1 Documentar en CATALOGO §8.14 el flujo de intentos múltiples (D1-D8)
      con textos en español limpios (sin mojibake), referenciando las
      decisiones cerradas con el frontend y corrigiendo el value/pct de
      progress.
- [ ] 5.2 Documentar en FRONTEND_IMPACTO el contrato nuevo/firmas cambiadas de
      cara al consumo del Gap 36: GET attempts con sus null-rules,
      attempts_used/can_attempt/max_attempts, progress route shape, autorización
      WS por instancia de intento, y lo que explícitamente NO cambia (frames).
- [ ] 5.3 Si hay deuda específica que documentar en DEUDA (borrado/edición de
      intentos v1), dejarla apuntada; si no aplica, no forzar el archivo.
- [ ] 5.4 Tildar los checkboxes de Task 1-4 en este tasks.md (memoria
      persistente real) y re-verificar que no quedó ninguna sin cerrar.
- [ ] 5.5 `openspec validate competencias-ruta-intentos-multiples --strict` →
      VALID (releer todo el change para corregir cualquier texto corrupto).
- [ ] 5.6 Verificación final: suite completa contra Postgres real (`docker
      start paceron-test-db` + env completo de la sección Global Constraints),
      `go clean -cache` + `make coverage-with-db` con gate ≥ 85, vet/build/
      gofmt en verde, diff de swagger limpio; commit único de artefactos
      OpenSpec con stage explícito de rutas bajo el change (jamás
      `git add -A`).
