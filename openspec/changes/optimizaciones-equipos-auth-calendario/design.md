# Design

## Context

Cuatro flujos internos hacen lecturas secuenciales evitables. La ronda 1 ya introdujo helpers de batch y `UPDATE ... RETURNING`; este change continúa el mismo criterio sin cambiar contratos.

## Goals / Non-Goals

- Goal: bajar round-trips con consultas equivalentes y tests que prueben paridad.
- Goal: mantener auth, 404/403/409/500, orden, timezone y soft-delete.
- Non-goal: índices, cachés, migraciones, cambios de DTO/HTTP/frontend.

## Decisions

### D1 — Auth de calendario con grupo + owner en una query

Agregar a `GroupDaoInterface` un `FindWithTeamOwnerByID` con `LEFT JOIN teams` (`groups.team_id = teams.id`, ambos `deleted_at IS NULL`). `isGroupOwner` y `isGroupOwnerOrMember` lo usan. Grupo inexistente conserva `ErrCalendarGroupNotFound`; owner nil/distinto conserva `ErrCalendarForbidden`; error de DB conserva el genérico de auth.

Alternativa descartada: plegar también la membresía en la misma query. Ahorraría otra query al miembro, pero cambia el error de membresía y vuelve más delicada la revisión de seguridad.

### D2 — `GetUsersByTeam` autoriza desde el roster

Leer equipo, luego roster una vez (`FindByTeamID`) y buscar al caller por `UserID`. Si no está, devolver el mismo error de no-miembro (403). Hoy `FindByTeamAndUser` y `FindByTeamID` comparten `team_id` + `deleted_at IS NULL`; no hay `date_end` en `team_users`.

Alternativa descartada: mantener la lectura dedicada para autorizar; es la query que se elimina.

### D3 — Colisiones presenciales con nombres de equipo incluidos

Usar `FindByOwnerIDWithTeamName` para construir `groupByID` y `teamNameByID` desde las mismas filas. La consulta ya filtra grupos activos y hace `INNER JOIN teams` activo, igual que `FindByOwnerID`; el orden de conflictos sigue viniendo de `FindPresencialForGroupsInRange`.

Alternativa descartada: `FindWithTeamNameByIDs`, que agrega una query en vez de eliminarla.

### D4 — Listado owner + member con `EXISTS`

Agregar una consulta `TeamDao` que combine `teams.owner_id = ?`, `teams.deleted_at IS NULL` y `EXISTS (SELECT 1 FROM team_users ... user_id = ? AND deleted_at IS NULL)`. Usar `EXISTS` evita duplicados; no agregar `ORDER BY` porque el endpoint no lo garantiza.

Alternativa descartada: `JOIN` simple a `team_users`, que puede duplicar equipos.

## Risks / Trade-offs

- [Cambia el mensaje interno de un error de DB al unificar grupo/equipo] → El status/body HTTP no cambia; tests de error conservan el contrato.
- [La auth queda concentrada en una query] → Cubrir grupo inexistente, team nil, no-owner y error de DB.
- [D2 depende de que `team_users` no tenga vigencia] → Documentado; si se agrega `date_end`, revisar la equivalencia.
- [D4 con join puede duplicar] → Usar `EXISTS` y test de dos membresías activas.
- [Reordenar lecturas puede cambiar precedencia de errores] → Mantener el orden actual: equipo 404 primero, luego autorización.

## Migration Plan

No hay migración de datos ni schema. Rollback = revertir el commit; no hay cambios persistidos.

## Open Questions

Ninguna.
