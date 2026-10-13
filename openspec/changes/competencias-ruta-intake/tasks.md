# Tasks

> Proyecto: competencias-ruta-intake (Spec 4b). Decisiones cerradas con el frontend,
> 2026-10-13. Fuente de verdad: `design.md` (D1-D7). Base: commit `21a5e84`
> (`gamificacion-core`). Implementación en orden Task 1 → Task 4; cada Task es un commit
> Conventional con subject exacto y deja la rama verde.

## Global Constraints (aplicable a TODAS las tasks)

- Rama `feature/competencias-ruta-intake` ya creada desde develop; los artefactos OpenSpec
  viven en `openspec/changes/competencias-ruta-intake/`.
- Commits Conventional con el subject exacto indicado en cada task. Stage SOLO rutas
  explícitas del task — jamás `git add -A` / `git add .`; nada de `.superpowers/`; los
  artefactos de openspec se commitean ÚNICAMENTE en Task 4.
- DAOs testeados contra Postgres real (`make test-db-up`; env
  `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres
  TEST_DB_NAME=paceron_test`). Services/controllers con mocks. La columna nueva
  `route_session_instance_id` entra por AutoMigrate (patrón transparente del repo: BIGINT
  nullable sin constraint).
- Si el Postgres de test no arranca, los DAO tests se skipean solos; no es excusa para
  bajar coverage.
- FailingDB pins no-leak: cualquier 500 de endpoint nuevo devuelve body genérico sin
  filtrar detalles internos de DB (assert en test de controller).
- Guards de 403/400/404 con assert de que el DAO de escritura no se tocó (patrón estándar
  del repo).
- Swagger: regenerar con el comando canónico
  `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`; revisar el diff
  para que no suba churn de metadata ajena a competencias.
- Coverage gate 85 (meta 90): antes de tildar cada task, correr `go clean -cache` +
  `make coverage-with-db` con el analyzer `go-test-coverage --profile`; no bajar el
  umbral.
- 0 cambios de API existente, salvo: validaciones aditivas del create route (Task 1),
  relax del guard de Close para route (Task 3). El servicio de feedback/syncRun NO se
  toca (verification only).
- Comentarios en código: mínimos (solo invariantes/trampas no obvias para IA, en español).
- Repo layout: `cmd/api/{controllers,services,daos,delegates,dbs,domains,constants,
  realtime,restclients,testutils}`; convenciones de capas en `.agentics/CONVENTIONS.md`.

## Tasks

### Task 1: Validaciones de create route (D4/D5) + test suite

Commit: `feat(competencias): validaciones de ruta en el create (D4/D5)`

Files/interfaces:
- `cmd/api/services/competition_service.go` (branch `TypeRoute` de
  `validateCreateRequest` + helper local de longitud Haversine sobre vértices).
- `cmd/api/services/competition_service_test.go` (o archivo nuevo de validation suite).
- `cmd/api/docs/` (regeneración swagger de create).

Steps:
- [ ] 1.1 Branch TypeRoute de `validateCreateRequest`: después del shape existente,
      agregar `routematch.ParseLineString` válido (400 en caso contrario), máximo 500
      vértices (400 "la ruta admite hasta 500 puntos"), longitud Haversine total ≤ 100 km
      (400 "la ruta no puede superar los 100 km").
- [ ] 1.2 Rango de `acceptance_radius_meters` acotado a [5, 500] (400 "el radio de
      aceptación debe estar entre 5 y 500 metros"); campo requerido sin default
      (regresión del shape ya existente cubierta por test).
- [ ] 1.3 Suite de validación en test del service: geojson inválido / >500 vértices /
      >100 km / radio 3 / radio 600 / radio faltante / happy path route en el límite
      (500 vértices y ~100 km exactos pasan).
- [ ] 1.4 Assert de que la ruta inmutable post-create NO introduce endpoint de edición
      (solo nota de test/convención; nada que agregar).
- [ ] 1.5 Regenerar swagger (create route validations) y revisar diff limpio.
- [ ] 1.6 `go clean -cache` + `make coverage-with-db` + vet/gofmt/build en verde antes de
      tildar.

### Task 2: Instancia sintética + POST attempts + rama de autorización WS (D1/D2/D8)

Commit: `feat(competencias): instancia sintética de ruta y intake POST attempts (D1/D2/D8)`

Files/interfaces:
- `cmd/api/domains/dbs/competition.go` (campo `RouteSessionInstanceID *int64`).
- `cmd/api/daos/competition_dao.go` (Create tx-extendido + `ExistsAttemptByCompetitionAnd
  Athlete` + create de runner_session para el intake) + `cmd/api/daos/competition_dao_
  test.go` (Postgres real).
- `cmd/api/services/competition_service.go` (branch Create route: tx de instancia
  sintética + `CreateAttempt` con guards) + `cmd/api/services/competition_service_test.go`
  y `competition_attempt_test.go` (mocks).
- `cmd/api/controllers/competition_controller.go` (handler `CreateAttempt`) + test.
- `cmd/api/app/url_mappings.go` (ruta `POST /api/v1/competitions/:id/attempts`).
- `cmd/api/docs/` (regeneración swagger del endpoint nuevo).

Steps:
- [ ] 2.1 Columna `route_session_instance_id` en el model `dbs.Competition` (BIGINT
      nullable sin FK) + AutoMigrate del test bootstrap si corresponde.
- [ ] 2.2 Branch route del `Create` del service: en la MISMA transacción, crear
      session_instance {name "Competencia ruta N", description instancia sintética de la
      competencia de ruta, source_session_id NULL} + exercise_instance {name "Corrida",
      kind `velocidad_distancia`, resto NULL} + session_exercise_instances
      {role='main', repeat_count=1, rest_minutes=0}; la instancia queda sin día (group_
      calendar_days NO la referencia — no se crea ningún día).
- [ ] 2.3 Persistir `competition.route_session_instance_id` en el insert de la
      competencia; `type='metric'` la deja NULL (test: create metric no crea instancia).
- [ ] 2.4 Creación de instancia dentro del tx de Create: verificar rollback (si falla la
      creación de la instancia, no queda competencia huérfana ni viceversa — test contra
      Postgres real).
- [ ] 2.5 Service `CreateAttempt` con los 5 guards de D2 en orden: 404 inexistente → 400
      type != route ("solo las competencias de ruta admiten intentos") → 400 status !=
      active ("la competencia no está activa") → 403 no participante → 400 doble intento
      (via ExistsAttempt + respaldo del unique DB). Fecha/rango: no aplica.
- [ ] 2.6 Insert de `runner_session {session_instance_id = route_session_instance_id,
      athlete_user_id = caller, status='wip', competition_id}` sin gate presencial;
      respuesta 201 `{data: {runner_session_id, session_instance_id, competition_id}}`.
      El `competition_id` seteado al crear hace que el finish YA existente
      (`PATCH /session-instances/:id/runner`, sin cambios) emita el frame de D6 tal
      cual.
- [ ] 2.7 Controller handler + mapping de errores de servicio (400 ErrCompetitionInvalid,
      403 ErrCompetitionForbidden, 404) con 500 genérico sin leak en FailingDB; registro
      de ruta en url_mappings; comando canónico de swagger; e2e DAO de doble intento
      contra Postgres real (segundo insert viola el unique).
- [ ] 2.8 Tercera rama en `HasInstanceAccess` (D8): join
      `competitions` + `competition_participants` por `route_session_instance_id`,
     authorize participant de competencia `type='route'` SIN filtro de status +
      tests DAO de la rama nueva contra Postgres real: participante sí /
      usuario ajeno no / competencia metric NO autoriza por esta rama / runner session
      wip sin feedback igualmente autoriza por la 3ª rama (regresión: ramas dual día y
      feedback intactas).
- [ ] 2.9 `go clean -cache` + `make coverage-with-db` + vet/gofmt/build en verde antes de
      tildar.

### Task 3: Cierre route sin end_at + WS frame de intento-nuevo e2e (D3/D6)

Commit: `feat(competencias): cierre de ruta sin end_at y frame WS de intento (D3/D6)`

Files/interfaces:
- `cmd/api/services/competition_service.go` (guard end_at de `Close` condicionado a
  TypeMetric) + `cmd/api/services/competition_close_route_test.go` (o el existente de
  winners).
- `cmd/api/controllers/competition_controller_test.go` (e2e hub del frame
  update:competition_progress SOLO si falta y cabe en ≤ 20 líneas de test).

Steps:
- [ ] 3.1 `Close`: el guard "end_at ya pasado" (`EndAt != nil && now < *EndAt` → 400)
      pasa a aplicarse SOLO cuando `type='metric'`; route se cierra sin ventana. El resto
      queda byte-idéntico: guard creador/owner 403, solo active → 400, race-safety del
      Finish.
- [ ] 3.2 Test de cierre route: happy path con 2 intentos finished anclados a la
      instancia sintética (winners por match % reutilizando `routeAttempts`/`MatchRatio`
      — sin duplicar cálculo), cierre sin intentos → finished con winners vacía, cierre
      de metric antes de end_at sigue dando 400 (regresión del guard original), cierre
      route por creador runner OK y por corredor ajeno 403.
- [ ] 3.3 Verificar (sin código nuevo de protocolo) que el `Finish` de la runner session
      con competition_id seteado emite `update:competition_progress {competition_id,
      user_id}` por `session:{route_session_instance_id}`; si el e2e contra el hub de
      competition_controller_test no cubre el caso "intento nacido de POST attempts",
      extender el existente (SOLO si ≤ 20 líneas).
- [ ] 3.4 Asegurar que `control:competition_finished` del close route emite por los
      intentos registrados (ya cubierto por gamificacion-core — reuso del test existente
      si corresponde).
- [ ] 3.5 `go clean -cache` + `make coverage-with-db` + vet/gofmt/build en verde antes de
      tildar.

### Task 4: Docs + validación openspec + verificación final

Commit: `docs(openspec): propuesta competencias-ruta-intake (spec 4b)`

Files/interfaces:
- `docs/CATALOGO_Y_CALENDARIO.md` (nota de competencia route + instancia sintética, D1/D5
  radio default sugerido 50 m para el front).
- `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (impacto del endpoint POST attempts).
- `docs/DEUDA_TECNICA_Y_PENDIENTES.md` (RUTA v1: intento wip/interrupted que no borra
  ni reabre; leaderboard live mid-attempt; auto-cierre).
- `openspec/changes/competencias-ruta-intake/**` (este commit ÚNICO de artefactos).

Steps:
- [ ] 4.1 Documentar en las 3 docs el flujo de ruta v1 (D1-D7), la deuda marcada y el
      default sugerido de radio (50 m) — textos en español y limpios (sin mojibake).
- [ ] 4.2 Tildar los checkboxes de Task 1-3 en este tasks.md (memoria persistente real).
- [ ] 4.3 `openspec validate competencias-ruta-intake --strict` → valid.
- [ ] 4.4 Verificación final: `grep -P '[\x{4E00}-\x{9FFF}\x{3040}-\x{30FF}]'` sobre
      openspec/changes/competencias-ruta-intake/ → 0 matches; suite completa post-clear
      verde; coverage gate 85; build/vet/gofmt OK.
- [ ] 4.5 Commit único de artefactos OpenSpec (stage explícito de rutas bajo el change;
      jamás `git add -A`).
