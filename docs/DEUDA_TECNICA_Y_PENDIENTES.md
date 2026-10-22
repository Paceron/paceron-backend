# Deuda técnica y decisiones pendientes conocidas

Conocimiento acumulado en sesiones previas de asistente de IA (memoria privada de esas sesiones, no reconstruible por otra herramienta) que nunca llegó a quedar escrito en el repo. Volcado acá el 2026-09-19 al migrar el desarrollo de este repo a OpenCode, para que no se pierda. Actualizar esta lista a medida que cada ítem se resuelve o se descarta — no es un backlog formal, es memoria institucional.

## Bugs conocidos, no arreglados

### `ActivateEntrenador` falla 500 en testing (nombre de tier)

`POST /api/v1/users/:id/trainer-role` (`services/user_role_service.go`, `defaultTierName = "base"`) busca un tier llamado exactamente `"base"` para el rol `entrenador` vía `tierDao.FindByNameAndRole`. En la DB de testing, los tiers de `entrenador` están nombrados `"base entrenador"`/`"medium entrenador"`/`"premium entrenador"` — no `"base"` a secas. Cualquier usuario nuevo que intente activar entrenador en testing hoy falla con 500. No verificado en producción. No es un bug introducido por ningún PR reciente, el código no se tocó.

Dos alternativas, a decidir:
1. Corregir el seed de testing para que el tier base de entrenador se llame `"base"`.
2. Cambiar la búsqueda en `user_role_service.go` para matchear por `hierarchy`/`payment_required=false` en vez de nombre literal — más robusto, pero es cambio de código. Conecta con `openspec/changes/cambio-tier-suscripciones/` (usa `hierarchy` como fuente de verdad) — si esa spec avanza, este bug se resuelve como efecto colateral.

### `HandleWebhook` de Mercado Pago no valida la firma HMAC

**RESUELTO** (change `optimizaciones-y-seguridad-api`, 2026-10-19, Task 7): ambos webhooks MP (pagos y deauth) validan la firma con `MERCADOPAGO_WEBHOOK_SECRET`; sin secret configurado aceptan sin validar + warning (rollout gradual). Mecánica en `docs/CATALOGO_Y_CALENDARIO.md` §8.15. Residual: tolerancia temporal no configurada — ver sección "Seguridad" de esta lista.

### El webhook lee los pagos de equipo con el token de la plataforma

`HandleWebhook` (`services/payment_service.go`) consulta el pago a Mercado Pago con `s.accessToken`, el token de Paceron. Los pagos de membresía de equipo (`concept = team_subscription`) se crean con el token OAuth del **entrenador** (`resolveTeamSplitConfig`), así que el pago vive en la cuenta del vendedor y esa lectura probablemente falla (404/403). No verificado contra MP.

Consecuencias, si se confirma:
- La cuota de equipo no pasa a `paid`: el paso que la confirma (`applyApprovedTeamInstallment`) corre dentro del webhook.
- `payments.raw_response` se queda con el `PaymentResult` que guardó `ProcessPayment`, que no trae `transaction_details`. Por eso el historial de cobros (`GET /api/v1/payments/received`, change `historial-pagos-cobros-entrenador`) devuelve `net_amount = null` en casi todos los cobros: el neto real solo existe cuando el webhook guardó el `payment.Response` completo.

Fix probable: en el webhook, si el pago local es `team_subscription`, resolver el vendedor por `seller_user_id`, descifrar su token desde `seller_connections` y consultar con ese. Toca el flujo de pagos, así que va en su propio change.

### `CreatePreference` deja una fila `pending` sin `payment_id` por cada cobro

`CreatePreference` inserta un `payments` con `status = pending` y `payment_id = ''`, y le pone `external_reference` = su propio id local. Después `ProcessPayment` busca una fila existente con `FindByExternalReference(req.PreferenceID)`: compara contra el `preference_id` de MP, nunca coincide y crea otra fila. Resultado: por cada pago real queda una fila huérfana que nunca llegó a Mercado Pago.

El historial de pagos las filtra con `COALESCE(payment_id, '') <> ''` (design D7 de `historial-pagos-cobros-entrenador`). Cualquier consulta nueva sobre `payments` tiene que hacer lo mismo, o va a contar pendientes de más. Fix de fondo: que `ProcessPayment` reutilice la fila de la preferencia buscándola por `preference_id`, o que `CreatePreference` no inserte nada.

## Datos desactualizados (testing)

### `tiers`: `hierarchy`/`payment_required`/`tier_amount` sin backfill

Las filas de tiers no-base de la DB de testing fueron creadas antes de que `payment_required`/`tier_amount`/`hierarchy` importaran (feature de pagos llegó después) — nadie las backfilleó. `hierarchy` está en `0` en las 6 filas (debería ser base=1 < medium=2 < premium=3 por rol).

Fix acordado, no aplicado (esperando luz verde explícita del usuario para correrlo):

```sql
UPDATE tiers SET hierarchy = 1 WHERE id = 1;  -- base corredor (payment_required/tier_amount ya correctos)
UPDATE tiers SET payment_required = true, tier_amount = 10000, hierarchy = 2 WHERE id = 4;  -- premium corredor
UPDATE tiers SET hierarchy = 1 WHERE id = 3;  -- base entrenador
UPDATE tiers SET payment_required = true, tier_amount = 100000, hierarchy = 2 WHERE id = 5;  -- medium entrenador
UPDATE tiers SET payment_required = true, tier_amount = 350000, hierarchy = 3 WHERE id = 2;  -- premium entrenador
-- id = 6 (medium corredor) NO tocar — ver ítem siguiente
```

**Gap de código relacionado** (no cubierto por este fix de datos): `tier_service.go` `Create`/`Update` nunca escriben `Hierarchy` (siempre default 0) y no fuerzan `payment_required=false` para tiers llamados "base". Cualquier tier creado hoy vía API repite el mismo problema.

### `tiers.id=6` ("medium corredor") no debería existir

Regla de negocio del usuario: el rol `corredor` solo debe tener 2 tiers (Base gratis, Premium $10000) — `entrenador` sí tiene 3 (base/medium/premium) a propósito. La fila `id=6` fue agregada por error junto con el medium de entrenador (legítimo). **No tocar sin confirmación fresca** — antes de borrar (aunque sea soft-delete), confirmar que ninguna `subscription`/`installment` ya apunta a `tier_id=6`. Pendiente comunicárselo a quien administra los tiers.

## Features deferidas, con diseño ya charlado

### Aceptar invitación por botón en el email (magic link)

Hoy solo se acepta/rechaza in-app (`POST /api/v1/invitations/:id/accept|reject` con `user_id`, JWT de sesión normal) — explícitamente fuera de alcance en `openspec/changes/persistir-invitaciones-aceptar-rechazar/design.md`. Decisión: cuando se aborde, un botón en el email que acepte vía un **token de un solo uso, scopeado únicamente a aceptar esa invitación puntual** (no el JWT de sesión regular) — mismo patrón que `password_reset_token` pero como link en vez de código tipeado. Agregar un endpoint no autenticado `GET/POST /api/v1/invitations/:id/accept?token=...` que valide el token en vez de confiar en `user_id`, expira igual que la invitación. Los endpoints in-app existentes quedan sin cambios — esto es un camino adicional, no un reemplazo.

### Rol "admin" global (post-MVP)

Planeado para después del MVP: moderación a nivel plataforma (banear usuarios, soporte) — distinto del rol `entrenador` (rol de negocio, no de plataforma). Por eso `/api/v1/roles`, `/api/v1/tiers`, `/api/v1/permissions`, `/api/v1/auth/permissions` y `/api/v1/platform-settings/marketplace-fee` (GET y PUT) hoy solo exigen login, sin role-gating (deuda documentada en `docs/AUTH_MIGRATION.md` §6). No inventar un check de admin ad-hoc para tapar ese gap — cuando se aborde, es su propio change de OpenSpec (qué puede hacer, cómo se otorga, audit trail). No tiene relación con la auto-activación de entrenador (`POST/DELETE /api/v1/users/:id/entrenador-role`), que es una transacción del propio usuario sobre su cuenta, no una acción de moderación sobre la cuenta de otro.

Re-auditado en la ronda de seguridad del change `optimizaciones-y-seguridad-api` (2026-10-16): los guards de roles/permissions/tiers y de platform-settings-fee siguen sin autorización — **esperan el módulo admin**, no se tapan ad-hoc.

### Persistencia de mensajes WS críticos (deferida — Gap 27)

El relay dirigido de presence/control (change `sesion-interrumpida-y-eventos-en-vivo`) y todos los frames del gateway son **efímeros, en memoria del hub del proceso** — sin persistencia ni replay: quien esté offline al emitirse (o ante corte por deploy/overflow) no recibe nada; la recuperación es refetch HTTP (también explícito para el cliente en `REALTIME_WS.md` §6). Decisión de diseño explícita de Gap 27: la persistencia de mensajes críticos (con replay al resuscribir) quedó DEFERIDA. Si se aborda: es un change propio (storage de eventos, cursor/last-event-id por conexión, replay al subscribe/reconectar, y probablemente el salto a multi-instancia con Redis pub/sub — el punto "Escalar a múltiples instancias" de `docs/REALTIME_WS.md` §8).

### Búsqueda de equipos + solicitudes de ingreso (join requests)
Spec de frontend: `paceron-frontend/docs/superpowers/specs/2026-09-03-team-search-join-requests-design.md` (aprobada del lado frontend, contrato de backend todavía **no** confirmado — esa sección es una propuesta). Frontend construye contra mocks mientras tanto, no está bloqueado.

Deferido explícitamente para evitar colisión con el branch en paralelo `feature/suscripciones-tier-equipos` (team-split payments) — ambas features tocan el mismo punto exacto: la creación de `TeamUser` necesita pasar por un único lugar, porque ahí es donde aplica el gate de cuota de membresía. Ese branch ya extrajo `ApplyTeamMembershipGate(ctx, db, teamUserDao, installDao, teamUser, membershipFee)`, llamado desde `AddUser` y `AcceptInvitation`.

**Decisiones ya confirmadas, siguen válidas al retomar:**
1. `visible`/`is_public` en `Team`: no endpoint nuevo, extender `UpdateTeamRequest` (`Visible *bool`/`IsPublic *bool`, mismo patrón que `ShowGroupsToRunners`), servido por el `PUT /api/v1/teams/:id` ya existente.
2. Badge de pendientes: endpoint dedicado `GET /api/v1/join-requests/pending-count` (COUNT agregado sobre todos los equipos del caller), simétrico a `myInvitationsCount`.
3. Grupo default al aceptar: mismo fallback que `AcceptInvitation` (`Group.IsMain` si no se elige uno).

**Cómo retomar:** confirmar que `feature/suscripciones-tier-equipos` ya mergeó (`git log develop`), verificar que `ApplyTeamMembershipGate` (o su nombre post-merge) sigue con forma similar, diseñar `AcceptJoinRequest` para llamarlo directo desde el día uno (sin refactor previo). Diseño se cortó en brainstorming, antes de secciones formales — todavía pendiente: un change o dos (search vs. join-requests), códigos de error, forma de paginación (sin precedente en el repo hoy), plan de tests.

### Race condition de cupo de equipo (`MaxMembers`)

Check-then-act sin lock, en 3 call sites: `team_user_service.go:AddUser`, `invitation_service.go:AcceptInvitation`, y el futuro `join_request_service.go:Accept`. Dos altas casi simultáneas para el mismo equipo pueden pasar el check ambas y pasarse del cupo por 1.

Decisión: no parchear un solo call site (evaluado `SELECT ... FOR UPDATE` solo para `join-requests`, descartado por inconsistente). El efecto de la carrera es auto-corregible (el entrenador saca al corredor de más manualmente, sin corrupción de datos/dinero) — se acepta el riesgo hasta poder arreglar los 3 call sites juntos, misma estrategia de lock (`SELECT FOR UPDATE` sobre la fila de equipo dentro de una transacción es la más probable), no uno por uno.

### `GET /auth/permissions?user_id=` sin guard self/owner (decisión de producto pendiente)

Al auditar la authz del change `latlng-optimizaciones-y-endurecimiento` (2026-10-11) quedó marcado: cualquiera autenticado puede consultar permisos de `user_id` distintos del propio — solo exige login. Se dejó **sin cambio a propósito**: meter un guard self/owner sin definición de producto rompería cualquier uso legítimo hoy existente de consultas cruzadas. Cuando se decida qué usuarios pueden leer permisos de quién, implementar el guard (403 al patrón member-calendar). No considerar bug cerrado hasta esa decisión.

## Pendientes de limpieza

### Flake de `TestFailingDB_*`: race de migraciones concurrentes sobre la misma test DB

Cuando corren a la vez dos paquetes que migran (SetupTestDB de `testutils`) contra la misma DB de test, a veces uno falla con SQLSTATE 23505 (unique violation en tablas de migración). Pasa aislado y en re-run; no es un bug del helper. Se detectó con `go test ./...` en paralelo (2026-09-22, change `cobertura-calendario-recientes`, confirmado también en `workout-feedback-history`). **MITIGADO**: `SetupTestDB` toma ahora un advisory lock de Postgres (`pg_advisory_lock`) con conexión dedicada antes de migrar (`connectSerialized` en `cmd/api/testutils/db.go`), serializando migraciones concurrentes. La mitigación quedó en su lugar con el change `sesion-interrumpida-y-eventos-en-vivo`; si el flake reaparece en otra forma, cazarlo de nuevo ahí.

### Normalización de escritura de `date_start`/`date_end` NO requerida (Gap 25)

El fix de membresía por fecha (change `sesion-interrumpida-y-eventos-en-vivo`) compara por `::date` en lectura, **no** normaliza la escritura: `group_users.date_start`/`date_end` siguen timestamptz y las filas existentes (y las nuevas que viaje el frontend con hora) quedan como vienen. Fuente de verdad de la elegibilidad es la comparación por fecha en `activeGroupMemberWhere`/`findGroupRosterWithAttendanceSQL` — si algún día se quisiera normalizar a medianoche local, es mejora cosmética, no requisito.
### Archivo trackeado por error: `.superpowers/sdd/tasks/task-3-report.md`

Sobrevive en `develop` trackeado en git (entró con commits de Task 3 del change `asignacion-por-instanciacion`, `df18d60`/`2babf15`, por un `git add` amplio del implementador que forzó/bypaseó el ignore del workspace SDD). El workspace `.superpowers/sdd/` es scratch auto-ignoreado (su propio `.gitignore` con `*`) — briefs/reports/ledger de `subagent-driven-development` son artefactos de recuperación intra-sesión, sin valor en el historial. Fix: `git rm .superpowers/sdd/tasks/task-3-report.md`, aprovechado cualquier rama futura (no amerita PR propio). Prevención: en dispatchs de implementadores exigir stageear rutas explícitas por commit, nunca `git add -A`/`git add .` en la raíz del repo ni force-add de rutas ignoradas (registrado también en `AGENTS.md` §9).

## Rendimiento residual (post change `latlng-optimizaciones-y-endurecimiento`)

### Detección de colisiones presencial por fecha en stamp/bulk

El guard pre-escritura de stamp/bulk quedó en lecturas constantes (Task 8), pero la detección de colisiones presencial sigue corriendo **1 query por fecha con día presencial** (`findPresencialCollisions` reutilizado per-date). Para lotes asincrónicos no agrega queries. Resolución futura: reutilizar la clasificación in-memory de `AdministeredCalendar` (T5) alimentada por 1 lectura de todas las fechas del owner, preservando el 409/same_team_warnings byte-idéntico.

## Gamificación (change `gamificacion-core`, 2026-10-13)

Ítems de deuda/decisión del módulo de gamificación (rachas, logros, leaderboard, competencias). Mecánica en `docs/CATALOGO_Y_CALENDARIO.md` §8.14; contratos en `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §16 y §17 (ruta, `competencias-ruta-intake`).

### Auto-cierre de competencias por cron (deuda explícita, D13)

El cierre de competencias es **SIEMPRE manual** (`PATCH /competitions/:id/close`): no existe el cron que cierra al vencer `end_at`. El guard de ventana es **metric-only** (spec 4b, `competencias-ruta-intake`): cerrar una `metric` antes de que termine su `end_at` da `400`; una `route` se cierra cuando el creador/owner decide (no tiene ventana). El frontend debe mostrar el botón de cierre al creador/owner en ambos tipos.

### Deriva del estado de triggers de racha (aproximado, D14)

`gamification_streak_state` es el baseline de los triggers de racha; su fallo/deriva solo puede producir **punto rojo tarde o doble** — el GET de rachas es exacto y no la consulta. Aceptado por diseño; no "arreglar" sin reabrir D3/D14.

### `achieved_at` del record de métricas SUM = última fila contribuyente

Para `distance`/`duration` (agregación SUM), `record.achieved_at` es la `session_date` de la **última fila que contribuyó** al valor acumulado — es la interpretación elegida de "cuándo se alcanzó el récord" (el valor acumulado solo existe al cerrar esa fila). Para `peak_speed` es la fila del argmax y para `sessions_count`/`attendance_count` la última actividad.

### Wording "record máximo histórico" para pace = MEJOR (mínimo) ritmo

La spec decía "máximo histórico del team" genérico; para `pace` el record es el valor **más bajo** (mejor ritmo) — implementado así y aclarado en el spec delta (`gamification-leaderboards`).

### `BulkCreatePoints` del DAO de feedback es operación muerta

Solo `BulkCreatePointsRecompute` está en uso (subida de points del leaderboard, D6). La variante sin recompute quedó sin call sites — marcar/eliminar en una futura limpieza (no hacerla en caliente).

### Frontera de membresía mixta: `date_end > NOW()` (instante) vs `::date >= hoy`

`FindActiveGroupMemberships` (gamificación) evalúa la vigencia con **instante** (`date_end > NOW()`), mientras otros dominios del repo comparan por `::date >= fecha`. Consistente **dentro del módulo** (todas las queries de gamificación usan la misma frontera); unificar con el resto del repo es cambio transversal aparte.

### Rachas solo sobre memberships activas: el stint histórico no cuenta

Las rachas se computan sobre los grupos con membresía **activa hoy**: si la membresía de un grupo cerró (`date_end` pasada), sus días de calendario dejan de contar para current/best aunque hayan contado en su momento. Racha "histórica" de equipos abandonados desaparece del GET por diseño.

### `date_start` entre 21:00 y 23:59 Cordoba resta hasta 1 día en la frontera de alta

El mapeo de día (`utcDateToDay`) toma la **fecha UTC** del timestamptz: un `date_start` a la noche de Cordoba (UTC-3) cae en el día UTC siguiente y ese día de alta puede quedar `skipped` (previo a la membresía) en vez de contable. Cosmético, borde de alta; arreglaría comparando en TZ Cordoba.

### Minors de tests/internals (tasks 1-6)

- **Root cause del error del trigger sin log crudo:** `EvaluateRunnerCompletion` envuelve los errores como `ErrStreakTrigger` con sufijo pero **descarta el error de driver original** — el log del trigger no muestra la causa cruda (`gamification_streak_trigger.go`). Wrap con `%v` del error fuente cuando se toque.
- **Pin de queries del trigger sin rama "sin evento" assertionada:** `TestStreakTrigger_Evaluate_QueriesConstantes` pinea el conteo (6) de la corrida **con** evento y compara 3 vs 12 días; la corrida sin evento (sin INSERT de unseen) solo se loguea (`t.Logf`), sin assert de conteo.
- **Gate del hook por service-success sin chequeo explícito de status:** los hooks de racha/logros corren tras **cualquier** PATCH exitoso del runner (finished, interrupted, interrupted idempotente) sin inspeccionar el status final de la fila; es inofensivo (recálculo idempotente + baseline) pero un gate más fino exigiría chequear status.
- **Idx redundante `idx_user_achievement_progress_user`:** `user_id` ya es prefijo de la PK compuesta `(user_id, achievement_id)`; el índice separado no aporta y podría eliminarse (requiere migración).
- **`gps_N` cuenta puntos de feedbacks borrados (decisión consciente):** `CountGPSFinishedSessions` cuenta por existencia de puntos en `workout_feedback_points`; un feedback soft-deleted deja sus puntos contando para el criterio.
- **`GetUserStamps` sin test de rama 500:** la suite del controller cubre 200/403/401 del GET de stamps pero no el pin FailingDB de la rama de error interno.
- **Sin assert negativo "record nil en period=week":** el scenario existe en el spec delta pero la suite del service no tiene el assert negativo explícito (solo cubre record presente en `all`).

## Cobertura de tests

> Registrado el 2026-09-22 con el change `cobertura-calendario-recientes` (2026-09-22, ronda de tests de calendario/controller/session/exercise/DAOs). Estado luego del change: analyzer **85.2% (7649/8976)**, meta puntual ≥84% superada, gate de `make coverage` subido a 85% (2026-09-23, `chore/coverage-threshold-85`).

### Etapa 2 de coverage: legacy → 90% (trabajo futuro / repuesto)

La meta de esta etapa cubrió calendario/recientes; el resto del repo (paquetes legacy de pagos/usuarios sin calendar) quedó para una **etapa 2** con objetivo 90% total, a usar como trabajo de repuesto cuando no haya gaps ni features nuevas para cubrir. Gaps principales conocidos:

- `payment_service` (~102 ramas), `customlogger` (~78), `mercadopagoclient` (~68, con `httptest` para los paths HTTP reales)
- `user_service` (~54), `tier_service`, `training_plan_service`, `join_request_service`, `workout_feedback_service`
- `team_dao.SearchPublic` (~5, ver parked más abajo)

### Riesgo de acoplamiento del helper `FailingDB` (declarado en design D1)

`cmd/api/testutils/failing_db.go` inyecta fallas discriminando por operación (`nthFail`), lo que acopla los tests al **orden de statements** que gorm ejecuta por query. Un refactor o upgrade de gorm que reordente/autoenvuelva statements rompe estos tests ruidosamente, incluso sin bug real — si pasa, re-ajustar los umbrales/posición de `Nth` de los tests, no "arreglar" el servicio. Ver `openspec/changes/cobertura-calendario-recientes/design.md` D1.

### Ramas parked legítimas (inalcanzables desde fuera, no forzar)

- `calendar_service` bulk con kind `cancelled`: la rama existe pero `BulkRequest` no expone `cancelled_reason`, inalcanzable vía API — se cubrirá junto a la feature real de bulk-cancel.
- Ramas defensivas de `calendar_service` (TOCTOU re-check de "día cerrado", JSON imposible del body): cubren condiciones de carrera/payloads que el handler previo ya filtra; forzarlas en test testearía el test.
- `team_dao.SearchPublic`: se implementa junto a la feature de búsqueda de equipos (deferida, ver arriba).

## Decisiones de "no tocar"

- **`utils.StringToInt64`/`Int64ToString`/`Contains`/`IsPositiveInteger`/`ParseInt64`** (`cmd/api/utils/`): cero call sites en todo el repo, confirmado por grep + `git log --diff-filter=A` (vienen del scaffold inicial). Decisión explícita del usuario: **no borrar**, se guardan para trabajo futuro de métricas/pagos que probablemente los necesite. No re-flaguearlos como dead code en una futura limpieza de coverage.

## Resueltos (change `optimizaciones-calendario-y-indices`, 2026-10-09)

Estos ítems se resolvieron antes de quedar registrados en esta lista — quedan acá como registro del veredicto. Detalle: `docs/CATALOGO_Y_CALENDARIO.md` §9.

- **GetRange N+1 del calendario** (y las demás vistas de colección): el builder batch `toCalendarDayResponses` resuelve las instancias embebidas en 3 queries por colección, sin una query por día. Resuelto en `077d395`.
- **`workout_feedback_points.feedback_id` sin índice**: índice agregado, junto a 6 más sobre `group_users`/`team_users`/`session_exercise_instances`. Resuelto en `fcb64c5`.
- **Team search 2N+1** (owners N + miembros N + resto): `Search` de equipos y `CalendarSummary` resuelven owners/miembros/nombres en queries por colección. Resuelto en `d147b1f`.

## Resueltos (change `latlng-optimizaciones-y-endurecimiento`, 2026-10-11)

N+1 de escritura/lectura del dominio calendario, migrados a batch con el número de round-trips **constante** (no crece con los días/invitaciones); respuestas y errores byte-idénticos. Detalle técnico con los conteos: `docs/CATALOGO_Y_CALENDARIO.md` §9.

- **N+1 de escritura en stamp/bulk:** migrados a `UpsertMany` (1 `INSERT` multi-fila `ON CONFLICT (group_id,date) DO UPDATE`) + instanciación batch (`session_instances`/`exercise_instances` multi-INSERT con `RETURNING`, links/exercicios de catálogo resueltos con `IN`). Stamp: 19 queries fijas + 1 guard de lectura por día (4 días = 19, 30 días = 45). Resuelto en `f6faebd`/`73f2c9d`.
- **N+1 de borrado de instancias y N reads del bulk-clear:** `deleteSupersededInstances` batch (semántica de §8.4 intacta — protege instancia si hay feedback de la sesión o de cualquier ejercicio; soft-deleted feedback no protege) + bulk-clear reescrito (1 read `IN` + guards en memoria + 1 delete + supersede batch), fijo en 11 queries. Resuelto en `239dbde`.
- **Loop de colisiones de AdministeredCalendar** (3 queries por fecha colisionante): clasificación in-memory sobre las filas de `FindForGroupsInRange` — 3 queries fijas para todo el rango. Resuelto en `8a17e57`.
- **Invitaciones 3N+1** (users resueltos de a 1..2 por invitación): `ListPendingInvitationsForUser` en 3 queries, listado del team en 4 queries (teams/users con `IN`). Resuelto en `8a17e57` (Task 8 completó el residuo: `can_receive_payments` del listado hoy sale en 1 query batch de `seller_connections`, ver `f4dd3e8`).

## Competencias de ruta — spec 4b (change `competencias-ruta-intake`, 2026-10-13)

Pendientes del intake/cierre de competencias de ruta. Mecánica en `docs/CATALOGO_Y_CALENDARIO.md` §8.14 (subsección "Competencias de ruta"); contratos en `docs/FRONTEND_IMPACTO_INSTANCIACION.md` §17.

### Intento wip/interrupted no se borra ni se reabre (RUTA v1)

El anti-doble intento original contaba runner sessions con `competition_id` **de cualquier status**: un intento `interrupted` consumía el único intento del atleta y **no hay endpoint de reset/borrado/re-apertura**. Desde el Gap 36 (`competencias-ruta-intentos-multiples`) el atleta puede reintentar mientras no agote `max_attempts` — pero el intento trunco igual **consume uno de los turnos** y sigue sin existir reset ni borrado de intentos. Rediseñar borrado/edición de intentos (p. ej. cuando el límite se agota con intentos fallidos) toca guards + spec — propio change futuro.

### Leaderboard en vivo mid-attempt

Durante el intento en curso (`wip`) no hay broadcast WS de nada (ni ranking ni traza GPS); los frames WS son solo post-finish (`update:competition_progress`) y post-close (`control:competition_finished`). Progreso intermedio on-demand vía `GET /competitions/{id}`. Escenario declarado en el spec delta y al front en §17.5. **Sigue planteado con intentos múltiples** (Gap 36): ni ranking ni traza por intento en vivo; el acceso WS de las instancias de intento 2+ (D3) no agrega frames.

### Microventana race del pre-check anti-doble

**RESUELTO como efecto del Gap 36** (`competencias-ruta-intentos-multiples`, 2026-10-22): el guard de límite vivió dentro de `CreateAttemptSession` con `SELECT ... FOR UPDATE` de la fila de competencia, que serializa los intentos concurrentes; el único `(session_instance_id, athlete_user_id)` de runner_session queda como respaldo físico del intento 1.

### Menores del Gap 36 (`competencias-ruta-intentos-multiples`, 2026-10-22)

- **`ExistsAttemptByCompetitionAndAthlete` sin caller en producción:** el guard "doble intento" que la usaba fue reemplazado por el COUNT dentro de `CreateAttemptSession`. La función quedó viva con tests y sin call sites de prod — candidata a borrarse en una limpieza futura o a reusarse si algún guard vuelve a necesitar el check puntual (decisión documentada: NO se borró en el change).
- **Empate exacto intra-atleta en `progress` sin desempate definido:** `bestAttemptByAthlete` (progreso del detalle) consume `ListAttemptSessions`, que **no trae ORDER BY** — dos intentos finished del mismo atleta con pct y duración idénticas pueden alternar cuál sale como "mejor" entre GETs (enums no determinista, inofensivo: el valor mostrado es igual). Los winners no sufren (empate exacto = todos ganan) y el GET de intentos sí es determinista (ORDER BY id ASC). Fix cosmético si alguna vez molesta: agregar ORDER BY al DAO.

### Default de `acceptance_radius_meters` solo sugerido por el front

Backend NO aplica default: el campo es requerido en [5, 500] metros (`400` "el radio de aceptación debe estar entre 5 y 500 metros"). El frontend debe **sugerir 50 m** en su UI de creación de competencia de ruta.

### Memoización del matching de ruta (ronda seguridad 2026-10-14)

Dos recomputos que un memo por request/bundle ahorraría, documentados como mejora futura (no afectan correncta DIR): (a) `Close` de route corre `routeAttempts`/matching **2×** y `ListAttemptSessions` 3× (para canales WS + winners + respuesta final) — un campo memo en el bundle del request lo reduciría 1 query + 1 matching completo; (b) el progreso route del `GET /competitions/{id}` recomputa todo el matching O(samples×points) por GET aunque la ruta es inmutable — los samples son cacheables por competencia indefinidamente (hoy ~2500 allocs por GET). Bound actual aceptable (≤500 vértices / ≤100 km / ≤5000 puntos por bulk).

### Sin rate-limit en los POST de competencias (ronda seguridad 2026-10-14)

**MaxBytesReader: resuelto** (change `optimizaciones-y-seguridad-api`, S9): el límite global de 10 MB (`MaxBodySize()` en `middleware.go`) cubre todos los routes incluidos los POST de competencias; el 413 de gin corta el parseo antes de las validaciones. **Rate-limit: sigue ausente** en esta ruta — el `IPRateLimiter` solo cubre las 6 rutas públicas de usuario (register/login/refresh/auth-user/forgot/reset; logout queda sin limiter); las rutas autenticadas (entre ellas `POST /teams/{id}/competitions`) no tienen cuota.

## Resueltos (change `competencias-ruta-intake`, 2026-10-13)

- **Guard de status de cancel** (ronda seguridad 2026-10-14): `DELETE cancel` tenía el DELETE sin condición de status en el SQL (race con approve podía borrar una competencia ya `active`). Ahora el DELETE exige `status = 'pending_approval'` en el WHERE y `RowsAffected==0` → `400` (patrón Approve/Reject/Finish), con test `TestCompetitionDAO_CancelVsApproveRace`. Resuelto en `5aed0e4`.
- **Exclusión del owner como participante** (ronda seguridad 2026-10-14): solo se bloqueaba cuando el caller era owner; un corredor creador podía incluir al owner en participants. Ahora el owner se excluye SIEMPRE (resolución `TeamOwnerID` + guard incondicional), con test del 400 "el entrenador no compite". Resuelto en `5aed0e4` (mezcla: commit `f71c713` cerró antes el I1 de competencia rota tagueada sobre instancia ajena, guard bidireccional con `FindByRouteSessionInstance`)

- **Validación de `route_geojson` en Create solo estructural** (deuda de `gamificacion-core`, entry eliminada de las Gamificación): Create ahora parsea el LineString y rechaza con `400` inválido / >500 vértices / >100 km Haversine (`routematch.ParseLineString` + `LineStringLength`) — el passthrough al `500` fail-safe en el matching ya no aplica en el alta (el matching sobre una competencia creada siempre tiene ruta confiable). Resuelto en `3e6d0c3`.

## Seguridad (change `optimizaciones-y-seguridad-api`, 2026-10-19)

Limitaciones conscientes y pendientes de la ronda de endurecimiento. Ownership de pagos y firma HMAC de webhooks quedaron implementados (mecánica en `docs/CATALOGO_Y_CALENDARIO.md` §8.15); lo que sigue quedó a propósito o deferido.

### Rate limiter en memoria, single-instance, sin cap duro

El `IPRateLimiter` acumula un bucket por IP en un `map` en memoria del proceso (con `sync.Mutex`): suficiente mientras Render corra **una sola instancia** — escalar a multi-instancia lo invalida (cada réplica contaría su propio bucket; necesitaría Redis o similar). La **poda** del map corre cada 256 requests y borra entradas inactivas hace más de 1 hora, **sin cap duro de entradas**: un ataque con IPs espúreas puede inflar el map entre podas (cada entrada es ~un puntero a `rate.Limiter`, acotado en la práctica por el TTL de 1 hora, pero no hay tope explícito).

### El access token sobrevive la revocación de sesiones (limitación estructural)

El access token es JWT stateless sin chequeo de DB: revocar refresh tokens (logout, cambio/reset de contraseña — Task 10) mata la **renovación**, pero un access token ya emitido sigue válido hasta expirar en los endpoints que solo miran el JWT. Es estructural al diseño stateless (no hay denylist); la ventana es la TTL del access token. Si algún día hiciera falta revocación inmediata, es su propio change (denylist por `auth_session_id` o tokens de corta vida).

### Replay de webhook dentro de la tolerancia no bloqueado

La validación HMAC del SDK de MP corre **sin `WithTolerance`**: la firma de una notificación vieja (capturada/reenviada) sigue verificando — el ts del header no se compara contra el reloj. El SDK lo soporta (`webhook.WithTolerance(d)` como option de `ValidateSignature`); configurar la ventana es un one-liner cuando se decida el valor (los webhooks son idempotentes, por eso el riesgo es bajo).

### `GET /groups` sin scope + `test-card-token` vivo — limpieza futura

Ambos se dejaron a propósito (constraint del change): `GET /api/v1/groups` sigue sin scope (devuelve todos los grupos a cualquier autenticado) y `POST /api/v1/payments/test-card-token` sigue abierto (sandbox). Limpieza que toca contratos — propio change cuando se defina.

### `dataID` del webhook de deauth usa `user_id`

El manifest firmado del deauth lleva `fmt.Sprintf("%d", req.UserID)` del payload — MP firma el id del **user** conectado, no hay `data.id` de pago ahí. Funciona, pero si MP agrega formato de notificación con `data.id`, el fallback a ese campo conviene a futuro (hoy el binding exige `user_id != 0`, payload sin él → 400).

### Posible degradación al bucket del LB si el peer de Render no fuera RFC1918

`trustedProxies` fija los peers de confianza a los rangos RFC1918 (`10/8`, `172.16/12`, `192.168/16`): `c.ClientIP()` camina el `X-Forwarded-For` desde la derecha solo si el peer directo es un IP privado (el LB interno de Render). Si Render cambiara a un peer de egress **público**, gin lo trataría como cliente final y todos los requests compartirían el bucket de ese IP — degradaría a rate limit global. No verificado contra producción: **chequear con un smoke post-deploy** (un login sano desde el front no debe dar 429).
