## ADDED Requirements

### Requirement: El sistema SHALL generar un QR determinista para una sesión de entrenamiento

El sistema SHALL exponer el endpoint `GET /api/v1/attendance/qr` (detrás del `AuthMiddleware`) que recibe los query params `team_id` y `training_session_id` (ambos obligatorios) y genera un código QR. El algoritmo de generación SHALL ser determinista: mismos inputs producen exactamente el mismo output binario/PNG. El QR SHALL codificar la URL `<urlbase>/api/v1/attendance/team/{team_id}/session/{training_session_id}`. La respuesta SHALL incluir el QR en base64 y la URL codificada.

#### Scenario: QR generado correctamente
- **WHEN** un usuario autenticado envía `GET /api/v1/attendance/qr?team_id=5&training_session_id=9`
- **THEN** el sistema responde `200 OK` con un body que incluye `qr_code_base64` (imagen PNG en base64) y `url_encoded` con la URL que codifica el QR

#### Scenario: El QR es determinista
- **WHEN** se genera el QR dos veces con los mismos `team_id` y `training_session_id`
- **THEN** el `qr_code_base64` resultante es idéntico en ambas respuestas

#### Scenario: Faltan parámetros obligatorios
- **WHEN** se envía `GET /api/v1/attendance/qr` sin `team_id` o sin `training_session_id`
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Parámetro no válido en la generación del QR
- **WHEN** se envía `GET /api/v1/attendance/qr` con `team_id` o `training_session_id` menor o igual a 0
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Acceso sin autenticación
- **WHEN** se envía `GET /api/v1/attendance/qr` sin header `Authorization` válido
- **THEN** el sistema responde `401 Unauthorized`

### Requirement: El sistema SHALL registrar asistencias de forma idempotente

El sistema SHALL exponer el endpoint `POST /api/v1/attendance/team/{team_id}/session/{training_session_id}` (detrás del `AuthMiddleware`) que registra la asistencia del usuario autenticado (user_id extraído del token) a la sesión del team. El insert SHALL hacerse directamente contra la base de datos atrapando la violación de la constraint UNIQUE `(team_id, training_session_id, user_id)` — sin `SELECT` previo de verificación — para evitar race conditions.

#### Scenario: Primera asistencia
- **WHEN** un usuario autenticado envía `POST /api/v1/attendance/team/5/session/9` y no existe una asistencia previa del usuario para ese team y sesión
- **THEN** el sistema inserta el registro y responde `201 Created` con `{ "message": "asistencia registrada" }`

#### Scenario: Asistencia duplicada
- **WHEN** un usuario autenticado envía `POST /api/v1/attendance/team/5/session/9` y ya existe una asistencia del usuario para ese team y sesión
- **THEN** el sistema NO inserta un registro duplicado y responde `200 OK` con `{ "message": "esta asistencia fue previamente registrada" }`

#### Scenario: Registro sin autenticación
- **WHEN** se envía `POST /api/v1/attendance/team/5/session/9` sin header `Authorization` válido
- **THEN** el sistema responde `401 Unauthorized`

### Requirement: El sistema SHALL validar los parámetros del endpoint de búsqueda

El sistema SHALL exponer el endpoint `GET /api/v1/attendance/search` (detrás del `AuthMiddleware`) con los query params opcionales `team_id`, `training_session_id` y `user_id`. Si se envía cualquiera de ellos, su valor SHALL ser estrictamente mayor a 0; de lo contrario el sistema responde `400 Bad Request`.

#### Scenario: Parámetro con valor menor o igual a cero
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=0`
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Respuesta de búsqueda exitosa
- **WHEN** una búsqueda válida devuelve resultados
- **THEN** el sistema responde `200 OK` con `{ "data": [ { "id": 1, "team_id": 1, "training_session_id": 1, "user_id": 1, "created_at": "..." } ] }`

### Requirement: El sistema SHALL aplicar la matriz de autorización a la búsqueda de asistencias

El sistema SHALL evaluar los parámetros de `GET /api/v1/attendance/search` contra el `auth_user_id` extraído del token según la siguiente matriz estricta. `auth_user_id` es el owner de un team cuando es igual al `teams.owner_id` de ese team; un usuario pertenece a un team cuando existe un registro activo en `team_users`.

#### Scenario: Búsqueda sin parámetros
- **WHEN** se envía `GET /api/v1/attendance/search` sin ningún query param
- **THEN** el sistema busca las asistencias del propio `auth_user_id` y responde `200 OK`

#### Scenario: Búsqueda con user_id propio
- **WHEN** se envía `GET /api/v1/attendance/search?user_id={auth_user_id}`
- **THEN** el sistema busca las asistencias de ese user_id y responde `200 OK`

#### Scenario: Búsqueda con user_id ajeno
- **WHEN** se envía `GET /api/v1/attendance/search?user_id=X` donde X es distinto del `auth_user_id`
- **THEN** el sistema responde `403 Forbidden` a menos que el `auth_user_id` sea owner de al menos un equipo al que pertenezca el usuario X

#### Scenario: Búsqueda como owner por team
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T` y el `auth_user_id` es el owner del team T
- **THEN** el sistema busca las asistencias del team T (aplicando `training_session_id` si viene) y responde `200 OK`

#### Scenario: Búsqueda como no-owner por team
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T` y el `auth_user_id` NO es el owner del team T
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Búsqueda de un corredor que pertenece a un equipo del owner
- **WHEN** se envía `GET /api/v1/attendance/search?user_id=X` con X distinto del `auth_user_id`, y X pertenece a un team cuyo owner es el `auth_user_id`
- **THEN** el sistema busca las asistencias de X (aplicando `team_id`/`training_session_id` si vienen) y responde `200 OK`

#### Scenario: Búsqueda con team inexistente
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T` donde el team T no existe en la base de datos
- **THEN** el sistema responde `404 Not Found`

### Requirement: El sistema SHALL persistir las asistencias con índices optimizados

El sistema SHALL almacenar las asistencias en la tabla `attendances` con las siguientes columnas: `id`, `team_id`, `training_session_id`, `user_id`, `created_at`, `updated_at`. La tabla SHALL contar con la constraint `UNIQUE (team_id, training_session_id, user_id)` que garantiza la regla de negocio de no duplicados, y con los índices `(team_id, training_session_id)` y `(user_id, team_id)` para optimizar las búsquedas. `training_session_id` SHALL ser una FK opaca (columna NUMÉRICA mayor a 0 sin constraint a la tabla `training_sessions`, que aún no existe).

#### Scenario: El registro persiste la asistencia
- **WHEN** se registra una asistencia exitosamente
- **THEN** la fila queda persistida en `attendances` con `team_id`, `training_session_id` y `user_id` correctos

#### Scenario: La constraint UNIQUE rechaza duplicados
- **WHEN** se intenta insertar una segunda fila con el mismo `(team_id, training_session_id, user_id)`
- **THEN** la base de datos rechaza el insert por violación de la constraint UNIQUE y el sistema responde como asistencia ya registrada