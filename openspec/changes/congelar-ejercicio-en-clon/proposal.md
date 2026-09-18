## Why

`calendario-asignacion-grupos` (D8/D13) ya congela una `Session` completa cuando un `GroupCalendarDay` que la referencia está cerrado (pasado, presencial después de su horario, async del mismo día) — clona la sesión y repuntea el día para que la edición no le pise el contenido histórico.

Gap detectado (reportado por un compañero mientras encaraba la feature de feedback de ejercicios, `feature/feedbackExcercise`, ya mergeada): ese congelamiento clona la `Session` y sus filas `SessionExercise`, pero **no** clona el `Exercise` referenciado por cada una — el clon sigue apuntando al mismo `exercise_id` original. Si después se edita el `Exercise` directamente (`PUT /exercises/{id}`), el cambio pega igual sobre días ya "congelados", porque el contenido real (`distance_m`, `minutes`, etc.) nunca vivió copiado en ningún lado, solo la referencia.

Ejemplo del reporte: ejercicio "Correr 100mts" asociado a una sesión ya estampada en el calendario; un corredor/entrenador carga una actividad sobre ese día; a la semana siguiente se edita el ejercicio a "50mts" — el feedback ya cargado pierde sentido, porque el sistema ahora muestra 50mts para algo que, cuando se hizo, decía 100mts.

Además, `ExerciseService.Update` hoy no tiene ningún awareness de calendario — a diferencia de `SessionService.Update`, editar un `Exercise` nunca dispara ningún chequeo de días cerrados.

## What Changes

- **`cloneSessionInternal` clona también los `Exercise`** referenciados por cada `SessionExercise` cuando el clon es por congelamiento histórico (no cuando es un `POST /sessions/{id}/clone` manual — ese sigue siendo una copia "para editar", comparte catálogo a propósito, sin cambios).
- **`ExerciseService.Update` gana el mismo chequeo de días cerrados que ya tiene `SessionService.Update`** (D13): al editar un `Exercise`, busca todos los `GroupCalendarDay` que lo referencian transitivamente (vía `session_exercises.session_id`), agrupa por sesión, y por cada sesión con al menos un día cerrado, clona esa sesión (con sus ejercicios, congelamiento profundo) y repuntea solo esos días — los días abiertos de la misma sesión siguen en vivo.
- **Nuevo método de DAO**: `GroupCalendarDaoInterface.FindByExerciseID(exerciseID)` — join `group_calendar_days` × `session_exercises` por `session_id`, filtra por `exercise_id`.
- Sin cambio de schema, sin backfill: el bug lleva existiendo desde que se mergeó D13 pero **nunca hubo datos reales de feedback ni de calendario en producción** que dependieran de la protección faltante (la feature de feedback recién se mergeó, sin uso real todavía) — no hay nada que migrar.
- **Cero acoplamiento con `workout_feedback`** (feature ya mergeada en paralelo): `assigned_session_id`/`assigned_exercise_id` son FK opacas, sin validación ni join contra `sessions`/`exercises` — este fix no toca ningún archivo de esa feature.

## Capabilities

### Modified Capabilities

- `session-divergence-clone` (de `calendario-asignacion-grupos`): profundiza el congelamiento para incluir el contenido del `Exercise`, no solo el de la `Session`, y agrega `Exercise.Update` como segundo disparador (además de `Session.Update`).

## Non-Goals

- No se introduce la entidad `assigned_session`/`assigned_exercise` que anticipa el FK opaco de `workout_feedback` — eso es un change futuro y más grande (reemplazar el FK opaco por uno real). Este fix solo cierra el gap de integridad histórica en el catálogo, independiente de esa migración futura.
- No se agrega ninguna forma de "registrar lo que realmente se hizo" (logging de actividad) — sigue fuera de scope, igual que en D13.
