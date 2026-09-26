# Spec — session-registration-review

## Requirement: El sistema SHALL persistir el estado de sesión del corredor

El sistema SHALL persistir en la tabla `runner_session` el estado de una sesión por atleta con las
columnas: `id` (BIGSERIAL PK), `session_instance_id` (BIGINT NOT NULL, FK opaca a
`session_instances` — sin constraint, consistente con `workout_feedback`), `athlete_user_id`
(BIGINT NOT NULL, usuario atleta), `status` (TEXT NOT NULL DEFAULT `'wip'`, valores `'wip'` o
`'finished'`), `start_date` (TIMESTAMPTZ NOT NULL), `end_date` (TIMESTAMPTZ NULL), `created_at`
(TIMESTAMPTZ NOT NULL DEFAULT NOW()) y `updated_at` (TIMESTAMPTZ NOT NULL). La tabla SHALL tener un
constraint único sobre `(session_instance_id, athlete_user_id)`: un corredor tiene un solo estado
por sesión asignada, y dos atletas pueden compartir la misma sesión (día de grupo).

### Scenario: La migración crea la tabla con el constraint único

- **WHEN** se corre la migración (AutoMigrate + SQL de inicialización)
- **THEN** existe la tabla `runner_session` con las columnas de la spec y el constraint único sobre
  `(session_instance_id, athlete_user_id)`

## Requirement: El sistema SHALL crear el estado de forma idempotente

El sistema SHALL exponer `POST /api/v1/session-instances/:id/runner` con body
`{ "athlete_user_id"?, "start_date" }` (atleta default = usuario autenticado; `start_date`
obligatorio, RFC3339). El endpoint SHALL crear la fila en `wip` o, si ya existe, responder `200`
con el estado **actual** sin pisar `start_date` ni bajar de `finished` a `wip` (INSERT
`ON CONFLICT DO NOTHING`). SHALL rechazar (400) `start_date` no parseable o `id` de sesión inválido.
SHALL responder `404` si la sesión no existe. SHALL responder `403` si `athlete_user_id` viene y es
ajeno y el auth no es owner de un equipo al que pertenezca ese atleta.

### Scenario: Alta del estado en el primer Play

- **WHEN** un usuario hace POST `:id/runner` con un `start_date` válido y sin atleta
- **THEN** se crea la fila en `wip` y se responde 201 con el estado creado

### Scenario: Reintento del mismo POST (idempotencia)

- **WHEN** se repite el POST sobre la misma sesión y atleta (ya existía la fila)
- **THEN** no se duplica fila y se responde 200 con el estado actual existente (mismo `start_date`,
  sin bajar de `finished` a `wip`)

### Scenario: Sesión asignada inexistente

- **WHEN** se hace POST sobre un id de sesión inexistente
- **THEN** se responde 404

### Scenario: Entrenador creando para un atleta de su equipo

- **WHEN** el auth es owner de un equipo al que pertenece el atleta e indica su
  `athlete_user_id`
- **THEN** se crea/retorna el estado de ese atleta (201 o 200 idempotente)

### Scenario: Atleta ajeno sin relación de equipo

- **WHEN** `athlete_user_id` es ajeno y el auth no es owner de un equipo del atleta
- **THEN** se responde 403

## Requirement: El sistema SHALL marcar la sesión como finalizada

El sistema SHALL exponer `PATCH /api/v1/session-instances/:id/runner` con body
`{ "status": "finished" }` (validando que sea exactamente `finished`; 400 en otro valor). El
endpoint SHALL pasar la fila a `finished` y setear `end_date` con el `now()` del servidor **solo si**
estaba en `wip`; si ya estaba `finished` SHALL responder `200` con el estado actual sin cambios
(idempotente, no reescribe `end_date`). SHALL responder `404` si no existe la fila para
(sesión, atleta). Misma autorización (atleta dueño o owner de un equipo del atleta).

### Scenario: Finalización desde `wip`

- **WHEN** se hace PATCH `finished` sobre una fila en `wip`
- **THEN** la fila pasa a `finished` con `end_date` seteada por el servidor y se responde 200

### Scenario: Repetir finished ya finalizada

- **WHEN** se hace PATCH `finished` sobre una fila ya en `finished`
- **THEN** se responde 200 sin cambios (misma `end_date`)

### Scenario: Sin fila creada

- **WHEN** se hace PATCH sobre una sesión/atleta sin `runner_session`
- **THEN** se responde 404

## Requirement: El sistema SHALL exponer el estado de la sesión

El sistema SHALL exponer `GET /api/v1/session-instances/:id/runner` con query opcional
`athlete_user_id` (default = usuario autenticado) que devuelve `200 { data: { id,
session_instance_id, athlete_user_id, status, start_date, end_date } }`, o `404` si no existe la
fila. Misma autorización (atleta dueño o owner de un equipo del atleta).

### Scenario: Consulta del propio corredor

- **WHEN** el corredor consulta su estado (sin `athlete_user_id`)
- **THEN** se responde el estado actual de su fila

### Scenario: Consulta del entrenador por atleta

- **WHEN** el auth, owner de un equipo del atleta, consulta con `athlete_user_id`
- **THEN** se responde el estado actual de ese atleta

### Scenario: Sin fila

- **WHEN** no existe `runner_session` para esa sesión/atleta
- **THEN** se responde 404

## Requirement: El sistema SHALL listar el feedback de una sesión

El sistema SHALL exponer `GET /api/v1/session-instances/:id/feedback` con query opcional
`athlete_user_id` (default = self) que devuelve `200 { data: [ WorkoutFeedbackResponse ] }` con los
`workout_feedback` activos cuyo `assigned_session_id = :id` (más el atleta elegido), ordenados por
`(assigned_exercise_id, set_number)`. Misma autorización. El frontend agrupa por ejercicio contra
el shape del `session_instance` (que ya trae los nombres de ejercicios), por lo que el endpoint no
enriquece con `exercise_name` (sin join).

### Scenario: Listado autorizado con datos

- **WHEN** un corredor (o trainer autorizado) consulta el feedback de una sesión con filas
- **THEN** se responden los feedbacks activos ordenados por `(assigned_exercise_id, set_number)` en
  `data` (una fila por serie; las series sin feedback simplemente no aparecen)

### Scenario: Sesión sin feedback

- **WHEN** la sesión no tiene feedbacks activos
- **THEN** se responde 200 con `data: []`

### Scenario: No autorizado

- **WHEN** el usuario no es atleta dueño ni owner de un equipo del atleta consultado
- **THEN** se responde 403

## Requirement: El alta manual reusa el POST de feedback existente

El sistema SHALL NO agregar un endpoint nuevo de creación de feedback: el alta manual de una serie
en la pantalla de registro usa el `POST /api/v1/workout-feedback` existente (idempotente vía
`unique_feedback_per_set`, 409 → el cliente convierte a `PUT`).

### Scenario: Alta manual de una serie

- **WHEN** la pantalla de registro manual crea un feedback de una serie
- **THEN** se usa el `POST /api/v1/workout-feedback` existente, sin ruta nueva en este change