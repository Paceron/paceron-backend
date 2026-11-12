# Proposal

## Why

La auditoría posterior a `optimizaciones-lecturas-cache-2` encontró trabajo repetido en métricas del dashboard, competencias y gamificación. También detectó que `gps_N` puede atribuir puntos de un corredor a otro en una sesión compartida. Estas mejoras pueden hacerse en backend sin modificar el contrato consumido por el frontend.

## Objetivo

Reducir trabajo de base de datos, CPU y asignaciones en cálculos de métricas, manteniendo los resultados existentes salvo la corrección de atribución de `gps_N` descrita en la spec delta.

## Alcance

- Atribuir los puntos GPS de `gps_N` al atleta del feedback que los contiene.
- Evitar cálculos completos de dashboard cuando solo se necesita `data_quality`, y eliminar validaciones/lecturas duplicadas dentro de una request.
- Reutilizar agregados y listas de intentos ya cargados durante consultas/cierres de competencias.
- Agrupar la evaluación de logros y asistencia masiva cuando se preserve la idempotencia, el resultado por usuario y el carácter best-effort.
- Reducir el trabajo del borrado de un día de calendario y las columnas leídas al recalcular métricas de puntos GPS.
- Añadir tests de paridad, query-count y benchmarks para comprobar las mejoras sin depender de cambios del frontend.

## What Changes

- Se corrige el criterio interno de `gps_N`: los puntos deben pertenecer a feedback del mismo atleta cuya sesión finalizada se evalúa. No se revocan logros existentes.
- Se separa el cálculo de calidad del resumen completo y se reutiliza información ya validada/resuelta durante cada request.
- Se elimina trabajo repetido en el cierre y progreso de competencias dentro de la misma request.
- Se reduce el fan-out por fila de la evaluación de logros/asistencias masivas, conservando los mismos unlocks, unseen y notificaciones.
- Se usan operaciones batch existentes para borrar instancias supersedidas por `DeleteDay` y se proyectan solo las columnas necesarias al recomputar métricas GPS.
- No se agregan ni modifican rutas, parámetros, DTOs, nombres JSON, status HTTP ni payloads WebSocket.

## No alcance

- Cambios en `paceron-frontend`, endpoints o shapes de respuesta.
- Límites/paginación nuevos en búsquedas o calendarios.
- Reescritura completa de `FindSeries` a SQL, estado incremental de rachas, recompute incremental de GPS o cachés persistentes de datos mutables.
- Índices nuevos sin evidencia de uso/rendimiento con datos representativos; no se dispone de un plan de producción en esta auditoría estática.
- Cambios en los flujos de pagos y en el endpoint público `/auth/user`.
- Cambiar el comportamiento histórico de puntos GPS asociados a feedback soft-deleted, salvo la atribución al atleta.

## Capabilities

### New Capabilities

- `gamification-achievements`: precisa que los puntos usados para evaluar logros GPS pertenecen al atleta evaluado. Aclara el evaluador `gps_N` ya implementado; no modifica el contrato HTTP de logros.

### Modified Capabilities

Ninguna. El resto de la propuesta optimiza implementación interna sin alterar comportamiento observable.

## Métrica de éxito

- Los tests demuestran que puntos de otro atleta en una instancia compartida no cuentan para `gps_N`, y que los puntos del atleta evaluado sí cuentan.
- Las respuestas de métricas existentes son iguales antes/después para fixtures equivalentes, excepto por la atribución correcta del logro GPS.
- Los query-counts o benchmarks muestran menos consultas/pasadas en los caminos optimizados; donde no haya una mejora reproducible, el código no se reescribe y el residual queda documentado.
- `go test -count=1 ./...`, build, vet y `openspec validate --strict` pasan; coverage mantiene el mínimo de 85% y no baja del valor base medido al iniciar la implementación.
- Cero cambios de endpoints, DTOs, JSON, autorización o frontend.

## Impact

Cambios internos en `dashboard`, `competition`, `gamification`, `workoutfeedback` y `calendar`, con tests de servicios/DAOs. Se agrega un escenario de spec delta para la atribución de GPS. Sin migraciones ni dependencias externas previstas.
