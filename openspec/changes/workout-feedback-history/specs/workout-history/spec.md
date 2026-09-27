# Spec Delta — workout-history

## ADDED Requirements

### Requirement: Historial de feedback del atleta

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /users/{id}/workout-feedback-history` lista los feedbacks donde `athlete_user_id = {id}`, con `deleted_at IS NULL`.
- Si `{id}` no es el usuario autenticado, responde `403`.
- Filtros soportados: `team_id`, `group_id`, `date_from`, `date_to`, `exercise_id` (matchea por familia de catálogo: `catalog_exercise_id`, con fallback al id de instancia propio en instancias legado sin origen), `set_number`.
- `group_id` sin `team_id` responde `400`.
- `date_from` y `date_to` vienen juntos o ninguno; si viene uno solo responde `400`; `date_from > date_to` responde `400`; iguales = un día.
- `sort` acepta solo `feedback_date`, `set_number`, `exercise_name` (default `feedback_date`); otro valor responde `400`. `order` acepta solo `asc`/`desc` (default `desc`); otro valor responde `400`.
- `page` (default 1, ≥1) y `page_size` (default 20, 1..100); fuera de rango responde `400`.
- Responde `200` con `{items, total, page, page_size, available_athletes, available_exercises}`.

#### Scenario: Sin filtros trae historial completo

- **WHEN** el atleta llama sin filtros
- **THEN** responde `200` con sus feedbacks en todos sus equipos, ordenados por fecha descendente, paginados.

#### Scenario: group_id sin team_id

- **WHEN** se envía `group_id` sin `team_id`
- **THEN** responde `400` sin ejecutar la consulta.

#### Scenario: Rango de fechas inválido

- **WHEN** se envía solo `date_from` o `date_from > date_to`
- **THEN** responde `400`.

#### Scenario: Sort fuera de whitelist

- **WHEN** se envía `sort=foo`
- **THEN** responde `400`.

### Requirement: Historial de feedback administrado por el entrenador

El sistema DEBE cumplir lo siguiente (MUST):

- `GET /users/{id}/administered-workout-feedback-history` con `team_id` obligatorio: sin `team_id` responde `400`.
- `{id}` debe ser el usuario autenticado (`403` si no) y owner del `team_id` (`404` si el equipo no existe, `403` si no lo administra).
- Soporta los mismos filtros del historial del atleta más `athlete_user_id` (filtro de segundo nivel sobre atleta).
- Responde `200` con el mismo shape `{items, total, page, page_size, available_athletes, available_exercises}`.

#### Scenario: team_id obligatorio

- **WHEN** se omite `team_id`
- **THEN** responde `400`.

#### Scenario: Equipo ajeno

- **WHEN** el entrenador consulta un `team_id` que no administra
- **THEN** responde `403` (y `404` si el equipo no existe).

### Requirement: Ítems y pools del historial

El sistema DEBE cumplir lo siguiente (MUST):

- Cada ítem expone: `id`, `athlete_user_id`, `athlete_name`, `team_id`, `team_name`, `group_id`, `group_name`, `date` (`session_date`), `session_name`, `session_instance_id` (siempre presente: es `assigned_session_id`, el id de la instancia de sesión que la pantalla de revisión carga por `GET /session-instances/:id/feedback`), `exercise_id` (id de instancia), `exercise_name`, `catalog_exercise_id` (nullable, id de catálogo si existe), `set_number`, `completion_status`, `duration_ms`, `active_duration_ms`, `distance_meters`, `started_at`, `ended_at`.
- Feedbacks huérfanos se incluyen: `group_id`/`group_name` `null` cuando el día de calendario ya no existe o la instancia nunca se asignó a un día; `team_id`/`team_name` `null` cuando el feedback no registró equipo.
- `total` = cantidad de feedbacks que matchean TODOS los filtros, sin paginación.
- `available_athletes` y `available_exercises` = DISTINCT sobre los que matchean solo los filtros de PRIMER nivel (equipo/grupo/rango de fechas y scope de autorización), sin filtros de segundo nivel (atleta/ejercicio/set) ni paginación. `available_exercises` va dedupeado por familia: un ítem por ejercicio de catálogo (`catalog_exercise_id`, fallback al id de instancia propio en instancias legado sin origen), con el nombre común.

#### Scenario: Huérfano se conserva

- **WHEN** un feedback cuya instancia ya no tiene día de calendario matchea los filtros
- **THEN** aparece en `items` con `group_id` y `group_name` `null`.

#### Scenario: Pools ignoran segundo nivel

- **WHEN** se consulta con `exercise_id` y `athlete_user_id` (endpoint entrenador)
- **THEN** `available_athletes` y `available_exercises` siguen listando todos los atletas/ejercicios del rango filtrado por equipo/fechas, sin verse recortados por esos filtros.

#### Scenario: available_exercises dedupeado por familia

- **WHEN** el rango tiene feedbacks de 3 instancias del mismo ejercicio de catálogo y 1 instancia legado sin origen
- **THEN** `available_exercises` expone 2 ítems: el de catálogo (id = `catalog_exercise_id`) y el de la instancia legado (id = su propio id de instancia).

#### Scenario: exercise_id filtra por familia de catálogo

- **WHEN** se consulta con `exercise_id` = id de catálogo que tiene 3 instancias con feedback
- **THEN** `items` incluye las filas de TODAS esas instancias (y `total` las cuenta), sin incluir filas de otras familias.

#### Scenario: Paginación y total

- **WHEN** se consulta con `page=2&page_size=10` y hay 25 feedbacks que matchean
- **THEN** `items` trae 10 (los 11º-20º según el orden), `total=25`, `page=2`, `page_size=10`.
