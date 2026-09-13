## ADDED Requirements

### Requirement: CRUD de ejercicios del catálogo de un entrenador

El sistema SHALL permitir crear, leer, actualizar y borrar (lógicamente) ejercicios propiedad de un `owner_id`. `name`/`kind`/`owner_id` SHALL ser obligatorios; `description`, `intensity`, `minutes`, `distance_m`, `speed_kph` y `muscle_group` SHALL ser opcionales y válidos para cualquier combinación de `kind` (sin reglas cruzadas).

#### Scenario: Crear ejercicio mínimo
- **WHEN** se crea un ejercicio con solo `owner_id`, `name` y `kind`
- **THEN** el sistema lo crea con el resto de los campos opcionales en `null`

#### Scenario: Kind inválido
- **WHEN** se crea o edita un ejercicio con un `kind` fuera de la lista válida
- **THEN** el sistema responde `400` con `{"message": "..."}`

#### Scenario: Listado excluye borrados
- **WHEN** se lista `GET /exercises?owner_id={id}`
- **THEN** el sistema no incluye ejercicios con borrado lógico aplicado

#### Scenario: Ejercicio no encontrado
- **WHEN** se pide `GET/PUT/DELETE /exercises/{id}` de un id inexistente o ya borrado
- **THEN** el sistema responde `404`

### Requirement: Borrado lógico no rompe referencias existentes

El sistema SHALL marcar `deleted_at` en vez de borrar físicamente un ejercicio, para que sesiones que ya lo referencian sigan resolviendo correctamente.

#### Scenario: Ejercicio borrado sigue resolviendo en una sesión existente
- **WHEN** un ejercicio referenciado por una `SessionExercise` es borrado
- **THEN** `GET /sessions/{id}` de esa sesión sigue devolviendo el detalle completo del ejercicio, aunque ya no aparezca en `GET /exercises?owner_id={id}`

### Requirement: Clonar un ejercicio

El sistema SHALL permitir clonar un ejercicio (`POST /exercises/{id}/clone`) creando una copia con todos los campos iguales salvo `name` (sufijo `" (copia)"`) e `id`/timestamps nuevos.

#### Scenario: Clonar ejercicio
- **WHEN** se clona un ejercicio existente
- **THEN** el sistema crea uno nuevo con el mismo `owner_id` y campos, `name` con sufijo `" (copia)"`, sin afectar al original ni a sesiones que lo usaban
