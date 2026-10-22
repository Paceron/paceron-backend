# Spec Delta — gamification-competitions

> Delta sobre la capability `gamification-competitions`, ya implementada por
> `gamificacion-core` y ampliada por `competencias-ruta-intake`. La capability
> principal nunca fue archivada: todo se declara aquí bajo `## ADDED
> Requirements` (un delta `MODIFIED` contra una spec base no archivada sería
> rechazado por el archive — convención del repo, ver
> `competencias-ruta-intake` y `asignacion-por-instanciacion`). Cuando un
> requirement reemplaza una versión previa, lo declara completo con nota de
> qué reemplaza.

## ADDED Requirements

### Requirement: Límite de intentos (max_attempts)

El sistema DEBE cumplir lo siguiente (MUST):

> Reemplaza (extiende) el requirement "Límites y validación de ruta en el
> create" de `competencias-ruta-intake` (versión nueva): agrega `max_attempts`
> al create route y su rechazo al create metric; las reglas de geojson/radios
> quedan intactas.

- `max_attempts` es un campo SOLO de competencias `type=route`, nullable:
  - En el create route es OPCIONAL. Si NO viene → NULL = SIN límite (intentos
    ilimitados). Si viene → entero positivo mínimo 1: 400 si 0, negativo o
    no-entero (para no-entero el 400 es el error de binding de Gin; el mensaje
    "max_attempts debe ser un entero mayor o igual a 1" aplica a 0/negativo).
    SIN tope superior.
  - En el create metric es rechazado: 400 "type=metric no admite max_attempts".
- `max_attempts` es inmutable post-create (no existe edición de competencia; no
  se agrega ninguna vía de edición).
- Se persiste en la columna `competition.max_attempts` (BIGINT nullable, entra
  por AutoMigrate SIN backfill: las competencias pre-existentes quedan NULL =
  ilimitadas).
- Se expone en `CompetitionResponse` como `max_attempts` (null = sin límite;
  null para metric).

#### Scenario: Create route sin max_attempts

- **WHEN** se crea una competencia route sin `max_attempts`
- **THEN** se persiste NULL (sin límite de intentos) y la respuesta expone
  `max_attempts: null`.

#### Scenario: Create route con max_attempts

- **WHEN** se crea una competencia route con `max_attempts = 3`
- **THEN** se persiste 3 y la respuesta expone `max_attempts: 3`.

#### Scenario: max_attempts = 0 rechazado

- **WHEN** el create route envía `max_attempts = 0` (o negativo)
- **THEN** responde `400` con "max_attempts debe ser un entero mayor o igual a
  1" (con valor no-entero responde `400` con el error de binding de Gin).

#### Scenario: Create metric rechaza max_attempts

- **WHEN** el create metric envía `max_attempts`
- **THEN** responde `400` con "type=metric no admite max_attempts".

#### Scenario: Inmutable post-create

- **WHEN** una competencia fue creada con (o sin) `max_attempts`
- **THEN** ningún endpoint la modifica durante su ciclo (pendiente/active/
  finished existe pero no edita la columna).

### Requirement: Intentos múltiples en el intake (POST /competitions/{id}/attempts)

El sistema DEBE cumplir lo siguiente (MUST):

> Reemplaza el requirement "Intake del intento (POST /competitions/{id}/
> attempts)" de `competencias-ruta-intake`: los guards 404 / 400 type / 400
> status / 403 quedan intactos, y el "400 doble intento" se reemplaza por el
> límite de `max_attempts`.

- El guard de cantidad pasa de "existe intento → 400" a: si
  `max_attempts != null` y el COUNT de runner_sessions tagueadas con
  `competition_id` + `athlete_user_id = caller` (CUALQUIER status: un intento
  wip cuenta como en curso) `>= max_attempts` → `400` con mensaje "llegaste al
  límite de intentos de esta competencia". Con `max_attempts = null` no hay
  límite de cantidad.
- Intento 1 (COUNT = 0): reusa la instancia sintética compartida
  (`route_session_instance_id`) — comportamiento idéntico al actual.
- Intento 2+ (COUNT >= 1): en la MISMA transacción del insert de
  `runner_session` crea una `session_instance` NUEVA (rollback total ante
  cualquier fallo: sin instancias ni runner sessions huérfanas), con el
  ejercicio sintético igual ("Corrida", kind `velocidad_distancia`, link
  `role='main'` repeat 1 rest 0), sin día de calendario, y nombre
  `"Competencia ruta N (intento M)"` donde N = id de la competencia y
  M = número de intento (COUNT + 1). El `runner_session` se taguea con
  `session_instance_id` = la instancia nueva, `status='wip'` y
  `competition_id`.
- Race-safety: la transacción bloquea la fila de competencia (FOR UPDATE) y la
  decisión contra el límite corre dentro del tx — las requests concurrentes
  de un mismo atleta no pueden exceder `max_attempts` (el unique
  `(session_instance_id, athlete_user_id)` ya no arbitra: las instancias son
  distintas por intento).
- Respuesta sin cambios: `201` con `{message, data}` shape runner-session
  (`id`, `session_instance_id` del intento creado, `competition_id`, status
  `wip`).

#### Scenario: Primer intento reusa la instancia compartida

- **WHEN** un participante abre su primer intento de una competencia route
  active sin límite cumplido
- **THEN** su runner_session queda tagueada con `route_session_instance_id` y
  el nombre de la instancia sigue siendo "Competencia ruta N" (sin cambio).

#### Scenario: Segundo intento crea instancia nueva con nombre de intento

- **WHEN** el mismo participante (o cualquiera con 1 intento registrado) abre
  un segundo intento con `max_attempts = null` (o con límite aún no cumplido)
- **THEN** se crea una `session_instance` NUEVA (una por intento extra) con
  nombre "Competencia ruta N (intento 2)" y el mismo ejercicio "Corrida", y su
  runner_session queda tagueada con esa instancia nueva y `competition_id`.

#### Scenario: Límite de intentos alcanzado

- **WHEN** un participante con `max_attempts = 2` y ya 2 intentos registrados
  (de cualquier status) intenta abrir un tercero
- **THEN** responde `400` con "llegaste al límite de intentos de esta
  competencia" y no se crea ninguna fila (ni instancia).

#### Scenario: Intentos ilimitados

- **WHEN** la competencia tiene `max_attempts = null`
- **THEN** el participante puede registrar N intentos y ninguno es rechazado
  por cantidad.

#### Scenario: Carrera concurrente no excede el límite

- **WHEN** dos requests de CreateAttempt del mismo atleta corren en paralelo
  sobre una competencia cuyo límite admite exactamente 1 intento más
- **THEN** solo una crea el intento; la otra responde `400` y no quedan filas
  extra (roll back total).

### Requirement: Autorización WS de instancias de intento (tercera rama de HasInstanceAccess)

El sistema DEBE cumplir lo siguiente (MUST):

> Reemplaza la versión del requirement trasladado en `competencias-ruta-intake`
> (3ª rama autoriza SOLO la instancia compartida vía
> `route_session_instance_id`): se extiende con un OR para instancias de
> intentos 2+.

- `HasInstanceAccess` (y su espejo batch `HasInstanceAccessBatch`) autoriza a
  `caller` sobre una instancia si, además de las ramas correspondientes
  vigentes (día/grupo, feedback activo, instancia compartida), es participante
  de una competencia `type='route'` que tiene un `runner_session` tagueado con
  esa instancia (EXISTS sobre `runner_session × competition ×
  competition_participant`), SIN filtro de status del intento.
- El criterio del intento es la marca `runner_session.competition_id` (FK
  opaca); NO se agrega columna de competencia por intento.
- Las 2 ramas duales pre-existentes (día/grupo y feedback) quedan intactas.

#### Scenario: WS autoriza la instancia del intento 2

- **WHEN** un participante suscribe el canal `session:{id}` de la instancia
  nueva de SU intento 2 (runner_session wip ya tagueada, sin feedback aún)
- **THEN** `HasInstanceAccess` responde `true` (se le entregan tanto los
  broadcasts del intento como los frames de competencia que salgan por ese
  canal).

#### Scenario: Sin filtro de status del intento

- **WHEN** el runner_session del atleta sobre la instancia pedida está
  finished (o wip) y el caller es participante de dicha competencia route
- **THEN** la autorización da `true` en ambos casos (se puede recibir
  `control:competition_finished`).

#### Scenario: Usuario ajeno no accede a la instancia de un intento

- **WHEN** un usuario que NO es participante de ninguna competencia route con
  runner_sessions por esa instancia pide acceso
- **THEN** `HasInstanceAccess` responde `false` (salvo que las demás ramas
  autoricen por otra vía, como su día o feedback).

#### Scenario: Instancia regular de calendario sin cambios

- **WHEN** se evalúa una instancia con día de grupo y/o feedback regular (y sin
  runner_sessions de competencia route)
- **THEN** la decisión sale de las 2 ramas duales intactas, byte-idéntico al
  comportamiento previo al change.

### Requirement: Endpoint de intentos (GET /competitions/{id}/attempts)

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /api/v1/competitions/{id}/attempts` disponible, autenticado. Guard igual
  que el detalle de competencia: `404` ante competencia inexistente (antes de
  cualquier guard de autorización); luego `403` si el caller no es miembro de
  grupo del team ni owner del mismo (criterio del detalle). NO requiere
  status ni un tipo específico de competencia.
- Query `athlete_user_id` (entero positivo opcional): sin la query → por
  defecto el propio caller; con la query → los intentos de ese atleta. Cualquier
  miembro del team puede ver los intentos de otro (la participación no
  restringe la lectura).
- Atleta pedido que no es participante de la competencia → lista vacía `[]`,
  no 400.
- Respuesta envuelta `{data: [...]}` simple, SIN `has_unseen` (el unseen de
  competencias es por team y lo cubre el listado).
- Cada ítem, ordenado por `id` ASC (cronológico: más viejo primero = intento 1
  primero):
  - `id`: id de la runner_session (intento).
  - `session_instance_id`: instancia de ese intento (compartida en intento 1,
    nueva en 2+).
  - `status`: wip / finished / interrupted.
  - `started_at`: el instante de inicio del intento (fuente
    `runner_session.start_date`).
  - `ended_at`: el instante de fin, null si el intento está wip (fuente
    `runner_session.end_date`).
  - `distance_meters`: SUM de los distance_meters del feedback activo (series)
    del atleta sobre la instancia del intento; null si no hay feedback.
  - `compliance_ratio`: matching Haversine on-demand (mismo cálculo del
    detalle/close: samples de la ruta + radio de la competencia), null salvo
    intento finished (0 si finished pero sin puntos GPS).
  - `duration_ms`: end-start server-side en ms, null si el intento está wip.

#### Scenario: Listar los propios intentos (sin athlete_user_id)

- **WHEN** un participante pide `GET /competitions/{id}/attempts` sin query
- **THEN** responde `200` con `{data: [...]}` conteniendo SOLO sus intentos,
  ordenados por id ASC.

#### Scenario: Listar los intentos de otro atleta

- **WHEN** un miembro del team pide los intentos del participante X vía
  `?athlete_user_id=X`
- **THEN** responde `200` con `{data: [...]}` conteniendo todos los intentos de
  X en la competencia (cualquiera puede ver los de otro).

#### Scenario: Atleta no-participante

- **WHEN** el caller (miembro del team) pide los intentos de un usuario que no
  es participante de la competencia
- **THEN** responde `200` con `{data: []}` (lista vacía, no 400).

#### Scenario: 404 antes de 403

- **WHEN** el id de competencia no existe
- **THEN** responde `404` aunque el caller tampoco sea miembro del team.

#### Scenario: Caller ajeno al team

- **WHEN** un usuario autenticado que no es miembro de ningún grupo del team ni
  owner del mismo pide los intentos de una competencia existente
- **THEN** responde `403`.

#### Scenario: Reglas de null del ítem

- **WHEN** un intento está wip sin feedback
- **THEN** su ítem trae `ended_at: null`, `duration_ms: null`,
  `distance_meters: null`, `compliance_ratio: null`; y un intento finished
  trae esos 4 campos con valores (0 si finished sin puntos GPS).

### Requirement: Progreso y winners por mejor intento (route)

El sistema DEBE cumplir lo siguiente (MUST):

> Reemplaza la versión route de "Cierre manual y winners" (de
> `competencias-ruta-intake` para matching/empates; el resto del cierre queda
> intacto) y el shape de progreso route de ese mismo change. La doc previa que
> decía "value null para route" queda corregida en este change.

- El `progress` del detalle de una competencia `type=route` tiene shape
  `{user_id, compliance_ratio, duration_ms, session_instance_id}`:
  el MEJOR intento por atleta. `compliance_ratio` es el pct 0-100 del matching
  (mismo valor que hoy viaja en `value`); `duration_ms` su duración;
  `session_instance_id` la instancia de ese mejor intento. Orden por `user_id`
  ASC.
- El `progress` del `type=metric` SE MANTIENE byte-idéntico:
  `{user_id, value}` (value puede ser null si la persona nunca reportó).
- En el cierre route (`computeWinners`), ANTES de comparar entre atletas, los
  intentos de cada atleta se reducen a su MEJOR intento: max pct de match;
  empate exacto de pct → menor duración. Recién después se comparan los mejores
  intentos entre atletas: ganan TODOS los atletas cuyo mejor intento empata
  exacto en (pct, duración) con lo mejor. Un mismo atleta nunca aparece
  duplicado en `winner_user_ids`.
- Sin intentos → cierre igual que hoy: `finished` y `winner_user_ids` vacía.

#### Scenario: progress route con el nuevo shape

- **WHEN** el front consulta `GET /competitions/{id}` de una competencia route
  con participantes que ya hicieron intentos
- **THEN** cada ítem de `progress` trae `{user_id, compliance_ratio,
  duration_ms, session_instance_id}` (el mejor intento de cada uno) y NO trae
  `value`.

#### Scenario: progress metric intacto

- **WHEN** el front consulta el detalle de una competencia metric
- **THEN** cada ítem de `progress` sigue trayendo `{user_id, value}`
  (value null si esa persona no reportó nada).

#### Scenario: Winners reducidos por mejor intento

- **WHEN** un atleta tiene 2 intentos finished (pct 100 y 60) y otro atleta
  tiene 1 intento de pct 100 al cerrar
- **THEN** `winner_user_ids` contiene a ambos atletas (una vez cada uno); el
  intento de 60 del primero no gana ni duplica.

#### Scenario: Empate intra-atleta por duración

- **WHEN** dos atletas tienen su mejor intento en pct 100 exactos y el primero
  completó en menos duración
- **THEN** solo el de menor duración gana.

#### Scenario: Empate exacto entre atletas

- **WHEN** los mejores intentos de dos atletas empatan exactos en pct y
  duración
- **THEN** ambos ganan (todos los empatados ganan), una vez cada uno.

### Requirement: Estado de intentos en CompetitionResponse (max_attempts / attempts_used / can_attempt)

El sistema DEBE cumplir lo siguiente (MUST):

- `CompetitionResponse` (listado y detalle, mismo shape) suma 3 campos:
  - `max_attempts`: NULL = sin límite; null para metric.
  - `attempts_used`: cantidad de intentos del CALLER en la competencia
    (cualquier status); 0 si el caller es participante sin intentos; null si la
    competencia no es route o el caller no es participante (null = no
    aplicable, NO 0).
  - `can_attempt`: bool SIEMPRE presente: true solo si
    `type=route && status=active && caller participante && (max_attempts NULL
    || attempts_used < max_attempts)`; false en todos los demás casos
    (metric, no-participante, competencia no active, límite cumplido).
- Es un cálculo server-side (no derivable solo del cliente cuando el caller no
  es el dueño de los datos), batch sin N+1 en el listado.

#### Scenario: Participante route active sin intentos

- **WHEN** un participante consulta el detalle (o el listado incluye su
  competencia) de una competencia route ACTIVE con `max_attempts = 3` y él
  todavía no registró intentos
- **THEN** la respuesta trae `attempts_used: 0` y `can_attempt: true`.

#### Scenario: Límite cumplido

- **WHEN** un participante de una competencia route active con
  `max_attempts = 2` ya registra 2 intentos
- **THEN** la respuesta trae `attempts_used: 2` y `can_attempt: false`.

#### Scenario: Caller no participante (o metric)

- **WHEN** el caller NO está en competition_participants (o la competencia es
  metric)
- **THEN** la respuesta trae `attempts_used: null` y `can_attempt: false`
  (bool presente siempre, max_attempts null para metric).

#### Scenario: can_attempt mirror del límite en listado

- **WHEN** el listado de un team incluye la competencia route del caller
- **THEN** el ítem del listado trae los 3 campos con la misma regla
  (attempts_used/count/can_attempt del caller, calculados en batch).
