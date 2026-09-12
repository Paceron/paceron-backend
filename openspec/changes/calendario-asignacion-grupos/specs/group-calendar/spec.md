## ADDED Requirements

### Requirement: El calendario de un grupo es la asignación (sin entidad intermedia)

El sistema SHALL modelar la asignación de un plan a un grupo únicamente como filas dispersas de `GroupCalendarDay` (una por fecha con contenido) — sin ninguna entidad de "asignación" con ciclo de vida propio.

#### Scenario: Consultar calendario de un grupo
- **WHEN** el entrenador dueño o un corredor miembro pide `GET /groups/{id}/calendar?from=...&to=...`
- **THEN** el sistema devuelve solo las fechas del rango que tienen fila cargada, ambos límites inclusive

#### Scenario: Usuario ajeno intenta leer el calendario
- **WHEN** un usuario que no es dueño ni miembro del grupo pide `GET /groups/{id}/calendar`
- **THEN** el sistema responde `403`

### Requirement: Escritura de calendario restringida al entrenador dueño

El sistema SHALL rechazar con `403` cualquier operación de escritura (`PUT`/`DELETE`/`stamp`/`bulk`/`bulk-clear`/`shift`) de un usuario que no sea el entrenador dueño del grupo, incluyendo corredores miembros.

#### Scenario: Corredor miembro intenta editar el calendario
- **WHEN** un corredor miembro del grupo (no dueño) intenta `PUT /groups/{id}/calendar/{date}`
- **THEN** el sistema responde `403`

### Requirement: Validación de contenido de un día de calendario

El sistema SHALL validar, en `PUT`/`stamp`/`bulk`, que `kind=other` tenga `other_name`, `kind=training` tenga `session_id`, `kind=cancelled` tenga `cancelled_reason` y solo sea alcanzable transicionando desde `kind=training`, y que `is_presencial=true` implique `presencial_time`/`presencial_location` no nulos (limpiándolos si es `false`).

#### Scenario: Cancelar un día de descanso
- **WHEN** se intenta `PUT` con `kind=cancelled` sobre un día que hoy es `rest` o no tiene fila
- **THEN** el sistema responde `422`

#### Scenario: Cancelar un día de entrenamiento
- **WHEN** se hace `PUT` con `kind=cancelled` y `cancelled_reason` sobre un día que hoy es `kind=training`
- **THEN** el sistema acepta la transición y mantiene el `session_id` original como contexto

### Requirement: Estampar un plan sobre el calendario

El sistema SHALL copiar cada `PlanDay` de un `TrainingPlan` a filas de `GroupCalendarDay` a partir de `start_date` (día 1 → `start_date`, día N → `start_date + N - 1`), validando que el plan pertenezca al mismo entrenador dueño del grupo, y SHALL rechazar el estampado completo con `409` (listando las fechas en conflicto) si algún día del rango ya tiene contenido, salvo que `force=true`.

#### Scenario: Estampar sin conflictos
- **WHEN** el entrenador dueño estampa un plan de 5 días a partir de una fecha donde ninguno de los 5 días de destino tiene contenido previo
- **THEN** el sistema crea las 5 filas de `GroupCalendarDay` con `source_plan_id` apuntando al plan

#### Scenario: Estampar con conflicto sin force
- **WHEN** al menos 1 de los días de destino ya tiene contenido y `force` no viene o es `false`
- **THEN** el sistema responde `409` con la lista de fechas en conflicto, sin crear ni modificar ninguna fila

#### Scenario: Estampar con force sobreescribe
- **WHEN** hay conflicto y se manda `force=true`
- **THEN** el sistema reemplaza el contenido de las fechas en conflicto con el del plan

#### Scenario: Estampar un plan ajeno
- **WHEN** el entrenador dueño del grupo intenta estampar un plan cuyo `owner_id` es otro entrenador
- **THEN** el sistema responde `403`, sin leer siquiera los días del plan

### Requirement: Operaciones en lote sobre el calendario

El sistema SHALL exponer `bulk` (mismo contenido a N fechas), `bulk-clear` (borra N fechas) y `shift` (corre todas las fechas desde una fecha dada por N días), todas exclusivas del entrenador dueño.

#### Scenario: Bulk-assign a varias fechas
- **WHEN** el entrenador dueño manda `bulk` con 3 fechas y `kind=rest`
- **THEN** el sistema crea o reemplaza las 3 filas con ese contenido

#### Scenario: Shift sin colisión
- **WHEN** el entrenador dueño corre todas las fechas desde una fecha dada 2 días hacia adelante, sin que ninguna coincida con una fecha ya ocupada fuera del rango corrido
- **THEN** el sistema actualiza las fechas de todas las filas afectadas

#### Scenario: Shift con colisión
- **WHEN** correr las fechas haría coincidir dos filas en la misma fecha
- **THEN** el sistema responde `409`, sin mover ninguna fila
