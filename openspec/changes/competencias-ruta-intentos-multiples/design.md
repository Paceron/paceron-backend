# Design — competencias-ruta-intentos-multiples

Decisiones cerradas con el frontend (2026-10-22, 12 decisiones aprobadas). Base
de código: `develop` con `competencias-ruta-intake` mergeado (instancia sintética
compartida, intake POST attempts, matching Haversine en close y detalle
on-demand). Referencias de código lineadas: `cmd/api/services/competition_
service.go` (CreateAttempt :185 con 5 guards; computeRouteProgress :628;
computeWinners :759 branch route :774; routeAttempts :554; toCompetitionResponse
:898), `cmd/api/daos/competition_dao.go` (ExistsAttemptByCompetitionAndAthlete
:344; createSyntheticRouteInstance :235; deleteSyntheticRouteInstance :480;
ListAttemptSessions :539; ListAttemptPoints :550), `cmd/api/daos/session_
instance_dao.go` (HasInstanceAccess :142, 3ª rama :166; espejo batch :211).

## D1 — `max_attempts`: columna, validación de create, inmutabilidad

**Qué:** columna nueva `max_attempts` (BIGINT nullable, sin default, entra por
AutoMigrate; campo `MaxAttempts *int64` en `dbs.Competition`). Solo aplica a
type=route.

Validación en `validateCreateRequest`, junto a las reglas de ruta del branch
TypeRoute (el branch metric también gana un guard):

- **route:** `max_attempts` OPCIONAL. Si NO viene → NULL = SIN límite (intentos
  ilimitados; coherente con las competencias pre-existentes sin backfill). Si
  viene → entero positivo MÍNIMO 1: 400 si 0, negativo o no-entero (el binding
  del DTO `*int64` ya rechaza "1.5" con 400 de Gin). SIN tope superior.
  Mensaje de 400: "max_attempts debe ser un entero mayor o igual a 1".
- **metric:** 400 si viene ("type=metric no admite max_attempts") — mismo
  patrón de los guards `route_geojson`/`acceptance_radius_meters` del metric.
- **Inmutable post-create:** no existe edición de competencia (cancel borra,
  las demás transiciones no tocan datos de la competencia) → la inmutabilidad
  no necesita código nuevo, solo no agregar nada que la edite. Nota explícita
  en tests/convención.

Persistencia: el service copia `req.MaxAttempts` a `comp.MaxAttempts` en el
armado del struct (ambos caminos create/CreateWithInstance lo insertan con la
fila de competencia — cero código de DAO extra).

## D2 — CreateAttempt rework: límite por COUNT e instancia nueva por intento

**Guards que quedan intactos (orden actual):** 404 inexistente → 400
`type != route` → 400 `status != active` → 403 no participante → guard de
`RouteSessionInstanceID == nil`.

**Guard que se reemplaza:** "existe intento → 400" (`ExistsAttemptByCompetition
AndAthlete`, línea :203-209 del service) pasa a ser el LÍMITE: si
`comp.MaxAttempts != nil` y COUNT de `runner_session` tagueados con
`competition_id` + `athlete_user_id = caller` (CUALQUIER status: un wip cuenta
como intento en curso) `>= *comp.MaxAttempts` → 400 con mensaje
"llegaste al límite de intentos de esta competencia". Con `max_attempts` NULL
no hay guard de cantidad (intentos ilimitados). Con `max_attempts = 1` el
comportamiento es funcionalmente equivalente al doble-intento de hoy.

**Mecánica del insert (toda la pieza vive en el CompetitionDAO, que ya consulta
`runner_session` — precedente de `ExistsAttemptByCompetitionAndAthlete`):**
método nuevo `CreateAttemptSession(ctx, comp, athleteUserID, maxAttempts)`,
transacción única:

1. `SELECT ... FROM competition WHERE id = ? FOR UPDATE` — serializa todos los
   CreateAttempt de la competencia. CRÍTICO: el unique
   `(session_instance_id, athlete_user_id)` ya NO arbitra el límite (los
   intentos 2+ usan instancias distintas); sin este lock, dos requests
   concurrentes pueden ambos contar y ambos insertar, excediendo `max_attempts`.
2. COUNT de intentos del atleta en la competencia (cualquier status).
3. **Intento 1 (count == 0):** reusa la instancia compartida
   (`comp.RouteSessionInstanceID`) — comportamiento byte-idéntico al actual
   (incluida la semántica ON CONFLICT DO NOTHING del insert; si un intento
   concurrente ganó, RowsAffected = 0 → error de doble intento; el retry del
   cliente arma correctamente el intento 2).
4. **Intento 2+ (count >= 1):** crea una `session_instance` NUEVA reusando el
   helper `createSyntheticRouteInstance` (refactorizado para recibir el nombre
   parametrizado; las llamadas existentes del create siguen pasando
   `"Competencia ruta %d"`): mismo ejercicio sintético "Corrida" (kind
   `velocidad_distancia`, link `role='main'`, repeat 1, rest 0), sin día de
   calendario, nombre `"Competencia ruta N (intento M)"` con N = id de
   competencia y M = count + 1. Luego inserta el `runner_session` tagueado con
   `session_instance_id` = instancia nueva, `status='wip'`, `competition_id`,
   `start_date = gametime.Now()`.
5. Cualquier fallo en CUALQUIER paso → rollback total (no quedan instancias
   ni runner sessions huérfanas ni viceversa).

**Respuesta:** sin cambios — `201 {message: "intento registrado", data:
<RunnerSessionResponse>}` con el id de la runner session, su
`session_instance_id` (compartida en intento 1, nueva en 2+) y `competition_id`.
El finish sigue siendo `PATCH /session-instances/:id/runner` tal cual
(CompetitionID ya viene seteado; el frame `update:competition_progress` sale
igual, D8).

**Fuera de alcance v1:** no se pide vista alternativa ni borrado de intento.
Una wip/interrupted cuenta para el límite y no se reabre (deuda documentada).

## D3 — WS: tercera rama de HasInstanceAccess extendida (CRÍTICO)

**Problema:** los intentos 2+ corren sobre instancias nuevas que NINGUNA regla
actual de `HasInstanceAccess` autoriza (no tienen día/grupo ni feedback al
principio, y la 3ª rama solo matchea `route_session_instance_id` = compartida).
Sin extensión, el front no podría suscribirse a `session:{instancia-intento-2}`
y el intento quedaría sin realtime aunque exista su canal.

**Qué:** extender la 3ª rama (la del branch competition) con un OR:

```sql
-- rama actual (queda igual)
competition.route_session_instance_id = ? AND competition.type = 'route'
-- OR nuevo
OR EXISTS (
  SELECT 1 FROM runner_session rs
  JOIN competition c2 ON rs.competition_id = c2.id AND c2.type = 'route'
  WHERE rs.session_instance_id = ?           -- la instancia pedida
    AND EXISTS (
      SELECT 1 FROM competition_participant cp
      WHERE cp.competition_id = c2.id AND cp.user_id = callerID)
)
```

- caller es participante de una competencia `type=route` que tiene un
  `runner_session` tagueado con esa instancia (cualquier status — SIN filtro de
  status: el front suscribe en pre-start con wip y debe recibir
  `control:competition_finished` si el intento finished o incluso si otro miembro
  escucha post-close).
- La instancia compartida queda autorizada por ambas vías (la rama actual la
  matchea; el OR nuevo también la matchea — misma competencia, mismo resultado,
  sin efecto práctico).
- La competencia del OR es por el join `runner_session.competition_id` (FK
  opaca), NO por columna de competition — coherente con D10.

**Mirror obligatorio:** `hasAccessBatchCompetitionSQL` (espejo de
`HasInstanceAccessBatch`) se extiende con el mismo OR (`u.user_id` como
caller). Los tests DAO de la rama nueva deben cubrir el batch también: los
controller paths que resuelven acceso en lote no pueden divergir del singular.

**Test:** participante de la competencia route sí accede a la instancia de su
intento 2 con solo el `runner_session` wip (sin día, sin feedback); usuario
ajeno al competition_participant no; una instancia regular de calendario sigue
regida por las 2 ramas duales intactas (regresión); intento finished también
autoriza (sin filtro de status).

## D4 — Endpoint GET /api/v1/competitions/{id}/attempts

- Ruta: `r.GET("/api/v1/competitions/:id/attempts", ...ListAttempts)`. Handler
  autenticado.
- **Guard (igual que el detalle, 404 antes de 403):** `FindByID` → 404 si no
  existe; si query `athlete_user_id` viene, parsear positivo (400 si inválido);
  guard miembro-u-owner: `IsTeamOwner(comp.team_id, caller)` OR
  `IsTeamGroupMember(comp.team_id, caller)` → 403 (`ErrCompetitionForbidden`)
  en el resto. NO hay guard de tipo/status (los intentos de una competencia
  pendiente/rechazada son vacíos o históricos — se lista igual).
- **`athlete_user_id`:** default = caller. Cualquier MIEMBRO del team puede ver
  los intentos de otro atleta (decisión: participación no restringe lectura).
- **Atleta pedido no-participante** (jamás tuvo runner_session de la
  competencia): lista vacía `[]` — no 400 (la unicidad participante se mantiene
  en escritura, la lectura no exige).
- **Envoltura:** `{data: [...]}` simple (como el detalle). SIN `has_unseen`:
  el unseen de competencias es por team y ya lo cubre el listado
  (`HasUnseenTeam`); no se duplica.
- **Ítem** (DTO nuevo en `domains/competition`):

| campo | tipo | fuente / regla |
|---|---|---|
| `id` | int64 | runner_session id (convención runner-session, decisión cerrada) |
| `session_instance_id` | int64 | instancia del intento (compartida en 1, nueva en 2+) |
| `status` | string | wip / finished / interrupted |
| `started_at` | time | `runner_session.start_date` |
| `ended_at` | *time | `runner_session.end_date` (null en wip) |
| `distance_meters` | *float64 | SUM de `workout_feedback.distance_meters` de las series del atleta sobre la instancia del intento (feedback activo `deleted_at IS NULL`); null si no hay feedback/series |
| `compliance_ratio` | *float64 | matching Haversine on-demand (mismo cálculo del detalle: samples de la ruta + `MatchPercent`); null salvo intento finished de `type=route` (un intento finished de competencia metric NO computa: no hay ruta que matchear — queda null); 0 si finished sin puntos GPS |
| `duration_ms` | *int64 | server-side: `end_date - start_date` en ms (calcularlo en el service, nunca heredar la columna: route no tiene duración persistida); null si `end_date` nil (wip) |

  Con nota de nombre: `started_at`/`ended_at` son los nombres del DTO NUEVO; la
  fuente es `start_date`/`end_date` de `runner_session` (verificado). Se decide
  el nombre nuevo en vez del plural importado del módulo de sesiones para que
  el item lea como "intentos" y no como una runner session cruda.
- **Orden:** por id ASC (cronológico = número de intento).
- Service: método `ListAttempts(ctx, callerID, competitionID,
  athleteUserID)`. DAO helpers nuevos: `ListAttemptsByCompetitionAndAthlete`
  (runner sessions de la competencia + atleta, ORDER BY id ASC) y
  `SumFeedbackDistanceByInstances` (SUM distance_meters GROUP BY
  assigned_session_id, feedback activo del atleta sobre esas instancias) —
  batch, sin N+1. `compliance_ratio` reusa `routeSamples` + `ListAttemptPoints`
  + `MatchPercent` con los pares (instancia, atleta) del intento igual que
  `routeAttempts`.

## D5 — progress del detalle: nuevo shape para route

**shape route (pedido del front):** `{user_id, compliance_ratio, duration_ms,
session_instance_id}` = MEJOR INTENTO por atleta (misma reducción de D6).
- `compliance_ratio`: pct 0-100 (mismo valor que hoy viaja en `value` — ver
  nota de doc).
- `duration_ms`: duración del mejor intento (ms).
- `session_instance_id`: instancia que produjo el mejor intento (compartida si
  fue el intento 1).
- Orden por user_id ASC (igual que hoy).

**shape metric SE MANTIENE byte-idéntico:** `{user_id, value}` con
`CompetitionProgressItem` actual (value puede ser null para quienes nunca
reportaron).

**DTO:** `CompetitionProgressItem` se parte en `CompetitionMetricProgressItem`
(campos actuales) y `CompetitionRouteProgressItem` (4 campos del shape route).
El campo `Progress` de `CompetitionResponse` pasa a tiparse como slice de la
interfaz común que ambos implementan (serializa cada elemento según su struct
dinámico: el JSON de metric queda intacto, el de route ya no arrastra
`value`). Alternativa descartada: campos omitempty dentro de un único struct —
rompería la serialización `value: null` del metric.

**Corrección de doc:** hoy `computeRouteProgress` devuelve `value = pct`
(0-100) en el código, y `CompetitionProgressItem` documentaba "value null para
route hasta E4.2" — la doc quedaba desmentida. Con el shape nuevo esto se
resuelve de raíz (route deja de usar `value`); actualizar §8.14 de
`docs/CATALOGO_Y_CALENDARIO.md` en T5.

## D6 — computeWinners route: mejor intento por atleta primero

**Problema actual (:774-801):** `routeAttempts` devuelve un intento por
runner_session finished; con intentos múltiples, un mismo atleta con 2 intentos
puede matchear el `bestPct` global con ambos y entrar DOS veces en winners
(duplicados en `winner_user_ids`, doble stamp).

**Qué (rework del branch route de `computeWinners`):**
1. **Reducción por atleta primero:** por cada participante, su MEJOR intento = max pct; empate exacto de pct → menor `duration_ms` (el helper se comparte con `computeRouteProgress` para no duplicar la regla).
2. **Después, entre atletas:** ganan TODOS los atletas cuyo mejor intento empata EXACTO en (pct, duración) con el mejor valor global — igual semántica de empate-total del metric (D9 del core).
3. Sin intentos de un participante → queda fuera; sin intentos de nadie → winners vacía (sin stamps/pushes, efecto ya existente).

**Test clave:** atleta A con intentos de pct 100 y 60, atleta B con 100 →
winners = A y B (una vez cada uno); A con 100 (duración menor) y B con 100
(duración mayor) → solo A; empate exacto de pct+duración → ambos.

## D7 — CompetitionResponse: max_attempts / attempts_used / can_attempt

- **DTO:** 3 campos nuevos en `CompetitionResponse`:
  - `MaxAttempts *int64` `json:"max_attempts"` — nullable: NULL = sin límite;
    null para metric (decisión cerrada; el front distingue "sin límite" de
    "no aplica" porque la pantalla route siempre los consulta).
  - `AttemptsUsed *int64` `json:"attempts_used"` — intentos del CALLER
    (cualquier status, misma marca competition_id); 0 si el caller es
    participante sin intentos; NULL si no es route o el caller no es
    participante (decisión: null para no-aplicables, no 0).
  - `CanAttempt bool` `json:"can_attempt"` — bool SIEMPRE presente: true solo
    si `type=route && status=active && caller participante && (max_attempts
    NULL || attempts_used < max_attempts)`; false en todos los demás (metric,
    no-participante, competencia no active, límite cumplido). Server-side por
    decisión (no derivable solo del cliente cuando el caller no es el dueño de
    los datos).
- **Dónde se computa:** el mapper `toCompetitionResponse` sigue siendo puro —
  el service llena los 3 campos: helper de instancia
  (`fillAttemptFields(resp, comp, callerID)` con COUNT individual) para detalle
  y mutaciones (Approve/Reject confirman status fresco; el count del caller se
  lee fresco — sin riesgo relevante de race), y variante batch para
  `ListByTeam`: DAO nuevo `CountAttemptsByCompetitionAndAthlete(ctx,
  competitionIDs, athleteUserID)` → map competition_id → count, UNA query para
  todo el listado (sin N+1).
- En `Create` (respuesta de alta): la competencia recién creada trae
  `attempts_used = 0` si el caller está entre los participantes (creador
  corredor, 0 intentos posibles aún) y null si no (trainer: no compite) —
  `can_attempt` false en ambos casos (runner-creada arranca en
  `pending_approval`; el trainer no participa). Coherente para ese momento.
- Los 3 campos llenan también las filas del listado (misma decisión D7;
  batch sin N+1), no solo el detalle.

## D8 — WS frames sin cambios; sin migración destructiva; reuso total

- **Frames de protocolo intactos:** `update:competition_progress` sale al
  terminar CUALQUIER runner session tagueada (CompetitionID != nil) por el
  canal de SU instancia — con intentos 2+ ahora sale por más canales del
  correcto (`session:{instancia-del-intento}`). El front ya suscribe a la
  compartida y a partir de este change suscribe además a las instancias de sus
  intentos (autorización por D3). `control:competition_finished` al cerrar
  llega a los canales de TODAS las instancias de intentos registrados:
  `Close` ya dedupea instancias con el map `seen` (:716-722) y
  `ListAttemptSessions` trae cualquier status — los intentos 2+ entran
  gratis; sin código de protocolo nuevo.
- **Sin migración destructiva:** solo la columna nullable `max_attempts`
  (AutoMigrate) sin backfill — competencia vieja = NULL = ilimitada, coherente
  con D1. `route_session_instance_id` sigue apuntando a la compartida (intento
  1); los intentos 2+ se alcanzan por `runner_session.competition_id`, NO por
  columna de competition.
- **Un intento = 1 runner_session:** el feedback, sus puntos GPS, el gate, el
  WS y el matching reusan el flujo existente de punta a punta. El gate
  presencial no aplica (instancia sin día). El close con el límite cumplido
  sigue manual (creador/owner); cancel/reject siguen siendo solo-pending y
  por definición en ese estado no hay intentos: la limpieza
  `deleteSyntheticRouteInstance` de la compartida queda intacta.

## Criterios de implementación por componente

- **`cmd/api/domains/dbs/competition.go`** — campo `MaxAttempts *int64`
  (column `max_attempts`, BIGINT nullable, sin FK).
- **`cmd/api/domains/competition/competition.go`** — `CreateCompetitionRequest
  .MaxAttempts *int64`; `CompetitionResponse` + `MaxAttempts`/`AttemptsUsed`/`
  CanAttempt`; `CompetitionMetricProgressItem`/`CompetitionRouteProgressItem`
  (interfaz común para `Progress`); DTO del GET attempts
  (`CompetitionAttemptsResponse {Data}` + ítem).
- **`cmd/api/daos/competition_dao.go`** — `CreateAttemptSession` (tx con lock
  + count + instancia nueva/insert, D2); refactor firma de
  `createSyntheticRouteInstance` por nombre; `ListAttemptsByCompetitionAnd
  Athlete`; `SumFeedbackDistanceByInstances`; `CountAttemptsByCompetitionAnd
  Athlete` (batch). NO tocar `Delete`/`deleteSyntheticRouteInstance` (limpieza
  de cancel intacta).
- **`cmd/api/daos/session_instance_dao.go`** — extensión de la 3ª rama de
  `HasInstanceAccess` + `hasAccessBatchCompetitionSQL` (D3).
- **`cmd/api/services/competition_service.go`** — `validateCreateRequest`
  (D1); `CreateAttempt` rework (D2, delega la tx en el DAO); `ListAttempts`
  (D4); `computeRouteProgress` y branch route de `computeWinners` (D5/D6, con
  helper compartido de mejor intento); `fillAttemptFields` + variante batch
  (D7); armar `comp.MaxAttempts` en el código del create.
- **`cmd/api/controllers/competition_controller.go`** — handler `ListAttempts`
  (status 200, shape `{data}`), mapeo de errores a 400/403/404 con 500
  genérico sin leak en FailingDB; swagger annotations del endpoint nuevo.
- **`cmd/api/app/url_mappings.go`** — `GET /api/v1/competitions/:id/attempts`.
- **Tests:** DAO tests contra Postgres real; services/controllers con mocks.
- **Swagger** — regenerar con el comando canónico solo en tasks que tocan
  endpoints (T1 validación del create, T2, T3, T4 campos del response).

## Verificación (para todas las tasks)

- Suite completa contra Postgres real: `docker start paceron-test-db` (:5433) y
  `go test ./...` con
  `TEST_DB_HOST=localhost TEST_DB_PORT=5433 TEST_DB_USER=postgres TEST_DB_PASSWORD=postgres
  TEST_DB_NAME=paceron_test` (los DAO tests se skipean solos si no hay DB, pero
  la meta es correrlos: no es excusa para bajar coverage).
- Coverage: `go clean -cache` + `make coverage-with-db` (mismo comando de
  ci.yml) + analyzer sobre el perfil; gate 85 MÍNIMO — no conformarse: tests
  más exhaustivos que el mínimo (meta gradual 90). Nunca bajar el umbral.
- Swagger canónico: `swag init --parseDependency -g cmd/api/docs.go --output
  cmd/api/docs`; revisar el diff para que no suba churn de metadata ajena.
- `openspec validate competencias-ruta-intentos-multiples --strict` → VALID.
- gofmt SOLO sobre los archivos tocados; vet/build en verde antes de tildar.
- Stage SOLO rutas explícitas del task — jamás `git add -A` / `git add .`
  (los artefactos `.superpowers/` jamás se stageean).
