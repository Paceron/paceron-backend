# Proposal: competencias-ruta-intentos-multiples (Gap 36)

## Why

Gap 36 del frontend: reintentos en competencias de ruta. El change
`competencias-ruta-intake` cerró el ciclo de ruta (instancia sintética compartida,
intake `POST /competitions/{id}/attempts`, matching Haversine server-side) con UN
único intento por atleta: hoy `CreateAttempt` rechaza con `400` al segundo pedido
("ya tenés un intento registrado en esta competencia"). La pantalla de detalle de
competencia de ruta del frontend (Gap 36) ya está diseñada sobre "intentar de
nuevo": correr la ruta otra vez para mejorar el % de match. Hoy eso es imposible
por contrato del backend, así que el flujo de la pantalla queda cojo: el botón de
reintento no tiene endpoint que lo respalde.

El problema tiene dos partes: (a) no existe límite configurable de intentos ni
endpoint para informarlos (el front no puede ni renderizar "quedan N intentos"),
y (b) el modelo "instancia compartida" se rompe con los reintentos: si el segundo
intento reusara la instancia sintética, chocaría de frente con el unique
`(session_instance_id, athlete_user_id)` de `runner_session` (que es el que hoy
hace de único intento por atleta) y corrompería el matching por par (instancia,
atleta) que `routeAttempts`/`computeRouteProgress`/`computeWinners` asumen.
Además, la tercera rama de `HasInstanceAccess` solo autoriza la instancia
compartida: con instancias nuevas de intento, el gateway WS no dejaría al front
suscribirse al canal `session:{id}` de esos intentos.

## What

Un change sobre `feature/competencias-ruta-intentos-multiples`, decisiones
cerradas con el frontend (2026-10-22, todas aprobadas — detalle en `design.md`
D1-D8). Resumen:

1. **D1 — `max_attempts` route-only.** Columna nullable nueva en `competition`
   (AutoMigrate, sin backfill: las competencias viejas = NULL = ilimitadas).
   En el create route: opcional; si viene, entero positivo mínimo 1 (400 si 0,
   negativo o no-entero; SIN tope superior). Si no viene → NULL = intentos
   ilimitados. En el create metric: 400 si viene. Inmutable post-create.
2. **D2 — CreateAttempt rework.** El guard pasa de "existe intento → 400" a
   "COUNT >= max_attempts → 400" (mensaje "llegaste al límite de intentos de esta
   competencia"). Intento 1 reusa la instancia compartida (comportamiento
   actual). Intento 2+ crea una `session_instance` NUEVA en la MISMA
   transacción del insert de `runner_session` (rollback total ante cualquier
   fallo), con el ejercicio sintético igual ("Corrida", kind
   `velocidad_distancia`, link main/repeat 1/rest 0) y nombre
   `"Competencia ruta N (intento M)"`, N = id de competencia, M = número de
   intento (count + 1). Race-safety: lock de la fila de competencia + re-count
   dentro del tx (el unique de DB ya NO arbitra: las instancias son distintas).
3. **D3 — WS: tercera rama de `HasInstanceAccess` extendida.** Hoy solo
   autoriza la instancia compartida (`route_session_instance_id = ?`). Se
   extiende con un OR: caller es participante de una competencia `type=route`
   que tiene un `runner_session` tagueado con esa instancia (EXISTS
   runner_session × competition × participants, SIN filtro de status — para
   recibir `control:competition_finished` hay que poder suscribirse con el
   intento en cualquier estado). Mirror idéntico en
   `HasInstanceAccessBatch` (SQL batch).
4. **D4 — Endpoint `GET /competitions/{id}/attempts`.** Guard igual que el
   detalle (404 antes de 403; miembro de grupo del team u owner).
   `athlete_user_id` default = caller; cualquier miembro puede ver los intentos
   de otro. Atleta pedido no-participante → lista vacía `[]` (no 400).
   Envoltura `{data: [...]}` simple, SIN `has_unseen` (el unseen de competencias
   es por team y ya lo cubre el listado). Ítems: `{id, session_instance_id,
   status, started_at, ended_at, distance_meters, compliance_ratio,
   duration_ms}` — `id` por convención runner-session;
   `started_at`/`ended_at` mapean de `runner_session.start_date`/`end_date`;
   `distance_meters` = SUM de los `distance_meters` del feedback de esas series
   (null si no hay feedback); `compliance_ratio` = matching Haversine on-demand
   (mismo cálculo del detalle; null salvo intento finished, 0 si finished sin
   puntos GPS); `duration_ms` = end-start server-side (null si wip).
5. **D5 — `progress` del detalle cambia de shape para route** (pedido del front):
   `{user_id, compliance_ratio, duration_ms, session_instance_id}` = MEJOR
   intento por atleta. Metric SE MANTIENE `{user_id, value}`. Corrección de doc:
   `computeRouteProgress` hoy devuelve `value = pct` (0-100) en el código pese a
   que la doc decía `value: null`.
6. **D6 — computeWinners route rework.** Primero se reduce al mejor intento POR
   atleta (max pct, empate → menor duración), recién después se compara entre
   atletas (empate → todos ganan). Corrige el edge latente del código actual:
   con intentos múltiples, un mismo atleta podría aparecer duplicado en
   `winner_user_ids`.
7. **D7 — `CompetitionResponse` suma 3 campos server-side:** `max_attempts`
   (nullable, null para metric), `attempts_used` (entero de intentos del caller;
   0 si es participante sin intentos; null si no es route o el caller no es
   participante) y `can_attempt` (bool SIEMPRE presente: true solo si
   type=route && status=active && caller participante && (max_attempts NULL ||
   attempts_used < max_attempts); false en los demás casos). Batch sin N+1 en
   el listado.
8. **D8 — WS frames sin cambios; intento sigue siendo 1 `runner_session`.**
   `update:competition_progress` sale por finish de cualquier sesión tagueada
   (ahora de más instancias); `control:competition_finished` al cerrar llega a
   los canales de TODAS las instancias de intentos (dedupe existente intacto).
   El feedback/gate del intento reusa el flujo existente sin cambios; cancel/
   reject siguen siendo solo-pending (sin intentos) y la limpieza de la
   instancia compartida queda intacta; el close con el límite cumplido sigue
   siendo manual (creador/owner). Sin migración destructiva ni FK nueva:
   los intentos 2+ se alcanzan por `runner_session.competition_id`, no por
   columna de competition.

## Alcance (un solo change, sin stages)

Rama única con las 5 tasks encadenadas (T1 create/validación → T2 intake rework →
T3 WS/endpoint → T4 response/progress/winners → T5 docs). No hay deploy escalonado
ni rollout por etapas: todo se activa junto contra el frontend (Gap 36 consume las
piezas en conjunto), y el contrato es aditivo salvo los dos cambios de
comportamiento acotados que el front ya pidió explícitamente (reintento permitido,
progress shape route). Un único corte seguro.

## Capabilities

Modificadas: `gamification-competitions`. Delta declarado TODO bajo
`## ADDED Requirements` — la capability principal nunca fue archivada y un delta
`MODIFIED` contra ella sería rechazado por el archive (convención del repo,
mismo patrón que `competencias-ruta-intake` y `asignacion-por-instanciacion`).
Requirements declaran completos, con nota de qué reemplazan cuando hay versión
previa.

Sin endpoints existentes rotos: `POST /competitions/{id}/attempts` cambia SOLO su
guard anti-doble (reemplazado por el límite D2 — pedido explícito del front) y el
`progress` del detalle de route cambia de shape (también pedido explícito); todo
lo demás es aditivo.

## No-goals

- Borrar/editar intentos (v1 no borra ni edita: una runner_session wip/
  interrupted queda como está — deuda futura a documentar).
- Tope superior configurable para `max_attempts` (sin tope por decisión) ni
  default backend-mandado distinto de "sin límite".
- Competencias multi-atleta o por equipos (el shape de competidores queda como
  está).
- Deshabilitar `can_attempt` por schedule/ventana temporal (route no usa
  ventana; el límite es el único gate junto a status/participación).
- Cambios de protocolo WS (frames y canales intactos; solo extiende la
  autorización de suscripción).
- Borrado/backfill de datos: sin migración destructiva, sin recomputar datasets
  viejos, `route_session_instance_id` sigue apuntando a la compartida.
- Intake/cierre/moderación: sin cambios (el único cambio de intake es el guard
  D2; close y cancel quedan byte-idénticos salvo lo que ya comparten los
  intentos).
