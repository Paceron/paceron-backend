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

El sistema SHALL exponer el endpoint `GET /api/v1/attendance/search` (detrás del `AuthMiddleware`). Al menos un query param SHALL ser obligatorio y `team_id` SHALL ser obligatorio; de lo contrario el sistema responde `400 Bad Request`. `training_session_id` y `user_id` son opcionales. Si se envía cualquiera de ellos, su valor SHALL ser estrictamente mayor a 0; de lo contrario el sistema responde `400 Bad Request`.

#### Scenario: Búsqueda sin query params
- **WHEN** se envía `GET /api/v1/attendance/search` sin ningún query param
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Búsqueda sin team_id
- **WHEN** se envía `GET /api/v1/attendance/search?training_session_id=5` (u otro param) sin `team_id`
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Parámetro con valor menor o igual a cero
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=0`
- **THEN** el sistema responde `400 Bad Request`

#### Scenario: Respuesta de búsqueda exitosa
- **WHEN** una búsqueda válida devuelve resultados
- **THEN** el sistema responde `200 OK` con `{ "data": [ { "id": 1, "team_id": 1, "training_session_id": 1, "user_id": 1, "created_at": "..." } ] }`

### Requirement: El sistema SHALL aplicar la matriz de autorización a la búsqueda de asistencias

El sistema SHALL evaluar la búsqueda de `GET /api/v1/attendance/search` contra el `auth_user_id` extraído del token según la siguiente matriz estricta. El `auth_user_id` SHALL pertenecer al team como entrenador o corredor (un usuario pertenece a un team cuando existe un registro activo en `team_users`; el rol entrenador corresponde al owner del team). Si no es entrenador ni corredor del team, el sistema responde `403 Forbidden`.

- Si el `auth_user_id` es **entrenador** del team, SHALL ver las asistencias de todo el equipo (aplicando `training_session_id` y opcionalmente `user_id` si vienen).
- Si el `auth_user_id` es **corredor** del team, SHALL ver solo sus propias asistencias: la query a la base de datos SHALL forzar `user_id = auth_user_id`, y un `user_id` ajeno en la request responde `403 Forbidden`.

#### Scenario: Búsqueda como entrenador por team
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T` y el `auth_user_id` es entrenador del team T
- **THEN** el sistema busca las asistencias del team T (aplicando `training_session_id` si viene) y responde `200 OK`

#### Scenario: Búsqueda como entrenador filtrando por corredor
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T&user_id=X` y el `auth_user_id` es entrenador del team T
- **THEN** el sistema busca las asistencias de X dentro del team T y responde `200 OK`

#### Scenario: Búsqueda como corredor por team
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T` y el `auth_user_id` es corredor del team T
- **THEN** el sistema busca solo las asistencias del propio `auth_user_id` (con `user_id` forzado al token en la query) y responde `200 OK`

#### Scenario: Búsqueda como corredor pidiendo otro user_id
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T&user_id=X` con X distinto del `auth_user_id`, siendo el `auth_user_id` corredor del team T
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Búsqueda como no miembro del team
- **WHEN** se envía `GET /api/v1/attendance/search?team_id=T` y el `auth_user_id` no es ni entrenador ni corredor del team T
- **THEN** el sistema responde `403 Forbidden`

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