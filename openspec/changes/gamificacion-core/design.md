# Design — gamificacion-core

Decisiones cerradas con el frontend (2026-10-13); doc fuente del dominio: `BACKEND_API_GAPS.md`
Gaps 30-34. Un solo change en la rama `feature/gamificacion-core` con 4 etapas (E1-E4) que se
implementan en orden y cada una deja la rama utilizable.

Constraint transversal: **0 cambios de API fuera de lo nuevo** — los endpoints existentes quedan
byte-idénticos; la única modificación es agregar un campo opcional al body de
`POST /session-instances/:id/runner`. Performance: sin N+1 en los endpoints nuevos (batch con
`IN`). Seguridad: guards self-only o member/owner en todos los endpoints nuevos, y errores
internos devuelven 500 genéricos sin detalles (pins no-leak con FailingDB). Coverage gate 85
(meta 90). Comentarios mínimos. Staging: solo rutas explícitas de cada tarea (nunca
`git add -A`).

## D1. Timezone fija

TZ de gamificación: `America/Argentina/Cordoba` (UTC-3), fija. Todos los cálculos de gamificación
(fecha date-only, "ayer", "hoy", ventanas de leaderboard y umbrales horarios) usan esa TZ. Se
resuelve una vez en el init de un helper de paquete pequeño con una variable global
sobreescribible en tests — los casos deterministas fijan el "hoy" con `time.Date` y no dependen
de la hora real del runner.

## D2. GET /users/{id}/streaks

- Guard self-only (mismo criterio que work-feedback-history). Ventana de cálculo: desde
  `hoy - (days - 1)` hasta AYER inclusive, en TZ Cordoba. Hoy NUNCA corta (decisión del
  frontend). Query `days` opcional, default 42, tope 365; fuera de rango (`< 1` o `> 365`)
  responde 400.
- Queries batch, sin N+1 — 3 queries totales: (a) memberships activos del usuario, (b)
  `group_calendar_days` con `kind='training'` en el rango (`group_id IN`), (c) runner sessions
  `finished` en el rango por `session_instance_id IN`. Mapeo por día en Go. Los tests pinan el
  conteo de queries (3) para que el N+1 no se cuele.
- Cada día trae DOS estados, cada uno `completed|failed|skipped|pending`:
  - strict: día con training asignado y TODAS sus runner sessions `finished` (los salteados no
    importan) → completed; alguna training sin finished (interrupted, wip o nunca iniciada) →
    failed; rest/other/cancelled o día sin asignación → skipped.
  - flexible: al menos una finished → completed; ninguna finished con training asignado →
    failed; rest/other/cancelled o sin training → skipped.
- El día de hoy (cuando el rango lo incluye) va con estado `pending`: en curso, no corta ni
  rompe rachas.
- Racha A (estricta): corre con días completed, corta con cualquier failed; skipped NO corta.
  Racha B (flexible): igual pero corre con completed y corta con failed. En ambas, los días
  del histórico cuentan de corrido hacia atrás desde ayer; hoy nunca interrumpe una racha ni
  la rompe (mientras se siga la racha, el día de hoy agrega recién cuando pase a completed).
- `current` y `best` se computan sobre TODO el histórico hasta ayer: la ventana `days` SOLO
  recorta el array de días devuelto; best NO es local a la ventana.
- Respuesta: `{strict: {current, best}, flexible: {current, best},
  days: [{date, strict_status, flexible_status}], has_unseen}`.

## D3. Novedades (seen)

- Tabla `gamification_unseen`: id, user_id, section varchar(32) (streaks/achievements/
  competitions), team_id nullable (solo competitions), created_at. Index `(user_id, section)`.
  Guardamos el hecho "hay algo nuevo que ver"; el punto rojo del front es derivable de la
  existencia de filas.
- Tabla `gamification_streak_state`: user_id PK, strict_current, flexible_current, updated_at.
  SOLO alimenta los triggers de notificación; el GET de rachas es siempre exacto (recalcula
  fresco) y no la usa.
- `POST /users/{id}/gamification/seen` — body `{section, team_id?}`. Self-only. Borra filas de
  unseen del usuario para esa sección (team_id filtra solo competitions; sin team_id limpia
  todas). Cero filas = éxito no-op.
- `has_unseen` embebido como bool en: GET /users/{id}/streaks (E1), GET /users/{id}/achievements
  (E2), GET /teams/{id}/competitions (E4, por team_id).
- Triggers de racha: hook post-write del PATCH de runner session hacia finished/interrupted
  (ni el create wip ni los cambios del calendario disparan). El hook compara current antes y
  después (recalculado fresco de ambos lados con la misma lógica de D2, y persistido en
  `gamification_streak_state`): si una racha pasa de > 0 a 0 → inserta unseen de section
  streaks. Si cruza un hito de 10/20/50/100 hacia arriba → unseen de section streaks.
- Deriva del estado de trigger aceptada y documentada como riesgo (D14): en el peor caso una
  notificación roja llega tarde o llega doble; el GET exacto nunca se ve afectado.
- Sin push en E1 (solo punto rojo).

## D4

Reservado: la numeración saltea D4 (el id viene del borrador de diseño; se conserva para no
renumerar referencias).

## D5. Logros (E2) — catálogo + evaluación

- Catálogo en constants Go versionado (NO tabla): cada fila es un logro `once` de un solo
  threshold; la familia es metadata de agrupación (la escalera multi-threshold Chispa→Leyenda y
  los counters son regla FUTURA, fuera de v1). Campos por fila: `{id, family, name, description,
  icon_key, threshold, unit}`.
- La tabla definitiva del frontend define 15 filas (6 consistency + 4 volume + 5 sessions); a
  eso se agregan los bloques de familia restantes cerrados en consenso (gps 3 + early 1 +
  team 2). Lista COMPLETA de catálogo (21 filas):
  - consistency (6): first_session(1), streak_3(3), streak_7(7), streak_14(14), streak_30(30),
    streak_60(60).
  - volume (4): distance_10k(10 km), distance_50k(50 km), distance_100k(100 km),
    distance_42k_single(42.2 km acumulados de las series de UNA sesión).
  - sessions (5): sessions_5, sessions_25, sessions_50, sessions_100, sessions_200.
  - gps (3): gps_first(1), gps_10(10), gps_50(50).
  - early_bird (1): early_5(5) — sesiones empezadas antes de las 08:00 America/Argentina/Cordoba.
  - team (2): team_first(1), team_10(10) — criterio: días distintos con asistencia confirmada
    registrada (criterio A pendiente de plasmación definitiva en el front; este change usa A).
- Extras FUERA (early_10, distance_250k/500k, attendance_20 y cualquier métrica semanal):
  recorte de alcance del usuario, no de implementación.
- Criterios de evaluación:
  - first_session y sessions_N: COUNT histórico de runner sessions finished.
  - distance_10k/50k/100k: SUM distance_meters de feedbacks.
  - distance_42k_single: MAX(SUM distance_meters por session_instance_id) ≥ 42.2 km
    (una query GROUP BY).
  - streak_N: current O best de racha A (estricta, D2) ≥ N — reusa el cálculo de D2.
  - gps_N: COUNT de runner sessions finished con puntos GPS subidos.
  - early_N: COUNT de runner sessions finished con started_at antes de 08:00 TZ Cordoba.
  - team_N: COUNT de días distintos con alguna asistencia confirmada registrada.
- Motor: hooks post-commit en controllers, nil-safe, asíncronos. Cada trigger re-evalúa los
  logros de su mapa. Idempotente por unique `(user_id, achievement_id)` en
  `user_achievement_progress`.
- Tablas:
  - `user_achievement_progress`: user_id, achievement_id varchar(64), unlocked_at, created_at.
    PK compuesta `(user_id, achievement_id)`; index user_id.
  - `gamification_stamp`: id, user_id, type varchar(32) ('competition' en v1), source_id,
    source_name, granted_at, count, created_at. Index user_id. Se crea VACÍA: solo recibe
    emisiones en E4.
- Endpoints: `GET /achievements/catalog` (autenticado, sin ownership) y
  `GET /users/{id}/achievements` (self-only; shape `{achievement_id,
  current_level?(null en v1), count?(null en v1), unlocked_at, last_unlocked_at}`),
  con `has_unseen` de la sección achievements embebido.
- Push al desbloquear: `expopushclient` best-effort con `data.route "/achievements"`
  (placeholder). Fail silencioso con log.

## D6. pace y peak_speed denormalizados (E3)

Columnas nuevas nullable en `workout_feedback`: `pace_sec_per_km` (float, menor mejor) y
`peak_speed_kph` (float). Al subir points de un feedback (points count > 0), recompute
idempotente:

- pace = duration_ms / distance_meters (s/km).
- peak = máximo de velocidades instantáneas entre segmentos de points consecutivos.

NO se exponen en el DTO de feedback: la persistencia es una decisión interna que solo alimenta
el leaderboard (el front no debe leerlas).

## D7. GET /teams/{id}/leaderboard (E3)

- Authorization del caller: miembro de algún grupo del team O owner del team (403 otro caso).
- Query params: `metric` (distance|duration|pace|peak_speed|sessions_count|attendance_count),
  `period` (week|month|year|custom|all), `from`/`to` YYYY-MM-DD. Custom exige ambos y
  `from <= to` (400 si falta uno o violan el orden). Base "hoy" en TZ Cordoba (D1).
- Métricas:
  - distance = SUM distance_meters.
  - duration = SUM duration_ms (reloj total).
  - pace = SUM duration / SUM distance del período (razón agregada, menor mejor — NO promedio
    de ritmos).
  - peak_speed = MAX peak_speed_kph.
  - sessions_count = COUNT runner sessions finished en el período.
  - attendance_count = COUNT asistencias confirmadas en el período.
- Solo corredores puntúan: el owner/entrenador arma el board pero NO tiene fila propia.
- Aggregation sin N+1: 1 query GROUP BY user_id + batch de names/avatar. Sin N+1 en nada.
- Desempate: a igual valor, gana quien usó MENOS entrenamientos que contribuyeron al valor
  (COUNT de actividades con métrica > 0 en el período). Específicos: sessions_count → a igual
  valor empieza antes (start_date de la última); attendance_count → quien registró primero la
  última asistencia que le alcanzó. A igual valor los ranks COMPARTEN posición; la siguiente
  posición saltea N.
- Shape: `{rows: [{user_id, name, avatar_url, value, rank}] (top 5), viewer_rank: {user_id,
  value, rank} | null, record?: {user_id, name, value, achieved_at} — solo period=all}`.
  achieved_at = session_date de la fila que alcanzó el max; a empate, la más reciente.

## D8. Competencias (E4) — modelo

- Tabla `competition`: id, team_id, group_id nullable, type varchar ('metric'|'route'),
  metric nullable, start_at, end_at nullable, route_geojson nullable (GeoJSON LineString),
  acceptance_radius_meters nullable, created_by_user_id, created_by_role varchar
  ('trainer'|'runner'), status varchar ('pending_approval'|'active'|'finished'|'rejected'),
  approved_by_user_id nullable, rejection_reason nullable, finished_at nullable. SIN campo
  title — los pushes usan texto genérico ("Competencia Finalizada").
- Tabla `competition_participant`: competition_id, user_id. PK compuesta; index user_id.
- Tabla `competition_winner`: competition_id, user_id. PK compuesta; index user_id (el empate
  "todos ganan" inserta N filas).
- `runner_session` suma columna `competition_id` int64 nullable, FK opaca (patrón transparent:
  BIGINT > 0 sin constraint de DB, igual que `attendances.training_session_id`).

## D9. Competencias — endpoints, sesiones y cierre

- `POST /teams/{id}/competitions`: creador entrenador (owner) → status active directo; corredor
  (miembro del team) → pending_approval. Validaciones 400: type=metric exige metric y
  end_at > start_at; type=route exige route_geojson y acceptance_radius_meters > 0;
  participantes al menos 2, todos del team, sin incluir al trainer (400 si falla).
- `PATCH /competitions/{id}/approve` y `/reject` `{reason?}`: caller owner del team (o el
  trainer creador, caso defensivo); solo sobre pending; approve → active (+ push al creador);
  reject → rejected con reason opcional (+ push al creador).
- `DELETE /competitions/{id}/cancel`: creador corredor, solo pending; borrado físico con
  participantes.
- `GET /teams/{id}/competitions?status=&group_id=`: miembro de un grupo del team o owner.
  Filtros status y group_id opcionales. Con `has_unseen` de la sección competitions embebido
  bajo el team_id (D11).
- `GET /competitions/{id}`: autenticado miembro/owner. Detalle con progreso en vivo por
  participante: metric → acumulado de la métrica dentro de `[start_at, end_at o now]`; route →
  mejor % de match hasta ahora. Cómputo on-demand en consultas, sin broadcast WS en ese flujo.
- `POST /session-instances/:id/runner` acepta `competition_id` opcional en el body: si viene,
  valida competencia active + caller participant + rango activo (400 si falla); si no viene, el
  flujo queda byte-idéntico a hoy. Persistencia con FK opaca (D8).
- `PATCH /competitions/{id}/close`: caller creador (trainer o runner) u owner del team. SIEMPRE
  manual — el auto-cierre programado queda como deuda documentada (D13). Al cerrar:
  status = finished, finished_at = now, y se calculan winners:
  - metric: MAX de la métrica por participante en la ventana `[start_at, end_at o now]`.
    Empates exactos → TODOS ganan (`winner_user_ids[]`).
  - route: mejor % de match (D10); tie-break mayor % → a igual %, menor duración; a igual
    duración exacta → todos ganan.
  - Sin intentos → finished sin winner (`winner_user_ids[]` vacía).
- Efectos del cierre: winner/s reciben `gamification_stamp` (type competition, source_id
  competencia, source_name "Competencia N", count 1); push best-effort a los participantes con
  `data.route` placeholder; unseen por participante de section competitions con team_id del
  team de la competencia.
- WS: `update:competition_progress` al terminar un intento (channel `session:{id}` de la
  instancia del intento, payload `{competition_id, user_id}` — sin contenido, mismo patrón del
  broadcast de feedback, sin exclusión del emisor). `control:competition_finished` al cerrar
  (channels `session:{instance}` de los intentos registrados; emitir SOLO si hay intentos; el
  push alcanza al resto).

## D10. Route matching server-side

- Route matching ÚNICO al computar winners (close) o el detalle on-demand: sampleo de
  `route_geojson` a pasos fijos (aprox cada 30-50 m por segmento); match % = fracción de
  samples dentro de `acceptance_radius_meters` de cualquier punto GPS del intento
  (Haversine en Go). Sin GPS suficiente en el intento → match % = 0.
- Sin umbral mínimo de coincidencia: el mejor match gana si hay intentos; sin intentos →
  winner vacía.
- Tie-break: mayor % → a igual % menor duración → a igual duración exacta todos ganan.
- Aproximación sampleada avalada por el frontend; es la precisión que se acepta para v1.

## D11. has_unseen de competencias

- GET /teams/{id}/competitions embebe `has_unseen` por team_id (EXISTS user/section/team_id).
- `POST seen {section: 'competitions', team_id}` — el endpoint de E1 se reutiliza sin cambios.

## D12. Verificación y testing

- Postgres real para DAOs (`paceron-test-db :5433`, env `TEST_DB_*`); mocks para services y
  controllers.
- Pins FailingDB no-leak en todos los endpoints nuevos cuando toca error interno de DB (500
  genéricos sin leak).
- Tests de guard 403/400/404 con assert de que el DAO no se toca (patrón estándar del repo).
- Determinismo temporal vía la variable global del helper (D1) — los tests fijan el "hoy" con
  `time.Date`; nunca `time.Now()` para decisiones de día en tests.
- `pg_indexes` sobre las 7 tablas nuevas: gamification_unseen, gamification_streak_state,
  user_achievement_progress, gamification_stamp, competition, competition_participant,
  competition_winner.
- Swagger: `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs` (canónico;
  regenerar solo por los endpoints nuevos, sin churn de metadata ajena).
- Coverage gate 85 (meta gradual 90): `make coverage-with-db` con cache limpio.
- e2e de E4.2 con el hub real (patrón de ws-gateway-sesiones).

## D13. No-goals

- Auto-cierre de competencias por cron (queda documentado como deuda: el cierre es manual).
- Logros multi-threshold (escalera Chispa→Leyenda), familia counter y trofeos post-Leyenda:
  regla futura documentada, no implementada.
- Push con contenido: todos los pushes de gamificación son avisos best-effort con data.route
  placeholder.
- Persistir pace como hecho propio: pace es una razón agregada del período, solo viven las
  columnas denormalizadas de D6.
- Los logros EXTRAS: early_10, distance_250k/500k, attendance_20 y las métricas de volumen
  semanal.
- Push "exhaustivo" de competencias: solo hay push best-effort en cierre, aprobación y
  rechazo; sobre la nueva propuesta corre el punto rojo (unseen) de la sección competitions.

## D14. Riesgos

- Deriva del estado de triggers de racha (aproximado): el punto rojo puede llegar tarde o
  llegar doble; el GET exacto manda en la UI. Documentado también en
  docs/DEUDA_TECNICA_Y_PENDIENTES.md.
- Route matching sampleado: aproximación avalada; no prometemos exactitud geodésica.
- Leaderboard sobre teams gigantes: las pg_indexes cubren el caso común; si aparece una
  consulta lenta se queda documentado como deuda de perf (sin gate de latencia en CI).
- Criterio de team (asistencia confirmada) puede moverse si el front termina eligiendo B —
  el change lo plasmó como A y un ajuste posterior no rompe el shape.
