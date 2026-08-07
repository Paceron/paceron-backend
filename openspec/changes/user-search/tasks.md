## 1. Backend

- [x] 1.1 `daos/user_dao.go`: `SearchActive(ctx, query, limit)` — `ILIKE` sobre name/surname/email, `status = active`, orden por nombre, límite
- [x] 1.2 `domains/user/search_response.go`: `SearchResultItem` (`user_id`/`name`/`surname`/`email`), `SearchResponse`
- [x] 1.3 `services/user_service.go`: `Search(ctx, query)` — trim + mínimo 3 caracteres, límite 5, mapea a DTO
- [x] 1.4 `controllers/user_controller.go`: `Search(c)` — lee `q` de query string, 400 en validación, 500 en error interno
- [x] 1.5 `app/url_mappings.go`: `GET /api/v1/users/search`, detrás de `AuthMiddleware()`

## 2. Tests

- [x] 2.1 DAO: conformidad de interfaz (mismo patrón que el resto de `user_dao_test.go`, sin DB real)
- [x] 2.2 Service: éxito, trim de query, query corta (error), query en blanco (error), error de DAO
- [x] 2.3 Controller: éxito, query corta → 400, error interno → 500
- [x] 2.4 Mocks de `daos.UserDaoInterface`/`services.UserServiceInterface` en otros `_test.go` actualizados con el método nuevo (`mockUserDaoForUserRole`, `mockUserDaoForInvitation`, `mockUserDaoForQuery`, `mockUserService`)
- [x] 2.5 `go build`/`go vet`/`go test ./...` verdes

## 3. Docs

- [x] 3.1 Swagger regenerado
- [x] 3.2 `README.md`: fila nueva en la tabla de endpoints
- [x] 3.3 `docs/AUTH_MIGRATION.md`: sección nueva describiendo el endpoint
