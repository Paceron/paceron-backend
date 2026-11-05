# Tasks — seed de perfil full

## Global Constraints

- Rama `feature/seed-perfil-full` (ya creada desde `develop`). Un commit Conventional
  por task, staging **solo de rutas explícitas** del task; nunca `git add -A`, nunca
  commitear `.superpowers/`.
- El perfil `dashboard` debe quedar **byte-idéntico**: sus flags, ids 29/39, plan de
  defectos, verify y tests no se tocan. Todo lo nuevo vive detrás de `--profile full`.
- Guardas existentes obligatorias: `SEEDING_ENABLED=true` y `--stage=local`
  (`config.AssertSeedingStage`).
- Base de test: Postgres real `docker start paceron-test-db` (:5433) con
  `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres
  TEST_DB_PASSWORD=postgres TEST_DB_NAME=paceron_test`.
- Fechas siempre relativas a `NOW` con TZ America/Argentina/Cordoba (`gametime`); nada
  hardcodeado a un día calendario.
- Determinismo: semilla fija propia para full; mismo input ⇒ mismo universo.
- Comentarios de código mínimos (solo invariantes no obvios). Español en docs.
- Sin red: no llamar a MP, Expo ni storage. Todos los ids/tokens son sintéticos.
- Gate final: `go build ./...`, `go vet ./...`, `go test ./...` con Postgres real (0
  FAIL), coverage ≥85 con `go clean -cache` + analyzer, `openspec validate
  seed-perfil-full --strict`.
- No push/merge: el usuario pushea y mergea; el agente solo actualiza la PR.

### Task 1: Infraestructura del perfil full

**Files:**
- Modify: `cmd/api/scripts/seed_dashboard/main.go` (flag `--profile`, dispatch, guardas
  de scope/marker)
- Create: `cmd/api/scripts/seed_dashboard/profile_full.go` (seeder full, scope ids,
  marker, `runFullSeed`)
- Modify: `Makefile` (familia `fullseed-*`)
- Test: `cmd/api/scripts/seed_dashboard/profile_full_test.go` (dispatch, defaults,
  rechazo de perfil inválido, guard de ids ocupados)

- [x] 1.1 Agregar `--profile dashboard|full` con default `dashboard`; el camino
      dashboard no cambia en absoluto.
- [x] 1.2 Implementar el despacho a `runFullSeed` con scope ids en rango alto
      (equipos 1001+, grupos 2001+) y marker `[seed_full]`; abortar si un id de scope
      ya existe sin el marker.
- [x] 1.3 Agregar targets `fullseed-seed`, `fullseed-reset`, `fullseed-cleanup`,
      `fullseed-verify` sin tocar `seed-dashboard*` ni `demo-*`.
- [x] 1.4 Tests del dispatch y del guard; suite del paquete verde contra Postgres real.
- [x] 1.5 Commit `feat(seed): perfil full como modo del seed existente`.

### Task 2: Identidad, roles, tiers y suscripciones

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_identity.go` (+ test).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (wire `seedIdentity`).
- Interfaces: requiere los DAOs/modelos de `dbs` y el paquete `crypto` para cifrado.

- [x] 2.1 Crear usuarios del perfil full con bcrypt de `Paceron.123` (protagonistas y
      extras), status `active` salvo los casos de borde (`blocked`, `inactive`, sin
      rol, `allow_team_invitations=false`).
- [x] 2.2 Sembrar roles `entrenador`/`corredor`, 3 tiers por rol (base gratuito
      hierarchy=1, medium, premium), permisos y `tier_permissions` completos.
- [x] 2.3 Asignar `user_roles` (trainer con entrenador+corredor; corredores con
      corredor) y suscripciones coherentes con los índices únicos parciales: R1
      premium activa con #1/#2 pagas y #3 pendiente, R2 deuda, R3
      `first_payment_pending`.
- [x] 2.4 `platform_settings.marketplace_fee_percent` como jsonb numérico.
- [x] 2.5 Tests de contrato: login de protagonistas, rechazo de blocked/inactive,
      `GET /auth/permissions` completo, slugs/estados de suscripción.
- [x] 2.6 Commit `feat(seed): identidad, roles y suscripciones del perfil full`.

### Task 3: Equipos, grupos, memberships, invitaciones y solicitudes

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_teams.go` (+ test).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (wire `seedTeams`).

- [x] 3.1 Sembrar los 6 equipos de la matriz del design (A normal, B pago, C lleno, D
      oculto, E cerrado, F colisión) con su ubicación, fee, max_members y owner.
- [x] 3.2 `team_users` (owner entrenador + corredores con `subscription_status`
      correcto) y `group_users` (activa, vencida y futura), incluyendo un runner en
      dos equipos.
- [x] 3.3 Invitaciones: vigente, expirada, aceptada y rechazada; una al usuario con
      invitaciones desactivadas. Join requests: pendiente a A y E, una a C (cupo) y
      una rechazada.
- [x] 3.4 `seller_connections`: trainer 1 conectado (client_id = `MP_OAUTH_CLIENT_ID`,
      `public_key` no vacía, tokens cifrados con `TOKEN_ENCRYPTION_KEY`); trainer 2
      sin conexión.
- [x] 3.5 Tests de contrato: búsqueda de equipos (visible/oculto/ajeno), cupo lleno,
      join request a cerrado, invitación a usuario con invitaciones off,
      `can_receive_payments`.
- [x] 3.6 Commit `feat(seed): equipos, memberships e invitaciones del perfil full`.

### Task 4: Catálogo y calendario dinámico

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_catalog.go` y `full_calendar.go`
  (+ tests).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (wire `seedCatalogCalendar`).

- [x] 4.1 Catálogo ampliado: ejercicios de todos los kinds vigentes, sesiones con
      warmup/main/cooldown y planes de 7/14/21 días con `sequence_no` 1..N.
- [x] 4.2 Calendario pasado (-21..-1) con stamp real; futuro (+1..+14) con training
      presencial/async, other, rest y cancelled con motivo.
- [x] 4.3 HOY por inserción directa: presencial abierto en el grupo principal de A
      (ventana desde `max(now-30m, 00:00 local)` hasta `min(now+2h, 23:59 local)`,
      `opened_at` seteado), presencial no abierto en el secundario, presencial cerrado
      en F y training async hoy en `group_b` (Equipo B).
      El async va en B porque los dos grupos de A tienen ocupada su única fila de hoy
      por los escenarios presenciales; R1/R2 tienen acceso a B.
- [x] 4.4 Colisión presencial A vs F y warning same-team entre grupos de A.
- [x] 4.5 Tests de contrato: GetRange de A de -21..+14 y async de hoy en B, presencial
      abierto, next-session y next-presencial-session con banners, estados 409 de las
      variantes no-abierto/cerrado, y ventana HOY en horarios normales, tarde-noche y
      poco después de medianoche.
- [x] 4.6 Commit `feat(seed): catalogo y calendario dinamico del perfil full`.

### Task 5: Actividad histórica y de hoy

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_activity.go` (+ test).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (wire `seedActivity`).

- [x] 5.1 `runner_session`: finished pasadas, wip de hoy, interrupted histórica.
- [x] 5.2 Feedback con todos los `completion_status`, métricas coherentes, feedback
      del entrenador como owner y un soft-deleted; puntos GPS en 2 series para
      recomputar pace/peak.
- [x] 5.3 Asistencias en -14/-7 y hoy, mezcla `qr`/`manual`, con no-confirmados en el
      roster.
- [x] 5.4 Tests de contrato: historial de feedback (filtros team/group/fecha/
      ejercicio/set), roster con attended/not_confirmed, feedback soft-deleted
      ausente.
- [x] 5.5 Commit `feat(seed): actividad, feedback y asistencias del perfil full`.

### Task 6: Gamificación

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_gamification.go` (+ test).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (wire `seedGamification`).
- Modify: `cmd/api/scripts/seed_dashboard/full_catalog.go` (plan auxiliar de racha
  usado solo en el grupo secundario del perfil full).

- [x] 6.1 Crear un plan full-only de 9 días de training en el grupo secundario de A
      (-16..-8) y sesiones finished para R4 en esos días; junto al finished de T5 en
      -7, debe dar strict/flexible current=10. R5 termina los primeros cuatro días y
      deja fallar los siguientes (best=4, current=0); R9 prueba días skipped antes de
      su alta futura en el grupo. `streak_state` debe ser coherente.
- [x] 6.2 `user_achievement_progress` con ids del catálogo: un corredor con varios
      desbloqueados y otro vacío.
- [x] 6.3 Filas de `gamification_unseen` para `streaks`/`achievements` (un protagonista
      con punto rojo y otro visto); T7 sembrará `competitions` y las estampas al crear
      competencias terminadas con winners.
- [x] 6.4 Tests de contrato: GET streaks (current/best y estados por día), achievements,
      stamps en estado vacío hasta T7, unseen por sección y leaderboard de las 6 métricas
      con record all.
- [x] 6.5 Commit `feat(seed): gamificacion del perfil full`.

### Task 7: Competencias en todos los estados

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_competitions.go` (+ test).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (wire `seedCompetitions`).

- [x] 7.1 Sembrar pending_approval, active metric, active route (max_attempts=3, un
      intento 100% y otro parcial + wip), finished metric con empate, finished route
      con ganador por duración, finished route sin intentos, rejected con motivo y una
      con límite agotado.
- [x] 7.2 Geometría de ruta válida ([lng,lat], ≤500 vértices, ≤100 km, radio 50 m),
      instancias sintéticas compartidas y por intento, y puntos GPS sobre el feedback
      del intento (±10 m) para compliance 100%/parcial.
- [x] 7.3 Efectos del cierre para las finished: winners, estampas y unseen coherentes;
      `attempts_used`/`can_attempt` correctos por participante.
- [x] 7.4 Tests de contrato: listado/detalle por estado, attempts por atleta, progress
      route/metric, winners y `can_attempt`.
- [x] 7.5 Commit `feat(seed): competencias del perfil full`.

### Task 8: Pagos, mensajería y push

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_payments.go`,
  `full_messages.go` (+ tests).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (wire `seedPaymentsAndMessages`).

- [x] 8.1 Suscripciones/cuotas/pagos: historial pagado, próxima cuota, deuda que
      bloquea la baja del equipo, `first_payment_pending`, pagos rechazados; CHECK de
      arco exclusivo y contadores coherentes.
- [x] 8.2 Cobros del entrenador con split (`seller_user_id`, `marketplace_fee`,
      `raw_response.net_received_amount`) y `payment_id`/`external_reference` válidos.
- [x] 8.3 Mensajes de sesión en hoy y en una pasada: tipos, modos, reply y recipients
      coherentes.
- [x] 8.4 Push tokens sintéticos por protagonista.
- [x] 8.5 Tests de contrato: historial del corredor, cobros del entrenador, deuda
      bloqueando la baja, mensajes visibles por modo, `can_receive_payments`.
- [x] 8.6 Commit `feat(seed): pagos, mensajeria y push del perfil full`.

### Task 9: Verify y cleanup del perfil full

**Files:**
- Create: `cmd/api/scripts/seed_dashboard/full_verify.go`,
  `full_cleanup.go` (+ tests).
- Modify: `cmd/api/scripts/seed_dashboard/profile_full.go` (orchestrate verify/cleanup).
- Modify: `cmd/api/scripts/seed_dashboard/main.go` (wiring de `--verify` y cleanup full).

- [x] 9.1 `fullseed-verify`: login de protagonistas, calendario de hoy, banners,
      streaks/logros/estampas/unseen, leaderboard, competencias por estado e historial
      de pagos; sale con error si algo no es coherente.
- [x] 9.2 Cleanup full transaccional en orden inverso de FK, acotado a scope + marker,
      sin tocar el perfil dashboard ni datos reales; exige `--yes`.
- [x] 9.3 Test de ciclo completo: seed → verify → cleanup → verify vacío (contra
      Postgres descartable).
- [x] 9.4 Commit `feat(seed): verificacion y limpieza del perfil full`.

### Task 10: Documentación y verificación final

**Files:**
- Modify: `cmd/api/scripts/seed_dashboard/RUNBOOK.md`,
  `docs/ENTORNO_LOCAL.md`, `README.md` (si corresponde).

- [x] 10.1 RUNBOOK: sección "perfil full" con requisitos de `.env.local`, comandos,
      usuarios/password, mapa de equipos y casos, y advertencia de base reseteada.
- [x] 10.2 `docs/ENTORNO_LOCAL.md`: reemplazar la deuda "no existe todavía" por el
      perfil full y su familia de comandos.
- [x] 10.3 Verificación final completa (build/vet/suite/coverage/validate) y tildes.
- [x] 10.4 Commit `docs(seed): documentar el perfil full`.
