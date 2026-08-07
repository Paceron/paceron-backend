## 1. Backend

- [x] 1.1 `daos/user_dao.go`: `FindByIDs(ctx, userIDs)` — `WHERE id IN (?)`, sin filtro de status
- [x] 1.2 `domains/user/batch_lookup_response.go`: `BatchLookupResponse` (reusa `SearchResultItem`)
- [x] 1.3 `services/user_service.go`: `BatchLookup(ctx, userIDs)` — valida no-vacío y máximo 50, mapea a DTO
- [x] 1.4 `controllers/user_controller.go`: `BatchLookup(c)` — parsea `ids` de query string (coma-separado), 400 en validación, 500 en error interno
- [x] 1.5 `app/url_mappings.go`: `GET /api/v1/users`, detrás de `AuthMiddleware()`

## 2. Tests

- [x] 2.1 DAO: conformidad de interfaz
- [x] 2.2 Service: éxito, ids vacíos (error), más de 50 ids (error), error de DAO
- [x] 2.3 Controller: éxito, sin parámetro ids → 400, id no numérico → 400, error de validación de service → 400, error interno → 500
- [x] 2.4 Mocks de `daos.UserDaoInterface`/`services.UserServiceInterface` en otros `_test.go` actualizados con el método nuevo
- [x] 2.5 `go build`/`go vet`/`go test ./...` verdes

## 3. Docs

- [x] 3.1 Swagger regenerado
- [x] 3.2 `README.md`: fila nueva en la tabla de endpoints
- [x] 3.3 `docs/AUTH_MIGRATION.md`: sección nueva describiendo el endpoint
