## Why

`calendario-asignacion-grupos` (D8/D13) y `congelar-ejercicio-en-clon` resuelven la inmutabilidad histórica de una asignación calculando, **en el momento de editar** una `Session`/`Exercise` del catálogo, si algún día de calendario que la referencia ya está "cerrado" — y si lo está, clonando sesión+ejercicios para que ese día no reciba la edición. Decisión de equipo (2026-09-19): este mecanismo reactivo es más complejo de razonar y más frágil de lo que amerita — depende de acertar el disparo correcto en *cada* punto de edición del catálogo (ya se encontró un gap real: el primer intento clonaba la `Session` pero no los `Exercise`, ver `congelar-ejercicio-en-clon`), y cualquier futuro punto de edición nuevo del catálogo necesitaría repetir el mismo chequeo o reabre el bug.

**Se reemplaza por un mecanismo proactivo**: el catálogo (`Exercise`/`Session`/`TrainingPlan`) queda 100% template, nunca leído en vivo por una asignación ya guardada. Asignar contenido a un día de calendario **instancia** — copia el contenido a tablas propias, inmutables, en el momento de guardar la asignación, no en el momento de editar el catálogo después. Sin cálculo de "¿está cerrado?" para decidir si clonar: simplemente nunca hay nada que clonar, porque nunca hubo una referencia viva que romper.

Esto también alinea con un diseño ya anticipado: `workout_feedback.assigned_session_id`/`assigned_exercise_id` (feature ya mergeada, `feature/feedbackExcercise`) son FK opacas a propósito, documentadas como "las tablas de asignación (`assigned_*`) no existen aún" — este change es esa pieza faltante.

## What Changes

- **Nuevas tablas de instancia**: `session_instances`, `session_exercise_instances`, `exercise_instances` — mismo shape que sus contrapartes de catálogo (`Session`/`SessionExercise`/`Exercise`), sin `owner_id` (no son catálogo de nadie) ni `deleted_at` (no se editan ni se soft-borran, solo se crean y eventualmente se borran físicamente).
- **`GroupCalendarDay.session_id` → `GroupCalendarDay.session_instance_id`**: deja de apuntar al catálogo, pasa a apuntar a un `SessionInstance`. Se dropea la columna vieja (mismo criterio que `presencial-time-from-to`: sin backfill posible, no hay dato real que migrar).
- **Instanciación en cada punto de escritura que asigna contenido** (`PUT /groups/{id}/calendar/{date}` con `kind=training`, `POST .../bulk`, `POST .../stamp`): en vez de guardar el `session_id` recibido tal cual, el service resuelve la `Session` + `SessionExercise` + `Exercise` del catálogo en ese instante, crea una `SessionInstance` nueva con sus `SessionExerciseInstance`/`ExerciseInstance` (una instancia por día, **sin deduplicar** aunque el mismo `session_id` se asigne a 10 días en el mismo `save` — decisión explícita: más filas, cero acoplamiento entre días), y guarda `session_instance_id` en la fila de calendario.
- **Se elimina por completo** el mecanismo de D8 (`exclude_group_ids`/`clone_name`/`clone_description` en `PUT /sessions/{id}`), D13 (`isCalendarDayClosed` como trigger de clonado en `SessionService.Update`) y `congelar-ejercicio-en-clon` (mismo trigger en `ExerciseService.Update`, deep-clone de ejercicios). Código y tests correspondientes se borran, no quedan conviviendo con el mecanismo nuevo.
- **`isCalendarDayClosed` se reusa pero cambia de rol**: pasa de "¿clono o dejo pasar la edición?" a **guard de escritura en el calendario**: reasignar contenido (`training`) a un día ya cerrado (pasado, presencial ya empezada, o async del día actual) responde `422`/`409`, no se permite en absoluto — ver D2. Se muda de `session_service.go` a `calendar_service.go` (ahí es donde ahora tiene sentido de dominio).
- **Borrado de instancia superada**: al reasignar un día que ya tenía contenido (siempre sobre un día futuro, por la regla anterior), la `SessionInstance`/`ExerciseInstance` vieja se borra físicamente **salvo que algún `workout_feedback` ya la referencie** — chequeo explícito antes de borrar, aunque el caso sea improbable dado que la reasignación solo aplica a futuro (no debería haber feedback todavía). Ver D3.
- **`PUT /sessions/{id}` y `PUT /exercises/{id}` vuelven a ser ediciones simples de catálogo**, sin ningún chequeo de calendario — el body de `PUT /sessions/{id}` pierde `exclude_group_ids`/`clone_name`/`clone_description`.

## Capabilities

### Modified Capabilities

- `group-calendar` (de `calendario-asignacion-grupos`): el contenido de un día pasa de ser una referencia viva al catálogo a ser una instancia congelada; nuevo guard de "día cerrado" en escritura.

### Removed Capabilities

- `session-divergence-clone` (de `calendario-asignacion-grupos`, extendida por `congelar-ejercicio-en-clon`): reemplazada enteramente por instanciación — no queda ningún mecanismo de clonado por divergencia, porque no hay nada vivo que diverja.

## Non-Goals

- Tracking de completado/actividad realmente ejecutada — sigue fuera de alcance, sin cambios (ver `calendario-asignacion-grupos` §7 y `docs/CATALOGO_Y_CALENDARIO.md`).
- Wiring real de `workout_feedback.assigned_session_id`/`assigned_exercise_id` a `session_instances`/`exercise_instances` (reemplazar el FK opaco) — este change deja las tablas listas para eso, pero no toca `workout_feedback` ni sus servicios/controllers. Es un change aparte.
- Deduplicación de instancias entre días — decisión explícita de no hacerla (ver D4).
- Permitir asignar/reasignar contenido sobre días pasados de ninguna forma (ni siquiera para "cargar lo que realmente pasó") — sigue bloqueado, coherente con el non-goal de tracking de completado.
