## Why

Las lecturas de calendario, banners, equipos, invitaciones y push del cierre todavía hacen trabajo redundante: detalle de instancias en 3 consultas, mapas de ejercicios reconstruidos por sesión, `NextSession` con múltiples queries, listados que cargan invitaciones vencidas y lookups por fila. La auditoría de performance posterior a PR #117 confirmó mejoras seguras sin cambiar contratos.

## What Changes

- Calendario: construir el índice de ejercicios una vez por colección y resolver links+ejercicios en una sola consulta, manteniendo orden y shape.
- `NextSession`: reducir las consultas de banners agrupando kinds y resolviendo nombres/instancias en batch.
- Índices P1 para caminos de autorización/scope: `group_users(group_id,user_id)`, `team_users(team_id,user_id)`, `groups(team_id)`, `exercise_instances(source_exercise_id)`.
- Push de cierre: timeout por lote y chunking de `FindByUserIDs`.
- Equipos/invitaciones: filtrar invitaciones vencidas en SQL y eliminar N+1 en `filterByMember` y alta de equipo.

## Impact

- Sin cambios de endpoints, DTOs, shapes ni autorización.
- Solo tooling interno y consultas; los tests de contrato deben seguir verdes.
- Deuda restante (caché de samples, límites de búsqueda, rachas históricas, `FindSeries`, `WithContext`, etc.) se documenta, no se implementa acá.

## Capabilities

- `performance-reads` (interna; `skip_specs: true`, sin cambio de comportamiento observable).
