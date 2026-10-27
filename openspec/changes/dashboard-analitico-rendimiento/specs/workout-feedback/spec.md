# Spec Delta

> Este change **normaliza el formato de la spec baseline** de `workout-feedback`: le agrega
> el encabezado `# Spec`, `## Purpose` y `## Requirements`, y con eso sus 7 requisitos
> pasaron de **0 requirements detectados** por el tooling a 7 parseables.
>
> Aun así, el índice se declara como requisito **nuevo y aditivo** en lugar de `MODIFIED`,
> por dos razones: el baseline ya contenía la lista de índices dentro del texto de un
> requisito largo, y una delta `MODIFIED` obligaría a reescribir ese requisito completo para
> sumar seis palabras. Agregar un requisito es más barato y más legible de revisar que
> re-declarar un párrafo de 900 caracteres.

## ADDED Requirements

### Requirement: La tabla de feedback SHALL tener índice por sesión y fecha

La tabla `workout_feedback` SHALL tener un índice parcial `idx_feedback_session_date` sobre `(assigned_session_id, session_date)` con la condición `WHERE deleted_at IS NULL`.

El índice SHALL crearse de forma idempotente y sin downtime, mediante `CREATE INDEX CONCURRENTLY IF NOT EXISTS` ejecutado fuera de un bloque transaccional. El sistema SHALL ofrecer su eliminación correspondiente (`DROP INDEX CONCURRENTLY IF EXISTS`) en el runbook de rollback.

El índice existe porque las consultas analíticas filtran **simultáneamente** por sesión y por rango de fechas, y el índice vigente de `assigned_session_id` no incluye `session_date`: sin este índice el rango de fechas se filtra en memoria después del join.

Ser parcial (`deleted_at IS NULL`) es lo correcto porque **todas** las consultas del dominio filtran por baja lógica, y los registros dados de baja solo ocuparían espacio en el índice.

#### Scenario: La migración crea el índice

- **WHEN** se corre la inicialización de la base de datos
- **THEN** existe el índice `idx_feedback_session_date` sobre `workout_feedback (assigned_session_id, session_date)` con la condición `WHERE deleted_at IS NULL`

#### Scenario: La inicialización es idempotente

- **WHEN** se corre la inicialización dos veces
- **THEN** la segunda no falla y no altera los índices existentes

#### Scenario: El índice se usa para filtrar por rango

- **WHEN** se consulta el feedback de una sesión dentro de un rango de fechas sobre un volumen que no entra en memoria
- **THEN** el plan de ejecución usa `idx_feedback_session_date` en lugar de un escaneo secuencial

### Requirement: El historial de sesión tiene índice por corredor

`runner_session` SHALL tener un índice sobre `athlete_user_id`.

El UNIQUE vigente es `(session_instance_id, athlete_user_id)`, cuyo prefijo es la sesión: sirve para "dame la corrida de esta sesión", pero **no** para "dame el historial de este corredor", que es la consulta que hace el dashboard. Sin este índice, cada KPI del front es un seq scan sobre `runner_session`.

El índice SHALL ser `CREATE INDEX CONCURRENTLY IF NOT EXISTS` —`ConfigDB` no abre transacciones, así que `CONCURRENTLY` es viable— sin columna adicional ni predicado, y su rollback SHALL ser `DROP INDEX CONCURRENTLY IF EXISTS`.

#### Scenario: Consulta del historial por corredor

- **WHEN** el dashboard pide los indicadores de un corredor
- **THEN** la consulta filtra por `athlete_user_id` usando el índice, no por el prefijo de sesión del UNIQUE

#### Scenario: El índice no altera el modelo

- **WHEN** se aplica la migración
- **THEN** no se modifica ninguna tabla ni columna: el cambio es reversible con el `DROP` del runbook
