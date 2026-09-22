## Why

`asignacion-por-instanciacion` congeló el contenido asignado a un día de calendario en tablas propias (`session_instances`/`exercise_instances`), pero las instancias **no guardan ninguna referencia de vuelta al catálogo de origen**: una vez creada, no hay forma de saber de qué `Session`/`Exercise` salió. El frontend lo necesita para mostrar "la sesión que ya tenía este día" y preseleccionarla con certeza en el selector de edición (no por match de nombre, como hace hoy a la fuerza).

Además, el `PUT /groups/{id}/calendar/{date}` con `kind=training` **exige `session_id` siempre y reinstancia de cero en cada guardado** (`calendar_service.go`, `validateDayFields` + rama `training` de `UpsertDay`). Resultado: no se puede tocar solo el horario presencial de un día ya asignado sin volver a elegir sesión y regenerar la instancia completa (instancias nuevas, vieja borrada si no tiene feedback). El frontend pidió explícitamente evitar esto (Gap 7 en `paceron-frontend/docs/BACKEND_API_GAPS.md`).

## What Changes

- **`SessionInstance` guarda `source_session_id`** y **`ExerciseInstance` guarda `source_exercise_id`**: columnas nullable BIGINT, referencia opaca gestionada por la app (mismo patrón que `GroupCalendarDay.source_plan_id` — sin constraint de DB, sin `ON DELETE` real porque el catálogo solo se soft-borra y las filas nunca desaparecen). Se pueblan al instanciar. Sin backfill: las instancias existentes quedan con `NULL`.
- **Respuestas de calendario (D9) suman campos aditivos**: `session_instance.session_id` y `session_instance.exercises[].exercise_id`, ambos nullable. No rompen el contrato actual — solo agregan campos.
- **`PUT` de un día con `kind=training` y `session_id` omitido conserva la instancia actual** si el día ya tenía una (mismo mecanismo que ya usa `kind=cancelled`), en vez de reinstanciar. `session_id` sigue siendo obligatorio si no hay instancia previa que conservar (alta nueva), y sigue reinstanciar si viene un `session_id` distinto o igual. **`bulk` con `kind=training` sin `session_id` aplica la misma conservación por fecha**: cada día conserva su propia instancia; si alguna fecha no tiene instancia previa se rechaza el lote completo (`422` con la lista de fechas, all-or-nothing como el resto de validaciones por lote). `stamp` queda invariable (la sesión viene del `PlanDay`).

## Capabilities

### Modified Capabilities

- `group-calendar` (de `asignacion-por-instanciacion`): las instancias conservan referencia de origen al catálogo, expuesta en las respuestas; y guardar un día ya asignado (`PUT` individual o `bulk`) puede hacerse sin re-elegir sesión.

## Impact

- Modelos: `session_instance.go`, `exercise_instance.go` (+1 columna nullable cada uno; AutoMigrate la crea solo).
- DTOs/respuestas: `instance/instance_response.go` (`SessionInstanceResponse`, `InstanceExerciseResponse`, mappers) — aditivo.
- Services: `calendar_service.go` (`validateDayFields`, `UpsertDay` y `Bulk` rama `training`, condición de borrado de instancia superada, nuevo error por lote con fechas, path `db == nil` espejo).
- Swagger: regenerate (anotaciones de request `PUT` pasan a documentar `session_id` opcional sobre día ya asignado).
- Docs: `docs/CATALOGO_Y_CALENDARIO.md` §8, `docs/FRONTEND_IMPACTO_INSTANCIACION.md` (cambio aditivo, cerrar Gap 7 del lado frontend).
- Tests: DAOs (columnas viajan), service (conservación/422/regresión de reinstanciación), respuesta (nuevos campos).
- Frontend (`paceron-frontend`): desbloquea la opción "actual" del selector y el guardado parcial del día — no rompe nada existente (todo aditivo).

## Non-Goals

- Borrar/backfill de instancias viejas con `source_*_id` `NULL`: no hay dato reconstructible confiable; se tratan como "origen desconocido" (campo opcional ya lo soporta).
- Limpiar `source_session_id`/`source_exercise_id` cuando el catálogo se soft-borra: la referencia sigue apuntando a la fila (informativa); el frontend resuelve "sigue viva?" consultando el catálogo, que ya filtra `deleted_at`.
