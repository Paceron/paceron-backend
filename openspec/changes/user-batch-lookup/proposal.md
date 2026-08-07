## Why

Gap 2 de `paceron-frontend/docs/BACKEND_API_GAPS.md`: `TeamUserResponse`/`GroupUserResponse` (roster de equipo/grupo) solo traen `user_id`, sin nombre ni email. Hoy el frontend resuelve cada corredor único con `GET /auth/user?id=` en loop (`hooks/use-team-roster.js`), un N+1 mitigado con cache de cliente pero no resuelto de raíz. No bloqueante, pero de baja complejidad y buen momento para cerrarlo (`feature/user-search-endpoint` ya sentó el patrón de DTO acotado).

## What Changes

- Nuevo endpoint `GET /api/v1/users?ids=1,2,3`: cualquier usuario logueado, sin restricción de rol adicional — mismo criterio que `/users/search` (todavía no hay una relación previa que autorizar; acá directamente el caller ya conoce los ids porque salen de un roster al que tiene acceso).
- Hasta 50 ids por llamada, separados por coma. `400` si falta el parámetro, algún id no es numérico, o se piden más de 50.
- A diferencia de `/users/search`, **no filtra por `status`** — un id de roster ya es un miembro conocido (activo, pausado, etc.), no un resultado de búsqueda arbitraria.
- Reutiliza el DTO `SearchResultItem` (`user_id`/`name`/`surname`/`email`) de `feature/user-search-endpoint`, envuelto en `BatchLookupResponse` — mismo shape acotado, sin duplicar el tipo.
- Ids que no existen simplemente no aparecen en `results` (no es un 404 por id individual).

## Capabilities

### New Capabilities
- `user-batch-lookup`: resolución de nombre/apellido/email para varios `user_id` en una sola consulta.

## Impact

- **Nuevo**: `domains/user/batch_lookup_response.go` (`BatchLookupResponse`)
- **Modificado**: `daos/user_dao.go` (+`FindByIDs`, +test), `services/user_service.go` (+`BatchLookup`, +test), `controllers/user_controller.go` (+`BatchLookup`, +test), `app/url_mappings.go` (ruta nueva, detrás de `AuthMiddleware()`)
- **Swagger**: 1 endpoint nuevo, regenerado
- **Docs**: `README.md` (tabla de endpoints), `docs/AUTH_MIGRATION.md` (sección nueva)
- **Sin cambios**: `app/app.go` (mismo `userDao`/`userService` ya inyectados)
