# Design

## Context

Ver `proposal.md`. La auditoría es estática: cualquier mejora de query-plan debe demostrar paridad y reducción del trabajo con fixtures medibles. El diseño mantiene los endpoints, DTOs, JSON y autorización actuales.

## Goals / Non-Goals

**Goals:**
- Quitar trabajo repetido dentro de una misma request o evento.
- Reducir scans, materialización de filas y fan-out por fila donde la equivalencia se pueda probar.
- Atribuir los puntos de `gps_N` al atleta del feedback asociado.
- Añadir pruebas de paridad, query-count y benchmarks limitados que protejan las métricas.

**Non-Goals:**
- Caché persistente o compartida entre requests para datos mutables.
- Cambiar límites, paginación, filtros, visibilidad, autorización o shapes de respuesta.
- Reescribir todo `FindSeries` a SQL, incrementalizar rachas/GPS o crear índices sin prueba de plan.
- Ejecutar carga o benchmarks contra local/demo/producción.

## Decisions

### D1. GPS `gps_N` atribuido al atleta del feedback

La consulta debe enlazar `workout_feedback_points.feedback_id` con `workout_feedback.id` y exigir `workout_feedback.athlete_user_id` igual al atleta de la runner session finalizada. Se conserva `COUNT(DISTINCT session_instance_id)` y **no** se agrega un filtro de `deleted_at`: la decisión vigente de contar puntos históricos de feedback soft-deleted permanece. El cambio solo evita que los puntos de otro atleta en una instancia compartida otorguen logros al evaluado.

### D2. `DataQuality` sin construir el resumen completo

Extraer un cálculo puro que reutilice `dataQualityFromSeries` y los mismos validadores actuales. Usarlo en Leaderboard, ExerciseSeries, Comparisons, Periods y al final de TimeSeries; `BuildSummary` continúa armando el resumen completo para su endpoint. `open_sessions` sigue viniendo de la misma consulta.

### D3. Eliminar duplicación dentro de la validación del dashboard

`Comparisons` no repite `validateAthletes` después de `validate`, que ya lo ejecutó. Al resolver `group_id`, usar el grupo presente en `DashboardScope` si está; consultar el DAO solo cuando no está para mantener la distinción existente entre 404 inexistente y 403 fuera de scope. `SessionComparison` podrá compartir las lecturas comunes entre sus dos lados, pero debe conservar el orden de errores A antes de B.

### D4. `ScopeDateRange` con puerta de evidencia

La consulta actual define el rango desde feedback activo, rol `main`, runner statuses permitidos y membresía. Cualquier reescritura debe conservar esos predicados, incluidos los feedbacks huérfanos que hoy admiten `rs.status IS NULL`, y el mismo resultado nulo cuando no hay filas. Solo se reemplaza si un test de paridad y una medición con fixture grande prueban menor trabajo; de lo contrario queda sin cambio y se registra como deuda de medición.

### D5. Reutilización request-local en competencias

Reutilizar el agregado metric y las listas/resultados route que ya se cargaron para calcular winners al construir la respuesta de `Close`. No crear cachés entre requests. Mantener el conjunto y orden de winners, stamps, unseen, notificaciones, frames WS y progreso devuelto. Las lecturas deben pertenecer al mismo snapshot lógico usado en el cierre.

### D6. Agregados y unlocks de logros

Combinar consultas que leen la misma fuente para un evento (runner counts y temprano; suma total y máximo por sesión) y agrupar los inserts de unlocks con `ON CONFLICT DO NOTHING RETURNING`. Solo IDs realmente insertados producen unseen y push, conservando el contenido y el orden actual del catálogo. Un error de batching debe seguir el manejo best-effort del evaluador sin duplicar ni perder unlocks por una caída parcial.

### D7. Evaluación de asistencia masiva por lote

`BulkSaveAttendance` conserva el mismo resultado HTTP y todos los eventos WS. Para logros, envía los user IDs afectados en una sola evaluación batch, calcula los días distintos en una consulta agrupada y procesa unlocks por usuario. Una actualización de asistencia puede seguir disparando la re-evaluación (reintento idempotente); no se descartan filas `updated` solo para reducir trabajo. Limitar la concurrencia a un trabajo asíncrono por lote, con contexto independiente y recover.

### D8. Borrado de un día y lectura de puntos GPS

`DeleteDay` reutiliza `deleteSupersededInstances` y su protección D10 dentro de la transacción actual. `BulkCreatePointsRecompute` proyecta solo orden, instante, latitud y longitud al recalcular; mantiene el recompute completo sobre el recorrido persistido para cubrir subidas parciales, reintentos e inserciones fuera de orden.

### D9. Medición de índices

No se agrega ni elimina ningún índice en esta change. Los candidatos de índice de la auditoría se mantienen en deuda hasta tener `EXPLAIN (ANALYZE, BUFFERS)` con datos representativos. El test DB vacío puede comprobar validez/uso posible, no estimar el beneficio productivo.

## Risks / Trade-offs

- [La corrección de `gps_N` reduce desbloqueos que antes podían atribuirse por puntos de otro atleta] → test de dos atletas en la misma instancia; no se revocan logros existentes.
- [SQL combinado puede alterar bordes o filtros] → tests de paridad en valores, errores y timezone antes de cambiar la consulta.
- [Batch async puede retrasar evaluaciones] → mantener un re-ejecutado por lote/usuario y probar que el estado final de progreso/unseen coincide con la secuencia individual.
- [La medición de `ScopeDateRange` puede no justificar un rewrite] → dejar la implementación actual y registrar evidencia, sin forzar una optimización.
- [El recompute de GPS completo puede seguir siendo O(puntos)] → el alcance solo reduce columnas; un algoritmo incremental necesita otro diseño.

## Migration Plan

No hay migraciones, endpoints nuevos ni cambios de wiring externo. Los optimizadores se despliegan junto con el binario. Rollback: revertir los commits internos; la corrección de `gps_N` vuelve a la atribución previa solo para evaluaciones futuras, sin cambiar filas de progreso existentes.

## Open Questions

No quedan decisiones de producto abiertas. Las consultas de índices y la reescritura de `ScopeDateRange` quedan condicionadas a evidencia dentro de las tareas.
