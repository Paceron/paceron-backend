## ADDED Requirements

### Requirement: Asignar contenido a un día instancia el catálogo, no lo referencia en vivo

El sistema SHALL, al asignar una `Session` a un `GroupCalendarDay` (`kind=training`, vía `PUT` de un día individual, `bulk` o `stamp`), crear una `SessionInstance` nueva con copia congelada de la `Session` (`name`/`description`) y, para cada `SessionExercise` que la compone, una `SessionExerciseInstance` que referencia una `ExerciseInstance` nueva (copia congelada del `Exercise` correspondiente). `GroupCalendarDay.session_instance_id` SHALL apuntar a esa `SessionInstance`, nunca a la `Session` del catálogo.

El sistema SHALL crear una instancia **por día asignado**, sin deduplicar aunque el mismo `session_id` de catálogo se asigne a múltiples fechas en la misma operación.

#### Scenario: Asignar una sesión a un día crea instancias propias
- **WHEN** el entrenador dueño hace `PUT /groups/{id}/calendar/{date}` con `kind=training` y `session_id` de una `Session` del catálogo con 3 `SessionExercise`
- **THEN** el sistema crea 1 `SessionInstance`, 3 `SessionExerciseInstance` y 3 `ExerciseInstance` nuevas, y `GroupCalendarDay.session_instance_id` apunta a la `SessionInstance` creada

#### Scenario: Editar la Session del catálogo después de asignarla no afecta el día ya asignado
- **GIVEN** un día ya asignado con una `SessionInstance` congelada
- **WHEN** se edita la `Session` original del catálogo (`PUT /sessions/{id}`) o alguno de sus `Exercise` (`PUT /exercises/{id}`)
- **THEN** el día asignado no cambia — su `SessionInstance`/`ExerciseInstance` son copias independientes, sin ninguna referencia viva al catálogo

#### Scenario: Asignar la misma sesión a 10 días no comparte instancias
- **WHEN** en una misma operación (ej. `bulk` o `stamp`) se asigna la misma `Session` de catálogo a 10 fechas distintas
- **THEN** el sistema crea 10 `SessionInstance` independientes, una por fecha, sin compartir ninguna fila entre ellas

### Requirement: Reasignar o borrar un día ya cerrado está prohibido

El sistema SHALL rechazar (`422`) cualquier intento de asignar, reasignar o borrar contenido de un `GroupCalendarDay` cuya fecha ya está "cerrada": `date` pasada, `date` de hoy con `is_presencial=true` y `now >= presencial_time_from`, o `date` de hoy con `is_presencial=false` (async). `is_presencial` participa **solo** en decidir si el día está cerrado — una vez que un día es cerrado, la única operación permitida sobre él es la transición a `kind=cancelled` (sin excepción, independientemente de si era presencial o no); ninguna otra operación de escritura tiene trato especial por `is_presencial`.

Este guard aplica a los **5** endpoints de escritura del calendario: `PUT` de un día individual, `DeleteDay`, `stamp`, `bulk` y `bulk-clear`, y a `shift` sobre cualquier fila que el corrimiento afecte. La transición a `kind=cancelled` sobre un día que ya estaba en `training` SHALL seguir permitida sobre un día cerrado (no reinstancia nada, solo marca `cancelled_reason` conservando el `session_instance_id` existente).

`stamp`/`bulk`/`bulk-clear`/`shift` SHALL validar todas las fechas/filas afectadas contra esta regla antes de escribir cualquiera — si alguna ya está cerrada, el sistema rechaza la operación completa (`422`, con la lista de fechas en conflicto), sin aplicar ningún cambio parcial.

#### Scenario: Reasignar un día pasado
- **WHEN** el entrenador dueño intenta `PUT /groups/{id}/calendar/{date}` sobre una fecha pasada que ya tenía `kind=training`
- **THEN** el sistema responde `422`, sin crear ninguna instancia ni modificar la fila existente

#### Scenario: Reasignar un día futuro sí está permitido
- **WHEN** el entrenador dueño reasigna un día futuro que ya tenía una `SessionInstance` (ej. cambia a otra `Session` del catálogo)
- **THEN** el sistema crea una nueva `SessionInstance`/`ExerciseInstance` para ese día y repuntea `session_instance_id`

#### Scenario: Cancelar un día presencial ya empezado
- **WHEN** el entrenador dueño hace `PUT` con `kind=cancelled` sobre un día de hoy, presencial, ya empezado (`now >= presencial_time_from`)
- **THEN** el sistema acepta la transición (no es una reasignación de contenido, solo un cambio de estado)

#### Scenario: Bulk con una fecha cerrada entre varias abiertas
- **WHEN** el entrenador dueño manda `bulk` con 5 fechas, una de ellas ya pasada
- **THEN** el sistema responde `422` con la fecha pasada señalada, sin aplicar el cambio a ninguna de las 5

#### Scenario: Bulk-clear con una fecha cerrada
- **WHEN** el entrenador dueño manda `bulk-clear` con fechas que incluyen un día ya cerrado
- **THEN** el sistema responde `422` con esa fecha señalada, sin borrar ninguna de las fechas del lote

#### Scenario: Shift que movería una fila ya cerrada
- **WHEN** el entrenador dueño corre `shift` desde una fecha tal que al menos una fila con `date >= from_date` ya está cerrada
- **THEN** el sistema responde `422` con esa fila señalada, sin mover ninguna fecha del lote

### Requirement: Borrado de instancia superada respeta feedback existente

El sistema SHALL, al reasignar un día futuro que ya tenía una `SessionInstance`, borrar físicamente la `SessionInstance`/`SessionExerciseInstance`/`ExerciseInstance` superadas — salvo que algún registro de `workout_feedback` (`assigned_session_id`/`assigned_exercise_id`) ya las referencie, en cuyo caso SHALL conservarlas (huérfanas, sin ningún `GroupCalendarDay` activo apuntándolas).

#### Scenario: Reasignar un día futuro sin feedback previo
- **WHEN** se reasigna un día futuro cuya `SessionInstance` anterior no tiene ningún `workout_feedback` asociado
- **THEN** el sistema borra la `SessionInstance`/`ExerciseInstance` anteriores junto con la reasignación

#### Scenario: Reasignar un día futuro con feedback ya cargado (caso raro)
- **WHEN** se reasigna un día futuro cuya `SessionInstance` anterior sí tiene algún `workout_feedback` asociado
- **THEN** el sistema conserva la `SessionInstance`/`ExerciseInstance` anteriores (no las borra), y de todas formas crea la nueva instancia y repuntea el día

### Requirement: Las respuestas de calendario embeben el detalle de la instancia, no un ID bare

El sistema SHALL incluir en `CalendarDayResponse` y `NextSessionResponse` el detalle completo de la `SessionInstance` asignada (nombre, descripción, y cada `ExerciseInstance` con todos sus campos + `role`/`repeat_count`/`rest_minutes`) en vez de un `session_id` plano — no existe endpoint público que resuelva una instancia por ID, y resolverla contra el catálogo (`GET /sessions/{id}`) devolvería contenido en vivo, no el congelado para ese día.

#### Scenario: Consultar un día con sesión asignada
- **WHEN** se pide `GET /groups/{id}/calendar?from=...&to=...` y algún día tiene `kind=training`
- **THEN** la fila de ese día incluye un objeto `session_instance` con `name`/`description` y la lista completa de `exercises` (cada uno con su contenido congelado)

#### Scenario: Día sin sesión asignada
- **WHEN** un día tiene `kind` distinto de `training`/`cancelled`
- **THEN** `session_instance` viene `null`

### Requirement: Estampar un plan sobre el calendario

> Reemplaza en su totalidad el requirement homónimo de `calendario-asignacion-grupos` — ese change nunca fue archivado (este proyecto no practica el paso de archive en la práctica), así que se redeclara acá completo en vez de un delta `MODIFIED` contra una spec base inexistente.

El sistema SHALL copiar cada `PlanDay` de un `TrainingPlan` a filas de `GroupCalendarDay` a partir de `start_date` (día 1 → `start_date`, día N → `start_date + N - 1`), validando que el plan pertenezca al mismo entrenador dueño del grupo, instanciando (no referenciando) la `Session`/`Exercise` de cada `PlanDay` con `kind=training` de la misma forma que un `PUT` individual, y SHALL rechazar el estampado completo con `409` (listando las fechas en conflicto) si algún día del rango ya tiene contenido, salvo que `force=true`. SHALL además rechazar con `422` (listando las fechas correspondientes) si alguna fecha de destino ya existe y está cerrada — ver "Reasignar o borrar un día ya cerrado está prohibido".

#### Scenario: Estampar sin conflictos crea instancias para cada día `training`
- **WHEN** el entrenador dueño estampa un plan de 5 días (3 `training`, 2 `rest`) a partir de una fecha donde ninguno de los 5 días de destino tiene contenido previo
- **THEN** el sistema crea las 5 filas de `GroupCalendarDay` con `source_plan_id` apuntando al plan, y una `SessionInstance` (con sus `ExerciseInstance`) por cada uno de los 3 días `training`

#### Scenario: Estampar con conflicto sin force
- **WHEN** al menos 1 de los días de destino ya tiene contenido y `force` no viene o es `false`
- **THEN** el sistema responde `409` con la lista de fechas en conflicto, sin crear ni modificar ninguna fila ni instancia

#### Scenario: Estampar con force sobre días futuros
- **WHEN** hay conflicto sobre días futuros y se manda `force=true`
- **THEN** el sistema reemplaza el contenido de las fechas en conflicto (nuevas instancias, borrando las superadas salvo que tengan feedback) con el del plan

#### Scenario: Estampar un plan ajeno
- **WHEN** el entrenador dueño del grupo intenta estampar un plan cuyo `owner_id` es otro entrenador
- **THEN** el sistema responde `403`, sin leer siquiera los días del plan

#### Scenario: Estampar sobre un rango que incluye días ya cerrados
- **WHEN** `start_date` es tal que alguno de los días resultantes cae en el pasado
- **THEN** el sistema responde `422` con esas fechas señaladas, sin aplicar el estampado (ni siquiera parcialmente sobre los días futuros del mismo lote)
