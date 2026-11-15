# Design

## Context

Las operaciones de calendario y sesiones presenciales combinan lecturas de membresías, grupos, equipos y días con consultas posteriores para resolver nombres y permisos. En varios caminos se leen filas que luego no se usan, se repiten lecturas de la misma fila antes y después de una transacción, o se resuelve el acceso de dos usuarios con consultas secuenciales. El guardado de feedback repite verificaciones de acceso y de pertenencia del ejercicio, y algunos updates vuelven a leer la fila que acaban de modificar.

El contrato HTTP, los DTO, los estados, la autorización, el orden visible y las reglas de zona horaria no cambian. La optimización se limita a reducir round trips y lecturas redundantes dentro de la capa de servicios/DAOs, con pruebas de paridad y de cantidad de llamadas.

## Goals / Non-Goals

**Goals:**

- Reducir consultas redundantes en `MemberCalendar`, `NextSession`, `NextPresencialSession` y `UpsertDay`.
- Reducir lecturas repetidas en el inicio/cierre de sesión presencial y en el create/finish de `runner_session`.
- Reducir verificaciones secuenciales al guardar feedback, sin alterar validaciones ni errores.
- Agregar tests de paridad funcional y de cantidad de llamadas para cada camino tocado.
- Mantener intactos payloads, autorización, transiciones, orden, timezone, soft-delete y códigos de error.

**Non-Goals:**

- No cambiar endpoints, DTOs, JSON, status codes, permisos ni comportamiento visible.
- No agregar índices, migraciones, columnas, cachés de request ni nuevas dependencias.
- No limpiar datos históricos de membresías; eso es una remediación separada.
- No refactorizar código ajeno a los caminos listados.

## Decisions

### D1. Resolver nombres de grupo/equipo solo para las filas devueltas

`NextSession` y `MemberCalendar` seguirán obteniendo primero el conjunto de días que corresponde al usuario, pero los nombres de grupo/equipo se consultarán únicamente para los `group_id` distintos que aparecen en ese conjunto. `NextPresencialSession` solo resolverá el equipo del día seleccionado, no el de todos los equipos del owner. Se conservan las cláusulas de orden y los fallbacks actuales.

Alternativa descartada: reemplazar todo por un join único de días + grupos + equipos. Reduce consultas, pero mezcla la lógica de selección de días con la de presentación y dificulta conservar exactamente los filtros y el orden actuales.

### D2. Reutilizar la semántica de membresía activa

Las consultas de membresías activas seguirán usando `date_end IS NULL OR date_end > NOW()`, además de `deleted_at IS NULL`. Cuando haga falta una membresía con datos de grupo/equipo, se preferirá una consulta que devuelva la membresía activa junto con esos datos antes que dos lecturas separadas. No se cambia el significado de "activo" ni se reescriben membresías vencidas.

Alternativa descartada: filtrar solo por `date_end IS NULL`. Cambiaría el comportamiento de membresías con fecha futura de fin y rompería la semántica ya usada por el fix de baja de equipos.

### D3. Una sola lectura de ownership para operaciones presenciales

`isDayTeamOwner` se reemplazará por una lectura que resuelva día, grupo, equipo y owner en una sola consulta, reutilizada dentro del mismo request de gate/open/close. Las actualizaciones de estado usarán `UPDATE ... RETURNING` para evitar la relectura posterior. Si la fila no existe o no cumple la transición, se mantienen los mismos errores y códigos actuales.

Alternativa descartada: cargar el equipo completo y validar en memoria después de `FindByID`. Sigue haciendo dos consultas y no evita la relectura posterior al update.

### D4. Batch de acceso para caller y atleta

En `runner_session` create/finish y en feedback create se reutilizará `HasInstanceAccessBatch` para resolver el acceso del caller y del atleta en una sola operación cuando ambos participan. Para el caso self, se evita la consulta duplicada del mismo usuario. La regla de autorización no cambia: owner del equipo, miembro del equipo o acceso directo a la instancia siguen siendo las únicas vías válidas.

Alternativa descartada: mantener `HasInstanceAccess` secuencial por usuario. Es correcto, pero multiplica round trips en los caminos de coach + atleta.

### D5. Unificar pertenencia de ejercicio y derivación de equipo en feedback

La creación de feedback combinará en una sola consulta la verificación de que el `exercise_instance` pertenece a la `session_instance` y la obtención del `team_id` aplicable. Se mantendrá el orden de validaciones y los errores actuales; si la instancia no tiene equipo, el feedback sigue siendo válido según las reglas vigentes.

Alternativa descartada: mantener `ExerciseBelongsToSessionInstance` y `TeamIDForSessionInstance` como consultas separadas. La segunda ya tiene hasta tres fallbacks secuenciales y es el principal costo del camino.

### D6. `UPDATE ... RETURNING` en las escrituras que hoy releen

`workout_feedback.Update`, el cierre de sesión presencial y el finish de `runner_session` devolverán la fila actualizada en la misma sentencia cuando el motor lo permita. La validación previa de existencia y permisos se conserva; solo se elimina la relectura posterior.

Alternativa descartada: dejar el update y el `GetByID` posteriores. Es simple, pero agrega una consulta por request sin aportar información nueva.

### D7. Tests de paridad y de cantidad de llamadas, sin instrumentación de producción

Los tests de servicio usarán los mocks existentes para afirmar que los caminos nuevos hacen menos llamadas y que los resultados siguen siendo equivalentes. Los tests de DAO contra Postgres verificarán la semántica de las consultas nuevas (membresías activas, nombres acotados, `RETURNING`, acceso batch y derivación de equipo). No se agrega logging ni contadores en código productivo.

Alternativa descartada: agregar un hook global de conteo de queries en producción. Sería útil para métricas, pero excede este change y puede afectar performance.

## Risks / Trade-offs

- [Un join nuevo puede duplicar o perder filas si hay grupos/equipos repetidos] → tests con grupos sin días, IDs repetidos y resultados vacíos; usar `DISTINCT` solo donde la consulta actual ya lo hacía.
- [El batch de acceso puede cambiar el resultado para owner, miembro, admin o usuario ajeno] → tests de paridad con cada rol y reutilización de la misma condición SQL de `HasInstanceAccess`/`HasInstanceAccessBatch`.
- [UPDATE ... RETURNING puede cambiar el comportamiento ante fila inexistente o transición inválida] → tests de not-found y de conflicto de estado; conservar los errores actuales.
- [Los tests de cantidad de llamadas pueden ser frágiles] → afirmar cotas y llamadas a métodos de mock, no SQL literal ni tiempos.
- [Cambiar el orden de lecturas puede alterar el orden visible] → conservar `ORDER BY` y agregar tests de paridad de slices antes/después.
- [La derivación de equipo del feedback tiene fallbacks] → cubrir sesión con equipo directo, grupo, plan y sin equipo, y un caso cross-team.

## Migration Plan

Cambio solo de código backend. No hay migración de datos ni de schema. El deploy es compatible hacia atrás porque los payloads no cambian. Rollback: revertir el commit del change; no hay datos que restaurar.

## Open Questions

Ninguna.
