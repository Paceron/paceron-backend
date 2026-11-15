# Proposal

## Why

La ronda 1 de optimizaciones redujo lecturas en calendario, presencial y feedback, pero quedaron patrones internos de lecturas secuenciales en la autorización de calendario, el roster de equipo, el guard de colisiones presenciales y el listado de equipos con filtros `owner_id` + `member_id`. No hay cambios de contrato pendientes; el costo es de round-trips internos.

## Objetivo

Reducir una consulta por operación en esos cuatro flujos, preservando exactamente payloads, autorización, orden visible, timezone, errores HTTP y semántica de soft-delete, con tests de paridad y conteo de queries.

## Alcance

- Unir `group` + `team` en la autorización de calendario mediante un DAO con join.
- Derivar la membresía del caller del roster en `GetUsersByTeam`.
- Reusar `FindByOwnerIDWithTeamName` en `findPresencialCollisions`.
- Filtrar `owner_id` + `member_id` en SQL con `EXISTS` en el listado de equipos.
- Agregar/adaptar mocks y tests de query-count/paridad.

## What Changes

- `GroupDaoInterface` gana una lectura de grupo + owner de equipo; `isGroupOwner` y `isGroupOwnerOrMember` la usan.
- `team_user_service.GetUsersByTeam` lee el roster una vez y autoriza al caller desde esas filas.
- `findPresencialCollisions` construye grupos y nombres de equipo desde la misma consulta.
- `TeamDao` gana una consulta `owner_id` + `member_id` con `EXISTS` y el servicio la usa cuando ambos filtros están presentes.
- Tests de servicios/DAOs y mocks actualizados.

## No alcance

- No cambiar endpoints, DTOs, JSON, status codes, autorización, orden, timezone, errores, soft-delete, frontend, Swagger, schema, índices ni migraciones.
- No fusionar membresías con días en `MemberCalendar`/`AdministeredCalendar`.
- No agregar cachés ni instrumentación de producción.
- No tocar la data del corredor 1 en este change.

## Capabilities

Ninguna: cambio interno de performance sin cambio de comportamiento; `skip_specs: true`.

## Métrica de éxito

- Auth de calendario: owner 2→1 query; miembro 3→2; `GetRange` 5→4 owner y 6→5 miembro.
- `GetUsersByTeam`: 3→2 queries.
- `findPresencialCollisions`: 3→2 queries por lote.
- `GetAll` con ambos filtros: 3→2 queries; sin duplicados.
- Tests de paridad y query-count pasan; `go test -count=1 ./...`, `go vet ./...`, `go build ./...`, `openspec validate ... --strict`, `git diff --check` y coverage ≥85%.

## Impact

Servicios/DAOs de calendario y equipos, interfaces y mocks de tests. Sin impacto de contrato ni frontend; sin migración de datos.
