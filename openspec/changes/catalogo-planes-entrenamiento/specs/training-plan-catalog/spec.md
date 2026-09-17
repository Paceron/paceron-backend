## ADDED Requirements

### Requirement: CRUD de planes con días secuenciales validados

El sistema SHALL permitir crear, leer, actualizar y borrar (físicamente) planes de entrenamiento propiedad de un `owner_id`, cada uno con entre 2 y 31 `PlanDay` secuenciales (`sequence_no` 1..N sin huecos ni repetidos).

#### Scenario: Crear plan válido
- **WHEN** se crea un plan con entre 2 y 31 días, `sequence_no` cubriendo `1..N` sin repetidos
- **THEN** el sistema lo crea y devuelve `days` embebido, ordenados por `sequence_no`

#### Scenario: Cantidad de días fuera de rango
- **WHEN** se crea o edita un plan con menos de 2 o más de 31 días
- **THEN** el sistema responde `422`

#### Scenario: sequence_no con hueco o repetido
- **WHEN** los `sequence_no` de los días no cubren exactamente `1..N` (hueco o valor repetido)
- **THEN** el sistema responde `422`

### Requirement: Combinación kind/other_name/session_id coherente por día

El sistema SHALL validar que `kind=training` tenga `session_id` no nulo y `other_name` nulo; `kind=other` tenga `other_name` no nulo y `session_id` nulo; `kind=rest` tenga ambos nulos.

#### Scenario: training sin session_id
- **WHEN** un día tiene `kind=training` sin `session_id`
- **THEN** el sistema responde `422`

#### Scenario: other sin other_name
- **WHEN** un día tiene `kind=other` sin `other_name`
- **THEN** el sistema responde `422`

### Requirement: Campos default_* de sesión presencial

El sistema SHALL exigir `default_time` y `default_location` no nulos únicamente cuando `default_presencial=true`, limpiando ambos a `null` cuando es `false`.

#### Scenario: default_presencial true sin horario/ubicación
- **WHEN** un día tiene `default_presencial=true` sin `default_time` o sin `default_location`
- **THEN** el sistema responde `422`

### Requirement: PUT reemplaza el conjunto de días entero

El sistema SHALL reemplazar completamente las filas de `PlanDay` de un plan cuando `days` viene en el body de un `PUT`, revalidando todas las reglas de esta spec sobre el set completo nuevo.

#### Scenario: PUT parcial sin tocar days
- **WHEN** se edita un plan enviando solo `name`/`description`, sin `days`
- **THEN** el sistema actualiza únicamente esos campos, sin tocar los `PlanDay` existentes

### Requirement: Borrado físico sin cascada real a calendarios

El sistema SHALL borrar físicamente un `TrainingPlan` y sus `PlanDay` al eliminarlo, sin afectar filas de calendario de grupo que lo hayan estampado en el pasado (esas filas ya copiaron los datos físicamente — ver `calendario-asignacion-grupos`).

#### Scenario: Borrar plan ya estampado en algún calendario
- **WHEN** se borra un plan que fue usado para estampar el calendario de al menos un grupo
- **THEN** el sistema borra el plan y sus días sin error, dejando las filas de calendario ya existentes intactas

### Requirement: Clonar un plan

El sistema SHALL permitir clonar un plan (`POST /training-plans/{id}/clone`) con copia profunda de sus `PlanDay`.

#### Scenario: Clonar plan
- **WHEN** se clona un plan existente
- **THEN** el sistema crea uno nuevo con copia profunda de todos sus `PlanDay` (mismo `session_id` donde aplique), `name` con sufijo `" (copia)"`
