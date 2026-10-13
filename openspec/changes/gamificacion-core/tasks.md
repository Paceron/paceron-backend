# Tasks

> Proyecto: gamificacion-core (Gaps 30-34). Decisiones cerradas con el frontend, 2026-10-13.
> Fuente de verdad: `design.md` (D1-D14) y los 5 spec deltas. Implementación en orden Task 1
> → Task 7; cada Task es un commit Conventional con subject exacto y deja la rama verde.

## Global Constraints (aplicable a TODAS las tasks)

- Rama `feature/gamificacion-core` ya creada desde develop; los artefactos OpenSpec viven en
  `openspec/changes/gamificacion-core/`.
- Commits Conventional con el subject exacto indicado en cada task. Stage SOLO rutas
  explícitas del task — jamás `git add -A` / `git add .`; nada de `.superpowers/`; los
  artefactos de openspec se commitean ÚNICAMENTE en Task 7.
- DAOs testeados contra Postgres real (`make test-db-up`; env
  `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres
  TEST_DB_NAME=paceron_test`). Services/controllers con mocks. Las tablas nuevas se
  registran en el AutoMigrate del bootstrap de test (`testutils.SetupTestDB`).
- Si el Postgres de test no arranca, los DAO tests se skipean solos; no es excusa para bajar
  coverage.
- FailingDB pins no-leak: todo endpoint nuevo que toque error interno de DB devuelve 500 con
  body genérico sin filtrar detalles (assert en controller).
- Guards de 403/400/404 con assert de que el DAO no se tocó (patrón estándar del repo).
- Determinismo temporal: la var global del helper TZ (design D1) se fija con `time.Date` en
  tests; nunca `time.Now()` para decisiones de día.
- Swagger: regenerar SOLO cuando hay endpoints nuevos, con el comando canónico
  `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`; revisar el diff
  para que no suba churn de metadata ajena a gamificación.
- Coverage gate 85 (meta 90): antes de tildar cada task, correr `go clean -cache` +
  `make coverage-with-db` con el analyzer `go-test-coverage --profile`; no bajar el umbral.
- 0 cambios de API existente, salvo el campo opcional `competition_id` en el body de
  `POST /session-instances/:id/runner` (aditivo; sin el campo, byte-idéntico).
- Comentarios en código: mínimos (solo invariantes/trampas no obvias para IA).
- Repo layout: `cmd/api/{controllers,services,daos,delegates,dbs,domains,constants,realtime,
  restclients,testutils}`; convenciones de capas en `.agentics/CONVENTIONS.md`.

## Tasks

### Task 1: Rachas — tablas unseen/state + GET /users/{id}/streaks (E1)

Commit: `feat(gamification): rachas y tablas unseen/streak-state (E1)`

Files/interfaces:
- `cmd/api/dbs/gamification_unseen.go` (Unseen) y `cmd/api/dbs/gamification_streak_state.go`
  (StreakState) + AutoMigrate en el bootstrap Postgres y testutils.
- `cmd/api/utils/gametime/` — helper de paquete (design D1): var global sobreescribible en
  tests, default `America/Argentina/Cordoba`.
- `cmd/api/services/streak_service.go` + `cmd/api/services/streak_service_test.go`.
- `cmd/api/controllers/streak_controller.go` + test; ruta en `cmd/api/app/url_mappings.go`.
- `cmd/api/domains/gamification/` (DTOs del GET).
- `cmd/api/daos/gamification_dao.go` + `cmd/api/daos/gamification_dao_test.go`;
  pg_indexes de las 2 tablas en `daos_indexes_test.go`.

Steps:
- [ ] 1.1 Helper `gametime`: TZ Cordoba fija, funciones para hoy/ayer/now con vars globales
      sobreescribibles; test unitario del helper.
- [ ] 1.2 DB structs `gamification_unseen` (id, user_id, section varchar(32), team_id
      nullable, created_at; index (user_id, section)) y `gamification_streak_state`
      (user_id PK, strict_current, flexible_current, updated_at); AutoMigrate + SQL de
      índices post-migrate.
- [ ] 1.3 DAO batch: memberships activos del usuario, `group_calendar_days kind='training'`
      en rango (group_id IN), runner sessions finished en rango (session_instance_id IN)
      — 3 queries exactas, sin N+1 (test que pina el conteo de queries).
- [ ] 1.4 Servicio de rachas: mapeo por día en Go con estados
      strict/flexible completed|failed|skipped|pending (design D2), rachas A y B con
      current y best sobre TODO el histórico hasta ayer; el GET no consulta
      gamification_streak_state.
- [ ] 1.5 Controller `GET /users/{id}/streaks` con guard self-only (403) + validación
      `days` (default 42, tope 365, fuera de rango 400); FailingDB no-leak pin (500
      genérico). `has_unseen` embebido de la sección streaks.
- [ ] 1.6 Tests: DAO (Postgres real), service (mocks, incl. determinismo con time.Date de
      1.1), controller (guards + pin); `daos_indexes_test.go` extendido.
- [ ] 1.7 Swagger para el endpoint nuevo. `go build ./... && go vet ./...` verde; commit
      con subject exacto.

### Task 2: Novedades — POST seen + triggers de racha (E1)

Commit: `feat(gamification): seen y triggers de racha (E1)`

Files/interfaces:
- `cmd/api/services/gamification_seen_service.go` (+ test); extender `streak_service.go`
  con la computación de current reutilizable por los triggers.
- `cmd/api/controllers/gamification_seen_controller.go` (+ test); ruta en
  `url_mappings.go`.
- Hook post-write en el controller/service del PATCH de runner session
  (finished/interrupted).

Steps:
- [ ] 2.1 `POST /users/{id}/gamification/seen`: guard self-only, whitelist de section
      (streaks/achievements/competitions — 400 otro valor), team_id solo en competitions
      (400 en otra sección), DELETE unseen por sección (team_id filtra solo competitions;
      sin team_id limpia todas); no-op con 0 filas. Tests de servicio y controller.
- [ ] 2.2 Trigger post-write del PATCH de runner session hacia finished/interrupted:
      calcula current antes y después con la lógica batch del GET, actualiza
      `gamification_streak_state`, e inserta unseen section streaks si la racha pasa de
      > 0 a 0 o cruza hito 10/20/50/100 hacia arriba (old < hito && new >= hito).
- [ ] 2.3 El hook es nil-safe y asíncrono (post-commit): no bloquea la respuesta del
      PATCH; el fallo de la persistencia de estado se loguea sin romper (deriva D14
      documentada).
- [ ] 2.4 El create wip y los writes del calendario NO disparan el trigger (tests de
      negación).
- [ ] 2.5 Tests: service (mock DAO), controller (FailingDB no-leak del POST nuevo),
      integración DAO del upsert de streak_state; `go build ./... && go vet ./...` y
      coverage sin caída; commit con subject exacto.

### Task 3: Logros — catálogo, tablas, evaluadores y vitrina (E2)

Commit: `feat(gamification): catalogo y evaluadores de logros (E2)`

Files/interfaces:
- `cmd/api/constants/achievements.go` — lista versionada de las 21 filas
  {id, family, name, description, icon_key, threshold, unit} + test de shape.
- `cmd/api/dbs/user_achievement_progress.go` (PK compuesta user_id+achievement_id,
  index user_id), `cmd/api/dbs/gamification_stamp.go` (index user_id) + AutoMigrate
  (testutils); pg_indexes en `daos_indexes_test.go`.
- `cmd/api/daos/achievement_dao.go` (+ DAO test real).
- `cmd/api/services/achievements_service.go` (catálogo, merge de progreso, evaluadores)
  (+ test).
- `cmd/api/controllers/achievements_controller.go` (+ test); rutas en `url_mappings.go`.
- Hooks en los controllers que registran hechos (PATCH runner session a finished, subida
  de points de feedback, asistencia): post-commit evaluación, nil-safe async.
- `cmd/api/restclients/expopushclient` para el push al desbloquear (best-effort,
  data.route "/achievements").

Steps:
- [ ] 3.1 Catálogo de 21 filas en constants con test que pinea el recuento
      6 consistency + 4 volume + 5 sessions + 3 gps + 1 early_bird + 2 team = 21 y los
      campos obligatorios de cada fila.
- [ ] 3.2 Tablas con migraciones (bootstrap + testutils); pg_indexes pineados.
- [ ] 3.3 Endpoints: `GET /achievements/catalog` (autenticado, sin ownership) y
      `GET /users/{id}/achievements` (self-only; merge catálogo + progreso; shape
      {achievement_id, current_level?(null en v1), count?(null en v1), unlocked_at,
      last_unlocked_at}) con has_unseen embebido; `GET /users/{id}/stamps` (self-only,
      vacía hasta E4).
- [ ] 3.4 Evaluadores por criterio con sus queries de agregación (COUNT finished / SUM
      distance / MAX por session_instance_id para 42.2 km, streak_N reusa D2, gps_N,
      early_N con TZ Cordoba, team_N días distintos con asistencia confirmada),
      idempotentes por PK compuesta.
- [ ] 3.5 Hooks de evaluación post-commit nil-safe async en los controllers de runner
      session finished y feedback points; al desbloquear, push best-effort con
      data.route "/achievements" y fail silencioso con log (stamps NO participan en E2).
- [ ] 3.6 Tests: DAO real (inserciones/idempotencia), evaluadores unitarios con fixtures
      por criterio, controller con guards + FailingDB no-leak, push con stubs. Swagger
      para los 3 endpoints. `go build ./... && go vet ./...` + coverage; commit con
      subject exacto.

### Task 4: Leaderboards — pace/peak_speed denormalizados + GET /teams/{id}/leaderboard (E3)

Commit: `feat(gamification): pace/peak_speed y leaderboard de team (E3)`

Files/interfaces:
- `cmd/api/dbs/workout_feedback.go` suma columnas `pace_sec_per_km` y `peak_speed_kph`
  (nullable; SIN exponerlas en el DTO del dominio workoutfeedback).
- `cmd/api/services/leaderboard_service.go` (+ test);
  `cmd/api/controllers/leaderboard_controller.go` (+ test); ruta en `url_mappings.go`;
  batch names/avatar via el DAO de usuarios existente.
- Extensión del flujo de subida de points (workoutfeedback) para recomputar las columnas
  al subir points con count > 0 (misma transacción, sin backfill histórico).

Steps:
- [ ] 4.1 Columnas de feedback + recompute idempotente al subir points: pace =
      duration_ms / distance_meters; peak = max velocidad instantánea entre segmentos de
      points consecutivos; NULL cuando no hay distance o GPS. Test del recompute sobre
      puntos sintéticos + regresión del DTO de feedback que sigue sin exponerlas.
- [ ] 4.2 `GET /teams/{id}/leaderboard`: guard de caller (miembro de algún grupo del team
      O owner; 403 con DAO no tocado); validación de metric (6 valores) y period (5
      valores) con 400 fuera de whitelist; custom exige from y to con from <= to (400) y
      base "hoy" TZ Cordoba via gametime.
- [ ] 4.3 Agregación en 1 query GROUP BY user_id + batch de names/avatar (sin N+1, test
      que pina el conteo); métricas según design D7; solo corredores puntúan (el owner no
      genera fila propia).
- [ ] 4.4 Sorting con desempates: mismo valor → menos entrenamientos contribuyentes;
      específicos sessions_count (start_date de la última) y attendance_count (quien
      registró primero la última asistencia); ranks compartidos a igual valor con salto
      de N; viewer_rank; record solo en period=all con achieved_at (session_date de la
      fila del max, a empate la más reciente).
- [ ] 4.5 Tests: DAO real de las agregaciones contra fixtures (Postgres), service con
      mocks y TZ fija, controller con guards/400 + FailingDB no-leak. Swagger del
      endpoint. `go build ./... && go vet ./...` + coverage; commit con subject exacto.

### Task 5: Competencias E4.1 — tablas, CRUD y competition_id en runner session

Commit: `feat(competitions): modelo y CRUD de competencias (E4.1)`

Files/interfaces:
- `cmd/api/dbs/competition.go`, `cmd/api/dbs/competition_participant.go`,
  `cmd/api/dbs/competition_winner.go` + AutoMigrate/testutils + pg_indexes.
- `cmd/api/domains/runnersession/runner_session.go` suma CompetitionID *int64 (FK opaca,
  nullable; sin constraint de DB).
- `cmd/api/domains/competition/` DTOs del detalle y el listado.
- `cmd/api/daos/competition_dao.go` (+ DAO test real).
- `cmd/api/services/competition_service.go` (+ test);
  `cmd/api/controllers/competition_controller.go` (+ test); rutas en `url_mappings.go`.
- Extensión de `POST /session-instances/:id/runner` con competition_id opcional.

Steps:
- [ ] 5.1 Migraciones de las 3 tablas + columna CompetitionID opaca en runner_session;
      pg_indexes pineados; tests DAO reales (alta/lista/cancel/winner).
- [ ] 5.2 `POST /teams/{id}/competitions`: validaciones de body (type, metric/end_at,
      route, radius, participantes >= 2 y todos del team, trainer no incluido) con 400;
      creador owner → active directo; corredor miembro → pending_approval; caller fuera
      del team 403 con DAO no tocado.
- [ ] 5.3 `PATCH /competitions/{id}/approve` y `/reject {reason?}` (owner del team;
      transiciones solo desde pending, 400 en otro caso) con push best-effort al creador
      en ambos casos; reject persiste reason.
- [ ] 5.4 `DELETE /competitions/{id}/cancel`: creador corredor, solo pending; borrado
      físico con participantes (guard negativo para los demás callers).
- [ ] 5.5 `GET /teams/{id}/competitions?status=&group_id=` (miembro de grupo / owner, 403
      otro caso; 400 status fuera de whitelist) con has_unseen de competitions embebido
      por team_id; `GET /competitions/{id}` con progreso en vivo on-demand (metric
      acumulado en `[start_at, end_at o now]`; route mejor % de match hasta ahora) sin
      broadcast WS en consultas; DTO shape según design D9.
- [ ] 5.6 Extensión del `POST /session-instances/:id/runner`: body opcional
      competition_id; valida active + participant + rango (400); sin el campo, el flujo y
      la respuesta preexistentes quedan byte-idénticos (no tocar los tests viejos).
- [ ] 5.7 `PATCH /competitions/{id}/close`: cambia status a finished + finished_at; guard
      solo creador (trainer o runner) u owner; la computación de winners y efectos se
      agrega en Task 6 (acá queda conectado el stub del pipeline). Tests de todas las
      rutas; `go build ./... && go vet ./...` + coverage; commit con subject exacto.
      Swagger se regenera completo en Task 6.

### Task 6: Competencias E4.2 — winners, route matching, stamps, pushes y WS

Commit: `feat(competitions): cierre, winners y matching de ruta (E4.2)`

Files/interfaces:
- Extensión `cmd/api/services/competition_service.go`: cómputo de winners al close
  (metric MAX en ventana; route Haversine con sampleo a 30-50 m y
  acceptance_radius_meters), insert de competition_winner, stamps en gamification_stamp
  type competition, unseen por participante (section competitions + team_id), push
  best-effort con data.route a los participantes.
- Route matcher (paquete o servicio pequeño): Haversine + sampleo de GeoJSON LineString
  (+ unit tests de geometría pura).
- Integración con `cmd/api/realtime/notifier.go`: update:competition_progress (al
  terminar un intento, channel session:{id} de la instancia, payload {competition_id,
  user_id}, sin exclusión del emisor) y control:competition_finished (al cerrar, channels
  session:{instance} de los intentos registrados; solo si hay intentos).

Steps:
- [ ] 6.1 Route matcher: parser de route_geojson (LineString), sampleo a pasos fijos y
      Haversine en Go; unit tests de geometría (match 100, 0 sin GPS, match parcial,
      borde del radio). Sin umbral mínimo.
- [ ] 6.2 Winners del close: metric → MAX por participante en la ventana con empate
      exacto = todos ganan; route → mejor % con tie-break menor duración y duración
      exacta igual = todos ganan; sin intentos → sin winner. Insert de
      competition_winner y respuesta con winner_user_ids.
- [ ] 6.3 Stamp al ganador (gamification_stamp type competition, source_id competencia,
      source_name "Competencia N", count 1) — el GET de stamps de Task 3 ya los levanta.
- [ ] 6.4 Unseen por participante de section competitions con team_id del team (todos los
      participantes, no solo ganadores).
- [ ] 6.5 Pushes best-effort al creador en approve/reject (ya desde Task 5) y a los
      participantes al cierre, con data.route placeholder y fail silencioso con log.
- [ ] 6.6 WS: update:competition_progress post-write del intento finished y
      control:competition_finished al cierre, según el patrón del broadcast de feedback
      (notifier nil-safe).
- [ ] 6.7 Tests: service (winners, tie-breaks, stamps, unseen — mocks), controller con
      FailingDB no-leak del close, route matcher puro, e2e WS con hub real (patrón de
      ws-gateway-sesiones). Swagger regenerado con el comando canónico para los endpoints
      E4 (diff sin churn). `go build ./... && go vet ./...` + coverage; commit con
      subject exacto.

### Task 7: Docs, validación OpenSpec y verificación final

Commit: `docs(openspec): propuesta gamificacion-core (Gaps 30-34)`

Files:
- `docs/DEUDA_TECNICA_Y_PENDIENTES.md` — auto-cierre por cron, deriva de triggers de
  rachas (D14), leaderboard de teams gigantes, FK opaca de competition_id.
- `docs/FRONTEND_IMPACTO_INSTANCIACION.md` o doc de impacto del módulo nuevo si aplica —
  impactos del front para gamificación (streaks, logros, leaderboard, competencias).
- `docs/CATALOGO_Y_CALENDARIO.md` — solo si algo del flujo coach/corredor cambió en la
  doc.
- `.agentics/GLOSSARY.md` — términos nuevos del dominio de gamificación.
- `README.md` — listado de endpoints nuevos de gamificación.
- `openspec/changes/gamificacion-core/` — un solo commit de todos los artefactos.

Steps:
- [ ] 7.1 Actualizar README (endpoints nuevos), GLOSSARY y docs de dominio; deuda en
      DEUDA_TECNICA (auto-cierre, FK opaca, deriva de triggers, route matching
      aproximado).
- [ ] 7.2 `openspec validate gamificacion-core --strict` → valid.
- [ ] 7.3 Suite verde completa: `make test-db-up` y
      `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres
      TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test go clean -cache -testcache &&
      go test ./...`; pase de coverage-with-db reportando el porcentaje (no bajar de 85,
      meta 90).
- [ ] 7.4 Swagger final: `swag init --parseDependency -g cmd/api/docs.go --output
      cmd/api/docs` — diff chico y scoped a gamificación (sin churn de metadata ajena).
- [ ] 7.5 Último pase de QA: `go build ./...`, `go vet ./...`, `gofmt -l`.
- [ ] 7.6 Commit único de artefactos + docs + swagger con el subject exacto, stageando
      solo las rutas explícitas listadas arriba (nada de `git add -A`). No pushear ni
      abrir PR: el usuario mergea.
