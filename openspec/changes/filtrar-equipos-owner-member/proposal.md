## Why

`GET /api/v1/teams` devuelve todos los equipos del sistema sin filtro (gap 1 de `paceron-frontend/docs/BACKEND_API_GAPS.md`). El frontend resuelve hoy "los equipos que administro" filtrando client-side el listado completo por `owner_id` — no escala, y no hay forma de resolver "los equipos donde participo como corredor" (no hay dato para eso del lado del cliente).

## What Changes

- `GET /api/v1/teams` acepta query params opcionales `owner_id` y `member_id`.
- Sin filtros: comportamiento actual sin cambios (todos los equipos activos).
- `owner_id`: equipos donde el usuario es `Team.OwnerID`.
- `member_id`: equipos donde el usuario tiene un `TeamUser` activo (cualquier rol).
- Si se pasan ambos, se aplican como AND (equipos administrados por `owner_id` donde además `member_id` es miembro) — caso poco común, resuelto filtrando en memoria sin agregar una query nueva.

## Capabilities

### Modified Capabilities
- `team-management`: `GetAll` gana filtrado opcional, sin romper el contrato existente (mismos campos de respuesta, mismo comportamiento sin query params).

## Impact

- **Modificado**: `daos/team_dao.go` (+2 métodos), `services/team_service.go` (`GetAll` cambia de firma), `controllers/team_controller.go` (parseo de query params), tests de las 3 capas.
- **Sin cambios**: modelo `dbs.Team`, resto de endpoints de teams.
- **Swagger**: mismo endpoint, agrega 2 query params documentados.

### Alcance

- Filtro por `owner_id` y/o `member_id` en `GET /api/v1/teams`.

### No alcance

- Endpoint dedicado `GET /users/:id/teams` — se descarta, los query params sobre el endpoint existente cubren el mismo caso de uso sin duplicar recurso.
- Paginación — fuera de este cambio, el volumen actual no lo justifica.

### Métrica de éxito

- El frontend puede pedir "mis equipos administrados" y "equipos donde participo" directamente al backend, sin filtrar client-side el listado completo.
