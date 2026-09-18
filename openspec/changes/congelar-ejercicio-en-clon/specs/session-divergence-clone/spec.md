## ADDED Requirements

### Requirement: Congelamiento profundo incluye el contenido del `Exercise`, no solo la `Session`

El sistema SHALL clonar también cada `Exercise` referenciado por `SessionExercise` cuando congela una `Session` por cierre de día (D8 manual o D13 automático), y SHALL repuntear el `SessionExercise` clonado al `Exercise` clonado en vez del original. El clonado manual explícito (`POST /sessions/{id}/clone`) SHALL seguir compartiendo el mismo `exercise_id` que el original, sin cambios.

#### Scenario: Editar un ejercicio de una sesión con un día ya cerrado no debe afectar el histórico
- **GIVEN** una `Session` con un `Exercise` "Correr 100mts" asignada a un `GroupCalendarDay` ya pasado (cerrado)
- **WHEN** se edita ese `Exercise` a "Correr 50mts" vía `PUT /exercises/{id}`
- **THEN** el día cerrado queda apuntando (a través de un clon de sesión) a una copia de "Correr 100mts", con el valor original intacto

#### Scenario: Editar un ejercicio de una sesión con solo días abiertos sigue en vivo
- **GIVEN** una `Session` con un `Exercise` asignado únicamente a `GroupCalendarDay` futuros (abiertos)
- **WHEN** se edita ese `Exercise`
- **THEN** no se crea ningún clon, y todos los días siguen viendo el valor editado en vivo

#### Scenario: Ejercicio usado en múltiples sesiones con distinto estado
- **GIVEN** un `Exercise` referenciado por la `Session` A (con un día cerrado) y la `Session` B (con solo días abiertos)
- **WHEN** se edita ese `Exercise`
- **THEN** solo la `Session` A se clona (con todos sus ejercicios, no solo el editado) y su día cerrado se repuntea; la `Session` B sigue apuntando al `Exercise` original editado, sin clon

#### Scenario: Sesión con un día cerrado y otro abierto para el mismo `Exercise`
- **GIVEN** la misma `Session` asignada a un `GroupCalendarDay` cerrado en el Grupo 1 y a un `GroupCalendarDay` abierto en el Grupo 2
- **WHEN** se edita un `Exercise` de esa sesión
- **THEN** solo el día del Grupo 1 se repuntea a la sesión clonada; el día del Grupo 2 sigue apuntando a la sesión original con el ejercicio editado
