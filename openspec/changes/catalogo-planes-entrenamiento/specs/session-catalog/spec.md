## ADDED Requirements

### Requirement: CRUD de sesiones con ejercicios por rol

El sistema SHALL permitir crear, leer, actualizar y borrar (lógicamente) sesiones propiedad de un `owner_id`, cada una con una lista de `SessionExercise` (rol `warmup`/`main`/`cooldown`, `repeat_count`, `rest_minutes`). El sistema SHALL exigir al menos 1 ejercicio de cada uno de los 3 roles al crear o editar.

#### Scenario: Crear sesión válida
- **WHEN** se crea una sesión con al menos 1 ejercicio de cada rol (`warmup`, `main`, `cooldown`)
- **THEN** el sistema la crea y devuelve `exercises` embebido en la respuesta

#### Scenario: Falta un rol
- **WHEN** se crea o edita una sesión sin ningún ejercicio de alguno de los 3 roles
- **THEN** el sistema responde `422` con `{"message": "..."}`, sin crear/modificar nada

#### Scenario: Ejercicio referenciado no existe
- **WHEN** algún `exercise_id` de la lista no existe (o está borrado lógicamente)
- **THEN** el sistema responde `422`, sin crear/modificar nada

### Requirement: PUT reemplaza el conjunto de ejercicios entero

El sistema SHALL reemplazar completamente las filas de `SessionExercise` de una sesión en cada `PUT`, no aplicar un patch fila por fila, preservando el orden de inserción del array recibido.

#### Scenario: PUT con lista distinta
- **WHEN** se edita una sesión con una lista de `exercises` distinta a la actual (algunos removidos, otros agregados)
- **THEN** el sistema borra las filas de `SessionExercise` anteriores y crea exactamente las del nuevo array, en el orden recibido

### Requirement: Borrado lógico y clonado

El sistema SHALL aplicar el mismo criterio de borrado lógico que `Exercise` (deja de listarse, sigue resolviendo donde ya se usaba) y SHALL permitir clonar una sesión con copia profunda de sus `SessionExercise`.

#### Scenario: Clonar sesión
- **WHEN** se clona una sesión existente
- **THEN** el sistema crea una nueva con copia profunda de todas sus `SessionExercise` (mismos `exercise_id`, mismo `role`/`repeat_count`/`rest_minutes`), `name` con sufijo `" (copia)"`
