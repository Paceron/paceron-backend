## Why

`GET /user/:user_id` y `POST /user` (sin prefijo `/api/v1`, públicas) eran leftovers del template original del proyecto — confirmado leyendo el código, duplicaban `GET /api/v1/auth/user` y `POST /api/v1/auth/register`. `feature/protect-all-endpoints` las dejó públicas sin decidir su destino (documentado como deuda en `openspec/changes/protect-all-endpoints/design.md`, sección Risks). El usuario confirmó con el frontend que no están en uso — quedan libres para eliminar.

## What Changes

- Se eliminan las rutas `GET /user/:user_id` y `POST /user`.
- Se elimina `userController.GetUser`/`CreateUser` — el `userController.GetUser` del handler HTTP, no el método de `userService.GetUser` (ese sigue vivo, lo usa `userWeatherDelegate` para el demo de `/user/:user_id/weather`, que se mantiene).
- Se elimina `userService.CreateUser` y `userDao.Create` — sin otro caller.
- Se elimina `domains/user/user_request.go` (`CreateUserRequest`) — solo lo usaba el handler eliminado.
- `POST /user` guardaba la contraseña en texto plano (bug documentado, nunca corregido porque la ruta iba a eliminarse) — se va con el resto del código.

## Capabilities

### Removed Capabilities
- Ninguna capability formal de OpenSpec tenía estas rutas documentadas (eran pre-existentes a la adopción de OpenSpec en el repo, ver `protect-all-endpoints/design.md`) — no hay `specs/` que remover.

## Impact

- **Eliminado**: `userController.GetUser`/`CreateUser`, `userService.CreateUser`, `userDao.Create`, `domains/user/user_request.go`, rutas `GET /user/:user_id` y `POST /user` en `url_mappings.go`
- **Sin cambios**: `userService.GetUser`, `userDao.GetByID` (siguen en uso por `userWeatherDelegate`), rutas `/example/weather` y `/user/:user_id/weather` (demo, fuera de alcance)
- **Tests**: eliminados los tests de los métodos borrados; `url_mappings_test.go` invertido a `assert.False` para confirmar que las rutas ya no existen
- **Docs**: `README.md`, `docs/AUTH_MIGRATION.md`, `.agentics/GLOSSARY.md` / `cmd/api/docs/documentationdetail/GLOSSARY.md` actualizados
- **Swagger**: regenerado (sin cambios de contenido — estas rutas nunca tuvieron anotaciones Swagger)
