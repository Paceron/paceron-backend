## 1. Backend

- [x] 1.1 `app/url_mappings.go`: quitar `GET /user/:user_id` y `POST /user`, actualizar el comentario de rutas públicas
- [x] 1.2 `controllers/user_controller.go`: quitar `GetUser`/`CreateUser` de la interfaz y su implementación
- [x] 1.3 `services/user_service.go`: quitar `CreateUser` de la interfaz y su implementación (`GetUser` se mantiene, lo usa `userWeatherDelegate`)
- [x] 1.4 `daos/user_dao.go`: quitar `Create` de la interfaz y su implementación
- [x] 1.5 Eliminar `domains/user/user_request.go` (`CreateUserRequest`, sin otro caller)

## 2. Tests

- [x] 2.1 Quitar tests de `userController.GetUser`/`CreateUser` (`controllers/opt_controller_test.go`, `controllers/user_controller_test.go`)
- [x] 2.2 Quitar tests de `userService.CreateUser` (`services/opt_service_test.go`)
- [x] 2.3 Quitar `mockCreate`/`Create` de `mockUserDao` y de los mocks de `daos.UserDaoInterface` en otros `_test.go` (`user_role_service_test.go`, `invitation_service_test.go`, `permissions_query_service_test.go`) — ya no forma parte de la interfaz
- [x] 2.4 `app/url_mappings_test.go`: invertir las aserciones de `/user/:user_id` y `POST /user` a `assert.False` (confirmar que ya no existen, no que existen)
- [x] 2.5 `go build`/`go vet`/`go test ./...` verdes

## 3. Docs

- [x] 3.1 Swagger regenerado
- [x] 3.2 `README.md`: quitar filas de la tabla de endpoints y el diagrama mermaid
- [x] 3.3 `docs/AUTH_MIGRATION.md`: quitar de la lista de rutas públicas, agregar sección explicando la eliminación
- [x] 3.4 `.agentics/GLOSSARY.md` / `cmd/api/docs/documentationdetail/GLOSSARY.md`: actualizar el ejemplo de `url_mappings.go` y la tabla de controllers que referenciaban las rutas eliminadas
- [x] 3.5 Verificación manual contra staging: `GET /user/1` y `POST /user` → 404; `GET /api/v1/auth/user?id=1` (equivalente real) sigue funcionando; `GET /user/:id/weather` (demo, sin tocar) sigue funcionando
