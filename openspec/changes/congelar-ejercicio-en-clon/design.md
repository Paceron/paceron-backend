> **Nota (2026-09-20):** este change nunca fue archivado, pero **fue reemplazado y su mecanismo eliminado** por `openspec/changes/asignacion-por-instanciacion/` (implementado): el congelamiento por divergencia en `ExerciseService.Update` ya no existe — el historial se preserva con copias inmutables (`ExerciseInstance`) creadas al momento de asignar, no con clonado reactivo al editar. El documento permanece tal cual se escribió, sin reescribir la historia.

## D1: El punto de congelamiento correcto es el cierre del día, no la asignación ni la carga de feedback

Se descartaron dos alternativas antes de llegar a esta:

- **Snapshot al cargar feedback**: falla porque el feedback puede cargarse después de que la actividad ocurrió (ej. al día siguiente) — si en el medio se edita el ejercicio, el snapshot ya captura el valor editado, no el que era cierto cuando se hizo la actividad.
- **Snapshot al asignar (stamp)**: rompe la edición en cascada que D8 garantiza — el catálogo debe poder seguir mandando en vivo sobre los días todavía abiertos entre el momento de asignar y el momento en que el día cierra.

El único punto que es simultáneamente correcto y no rompe nada existente es el mismo límite que ya define D13: el momento en que el día se considera cerrado (`isCalendarDayClosed`, sin cron, calculado al vuelo). Antes de ese límite: vive del catálogo. Al llegar: se congela lo que era cierto en ese instante. Un feedback cargado en cualquier momento posterior lee contenido congelado, sin importar cuándo se cargó ni qué se editó después.

## D2: Disparo lazy, no eager — se mantiene la filosofía "sin cron" de D13

El congelamiento no se computa proactivamente cuando un día cruza el límite de cierre (eso requeriría un cron o un job periódico, explícitamente descartado en D13). Se mantiene 100% reactivo: el chequeo de "¿hay algún día cerrado que dependa de este contenido?" corre únicamente cuando alguien edita algo que podría romper esa historia — hoy eso ya pasa en `SessionService.Update`; este change agrega el mismo chequeo a `ExerciseService.Update`.

Esto es correcto porque, mientras nadie edite nada, el contenido "cerrado" y el contenido "vivo" son el mismo dato — no hay divergencia que congelar todavía. La divergencia solo puede nacer en el instante de una edición, así que ese es el único momento donde hace falta actuar.

## D3: Profundizar el clon, no agregar una tabla de snapshots nueva

Alternativas consideradas para representar "el contenido congelado":

1. **Deep-clone del `Exercise`** (elegida): al congelar una sesión, clonar también cada `Exercise` que referencia vía `SessionExercise`, y apuntar el `SessionExercise` clonado al `Exercise` clonado en vez del original. Reusa toda la infraestructura ya construida (`cloneSessionInternal`, `RepointDaysByID`, transacciones con DAOs frescas atadas a `tx`) — solo agrega un nivel de profundidad al mismo mecanismo.
2. **Tabla de snapshot dedicada** (`exercise_snapshots` con copia de campos, sin ser una fila real de `exercises`): evita "ensuciar" el catálogo del entrenador con filas clonadas, pero requiere un modelo nuevo, una respuesta de API distinta para contenido congelado vs. vivo, y duplica la lógica de serialización que ya existe para `Exercise`. Se descarta por mayor superficie para el mismo resultado.

Se elige (1) por consistencia: es exactamente la misma decisión que ya se tomó para `Session` en D8/D13 (clonar una fila real de `sessions`, no una tabla de snapshot aparte). El precedente ya está aceptado y en producción.

### Efecto secundario aceptado: los clones aparecen en el listado del catálogo

Ya es el comportamiento actual de los clones de `Session` — `SessionDao.FindByOwner` no distingue clones de sesiones "reales", ambos aparecen en `GET /sessions?owner_id=`. Se extiende el mismo comportamiento a los `Exercise` clonados por congelamiento, por consistencia, sin agregar ninguna columna nueva tipo `is_historical_snapshot`. Con el volumen de uso actual (ninguno en producción todavía) esto no es un problema práctico; si en el futuro se vuelve ruidoso, es una mejora de UI (filtrar/agrupar en el listado), no un cambio de modelo.

## D4: `cloneSessionInternal` gana un flag para no romper el clonado manual existente

`cloneSessionInternal` es compartida hoy por dos caminos con semántica distinta:

- `POST /sessions/{id}/clone` (manual, "duplicar esta sesión para editarla") — **debe seguir compartiendo el mismo `exercise_id`** que el original, a propósito: es un punto de partida editable, no un congelamiento histórico. Cambiar esto rompería la spec existente (`exercise-catalog/spec.md`: "Clonar sesión: mismos exercise_id").
- El congelamiento por D8 (`exclude_group_ids`)/D13 (auto-cierre) dentro de `Update` — necesita el clon profundo nuevo.

Se agrega un parámetro `deepCloneExercises bool` a `cloneSessionInternal`: `false` para el clonado manual (comportamiento sin cambios), `true` para cualquier camino de congelamiento histórico (D8 manual, D13 automático por `Session.Update`, y el nuevo trigger por `Exercise.Update`).

## D5: Agrupar por sesión al disparar desde `Exercise.Update`

Un mismo `Exercise` puede estar en N sesiones distintas, cada una asignada a M grupos/días con distinto estado (abierto/cerrado). El flujo:

1. `GroupCalendarDaoInterface.FindByExerciseID(exerciseID)` — nuevo método, join `group_calendar_days` × `session_exercises` por `session_id`, filtrado por `exercise_id`. Devuelve **todos** los días (de cualquier sesión) que referencian ese ejercicio.
2. Se filtran los cerrados (`isCalendarDayClosed`, ya existente, reusado tal cual) y se agrupan por `session_id`.
3. Por cada sesión con al menos un día cerrado: clonar esa sesión completa (deep, D4) dentro de una transacción, y repuntear **solo esos día IDs específicos** (`RepointDaysByID`, no `RepointSessionForGroups` — mismo motivo que D13: un grupo puede tener la misma sesión en una fecha cerrada y otra abierta).
4. Sesiones sin ningún día cerrado no generan ningún clon — el `Exercise.Update` sigue el camino simple existente si `closedBySession` queda vacío.

## D6: Todos los ejercicios de la sesión se clonan, no solo el editado

Cuando una sesión se congela (por cualquiera de los 3 disparadores: D8 manual, D13 por edición de sesión, o este change por edición de un ejercicio), se clonan **todos** los `Exercise` que esa sesión referencia, no únicamente el que originó el disparo. Si se clonara solo el ejercicio tocado, los demás ejercicios de esa misma sesión-instancia congelada seguirían apuntando a filas vivas — reabriendo la misma clase de bug para cualquiera de sus "hermanos" que se edite después. Congelar la sesión-instancia completa (nombre + lista de ejercicios + contenido de cada ejercicio) es la semántica ya establecida por D13; este change solo la profundiza un nivel, no la cambia.

## D7: Sin backfill de datos

No hay ninguna fila existente que requiera corrección: el gap existe desde que D13 se mergeó, pero no hubo actividad real (ni feedback, ni ediciones de ejercicio sobre días ya cerrados) que dependiera de la protección faltante — la feature de feedback (`workout_feedback`) recién se mergeó en paralelo a este change, sin datos de uso todavía. No se escribe ningún script de migración.

## Reemplazado por `openspec/changes/asignacion-por-instanciacion/`

Decisión de equipo (2026-09-19): el deep-clone de `Exercise` que agrega este change, y el trigger en `ExerciseService.Update` que lo dispara, quedan sin efecto — se reemplazan por instanciación proactiva en el momento de asignar (no hay más referencia viva al catálogo que romper, así que no hace falta clonar nada al editar). Ver ese change para el mecanismo nuevo.
