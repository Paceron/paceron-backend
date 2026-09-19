### Requirement: El sistema SHALL persistir el feedback de entrenamiento con la estructura definida

El sistema SHALL persistir feedback de entrenamiento en la tabla `workout_feedback` con las siguientes columnas según la spec del change: `id` (BIGSERIAL PK), `team_id` (nullable), `assigned_session_id` (NOT NULL), `assigned_exercise_id` (NOT NULL), `athlete_user_id` (NOT NULL), `feedback_owner_user_id` (NOT NULL), `report_source` (NOT NULL), `session_date` (DATE NOT NULL), `set_number` (INTEGER default 0), `started_at`/`ended_at` (TIMESTAMPTZ nullables), `duration_ms`/`active_duration_ms` (BIGINT nullables), `weight_kg` (FLOAT nullable), `reps` (INTEGER nullable), `distance_meters` (FLOAT nullable), `rpe` (SMALLINT nullable con CHECK 1..10), `avg_heart_rate`/`max_heart_rate` (SMALLINT nullables), `completion_status` (VARCHAR(20) nullable), `elevation_gain_meters` (FLOAT nullable), `cadence` (SMALLINT nullable), `annotations` (TEXT nullable), `media_urls` (TEXT[] nullable), `created_at`/`updated_at` (TIMESTAMPTZ DEFAULT NOW()), `deleted_at` (TIMESTAMPTZ nullable, baja lógica). `route_summary` NO se incluye en este change (depende de PostGIS, postergado). La tabla SHALL definirse con los índices `idx_feedback_team_date (team_id, session_date)`, `idx_feedback_athlete_date (athlete_user_id, session_date)`, `idx_feedback_session_exercise (assigned_session_id, assigned_exercise_id, set_number)` y un índice único parcial `unique_feedback_per_set` sobre `(assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, set_number)` con `WHERE deleted_at IS NULL`.

#### Scenario: La migración crea la tabla con los índices
- **WHEN** se corre la migración (AutoMigrate + SQL de inicialización)
- **THEN** existe la tabla `workout_feedback` con las columnas de la spec, los tres índices de búsqueda y el índice único parcial

#### Scenario: La restricción de RPE se aplica a nivel de base de datos
- **WHEN** se persiste un feedback con `rpe` fuera del rango 1..10
- **THEN** la base de datos rechaza la operación

### Requirement: El sistema SHALL validar los datos del feedback antes de persistir

El sistema SHALL validar el body de creación y de edición de feedback. `assigned_session_id` y `assigned_exercise_id` SHALL ser obligatorios y mayores a 0. `session_date` SHALL ser obligatoria con formato `YYYY-MM-DD`. `team_id` SHALL ser opcional y, si se envía, mayor a 0. `set_number` SHALL ser opcional (default 0) y mayor o igual a 0. `report_source` SHALL ser obligatorio y no vacío. Toda métrica numérica opcional (`weight_kg`, `reps`, `distance_meters`, `duration_ms`, `active_duration_ms`, `elevation_gain_meters`, `cadence`, `avg_heart_rate`, `max_heart_rate`) SHALL ser mayor o igual a 0 si se envía, y `rpe` SHALL estar entre 1 y 10. `media_urls` SHALL ser un array opcional de strings.

#### Scenario: Faltan campos obligatorios
- **WHEN** se envía el body sin `assigned_session_id`, `assigned_exercise_id`, `session_date` o `report_source`
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: ID menor o igual a cero
- **WHEN** se envía `assigned_session_id`, `assigned_exercise_id` o `team_id` con valor menor o igual a 0
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Fecha inválida
- **WHEN** se envía `session_date` con formato distinto a `YYYY-MM-DD`
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Métrica numérica negativa
- **WHEN** se envía una métrica numérica (ej. `duration_ms`) con valor negativo
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: RPE fuera de rango
- **WHEN** se envía `rpe` con valor 0 u 11
- **THEN** el sistema responde `400 Bad Request`

### Requirement: El sistema SHALL crear feedback con autorización del reportante

El sistema SHALL exponer `POST /api/v1/workout-feedback` (detrás del `AuthMiddleware`) que crea un feedback. `feedback_owner_user_id` SHALL setearse siempre con el `auth_user_id` extraído del token (el reportante real, nunca un valor del body). `athlete_user_id` SHALL por defecto ser el `auth_user_id`; si el body envía un `athlete_user_id` diferente, el sistema SHALL permitirlo solo si el `auth_user_id` es el owner de al menos un equipo al que pertenece el atleta (misma relación que valida la búsqueda de asistencias); de lo contrario responde `403 Forbidden`. Si el mismo conjunto activo `(assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, set_number)` ya existe (no soft-deleted), el insert viola el índice único y el sistema SHALL responder `409 Conflict`.

#### Scenario: Creación exitosa
- **WHEN** un usuario autenticado envía `POST /api/v1/workout-feedback` con datos válidos y sin `athlete_user_id`
- **THEN** el sistema inserta el feedback con `feedback_owner_user_id` = `athlete_user_id` = `auth_user_id` y responde `201 Created` con el feedback persistido

#### Scenario: Entrenador reporta por un atleta de un equipo propio
- **WHEN** un entrenador owner de un equipo envía `POST /api/v1/workout-feedback` con un `athlete_user_id` de un corredor de un equipo que administra
- **THEN** el sistema inserta el feedback con `feedback_owner_user_id` = id del entrenador y responde `201 Created`

#### Scenario: Reporte por un atleta que no pertenece a un equipo del reportante
- **WHEN** un usuario envía `POST /api/v1/workout-feedback` con un `athlete_user_id` distinto al suyo y no es owner de un equipo al que pertenezca ese atleta
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Feedback duplicado del mismo set activo
- **WHEN** se intenta crear un feedback cuyo conjunto `(assigned_session_id, assigned_exercise_id, athlete_user_id, feedback_owner_user_id, set_number)` ya existe y no está soft-deleted
- **THEN** el sistema responde `409 Conflict`

#### Scenario: Creación para el propio set ya soft-deleteado
- **WHEN** se intenta crear un feedback cuyo conjunto ya existió pero fue soft-deleteado
- **THEN** el sistema inserta el feedback y responde `201 Created` (el índice único parcial no lo bloquea)

#### Scenario: Creación sin autenticación
- **WHEN** se envía `POST /api/v1/workout-feedback` sin header `Authorization` válido
- **THEN** el sistema responde `401 Unauthorized`

### Requirement: El sistema SHALL devolver feedback por id con autorización

El sistema SHALL exponer `GET /api/v1/workout-feedback/:id` (detrás del `AuthMiddleware`) que devuelve el feedback con ese id si no está soft-deleteado. El acceso SHALL permitirse si el `auth_user_id` es el `feedback_owner_user_id`, el `athlete_user_id`, o el owner del equipo del feedback. Si el feedback no existe o está soft-deleteado, responde `404 Not Found`. Si existe pero el usuario no tiene ninguno de los accesos, responde `403 Forbidden`.

#### Scenario: El atleta consulta su propio feedback
- **WHEN** un atleta autenticado consulta `GET /api/v1/workout-feedback/:id` de un feedback donde es el `athlete_user_id`
- **THEN** el sistema responde `200 OK` con el feedback completo

#### Scenario: El entrenador owner consulta el feedback de un corredor de su equipo
- **WHEN** el owner del equipo consulta `GET /api/v1/workout-feedback/:id` de un feedback con `team_id` de su equipo
- **THEN** el sistema responde `200 OK` con el feedback completo

#### Scenario: Feedback inexistente o soft-deleteado
- **WHEN** se consulta `GET /api/v1/workout-feedback/:id` de un id que no existe o fue soft-deleteado
- **THEN** el sistema responde `404 Not Found`

#### Scenario: Acceso a feedback ajeno sin autorización
- **WHEN** un usuario consulta `GET /api/v1/workout-feedback/:id` de un feedback del que no es ni atleta, ni reportante, ni owner del equipo
- **THEN** el sistema responde `403 Forbidden`

### Requirement: El sistema SHALL buscar feedback con filtros y matriz de autorización

El sistema SHALL exponer `GET /api/v1/workout-feedback/search` (detrás del `AuthMiddleware`) con los query params opcionales `team_id`, `athlete_user_id`, `assigned_session_id`, `assigned_exercise_id`, `feedback_owner_user_id`, `session_date_from` y `session_date_to` (rango inclusive de `session_date`). Todo id SHALL ser mayor a 0 y toda fecha SHALL tener formato `YYYY-MM-DD`; de lo contrario `400 Bad Request`. Los resultados SHALL excluir feedbacks soft-deleteados. La matriz de autorización SHALL evaluarse contra el `auth_user_id`:
- Sin parámetros → scope del usuario autenticado (`athlete_user_id = auth_user_id` o `feedback_owner_user_id = auth_user_id`).
- `athlete_user_id` propio o `feedback_owner_user_id` propio → scope correspondiente, sin restricciones de equipo.
- `team_id` → el `auth_user_id` SHALL ser el owner de ese team (404 si el team no existe, 403 si no es owner). Los resultados SHALL filtrarse por ese team.
- `athlete_user_id` ajeno → el `auth_user_id` SHALL ser owner de al menos un equipo al que pertenezca ese atleta (403 si no hay relación), y los resultados SHALL filtrarse para ese atleta.

#### Scenario: Búsqueda sin parámetros
- **WHEN** un usuario autenticado envía `GET /api/v1/workout-feedback/search` sin query params
- **THEN** el sistema responde `200 OK` con los feedbacks donde el usuario es `athlete_user_id` o `feedback_owner_user_id`

#### Scenario: Búsqueda como owner por equipo
- **WHEN** el owner de un team envía `GET /api/v1/workout-feedback/search?team_id=5`
- **THEN** el sistema responde `200 OK` con los feedbacks de ese team (no soft-deleteados)

#### Scenario: Búsqueda por team siendo no-owner
- **WHEN** un usuario no-owner envía `GET /api/v1/workout-feedback/search?team_id=5`
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Búsqueda con team inexistente
- **WHEN** se envía `GET /api/v1/workout-feedback/search?team_id=999` y ese team no existe
- **THEN** el sistema responde `404 Not Found`

#### Scenario: Búsqueda del feedback de un atleta ajeno
- **WHEN** un usuario envía `GET /api/v1/workout-feedback/search?athlete_user_id=42` y el usuario 42 pertenece a un equipo del que es owner
- **THEN** el sistema responde `200 OK` con los feedbacks del atleta 42

#### Scenario: Búsqueda del feedback de un atleta sin relación
- **WHEN** un usuario envía `GET /api/v1/workout-feedback/search?athlete_user_id=99` y el usuario 99 no pertenece a ningún equipo del que el autenticado sea owner
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Filtro de rango de fechas
- **WHEN** se envía `GET /api/v1/workout-feedback/search?session_date_from=2026-01-01&session_date_to=2026-01-31`
- **THEN** el sistema responde `200 OK` con solo los feedbacks dentro del rango inclusive (dentro del scope autorizado)

#### Scenario: Id de filtro menor o igual a cero
- **WHEN** se envía `GET /api/v1/workout-feedback/search?team_id=0` o un `athlete_user_id` inválido
- **THEN** el sistema responde `400 Bad Request`

### Requirement: El sistema SHALL actualizar feedback con autorización

El sistema SHALL exponer `PUT /api/v1/workout-feedback/:id` (detrás del `AuthMiddleware`) que actualiza parcialmente el feedback (solo los campos enviados en el body, mismas validaciones que en el create) y actualiza `updated_at`. Si el feedback no existe o está soft-deleteado, responde `404 Not Found`. La edición SHALL permitirse únicamente si el `auth_user_id` es el `feedback_owner_user_id` (el reportante del registro) o el owner de un equipo que corresponda al feedback (y además pueda crear feedback para ese athlet según la matriz); de lo contrario `403 Forbidden`. `feedback_owner_user_id` y `athlete_user_id` no se modifican por este endpoint.

#### Scenario: Edición exitosa del reportante
- **WHEN** el `feedback_owner_user_id` envía `PUT /api/v1/workout-feedback/:id` con campos válidos
- **THEN** el sistema actualiza los campos enviados y responde `200 OK` con el feedback actualizado

#### Scenario: Edición por el entrenador owner
- **WHEN** el owner del equipo del feedback envía `PUT /api/v1/workout-feedback/:id`
- **THEN** el sistema actualiza los campos y responde `200 OK`

#### Scenario: Edición sin permiso
- **WHEN** un usuario que no es el reportante ni el owner del equipo envía `PUT /api/v1/workout-feedback/:id`
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Edición de feedback inexistente o soft-deleteado
- **WHEN** se envía `PUT /api/v1/workout-feedback/:id` de un id que no existe o fue soft-deleteado
- **THEN** el sistema responde `404 Not Found`

### Requirement: El sistema SHALL dar de baja feedback con baja lógica

El sistema SHALL exponer `DELETE /api/v1/workout-feedback/:id` (detrás del `AuthMiddleware`) que aplica baja lógica seteando `deleted_at` (nunca borrado físico). Si el feedback no existe o ya está soft-deleteado, responde `404 Not Found`. La baja SHALL permitirse solo si el `auth_user_id` es el `feedback_owner_user_id` o el owner del equipo del feedback; de lo contrario `403 Forbidden`. El feedback soft-deleteado queda oculto de get, search y update.

#### Scenario: Baja lógica exitosa
- **WHEN** el `feedback_owner_user_id` envía `DELETE /api/v1/workout-feedback/:id`
- **THEN** el sistema setea `deleted_at` y responde `204 No Content`

#### Scenario: Baja por el entrenador owner
- **WHEN** el owner del equipo envía `DELETE /api/v1/workout-feedback/:id`
- **THEN** el sistema setea `deleted_at` y responde `204 No Content`

#### Scenario: Baja sin permiso
- **WHEN** un usuario que no es el reportante ni el owner del equipo envía `DELETE /api/v1/workout-feedback/:id`
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Baja de feedback ya soft-deleteado o inexistente
- **WHEN** se envía `DELETE /api/v1/workout-feedback/:id` de un id inexistente o ya eliminado
- **THEN** el sistema responde `404 Not Found`

#### Scenario: El feedback con baja lógica no aparece en búsquedas
- **WHEN** un feedback fue soft-deleteado y luego se busca sin filtros
- **THEN** el feedback eliminado no aparece en los resultados