# Diseño — seed de perfil full

Contexto de código relevante: el script vive en `cmd/api/scripts/seed_dashboard/`; se
conecta con `postgresdb.ConfigDB` (que corre AutoMigrate + SQL post-migración, así que
funciona sobre una base vacía), usa servicios reales para catálogo/stamp/feedback y
DAOs para runner_session; las guardas son `SEEDING_ENABLED=true` +
`config.AssertSeedingStage()` (exige `--stage=local`).

## D1 — Perfil y despacho

- Flag nuevo `--profile dashboard|full` (default `dashboard`).
- `dashboard` no cambia: mismos flags, mismos ids default (29/39), mismo verify, mismo
  cleanup, mismos tests.
- `full` entra por una rama propia (`runFullSeed`) en `main.go`, con su propio seeder
  de scope. El esqueleto del `seeder` (DB, report, dry-run) se reutiliza.
- **Scope full propio**: ids explícitos en rango alto (equipos 1001.., grupos 2001..,
  usuarios seed-full con emails `@seed-full.local`) y marker `[seed_full]` en las
  descripciones que el script controla. Guarda de seguridad: si un id de scope full
  ya existe con datos que no llevan el marker, abortar con error en vez de mezclar.
- El perfil full asume base local recién reseteada (`fullseed-reset`), pero es
  idempotente: segunda corrida sobre el mismo universo no duplica.
- El estado de corrida vive en un `fullSeeder` creado una vez; las fases T2–T8 se
  implementan como métodos del mismo tipo y se conectan en orden en `profile_full.go`.
  T1 define ese contrato y cada fase preserva los IDs que consumen las siguientes.

## D2 — Un solo run que contiene el dashboard

- `full` reutiliza el seeder dashboard existente bajo los IDs reservados del Team A y
  su grupo principal: 2 semanas completas previas (martes/jueves), 7 arquetipos, mismo
  plan de defectos y mismo verify de 6 contadores. Luego agrega el universo extendido.
- Los 7 arquetipos viven en el grupo dashboard (grupo principal del Team A). Los 5
  corredores extra viven en un grupo secundario del Team A y en el Team B, fuera del
  scope del dashboard, para no alterar sus contadores.
- Total: 12 corredores (7 + 5). El verify dashboard sigue corriendo y debe pasar.
- Para conservar el perfil existente, ese segmento reutilizado mantiene su marker
  `[seed_dashboard]`; los registros exclusivos del perfil full usan `[seed_full]`.
  `fullseed-cleanup` reconoce ambos markers solo dentro del scope full reservado; los
  comandos de dashboard siguen limitados a sus IDs default 29/39.

## D3 — Usuarios, roles, tiers y suscripciones

- **Solo en perfil full**: password `Paceron.123` hasheada con bcrypt al momento del
  seed (pasa `ValidatePassword`: ≥8, mayúscula, minúscula, dígito, sin `<>"'&;%`). El
  perfil dashboard conserva su `"seed"` plano sin cambios.
- Protagonistas: trainer 1 (owner de A y F), trainer 2 (owner de B), corredores R1-R3
  (Team A, login y flujos normales), R4-R5 (Team A grupo secundario), R6-R8 (Team B),
  R9-R12 (repartidos para casos borde).
- Usuarios de borde: uno `blocked`, uno `inactive`, uno sin rol asignado, uno con
  `allow_team_invitations=false`.
- **Cadena de permisos completa**: roles `entrenador` y `corredor`; 3 tiers por rol
  (`base` hierarchy=1 gratuito, `medium`, `premium` pago); permisos y
  `tier_permissions` de modo que cada tier tenga ≥1 permiso. Sin esto
  `GET /auth/permissions` falla con "datos faltantes".
- `user_roles`: trainer con rol entrenador (tier base) + rol corredor; corredores con
  rol corredor. `status='active'`, `deleted_at NULL`.
- Suscripciones (`user_role_tier_subscriptions`) respetando los índices únicos
  parciales (máx 1 `active` + 1 `first_payment_pending` por user+role):
  R1 premium `active` con cuotas #1/#2 pagas y #3 pendiente (due hoy); R2 deuda
  (pendiente pasada de `blocked_date`); R3 `first_payment_pending` de cambio de tier.
- `platform_settings.marketplace_fee_percent` como jsonb numérico.

## D4 — Equipos, grupos, invitaciones y solicitudes

Matriz de equipos (todos con ubicación cargada; la búsqueda compara igualdad exacta):

| Equipo | Owner | Config | Casos que habilita |
|---|---|---|---|
| A normal | trainer 1 | fee 0, visible, público, seller conectado | Todos los flujos normales; grupo principal (dashboard) + secundario |
| B pago | trainer 2 | fee > 0, visible, público, sin seller connection | Deuda/baja bloqueada; 409 SELLER_NOT_CONNECTED; búsqueda de equipos ajenos |
| C lleno | trainer 1 | max_members alcanzado | AddUser / join request / aceptar invitación fallan por cupo |
| D oculto | trainer 2 | visible=false | No aparece en búsqueda |
| E cerrado | trainer 2 | is_public=false, visible=true | Aparece en búsqueda pero join request bloqueado; invitación sí funciona |
| F colisión | trainer 1 | visible; presencial solapado con A el mismo día | Colisión cross-team en administered-calendar y 409 al asignar |

- `team_users`: owner `entrenador` en cada equipo; corredores `corredor` con
  `subscription_status` coherente (gratis `active`; pago `first_payment_pending` o
  `active` con cuotas). El owner **cuenta** en `max_members`; C se llena contándolo.
- `group_users`: A tiene membresías activas (date_start -60d), una **vencida**
  (date_end -7d) y una **futura** (date_start +7d). R1 participa además en B.
- **Invitaciones** (Team A/B): una pendiente vigente (expires_at futuro), una expirada
  (expires_at pasado, invisible en listados), una aceptada y una rechazada con
  `responded_at`; una al usuario con invitaciones desactivadas para el camino de error.
- **Join requests**: pendiente a E (falla por cerrado si se intenta), pendiente a A
  (owner acepta/rechaza), una a C (falla por cupo) y una rechazada. Cancelar = DELETE
  físico: no existe estado `cancelled`.

## D5 — Catálogo y calendario

- Catálogo ampliado: al menos un ejercicio de cada kind vigente, sesiones con
  warmup/main/cooldown completos (la sesión exige ≥1 link por rol), y planes de
  varias duraciones (7, 14 y 21 días) con `sequence_no` 1..N sin huecos.
- Pasado (-21..-1): calendario sembrado con stamp (`SEEDING_ENABLED=true` relaja el
  guard de fechas pasadas) y feedback/asistencias.
- **HOY**: se inserta por DAO/SQL directo, sin pasar por el guard de día cerrado:
  - Grupo principal de A: training **presencial** con `time_from=now-30m`,
    `time_to=now+2h`, `presencial_opened_at=now-20m`, `closed_at=NULL` → usable al
    instante por el corredor.
  - Grupo secundario de A: presencial **no abierto** (`opened_at=NULL`) → demo 409
    `session_not_opened`.
  - Un grupo de F: presencial **cerrado** (`closed_at` seteado) → demo 409
    `session_closed`.
  - Equipo B (`group_b`): un training **async** de hoy. Se ubica en B porque
    `group_calendar_days` permite una fila por grupo/fecha y los dos grupos de A ya
    están ocupados por los dos estados presenciales requeridos; R1/R2 tienen acceso a B.
- Futuro (+1..+14): training presencial y async, un `other`, días `rest` y un
  `cancelled` con motivo. Via stamp (futuro está permitido).
- Horarios presenciales guardados con la convención UTC existente (fecha 0000-01-01 +
  `.UTC()`); la ventana se calcula con `gametime` (TZ Córdoba).
- Colisión A vs F: presenciales solapados el mismo día a la misma hora → visible en
  `administered-calendar` del trainer 1 y 409 al intentar asignar. Warning same-team:
  dos grupos de A con presenciales solapados.

## D6 — Actividad: sesiones, feedback y asistencias

- `runner_session`: pasadas `finished` con `end_date`, hoy `wip` (sesión en curso),
  una `interrupted` histórica; instancias por día.
- Feedback: series con todos los `completion_status` (`completed`, `skipped`,
  `unregistered`), métricas coherentes, `report_source` no vacío, `session_date` solo
  fecha; un feedback del entrenador como owner; un feedback soft-deleted que no debe
  aparecer. Para 2 series se siembran puntos GPS suficientes para recomputar
  `pace_sec_per_km`/`peak_speed_kph` (o se setean directo).
- Asistencias: días -14/-7 y HOY; mezcla `qr` (autorregistro) y `manual`
  (entrenador), con miembros sin confirmar para que el roster muestre
  `not_confirmed`. Respetar UNIQUE (team, instancia, user) y que el roster evalúa
  membresía a la fecha de la sesión.

## D7 — Gamificación

- Rachas: se derivan del historial (no hay tabla que sembrar); se garantiza R1 con 10
  días consecutivos (hito → unseen), R2 con racha rota (reset) y R3 con días previos a
  la membresía (skipped). `gamification_streak_state` se siembra opcionalmente para que
  el próximo trigger no dispare hitos ya avisados.
- Logros: `user_achievement_progress` con `unlocked_at` para un corredor con varios
  desbloqueados y otro vacío (empty state); ids del catálogo de
  `constants/achievements.go` (21 filas); `icon_key` = id.
- Estampas: solo `type='competition'` de competencias terminadas, `source_name
  "Competencia N"`. T6 deja el endpoint en su estado vacío; T7 siembra stamps junto
  con las competencias terminadas y sus winners.
- Unseen: T6 siembra filas de `streaks`/`achievements`; T7 agrega `competitions`
  junto con el cierre de las competencias. En el perfil completo un protagonista
  queda con novedades y otro con esas secciones ya vistas (demo de `POST seen`).
- Leaderboard: datos en Team A para las 6 métricas, con record `period=all`; el owner
  no puntúa.

## D8 — Competencias

Todas en Team A, participantes = corredores (el trainer nunca compite):

- `pending_approval` metric creada por corredor (demo approve/reject).
- `active` metric con progreso de 2 corredores (feedback `team_id` en ventana).
- `active` route con `max_attempts=3`: R1 con 1 intento finished al 100% de compliance;
  R2 con 1 finished parcial + 1 wip (→ `attempts_used=2`, `can_attempt=true`).
- `finished` metric con **empate exacto** → 2 ganadores, 2 estampas, unseen a todos.
- `finished` route con ganador por duración; `finished` route sin intentos (sin
  ganadores); `rejected` con `rejection_reason`; una route con `max_attempts` agotado
  (`can_attempt=false`).
- Ruta: GeoJSON LineString con coordenadas `[lng,lat]`, ≤500 vértices y ≤100 km, radio
  de aceptación 50 m. Cada competencia/intento usa su instancia sintética
  (`route_session_instance_id`, instancias "Competencia ruta N (intento M)" para
  intentos 2+). Los puntos GPS van sobre el feedback del intento, distribuidos sobre la
  ruta (±10 m) para que el matching dé 100% o parcial según el caso.
- Estado `cancelled` no existe: cancelar es DELETE físico, no se siembra.

## D9 — Pagos, MP connect, mensajería y push

- Suscripciones/cuotas: cuota #1 sin `due_date`/`blocked_date`; cuotas siguientes con
  `due_date/blocked_date` coherentes con `start_date`; CHECK de arco exclusivo
  (`subscription_id` XOR `team_id`); contadores `paid_installments` coherentes.
- Deuda: R2 con cuota pendiente pasada de `blocked_date`; como miembro de un equipo con
  fee, la baja del equipo queda bloqueada (regla D4 verificada en
  `team_user_service.go`).
- Payments: `payment_id` no vacío (el historial ignora pagos sin id MP),
  `external_reference` único, `concept` válido, `seller_user_id` + `marketplace_fee`
  para splits, `raw_response` con `net_received_amount` para el neto del entrenador;
  un pago rechazado/refundido para los chips del historial.
- `seller_connections`: trainer 1 con `client_id == MP_OAUTH_CLIENT_ID`, `public_key`
  no vacía, status `authorized`, tokens cifrados AES-GCM con `TOKEN_ENCRYPTION_KEY`
  (se usa el paquete `crypto`); trainer 2 sin conexión.
- Mensajería: mensajes en la instancia de la sesión de HOY y en una pasada; tipos
  `info/aviso/alerta`, modos `all/direct/multiple`, un reply; recipients solo en
  direct/multiple (all = sin filas), `sender_role` correcto.
- Push tokens: uno por protagonista (android/web), tokens `ExponentPushToken[...]`
  sintéticos; son best-effort (los reales los registra la app al loguearse).

## D10 — Verificación

- `fullseed-verify` (y `--profile full --verify`): además del verify dashboard,
  comprueba coherencia del universo full:
  - login de los 4 protagonistas (bcrypt) y rechazo de blocked/inactive.
  - calendario de HOY devuelve el presencial abierto de `group_a_main` y el async de
    `group_b`.
  - next-session/next-presencial-session devuelven banners.
  - streaks/logros/estampas/unseen responden con datos.
  - leaderboard de las 6 métricas devuelve filas; competencias por estado existen con
    sus winners/attempts.
  - historial de pagos del corredor y cobros del entrenador devuelven filas.
- Si cualquier chequeo falla, el comando sale con error (no deja un universo
  incoherente en silencio).

## D11 — Cleanup

- `fullseed-cleanup`: borra en orden inverso de FK todo lo sembrado por el perfil full,
  acotado por scope ids + marker + ventana: competencias (winners, participants,
  instancias sintéticas), mensajes, pagos/cuotas/suscripciones de tier, asistencias,
  feedback y points, runner_session, calendario/instancias, planes/sesiones/ejercicios,
  invitaciones/join requests, memberships, equipos/grupos y usuarios `@seed-full.local`.
- No toca datos del perfil dashboard ni datos reales; pide `--yes` explícito, igual que
  el cleanup actual.
- Primero borra los datos full-only que puedan referenciar instancias del calendario;
  después limpia el segmento dashboard-compatible reutilizando sus WHEREs existentes,
  acotado al Team A, grupo principal y ventana full reservados. Elimina también las
  memberships y usuarios seed-full; nunca limpia `group_users`/usuarios fuera de ese
  scope.

## D12 — Makefile y documentación

Familia nueva, sin tocar la existente:

```
fullseed-seed      # go run ./cmd/api/scripts/seed_dashboard --stage=local --profile=full --seed --bootstrap-scope
fullseed-reset     # local-reset + fullseed-seed
fullseed-cleanup   # --cleanup --yes
fullseed-verify    # --verify
```

- `seed-dashboard` y `seed-dashboard-cleanup` quedan intactos.
- RUNBOOK: sección "perfil full" con requisitos de `.env.local`
  (`SEEDING_ENABLED=true`, `TOKEN_ENCRYPTION_KEY`, `MP_OAUTH_CLIENT_ID`), comandos,
  usuarios/password, mapa de equipos y casos, y advertencia de base recién reseteada.
- `docs/ENTORNO_LOCAL.md`: reemplazar la nota "no existe todavía" por el perfil full.

## D13 — Determinismo y medio ambiente

- Misma semilla fija para el generador; el perfil full usa una semilla propia para no
  acoplar el plan de defectos del dashboard.
- Fechas siempre relativas a `NOW` en TZ Córdoba; nada hardcodeado a un día calendario.
- Guardas existentes sin cambios: `SEEDING_ENABLED=true` + `--stage=local` obligatorios.
- Sin red: no se llama a MP, Expo ni storage; todo sintético.

## D14 — Contrato interno de fases

- `profile_full.go` es el único orquestador del perfil full. `fullSeeder` mantiene la
  conexión, opciones, scope, un instante de referencia y los IDs estables que pasan
  entre fases.
- Las fases usan estas firmas y se conectan en orden: `seedIdentity() error`,
  `seedTeams() error`, `seedCatalogCalendar() error`, `seedActivity() error`,
  `seedGamification() error`, `seedCompetitions() error` y
  `seedPaymentsAndMessages() error`. T9 agrega `verifyFull() error` y
  `cleanupFull() error`.
- Cada task de fase modifica `profile_full.go` solo para añadir su implementación al
  estado compartido y conectarla al pipeline; se mantiene el orden de dependencias.
  Las pruebas de fase pueden invocar el método directamente contra una DB descartable.

## Riesgos

1. **Deriva con el código**: si cambia un flujo real, el seed puede quedar
   incoherente. Mitigación: D10 verifica por los reads reales y falla ruidosamente.
2. **Choque de ids/scope**: mitigado por rango alto + marker + guard de aborto.
3. **Contadores del dashboard**: mitigado manteniendo los 7 arquetipos y su defect plan
   en el grupo dashboard y poniendo los extras fuera del scope.
4. **Cifrado de tokens**: si `TOKEN_ENCRYPTION_KEY` falta o cambia, el seller
   connection queda inutilizable; el RUNBOOK lo exige y D10 lo detecta.
