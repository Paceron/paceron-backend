## Why

Con `feature/protect-all-endpoints` mergeado, la sesión ya es real (`AuthMiddleware()` valida el token en todas las rutas de negocio). Eso desbloquea el gap 1 documentado en `paceron-frontend/docs/BACKEND_API_GAPS.md`: no hay forma de sugerir usuarios ya registrados al invitar a un equipo — hoy el campo de invitar es un input de email libre, sin autocompletar, porque solo existe `GET /auth/user?id=`/`?email=` (lookup exacto, no búsqueda parcial).

## What Changes

- Nuevo endpoint `GET /api/v1/users/search?q=<texto>`: cualquier usuario logueado, sin restricción de rol adicional (no hay una relación previa entre quien busca y a quién busca, a diferencia de los patrones self-only/solo-entrenador de la rama anterior).
- Busca coincidencia parcial case-insensitive en `name`, `surname` o `email`, solo entre usuarios `status = active`.
- `q` exige mínimo 3 caracteres (recortando espacios) → `400` si no llega.
- Devuelve hasta 5 resultados. Cada resultado trae únicamente `user_id`, `name`, `surname`, `email` — los datos discretos necesarios para mostrar la sugerencia y disparar la invitación (`InviteRunnerRequest` ya toma `email`). Deliberadamente sin DNI, teléfono, dirección ni otro dato sensible del perfil completo.

## Capabilities

### New Capabilities
- `user-search`: búsqueda de usuarios activos por coincidencia parcial de nombre/apellido/email, para autocompletar al invitar.

## Impact

- **Nuevo**: `domains/user/search_response.go` (`SearchResultItem`, `SearchResponse`)
- **Modificado**: `daos/user_dao.go` (+`SearchActive`, +test), `services/user_service.go` (+`Search`, +test), `controllers/user_controller.go` (+`Search`, +test), `app/url_mappings.go` (ruta nueva, detrás de `AuthMiddleware()`)
- **Swagger**: 1 endpoint nuevo, regenerado
- **Docs**: `README.md` (tabla de endpoints), `docs/AUTH_MIGRATION.md` (sección nueva)
- **Sin cambios**: `app/app.go` (mismo `userDao`/`userService` ya inyectados, sin dependencias nuevas)
