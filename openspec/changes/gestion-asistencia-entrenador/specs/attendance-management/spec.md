# Spec Delta

## Purpose

Permite que el entrenador de un equipo gestione la asistencia de las sesiones
presenciales de sus grupos desde el panel web o la app: ver qué sesiones ya
ocurrieron, cargar y corregir la asistencia de los corredores en una sola
operación, y emitir el QR de la sesión.

## ADDED Requirements

> **Nota sobre el tipo de delta.** La capability `attendance-qr` (change
> `add-qr-attendance`) implementó los endpoints de QR, alta y búsqueda, pero
> **nunca fue archivada** a `openspec/specs/`. Por el precedente de
> `AGENTS.md`, los requirements de esos endpoints que este change endurece se
> redeclaran **completos** como `ADDED` acá (no como `MODIFIED`), en lugar de
> referenciar una spec que el archive no podría resolver.

### Requirement: El sistema SHALL listar las sesiones presenciales ya ocurridas de un grupo

El sistema SHALL exponer `GET /api/v1/groups/{group_id}/attendance-sessions?team_id={team_id}` (en el router el path param se llama `id`, por la convención que ya usan las demás rutas de grupos; el nombre es interno, la URL es la misma) (detrás del `AuthMiddleware`) para que el entrenador pueda elegir sobre qué sesión gestionar la asistencia.

El listado SHALL incluir exclusivamente los `GroupCalendarDay` del grupo que cumplen **las tres** condiciones: `kind = 'training'`, `is_presencial = true` y `date <= hoy` (fecha local del servidor, inclusiva). Los días con `session_instance_id IS NULL` SHALL quedar excluidos, así como los días `cancelled` y los no presenciales. Los resultados SHALL ordenarse por `date` descendente.

Cada elemento SHALL exponer `session_instance_id`, `name` (de la `SessionInstance`), `date`, `presencial_time_from`, `presencial_time_to` y `attended_count` (cantidad de asistencias de esa sesión). El filtro de `team_id` SHALL ser obligatorio y el `group_id` de la URL SHALL pertenecer a ese equipo; en caso contrario el sistema responde `404`.

El endpoint SHALL exigir que el usuario autenticado sea **entrenador** del `team_id` (ver el requirement de matriz de autorización).

#### Scenario: Listar las presenciales pasadas de un grupo
- **WHEN** un entrenador dueño del equipo 5 envía `GET /api/v1/groups/7/attendance-sessions?team_id=5` y el grupo 7 tiene tres días `training`+`is_presencial=true` con fecha 2026-09-20, 2026-09-24 y 2026-09-27, más un día `training`+`is_presencial=false` del 2026-09-22 y un `rest` del 2026-09-23
- **THEN** el sistema responde `200 OK` con exactamente 3 elementos, ordenados 2026-09-27, 2026-09-24, 2026-09-20

#### Scenario: Excluir una sesión presencial futura
- **GIVEN** el grupo 7 tiene un día `training`+`is_presencial=true` con fecha 2026-10-05 y hoy es 2026-09-27
- **WHEN** un entrenador pide `GET /api/v1/groups/7/attendance-sessions?team_id=5`
- **THEN** esa sesión NO aparece en la respuesta

#### Scenario: La sesión de hoy sí aparece
- **GIVEN** el grupo 7 tiene un día `training`+`is_presencial=true` con la fecha de hoy, que aún no terminó
- **WHEN** un entrenador pide el listado
- **THEN** la sesión aparece en la respuesta, con `date` igual a la fecha de hoy

#### Scenario: El conteo de asistentes por sesión
- **GIVEN** la sesión del 2026-09-24 tiene 3 asistencias registradas
- **WHEN** un entrenador pide el listado
- **THEN** ese elemento trae `attended_count: 3`

#### Scenario: Grupo que pertenece a otro equipo
- **WHEN** un entrenador del equipo 5 envía `GET /api/v1/groups/9/attendance-sessions?team_id=5` y el grupo 9 es del equipo 6
- **THEN** el sistema responde `404 Not Found`

#### Scenario: Falta el team_id obligatorio
- **WHEN** se envía `GET /api/v1/groups/7/attendance-sessions` sin `team_id`
- **THEN** el sistema responde `400 Bad Request`

### Requirement: El sistema SHALL devolver la grilla de asistencia de una sesión con sus agregados

El sistema SHALL exponer `GET /api/v1/attendance/session/{session_instance_id}?team_id={team_id}&group_id={group_id}` (detrás del `AuthMiddleware`) que resuelva en una sola respuesta el roster del grupo cruzado con el estado de asistencia de cada corredor, más los agregados de la sesión.

El roster SHALL ser el de las membresías del grupo **activas en la fecha de la sesión objetivo** — es decir, `group_users.deleted_at IS NULL` **y** `date_start <= date de la sesión` **y** (`date_end IS NULL` o `date_end >= date de la sesión`). La evaluación SHALL hacerse contra la fecha de la sesión, no contra la fecha de la consulta.

Consecuencia REQUIRED de ese criterio: el denominador `roster_size` de una sesión dada SHALL ser estable en el tiempo. Consultar el mismo endpoint más adelante no SHALL cambiar `roster_size` por el mero paso de los días.

El campo `name` de cada fila SHALL ser el nombre completo del corredor, componiendo `name` + `surname` de `users` con un espacio y recortando espacios sobrantes, con fallback al `email` cuando ambos vienen vacíos. `email` SHALL venir del batch lookup existente.

Cada fila SHALL exponer `user_id`, `name`, `email`, `attendance_id` (`null` si no hay asistencia), `status` (`"attended"` o `"not_confirmed"`), `source` (`null`, `"qr"` o `"manual"`) y `registered_at` (`null` o timestamp). Las filas SHALL ordenarse alfabéticamente por nombre.

`source` SHALL ser `null` **exactamente cuando** `status` es `"not_confirmed"`: una fila `attended` siempre proviene de una fila de asistencia persistida, y esa fila siempre tiene `source` poblado. Un `attended` con `source: null` SHALL considerarse un defecto de datos, no un caso válido.

El bloque `summary` SHALL exponer `roster_size`, `attended`, `not_confirmed` y `attendance_rate_pct` (entero entre 0 y 100, con dos decimales redondeados a 1; `0` cuando `roster_size` es 0), donde `attended` es la cantidad de asistencias de la sesión y `not_confirmed` es `max(0, roster_size - attended)`.

El bloque `session` SHALL exponer `session_instance_id`, `name`, `date`, `presencial_time_from`, `presencial_time_to`, `presencial_location`, `group_id`, `group_name`, `team_id` y `team_name`.

El endpoint SHALL validar que la sesión objetivo sea una sesión presencial no cancelada del grupo indicado y del equipo indicado (ver el requirement de validación de la sesión objetivo), y SHALL exigir que el usuario autenticado sea entrenador del `team_id`.

#### Scenario: Grilla con todos los corredores del grupo
- **GIVEN** el grupo 7 tiene 4 corredores activos, y a la sesión del 2026-09-24 hay 2 asistencias (una por QR y una manual)
- **WHEN** un entrenador pide `GET /api/v1/attendance/session/42?team_id=5&group_id=7`
- **THEN** el sistema responde `200 OK` con 4 filas (una por corredor), 2 de ellas con `status: "attended"`, y `summary: { roster_size: 4, attended: 2, not_confirmed: 2, attendance_rate_pct: 50.0 }`

#### Scenario: La fila sin asistencia no trae id ni timestamp
- **GIVEN** un corredor del grupo 7 sin asistencia para la sesión 42
- **WHEN** un entrenador pide la grilla
- **THEN** esa fila trae `status: "not_confirmed"` con `attendance_id: null`, `source: null` y `registered_at: null`

#### Scenario: La procedencia se distingue por fila
- **GIVEN** una asistencia registrada por el endpoint de QR y otra cargada por el entrenador
- **WHEN** un entrenador pide la grilla
- **THEN** una fila trae `source: "qr"` y la otra `source: "manual"`, ambas con `status: "attended"`

#### Scenario: Porcentaje con roster vacío
- **GIVEN** el grupo 7 no tiene corredores activos y la sesión 42 no tiene asistencias
- **WHEN** un entrenador pide la grilla
- **THEN** `summary` trae `roster_size: 0`, `attended: 0`, `not_confirmed: 0` y `attendance_rate_pct: 0`

#### Scenario: Asistencias de un corredor que ya no está en el grupo
- **GIVEN** un corredor tiene una asistencia para la sesión 42 pero su membresía fue borrada lógicamente (`deleted_at` no nulo)
- **WHEN** un entrenador pide la grilla
- **THEN** esa fila NO aparece, y esa asistencia NO cuenta para `attended`

#### Scenario: Corredor que dejó el grupo DESPUÉS de la sesión
- **GIVEN** la sesión 42 es del 2026-09-24, y un corredor del grupo 7 tiene `date_end` del 2026-10-01 (dejó el grupo una semana **después** de la sesión), sin asistencia registrada
- **WHEN** un entrenador pide la grilla de la sesión 42
- **THEN** esa fila **aparece** con `status: "not_confirmed"` y cuenta para `roster_size`, porque era miembro activo en la fecha de la sesión

#### Scenario: Corredor que dejó el grupo ANTES de la sesión
- **GIVEN** la sesión 42 es del 2026-09-24, y un corredor del grupo 7 tiene `date_start` del 2026-09-30 (membresía que empieza después de la sesión) y `deleted_at` nulo
- **WHEN** un entrenador pide la grilla de la sesión 42
- **THEN** esa fila NO aparece y no cuenta para `roster_size`

#### Scenario: El denominador no se mueve con el paso del tiempo
- **GIVEN** la sesión 42 es del 2026-09-24 y el grupo 7 tiene 4 miembros activos a esa fecha; luego un corredor fija `date_end` del 2026-10-01
- **WHEN** un entrenador pide la grilla de la sesión 42 después de esa fecha
- **THEN** `summary.roster_size` sigue siendo `4`, idéntico al de la consulta del 2026-09-24

### Requirement: El sistema SHALL validar la sesión objetivo de toda operación de asistencia del entrenador

El sistema SHALL resolver y validar la sesión objetivo (`session_instance_id`) de toda operación de asistencia del entrenador resolviéndola contra su `GroupCalendarDay` y exigiendo, en todos los casos: que exista, que su `group_calendar_day.kind` sea `training`, que `is_presencial` sea `true`, que no esté `cancelled`, y que su `group.team_id` coincida con el equipo de la operación.

Si la sesión no existe, o no cumple alguna de esas condiciones, el sistema SHALL responder `422 Unprocessable Entity` con un mensaje en español que identifique la condición incumplida. Si el equipo no existe, SHALL responder `404`.

Esta validación SHALL aplique a los endpoints de listado de la grilla, de carga masiva y de emisión del QR. La carga masiva y el borrado no imponen, en cambio, que la fecha de la sesión sea pasada: un entrenador puede cargar asistencia de una sesión de hoy o, vía el QR, de una próxima.

#### Scenario: Sesión asincrónica rechazada
- **WHEN** un entrenador pide `GET /api/v1/attendance/session/42?team_id=5&group_id=7` y la sesión 42 corresponde a un día con `is_presencial = false`
- **THEN** el sistema responde `422 Unprocessable Entity` indicando que la sesión no es presencial

#### Scenario: Sesión cancelada rechazada
- **WHEN** un entrenador pide la grilla de una sesión cuyo día tiene `kind = 'cancelled'`
- **THEN** el sistema responde `422 Unprocessable Entity` indicando que la sesión está cancelada

#### Scenario: Sesión de un grupo de otro equipo
- **WHEN** un entrenador del equipo 5 pide `GET /api/v1/attendance/session/42?team_id=5&group_id=7` y la sesión 42 pertenece a un día del grupo 9, que es del equipo 6
- **THEN** el sistema responde `422 Unprocessable Entity` indicando que la sesión no pertenece al grupo indicado

#### Scenario: Emitir el QR de una sesión próxima
- **GIVEN** la sesión 42 corresponde a un día presencial dentro de 5 días
- **WHEN** un entrenador pide `GET /api/v1/attendance/qr?team_id=5&training_session_id=42`
- **THEN** el sistema responde `200 OK` con el QR, aunque la fecha sea futura

### Requirement: El sistema SHALL cargar la asistencia de varios corredores en una sola operación

El sistema SHALL exponer `POST /api/v1/attendance/bulk` (detrás del `AuthMiddleware`) que registre, de forma masiva e idempotente, la asistencia de varios corredores a una misma sesión.

El body SHALL ser `{ "team_id": int, "training_session_id": int, "entries": [ { "user_id": int } ] }`. Cada `user_id` de `entries` SHALL ser un miembro activo del grupo de la sesión; el sistema SHALL rechazar la operación completa con `422` si algún `user_id` no lo es, **sin** escribir ninguna fila (todo-o-nada), e indicando los `user_id` rechazados.

El sistema SHALL insertar las filas que no existen y SHALL actualizar las que ya existen (refrescando `updated_at`, `source` y `registered_by_user_id`), sin crear nunca duplicados para un mismo `(team_id, training_session_id, user_id)`. Cada fila creada o actualizada por este endpoint SHALL llevar `source = 'manual'` y `registered_by_user_id` igual al usuario autenticado.

`entries` vacío SHALL ser un no-op que responde `200 OK` con los contadores en cero. El endpoint SHALL exigir que el usuario autenticado sea entrenador del `team_id`.

#### Scenario: Cargar varias asistencias de una vez
- **GIVEN** el grupo 7 tiene 4 corredores activos y ninguno tiene asistencia para la sesión 42
- **WHEN** un entrenador envía `POST /api/v1/attendance/bulk` con `entries` de 3 `user_id` distintos
- **THEN** el sistema responde `200 OK` con `{ created: 3, updated: 0 }` y quedan 3 asistencias para la sesión 42, todas con `source: "manual"` y `registered_by_user_id` del entrenador

#### Scenario: Guardar dos veces el mismo lote no duplica
- **GIVEN** ya existen 3 asistencias para la sesión 42 de los mismos 3 corredores
- **WHEN** el entrenador envía exactamente el mismo lote otra vez
- **THEN** el sistema responde `200 OK` con `{ created: 0, updated: 3 }` y la cantidad de asistencias sigue siendo 3

#### Scenario: Marcar y remarkar en el mismo lote
- **GIVEN** un corredor ya tiene asistencia para la sesión 42 y otro no
- **WHEN** el entrenador envía un lote con los mismos 2 `user_id`
- **THEN** el sistema responde `200 OK` con `{ created: 1, updated: 1 }` y no queda ningún duplicado

#### Scenario: Un corredor ajeno al grupo invalida todo el lote
- **GIVEN** el usuario 999 no es miembro del grupo 7
- **WHEN** un entrenador envía un lote con los `user_id` 12 y 999
- **THEN** el sistema responde `422 Unprocessable Entity` listando el `user_id` 999 y **no** se crea la asistencia del corredor 12

#### Scenario: Lote vacío
- **WHEN** un entrenador envía `POST /api/v1/attendance/bulk` con `entries: []`
- **THEN** el sistema responde `200 OK` con `{ created: 0, updated: 0 }` y no escribe ninguna fila

#### Scenario: Guardar sobre una sesión asincrónica
- **WHEN** un entrenador envía un lote válido contra una sesión cuyo día tiene `is_presencial = false`
- **THEN** el sistema responde `422 Unprocessable Entity` y no escribe ninguna fila

### Requirement: El sistema SHALL eliminar una asistencia individual

El sistema SHALL exponer `DELETE /api/v1/attendance/{attendance_id}?team_id={team_id}` (detrás del `AuthMiddleware`) que elimine una asistencia ya cargada.

El `team_id` SHALL ser obligatorio. Si la asistencia no existe, el sistema SHALL responder `404 Not Found`. Si existe pero su `team_id` no es el consultado, o el usuario autenticado no es entrenador de ese equipo, el sistema SHALL responder `403 Forbidden` — nunca revelar la existencia de una asistencia de un equipo ajeno. Ante el borrado efectivo, el sistema SHALL responder `204 No Content`.

La eliminación SHALL ser física. Tras un `204`, el mismo corredor SHALL poder volver a ser marcado para la misma sesión.

#### Scenario: Eliminar una asistencia
- **GIVEN** existe la asistencia 88, del equipo 5, del corredor 12, para la sesión 42
- **WHEN** el entrenador del equipo 5 envía `DELETE /api/v1/attendance/88?team_id=5`
- **THEN** el sistema responde `204 No Content` y la fila 88 ya no existe

#### Scenario: Volver a marcar después de eliminar
- **GIVEN** el entrenador acaba de eliminar la asistencia 88
- **WHEN** envía `POST /api/v1/attendance/bulk` con el `user_id` 12 para la sesión 42
- **THEN** el sistema responde `200 OK` con `created: 1` y vuelve a existir la asistencia

#### Scenario: Eliminar una asistencia de un equipo ajeno
- **GIVEN** existe la asistencia 88 del equipo 5
- **WHEN** el entrenador del equipo 6 envía `DELETE /api/v1/attendance/88?team_id=6`
- **THEN** el sistema responde `403 Forbidden` y la asistencia 88 sigue existiendo

#### Scenario: Eliminar una asistencia inexistente
- **WHEN** un entrenador del equipo 5 envía `DELETE /api/v1/attendance/99999?team_id=5`
- **THEN** el sistema responde `404 Not Found`

### Requirement: El sistema SHALL aplicar la matriz de autorización del entrenador a las operaciones de asistencia

El sistema SHALL definir "ser entrenador del equipo T" como: el `auth_user_id` es el `owner_id` de T (`teams.owner_id`), **o** tiene una fila activa en `team_users` para T con `role_in_team = 'entrenador'`. Esta SHALL ser la misma regla que ya usa `GET /attendance/search`, para que un usuario no pierda permisos entre endpoints del mismo módulo.

Toda operación de asistencia del entrenador —listar sesiones presenciales, obtener la grilla, cargar en lote y eliminar— SHALL exigir esa condición sobre el equipo de la operación; en caso contrario el sistema SHALL responder `403 Forbidden`. Un usuario que no sea entrenador de ningún equipo SHALL recibir `403` en todas ellas.

Los endpoints de lectura que ya existían (`GET /attendance/search`) SHALL conservar su comportamiento actual: el entrenador ve todas las asistencias del equipo y el corredor solo las suyas.

#### Scenario: El owner del equipo opera
- **GIVEN** el usuario 3 es `owner_id` del equipo 5
- **WHEN** el usuario 3 pide la grilla, carga un lote o elimina una asistencia del equipo 5
- **THEN** la operación se ejecuta con normalidad

#### Scenario: Un entrenador del roster con rol Entrenador opera
- **GIVEN** el usuario 7 tiene una fila activa en `team_users` del equipo 5 con `role_in_team = 'entrenador'`, y no es el owner
- **WHEN** el usuario 7 pide la grilla de una sesión del equipo 5
- **THEN** el sistema responde `200 OK`

#### Scenario: Un corredor no puede gestionar la asistencia
- **GIVEN** el usuario 12 es corredor del equipo 5
- **WHEN** el usuario 12 pide la grilla, el listado de sesiones, envía un lote o intenta eliminar una asistencia del equipo 5
- **THEN** el sistema responde `403 Forbidden` en los cuatro casos

#### Scenario: Un entrenador de otro equipo no accede
- **GIVEN** el usuario 3 es entrenador del equipo 6 y no del equipo 5
- **WHEN** el usuario 3 pide `GET /api/v1/attendance/session/42?team_id=5&group_id=7`
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: Sin autenticación
- **WHEN** se envía cualquiera de los endpoints nuevos sin header `Authorization` válido
- **THEN** el sistema responde `401 Unauthorized`

### Requirement: El sistema SHALL restringir la emisión del QR de asistencia al entrenador del equipo

El sistema SHALL exigir que el `auth_user_id` sea entrenador del `team_id` consultado en `GET /api/v1/attendance/qr` para emitir el QR. Si no lo es, el sistema SHALL responder `403 Forbidden`.

Esta condición es una corrección de una brecha del endpoint ya implementado: hasta ahora el handler solo verificaba que hubiera un usuario autenticado, por lo que **cualquier usuario autenticado podía generar el QR de cualquier equipo**. La respuesta `200 OK` con `qr_code_base64` y `url_encoded`, y la validación de que ambos query params sean enteros mayores a 0, SHALL permanecer sin cambios.

La generación SHALL seguir siendo determinista: la misma combinación de `team_id` y `training_session_id` SHALL producir siempre el mismo PNG base64.

#### Scenario: El entrenador del equipo emite su QR
- **GIVEN** el usuario 3 es entrenador del equipo 5
- **WHEN** envía `GET /api/v1/attendance/qr?team_id=5&training_session_id=42`
- **THEN** el sistema responde `200 OK` con `qr_code_base64` y `url_encoded`

#### Scenario: Un corredor no puede emitir el QR de su equipo
- **GIVEN** el usuario 12 es corredor del equipo 5
- **WHEN** el usuario 12 envía `GET /api/v1/attendance/qr?team_id=5&training_session_id=42`
- **THEN** el sistema responde `403 Forbidden` y no devuelve ninguna imagen

#### Scenario: Un usuario autenticado no puede emitir el QR de un equipo ajeno
- **GIVEN** el usuario 3 es entrenador del equipo 6
- **WHEN** el usuario 3 envía `GET /api/v1/attendance/qr?team_id=5&training_session_id=42`
- **THEN** el sistema responde `403 Forbidden`

#### Scenario: El determinismo se mantiene
- **WHEN** el entrenador del equipo 5 pide el QR de la sesión 42 dos veces
- **THEN** el `qr_code_base64` de ambas respuestas es idéntico

### Requirement: El sistema SHALL exigir pertenencia al grupo para registrar asistencia por QR

El sistema SHALL exigir que el `auth_user_id` tenga una membresía activa del grupo al que pertenece la sesión objetivo para que `POST /api/v1/attendance/team/{team_id}/session/{training_session_id}` registre la asistencia; en caso contrario SHALL responder `403 Forbidden`.

Esta condición corrige otra brecha del endpoint ya implementado: el handler solo verificaba que hubiera un usuario autenticado, por lo que cualquier usuario podía cargar asistencia en nombre de cualquier persona para cualquier equipo y sesión. Se conserva la idempotencia existente: la primera vez responde `201 Created`, y si ya existía la asistencia responde `200 OK` sin insertar un duplicado.

La fila creada por este endpoint SHALL llevar `source = 'qr'` y `registered_by_user_id` igual al `auth_user_id` (el propio corredor).

#### Scenario: El corredor del grupo registra su asistencia
- **GIVEN** el usuario 12 es miembro activo del grupo 7, del equipo 5, y la sesión 42 pertenece a un día del grupo 7
- **WHEN** el usuario 12 envía `POST /api/v1/attendance/team/5/session/42`
- **THEN** el sistema responde `201 Created` y la fila queda con `source: "qr"` y `registered_by_user_id: 12`

#### Scenario: Segundo registro del mismo corredor
- **GIVEN** el usuario 12 ya tiene asistencia para la sesión 42
- **WHEN** el usuario 12 envía de nuevo `POST /api/v1/attendance/team/5/session/42`
- **THEN** el sistema responde `200 OK` con el mensaje de asistencia previamente registrada y no inserta una fila nueva

#### Scenario: Un corredor no puede registrar asistencia en un grupo ajeno
- **GIVEN** el usuario 12 es miembro del grupo 7 y la sesión 42 pertenece a un día del grupo 9 (del mismo equipo 5, otro grupo)
- **WHEN** el usuario 12 envía `POST /api/v1/attendance/team/5/session/42`
- **THEN** el sistema responde `403 Forbidden` y no se inserta ninguna fila

#### Scenario: Un usuario que no pertenece al equipo
- **GIVEN** el usuario 77 no es miembro de ningún grupo del equipo 5
- **WHEN** el usuario 77 envía `POST /api/v1/attendance/team/5/session/42`
- **THEN** el sistema responde `403 Forbidden` y no se inserta ninguna fila

### Requirement: El sistema SHALL registrar la procedencia de cada asistencia

El sistema SHALL almacenar, en cada fila de `attendances`, la procedencia del registro: una columna `source` cuyo valor SHALL ser `"qr"` cuando la alta vino del endpoint de registro del corredor y `"manual"` cuando vino del endpoint de carga masiva del entrenador, y una columna `registered_by_user_id` con el identificador del usuario que la registró.

`attendances.training_session_id` SHALL referenciar a `session_instances.id` mediante una foreign key real, y su valor SHALL ser un entero mayor a 0. Esta columna dejó de ser una referencia opaca: la tabla contra la que puede apuntar (`session_instances`) ya existe, y `training_sessions` sigue sin ser parte de este change.

El backfill SHALL cubrir las filas preexistentes: las que tengan `source` nulo SHALL quedar con `source = 'qr'` y `registered_by_user_id` nulo, de modo que la columna nunca devuelva un valor fuera del dominio declarado. El valor del backfill es **`'qr'`, no `'manual'`**: el unico escritor de `attendances` antes de este change era el registro del corredor al escanear el QR (no existia el alta manual), asi que toda fila preexistente proviene necesariamente de un QR. Backfillear a `'manual'` etiquetaria como "cargada por el entrenador" la asistencia historica entera, y `source` es justamente el campo que la grilla muestra como procedencia, asi que el error seria visible para el usuario.

#### Scenario: Una asistencia cargada por el entrenador conserva su procedencia
- **WHEN** un entrenador carga en lote la asistencia de un corredor
- **THEN** la fila tiene `source: "manual"` y `registered_by_user_id` igual al id del entrenador

#### Scenario: Una asistencia por QR conserva su procedencia
- **WHEN** un corredor se registra escaneando el QR
- **THEN** la fila tiene `source: "qr"` y `registered_by_user_id` igual al id del corredor

#### Scenario: Una fila anterior al backfill queda marcada como qr
- **GIVEN** una fila de asistencia creada antes de que existiera la columna `source`, cargada por el corredor escaneando el QR
- **WHEN** se corre la migración
- **THEN** esa fila queda con `source: "qr"` y `registered_by_user_id` nulo, y la grilla la muestra sin error

#### Scenario: La foreign key rechaza una sesión inexistente
- **WHEN** se intenta insertar una asistencia con `training_session_id` apuntando a una `session_instances` inexistente
- **THEN** la base rechaza el insert por violación de la foreign key
