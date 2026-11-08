# Gap 40 — `is_presencial` en el historial de entrenamientos

## Por qué

El historial de entrenamientos (`GET /users/{id}/workout-feedback-history` y `GET /users/{id}/administered-workout-feedback-history`) devuelve fecha, sesión, equipo y grupo, pero no indica si la sesión fue presencial o asíncrona. El frontend necesita ese dato para mostrar el mismo badge presencial/async que ya usan el Home y el calendario, y para cruzar el historial con lo planificado sin una segunda llamada a `/activities/comparison`.

## Qué cambia

- Agregar `is_presencial` (boolean) a cada ítem de ambas respuestas, a partir de `group_calendar_days.is_presencial` de la sesión asociada.
- Si el ítem no tiene día de calendario (huérfano), `is_presencial` sale `false`; el `group_id`/`group_name` nulos siguen identificando ese caso.
- El campo es aditivo, por ítem (mismo valor para todas las series de una misma sesión).

## Alcance

- Solo backend y solo lectura: DTO de respuesta, fila del DAO, mapper y tests.
- Sin cambios de filtros, paginación, pools, índices ni endpoints.
- Sin cambios de frontend más allá de consumir el campo nuevo.

## No-goals

- No se agrega `compliance_pct` al historial.
- No se agrupa por sesión ni se cambia el shape existente.
- No se toca `/activities/comparison`.
- No se modifica la semántica de presencial de los endpoints de calendario.
