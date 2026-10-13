# Design — competencias-ruta-intake

Decisiones cerradas con el frontend (2026-10-13, "confirmo las 4 tal cual"): D2 (intake
POST attempts), D3 (cierre route sin end_at), D5 (rango del radio), D6 (WS sin canal
nuevo), más D1/D4/D7 definidas en esta misma sesión. Base de código: commit `21a5e84`
(`gamificacion-core`), con `routematch` y los frames WS ya mergeados.

## D1 — Instancia sintética de competencia route

**Qué:** al crear una competencia `type='route'`, la MISMA transacción del
`POST /teams/{id}/competitions` crea una única `session_instance` sintética compartida por
todos los participantes y guarda su id en la columna nueva
`competition.route_session_instance_id` (BIGINT nullable, FK opaca — patrón transparente
del repo, como `attendances.training_session_id`; seteada SOLO para route).

**Estructura creada (todo en un tx dentro del Create del competition service):**
- `session_instance {name: "Competencia ruta N" (N = id de competencia o contador
  descriptivo), description: "Instancia sintética de la competencia de ruta", source_
  session_id: NULL}` — no hay `group_calendar_days` que la referencie.
- 1 `exercise_instance` sintético: name "Corrida", kind `velocidad_distancia` (el kind
  vigente al que mapea `running` después de la migración de kinds, ver
  `cmd/api/domains/constants/exercise_kind.go`), sin distance/minutes obligatorios (la
  competencia no fija distancia; los campos quedan NULL).
- Link `session_exercise_instances {session_instance_id, exercise_instance_id,
  role='main', repeat_count=1, rest_minutes=0}`.
- `competition.route_session_instance_id` = id de la instancia.

**Por qué (hipótesis verificada):** los feedback rows llevan `assigned_session_id` /
`assigned_exercise_id` NOT NULL (>0) apuntando a `session_instance`/`exercise_instance`,
así que los `workout_feedback_points` del intento GPS necesitan una instancia real para
existir. Con la instancia compartida, el intento hereda sin cambios el flujo
pre-start → live → resultado del runner service (incluye WS publish y gates) sin duplicar
infraestructura. El Notifier.Emit del Create de feedback usa
`sessionChannel(AssignedSessionID)` (verificado en el controller), así que los broadcasts
de feedback del intento reutilizan el canal `session:{route_session_instance_id}`
automáticamente.

**Consecuencia para la autorización:** la instancia no tiene día ni grupo, así que el
primer brazo de `HasInstanceAccess` (miembro del grupo del día) no matchea; el acceso al
intento lo gobierna este change vía `competition_participant` (D2), no vía
`HasInstanceAccess`. No modificar el guard dual existente.

## D2 — POST /competitions/{id}/attempts

Ruta nueva: `r.POST("/api/v1/competitions/:id/attempts", app.competitionController.
CreateAttempt)`. Autenticado. Guards en orden (cada uno sin tocar el DAO de escritura):
1. `404` si la competencia no existe (`FindByID`).
2. `400 (ErrCompetitionInvalid)` si `type != 'route'`: "solo las competencias de ruta
   admiten intentos".
3. `400` si `status != 'active'`: "la competencia no está activa".
4. `403 (ErrCompetitionForbidden)` si el caller NO está en `competition_participant`
   (`IsParticipant` ya existe en el DAO).
5. `400` si ya tiene intento: runner_session con `competition_id = id` +
   `athlete_user_id = caller`. La unicidad la garantiza `uq_runner_session_session_
   athlete (session_instance_id, athlete_user_id)` + la instancia única de D1.

**Acción:** crea `runner_session {session_instance_id = comp.RouteSessionInstanceID,
athlete_user_id = caller, status='wip', competition_id = comp.ID}`. NO pasa por el gate
presencial (la instancia sintética no tiene día/grupo — ese check no aplica
estructuralmente). Fecha/rango: no aplica (route no usa ventana temporal).

**Respuesta:** `201` con `{data: {runner_session_id, session_instance_id,
competition_id}}` — misma convención del POST runner existente. El finish del intento
sigue siendo el `PATCH /session-instances/:id/runner` existente (CompetitionID ya viene
seteado al crear; el flujo de completitud/Gate a instancias sirve tal cual).

**Fuera de alcance v1:** GET de intento, DELETE de intento, cancelar/interrumpir que borre
o reabra (una sesión wip/interrupted queda como está; mejora futura).

## D3 — Cierre de ruta sin end_at

En `Close` (`cmd/api/services/competition_service.go:574`), el guard
`bundle.comp.EndAt != nil && gametime.Now().Before(*bundle.comp.EndAt) → 400` NO aplica
cuando `type='route'` (route no tiene end_at: el guard ya es neutral con EndAt nil, pero
se documenta explícito para route). El resto del cierre queda byte-idéntico:
- Guard creador u owner del team → `403` (`ErrCompetitionForbidden`).
- Solo `active` → `400 (ErrCompetitionConflict)`; race-safety del Finish
  (finished+finished_at) existente.
- `computeWinners` YA corre el matching route lazy dentro del close
  (`routeAttempts` → `ListAttemptSessions` + `ListAttemptPoints` +
  `routeSamples`/`SampleRoute` + `MatchRatio`, tie-break por % y duración): D3 NO
  recalcula ni duplica nada. Los efectos (stamps, unseen, push, WS
  `control:competition_finished` por los intentos registrados) reutilizan
  `closeEffects`/`notifyFinish` y los canales `session:{route_session_instance_id}`.

## D4 — Límites de ruta (create route)

En `validateCreateRequest` (branch `TypeRoute`) agregar, después del shape existente:
- `routematch.ParseLineString(*req.RouteGeoJSON)` válido → si no, 400 "route_geojson
  requiere un LineString GeoJSON válido".
- ≤ **500 vértices** → si no, 400 "la ruta admite hasta 500 puntos".
- Longitud Haversine total ≤ **100 km** → si no, 400 "la ruta no puede superar los 100
  km". La longitud se calcula sumando Haversine por segmento sobre los vértices crudos
  (helper local en el service; el matcher ya tiene Haversine — exponer o replicar el
  cálculo puntual).
- Ruta inmutable post-create: NO hay loop de edición (decisión documentada; cambiar una
  ruta = crear competencia nueva). No se agrega endpoint de PATCH de ruta.

**Performance:** el límite de 100 km con `SampleRoute` (step 40 m) da como máximo ~2500
samples in-memory en el close — OK para CPU, no hace falta gate ni changes al matcher.

## D5 — acceptance_radius_meters

En el create route, el campo ya es requerido (shape existente); acotar el rango a
**[5, 500]** → fuera de rango 400 "el radio de aceptación debe estar entre 5 y 500
metros". Sin default backend-mandado: si falta → 400 explícito (como hoy). El default
SUGERIDO al front es 50 m (aclarar en docs de Task 4).

## D6 — WS de progreso en vivo

NO cambiar el protocolo WS: el progreso live de competencias queda limitado a los 2
frames ya implementados (`update:competition_progress` / `control:competition_finished`
en `cmd/api/realtime/protocol.go`; convenciones update:* snake / control:* camel / channel
`session:{id}`). No se crea canal nuevo para el intento en curso; el progreso live
mid-intento (traza de GPS pendiente, leaderboard en vivo) es deuda/roadmap.

Punto a verificar en implementación: el `Finish` del runner session controller ya emite
`update:competition_progress` cuando el intento tiene CompetitionID != nil. Con D2, el
intento de ruta nace con competition_id SETEADO, de modo que al terminar el intento el
frame sale con `{competition_id, user_id}` por `session:{route_session_instance_id}` sin
código nuevo — el e2e (si falta y cabe en ≤ 20 líneas de test) valida esa emisión contra
el hub real.

## D7 — Shape de detalle de intento

`GET /competitions/{id}` para route mantiene el shape existente: el progreso on-demand
(quedó implementado por `gamificacion-core`, `computeRouteProgress`) sigue
disponible on-demand, PERO no se suma ninguna métrica ni shape nuevo para el front. En
vivo la vista del front refresca por `control:competition_finished` post-close; no hay
push durante el intento (D6). Nota de diseño: no endpoint nuevo, no shape nuevo.

## Reglas de D10 que este change reafirma

- Un intento por participante: cap garantizado por `uq_runner_session_session_athlete`
  (instancia compartida + athlete_user_id) — el "reuso del flujo toRunnerSession" no
  introduce segundos intentos.
- El matching corre SOLO en close y en el detalle on-demand (lazy): D2 no computa
  match al abrir el intento, y D3 no revisa datasets viejos — `routeAttempts` filtra por
  `status=finished` + participants en vivo al cerrar.

## Criterios de implementación por componente

- **`cmd/api/domains/dbs/competition.go`** — sumar `RouteSessionInstanceID *int64`
  (column `route_session_instance_id`, BIGINT nullable, sin FK de DB — patrón opaco).
- **`cmd/api/daos/competition_dao.go`** — `Create` extendido (o método tx-adjunto) para
  crear instancia + exercise + link dentro del mismo tx que comp + participants; DAO
  `CreateAttemptSession` (o reuso del dao de runner sessions) para el insert de
  runner_session; `ExistsAttemptByCompetitionAndAthlete` para el guard anti-doble.
- **`cmd/api/services/competition_service.go`** — `Create` branch route: tx =
  instancia sintética + guardar `route_session_instance_id`; `validateCreateRequest`
  branch route: ParseLineString + ≤ 500 vértices + ≤ 100 km + radius ∈ [5,500]; `Close`:
  guard end_at condicionado a `TypeMetric` (route se cierra sin ventana); método
  `CreateAttempt` con los 5 guards de D2.
- **`cmd/api/controllers/competition_controller.go`** — handler `CreateAttempt`
  (status 201, shape `{data: {...}}`); mapeo de errores de servicio a 400/403/404 con
  500 genérico sin leak en FailingDB.
- **`cmd/api/app/url_mappings.go`** — ruta nueva `POST /api/v1/competitions/:id/attempts`.
- **`cmd/api/realtime/protocol.go`** — sin cambios (solo consumo de los frames ya
  mergeados).
- **Swagger** — `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`
  SOLO en las tasks que agregan/validan endpoints (Task 1 create route validations,
  Task 2 attempts).
