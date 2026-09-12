## 1. DTOs (`cmd/api/domains/team`)

- [x] 1.1 Agregar `MembershipFee *float64 json:"membership_fee"` a `CreateTeamRequest` (`team_request.go`).
- [x] 1.2 Agregar `MembershipFee *float64 json:"membership_fee"` a `UpdateTeamRequest` (`team_update_request.go`).
- [x] 1.3 Agregar `MembershipFee float64 json:"membership_fee"` a `TeamResponse` (`team_response.go`).

## 2. Service (`cmd/api/services/team_service.go`)

- [x] 2.1 Definir sentinel `ErrInvalidMembershipFee` y validar `>= 0` en `Create` (si `req.MembershipFee != nil` y `< 0` → el sentinel; si nil → `0`), seteando `teamDB.MembershipFee`.
- [x] 2.2 Validar `>= 0` y asignar `teamDB.MembershipFee` en `Update` cuando `req.MembershipFee != nil`.
- [x] 2.3 Mapear `MembershipFee: t.MembershipFee` en `toResponse`.

## 3. Controller (`cmd/api/controllers/team_controller.go`)

- [x] 3.1 En `Create` y `Update`, mapear `errors.Is(err, services.ErrInvalidMembershipFee)` → `400` con `code = "INVALID_MEMBERSHIP_FEE"`.

## 4. Swagger

- [x] 4.1 Regenerar `cmd/api/docs` con `go run github.com/swaggo/swag/cmd/swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- [x] 4.2 Verificar en `swagger.json`/`swagger.yaml` que `membership_fee` aparezca en los schemas de `CreateTeamRequest`, `UpdateTeamRequest` y `TeamResponse`.

## 5. Tests

- [x] 5.1 `team_service_test.go`: `Create` con fee, sin fee (default 0) y fee negativo (sentinel); `Update` con fee, sin fee (no cambia) y fee negativo; `GetByID`/respuesta incluye fee.
- [x] 5.2 `team_controller_test.go`: `Create` y `Update` con fee negativo → `400` `INVALID_MEMBERSHIP_FEE`.
- [x] 5.3 `team_dao_test.go`: verificar persistencia/lectura de `membership_fee` en `Create` y `Update` contra DB real (testutils).
- [x] 5.4 Correr `go test ./...` y verificar que el coverage con DB (CI) no baje del gate (≥ 80%).

## 6. Documentación

- [x] 6.1 Actualizar `README.md` (tabla de endpoints): `POST /teams`, `PUT /teams/:id` y `GET /teams/:id` mencionan `membership_fee`.
- [x] 6.2 En `docs/CU/02-pago-participacion-equipo.md` y `docs/CAMBIO_SUSCRIPCION_TEAMS_SPLIT_TESTING.md`: reemplazar el workaround de `UPDATE ... SET membership_fee` por la configuración vía API (`POST`/`PUT`), y aclarar que el cambio no es retroactivo sobre membresías existentes.