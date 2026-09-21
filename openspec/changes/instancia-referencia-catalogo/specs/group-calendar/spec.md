## ADDED Requirements

> Nota: se declaran como `ADDED` (no `MODIFIED`) porque la capability `group-calendar` nunca fue archivada en `openspec/specs/` — ver convención en `AGENTS.md` §3. Estos requirements **extendían** los de `asignacion-por-instanciacion` (instanciación por día y respuestas D9): el requirement "Asignar contenido a un día instancia el catálogo, no lo referencia en vivo" mantiene toda su semántica salvo el caso nuevo de conservación (`PUT` individual sin `session_id` sobre día ya instanciado), especificado completo acá abajo.

### Requirement: Toda instancia conserva referencia informativa a su origen del catálogo

Al crear una `SessionInstance`/`ExerciseInstance` vía instanciación (`PUT` con `kind=training`, `bulk` o `stamp`), el sistema SHALL guardar en `session_instances.source_session_id` el id de la `Session` de catálogo origen, y en `exercise_instances.source_exercise_id` el id del `Exercise` de catálogo del que se copió cada `ExerciseInstance`. Las referencias SHALL ser gestionadas por la app sin constraint de base de datos (patrón `source_plan_id`), SHALL ser nullable, y los valores de instancias creadas antes de este cambio SHALL permanecer `NULL` (sin backfill). El sistema NO SHALL limpiarlas cuando el catálogo de origen se soft-borra — son informativas; la vigencia del origen se determina consultando el catálogo.

Las respuestas de calendario que embeben `session_instance` (diseño D9 de `asignacion-por-instanciacion`) SHALL exponer `session_id` (nullable) en el objeto de la sesión instancia y `exercise_id` (nullable) en cada elemento de `exercises`, con los valores persistidos de `source_session_id`/`source_exercise_id` respectivamente.

#### Scenario: Instanciar una sesión guarda sus orígenes
- **WHEN** se asigna una `Session` de catálogo con dos `Exercise` a un día (`kind=training`)
- **THEN** la `SessionInstance` creada queda con `source_session_id` igual al id de la `Session`, y cada `ExerciseInstance` con `source_exercise_id` igual al id del `Exercise` del que se copió

#### Scenario: La respuesta de calendario expone los orígenes
- **GIVEN** un día asignado cuya `SessionInstance` tiene `source_session_id = 42` y cuyos ejercicios instancia tienen `source_exercise_id` propios
- **WHEN** se lee el calendario (`GET` de rango o `next-session`)
- **THEN** `session_instance.session_id == 42` y cada `session_instance.exercises[].exercise_id` trae su origen

#### Scenario: Instancias creadas antes del cambio responden null
- **GIVEN** una `SessionInstance` creada antes de este cambio (`source_session_id` nulo)
- **WHEN** se lee el calendario que la embebe
- **THEN** `session_instance.session_id` responde `null` y los `exercise_id` de sus ejercicios responden `null`, sin error

### Requirement: Guardar un día ya instanciado sin session_id conserva la instancia actual

En el `PUT /groups/{id}/calendar/{date}` individual, cuando `kind=training` y el request omite `session_id`, el sistema SHALL conservar el `session_instance_id` existente del día sin crear instancias nuevas ni borrar ninguna, si y solo si el día ya tenía una instancia. Si `session_id` viene `nil` y el día NO tiene instancia previa (alta nueva, o día con otro `kind` sin instancia), el sistema SHALL responder `422` (`ErrCalendarFieldMismatch`) — mismo error que hoy para `kind=training` sin sesión. Si `session_id` viene presente, el comportamiento no cambia: reinstancia de cero y borra la instancia superada según las reglas de borrado con feedback de `asignacion-por-instanciacion`.

`POST .../bulk` con `kind=training` y `session_id` omitido SHALL aplicar la misma conservación **por fecha**: cada día del lote conserva su propia instancia. Si alguna fecha no tiene instancia previa que conservar, el sistema SHALL rechazar el lote completo (`422`, listando las fechas sin instancia en el mensaje), sin aplicar ningún cambio parcial — mismo criterio all-or-nothing que las fechas cerradas. `stamp` no participa de esta regla (su `session_id` proviene del `PlanDay`). El guard de día cerrado y la transición a `cancelled` siguen exactamente como estaban.

#### Scenario: Tocar solo el horario presencial sin reinstanciar
- **GIVEN** un día futuro con `kind=training` e instancia creada
- **WHEN** el entrenador dueño hace `PUT` con `kind=training`, sin `session_id`, cambiando solo `is_presencial`/horarios
- **THEN** el sistema responde `200`, el día conserva el mismo `session_instance_id`, y no se creó ni borró ninguna fila de `session_instances`/`exercise_instances`

#### Scenario: Alta nueva sin session_id sigue siendo 422
- **WHEN** se hace `PUT` con `kind=training` y `session_id` omitido sobre un día vacío o sin instancia
- **THEN** el sistema responde `422` (`ErrCalendarFieldMismatch`), sin escribir nada

#### Scenario: session_id presente reinstancia igual que antes (regresión)
- **GIVEN** un día futuro con instancia
- **WHEN** se hace `PUT` con `kind=training` y un `session_id` de catálogo
- **THEN** se crea una nueva `SessionInstance` (con sus `source_*_id` poblados), se repuntea `session_instance_id`, y la instancia superada se borra o se conserva huérfana según las reglas de feedback vigentes

#### Scenario: Bulk sin session_id conserva la instancia de cada fecha
- **GIVEN** tres fechas futuras que ya tienen `kind=training` con instancias propias
- **WHEN** se envía `POST .../bulk` con `kind=training`, sin `session_id`, sobre esas tres fechas
- **THEN** cada día conserva su propio `session_instance_id` (no se crea ni se borra ninguna instancia) y la respuesta lista los tres días con sus instancias embebidas

#### Scenario: Bulk con una fecha sin instancia rechaza el lote completo
- **GIVEN** dos fechas con instancia y una fecha vacía
- **WHEN** se envía `POST .../bulk` con `kind=training` y sin `session_id` sobre las tres
- **THEN** el sistema responde `422` listando la fecha sin instancia en el mensaje, sin modificar ninguna de las tres filas
