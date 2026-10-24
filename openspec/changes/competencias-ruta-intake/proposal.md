# Proposal: competencias-ruta-intake (Spec 4b)

## Why

La spec 4b del frontend (trayectoria en mapa) necesita la última pieza de gamificación que
quedó abierta cuando se mergeó `gamificacion-core`: **el intake del intento de ruta y el
cierre sin ventana temporal**. Hoy el backend ya tiene el matching server-side
(`cmd/api/utils/routematch/`, empate/tie-break, verificación por radio Haversine) y los
frames WS (`update:competition_progress` / `control:competition_finished`), pero no existe
la forma de que un corredor participante **abra su intento** sobre una competencia de tipo
route: `POST /session-instances/:id/runner` exige una instancia con día de calendario y
autorización dual (`HasInstanceAccess`), y una competencia route no referencia ninguna
instancia. Además, el guard de cierre exige `end_at` vencido, y una ruta no tiene `end_at` —
hoy una route `active` es imposible de cerrar.

## What

Un change acotado sobre `feature/competencias-ruta-intake` (base: commit `21a5e84` de
`gamificacion-core`), 7 decisiones cerradas con el frontend este mismo día:

1. **D1 — Instancia sintética de ruta.** Al crear una competencia `type='route'`,
   la misma transacción del `POST /teams/{id}/competitions` crea UNA `session_instance`
   sintética, compartida por todos los participantes (no una por competidor): sin día de
   calendario, name descriptivo ("Competencia ruta N"), con un único `exercise_instance`
   sintético "Corrida" (kind `velocidad_distancia`) y su link en
   `session_exercise_instances`. El id se persiste en la columna NUEVA
   `competition.route_session_instance_id` (BIGINT nullable, FK opaca — seteada solo para
   route). Motivo: los feedback rows usan `assigned_session_id`/`assigned_exercise_id`
   NOT NULL apuntando a instancias, así que el tracking GPS del intento
   (`workout_feedback_points`) necesita una instancia real para existir y para heredar el
   flujo pre-start → live → resultado (WS y gates del runner service) sin duplicar
   infraestructura; los broadcasts de feedback del intento reutilizan el canal
   `session:{id}` de esa instancia automáticamente.
2. **D2 — POST /competitions/{id}/attempts.** Endpoint nuevo para que el corredor
   participante abra su intento: 404 competencia inexistente; 400 si type != 'route'; 400
   si status != 'active'; 403 si el caller no es `competition_participant`; 400 si ya
   tiene intento previo (runner_session con ese competition_id + athlete_user_id, único
   garantizado por el uq `(session_instance_id, athlete_user_id)` + la instancia única de
   D1). Crea `runner_session {session_instance_id = route_session_instance_id,
   athlete_user_id = caller, status='wip', competition_id = id}` — NO toca el gate
   presencial (instancia sin día/grupo). Responde `201` con
   `{data: {runner_session_id, session_instance_id, competition_id}}` (convención del POST
   runner existente). No se pide vista de intento ni DELETE en v1.
3. **D3 — Cierre de ruta sin end_at.** En `Close`, el guard "end_at ya pasado" no aplica
   cuando `type='route'` (route no tiene end_at). El resto queda igual: guard
   creador/owner 403, solo active → 400, race-safety existente de
   finished+finished_at. `computeWinners` YA corre el matching route lazy dentro del
   close (routeAttempts → SampleRoute + MatchRatio, commit `21a5e84`) — D3 es una
   modificación PEQUEÑA del guard existente, no un nuevo cálculo.
4. **D4 — Límites de ruta.** Validación en el create route: `route_geojson` debe ser un
   LineString parseable (`routematch.ParseLineString`); máximo 500 vértices; longitud
   Haversine total ≤ 100 km. La ruta es inmutable post-create (editarla = crear
   competencia nueva).
5. **D5 — acceptance_radius_meters.** Rango válido **[5, 500]** metros en el create
   route; campo requerido (sin default backend-mandado; el front sugiere 50).
6. **D6 — WS sin cambios de infra.** El progreso live queda limitado a los 2 frames ya
   implementados (update:competition_progress al terminar intento + control:competition_
   finished al cerrar); el progreso mid-intento (leaderboard en vivo) es deuda/roadmap.
   Al terminar un intento sobre la instancia de ruta (CompetitionID != nil), el frame
   update igual se emite — sin canal nuevo.
7. **D7 — Shape de detalle.** `GET /competitions/{id}` para route sigue mostrando el
   shape existente de progreso (null en vivo); el resultado se refresca vía
   `control:competition_finished` post-close. Sin endpoint ni shape nuevo.

## Capabilities

Modificadas: `gamification-competitions` (delta sobre la spec ya implementada — ADDED para
intake/instancia sintética/límites, MODIFIED del requirement "Cierre manual y winners"
para relajar el guard de end_at en route). Sin endpoint existente roto: el único cambio de
comportamiento en un flujo existente es el relax del guard de Close para route, y la
validación aditiva del create route.

## No-goals

- WS en vivo mid-intento (leaderboard en vivo, traza del GPS propia) —
  deuda/roadmap ya marcada.
- Editar la ruta post-create (un cambio de ruta = competencia nueva).
- Re-intentos: un participante tiene UN intento por competencia; cancelar/interrumpir
  una sesión wip NO la borra ni la reabre (mejora futura).
- Vista de intento (`GET attempts`) ni borrado de intento en v1.
- Auto-cierre programado (sigue siendo deuda de `gamificacion-core`).
- Cualquier métrica o shape nuevo para el frontend más allá de lo acordado.
