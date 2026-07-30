## Why

El frontend tiene un toggle en "Editar equipo" para que el entrenador decida si los corredores ven a qué grupo pertenece cada compañero (gap 2 de `paceron-frontend/docs/BACKEND_API_GAPS.md`). Hoy esa preferencia no persiste — queda interactiva del lado del cliente y se pierde al recargar.

## What Changes

- Nuevo campo `show_groups_to_runners` (bool, default `false`) en `Team`.
- `POST /api/v1/teams` y `PUT /api/v1/teams/:id` aceptan el campo opcionalmente.
- `TeamResponse` lo expone en todas las respuestas de equipo.

## Capabilities

### Modified Capabilities
- `team-management`: `Team` gana un campo de configuración nuevo, sin romper el contrato existente (campo opcional en request, default `false`).

## Impact

- **Modificado**: `domains/dbs/team.go` (columna aditiva), `domains/team/{team_request,team_response,team_update_request}.go`, `services/team_service.go` (`Create`/`Update`/`toResponse`).
- **Sin cambios**: controllers (el bind de JSON ya pasa cualquier campo del body sin filtrar, mismo patrón que `create_default_group`), rutas.
- **Swagger**: mismo endpoint, campo nuevo documentado en request/response.

### Alcance

- Persistir y exponer la preferencia de visibilidad de grupos por equipo.

### No alcance

- Enforcement real de la visibilidad en otros endpoints (ej. ocultar el grupo de un compañero en `GET /teams/:id/users` si `show_groups_to_runners` es `false`) — el frontend hoy solo necesita persistir el toggle, la lógica de ocultamiento queda del lado del cliente por ahora. Si se necesita enforcement server-side más adelante, es un cambio aparte.

### Métrica de éxito

- El toggle de "Editar equipo" persiste entre sesiones.
