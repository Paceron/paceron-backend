## 1. Modelo de datos y base

- [x] 1.1 Agregar `github.com/skip2/go-qrcode` a `go.mod`/`go.sum` (`go get github.com/skip2/go-qrcode`)
- [x] 1.2 Crear `cmd/api/domains/dbs/attendance.go`: modelo `Attendance` con `TableName()` = `"attendances"`, columnas `id`, `team_id`, `training_session_id`, `user_id`, `created_at`, `updated_at`, la UNIQUE compuesta `(team_id, training_session_id, user_id)` y los índices `(team_id, training_session_id)` y `(user_id, team_id)` vía tags GORM (sin FK a `training_sessions`: FK opaca)
- [x] 1.3 Registrar `dbs.Attendance{}` en la lista de modelos del `AutoMigrate` en `cmd/api/infrastructure/postgresdb/postgres.go`
- [x] 1.4 Agregar config `ATTENDANCE_BASE_URL` (default `http://localhost:8080`) en `cmd/api/config/config.go`, documentarla en `docs/ENVIRONMENTS.md` y setearla en `render.yaml` para testing/producción

## 2. Capa DAO

- [x] 2.1 Crear `cmd/api/daos/attendance_dao.go`: interfaz `AttendanceDAOInterface` + implementación con los métodos `Create(attendance) (Attendance, error)` (insert directo; detecta `gorm.ErrDuplicatedKey`/violación de UNIQUE), `Search(filters) ([]Attendance, error)` (WHERE dinámico), `TeamExists(teamID) (bool, error)`, `IsTeamOwner(teamID, userID) (bool, error)` y `ExistsUserInTeamOwnedBy(targetUserID, ownerUserID) (bool, error)` (JOIN `team_users` + `teams` por `owner_id`, respetando `deleted_at`)
- [x] 2.2 Crear `cmd/api/daos/attendance_dao_test.go` con `testutils.SetupTestDB(t)`: cubrir insert nuevo, duplicado (viola UNIQUE), búsqueda con cada filtro y combinaciones, `TeamExists`, `IsTeamOwner` y la relación owner↔miembro (positivo y negativo)

## 3. Capa Service

- [x] 3.1 Crear `cmd/api/services/attendance_service.go`: interfaz `AttendanceServiceInterface` + implementación:
  - `BuildQR(ctx, teamID, sessionID) (QRResponse, error)` — construye la URL `<base>/api/v1/attendance/team/{team_id}/session/{training_session_id>` y genera el QR con `qrcode.Encode(url, qrcode.Medium, 256)` (determinista)
  - `Register(ctx, userID, teamID, sessionID) (string status, error)` — `Create`; duplicado → flujo "previamente registrada"
  - `Search(ctx, authUserID, filters) ([]Attendance, error)` — implementa la matriz A/B/C del spec: Evalúa `user_id`/`team_id` y devuelve `Forbidden`/`NotFound` acorde
- [x] 3.2 Crear `cmd/api/services/attendance_service_test.go` con mocks del DAO: determinismo del QR (dos generaciones → bytes iguales), registro nuevo vs. duplicado, y todas las ramas de la matriz (A self, A user_id propio, B owner/no-owner, B team inexistente → NotFound, C owner de equipo del target, C sin relación → Forbidden)

## 4. Capa HTTP y wiring

- [x] 4.1 Crear DTOs en `cmd/api/domains/attendance/`: `QRResponse{QRCodeBase64, URLEncoded}` y `SearchResponse{Data []dbs.Attendance}` + constantes de mensajes de registro
- [x] 4.2 Crear `cmd/api/controllers/attendance_controller.go` con los handlers `GenerateQR`, `RegisterAttendance` y `Search` (validación de params > 0 → 400; respuestas 200/201/400/403/404 con `APIError`; anotaciones Swagger completas)
- [x] 4.3 Registrar rutas en `cmd/api/app/url_mappings.go` (detrás del `AuthMiddleware` existente): `GET /api/v1/attendance/qr`, `POST /api/v1/attendance/team/:team_id/session/:training_session_id`, `GET /api/v1/attendance/search` — y el wiring controller→service→dao en `cmd/api/app/app.go`
- [x] 4.4 Crear `cmd/api/controllers/attendance_controller_test.go` con mocks del service: casos felices (201/200/200 con data), 400 por params faltantes/≤0 y 403/404 mapeados desde el service

## 5. Verificación final

- [x] 5.1 Regenerar Swagger (`swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`) con la nueva documentación de los 3 endpoints
- [ ] 5.2 Correr `go test ./...` (suite completa en verde) y `make coverage-with-db` (coverage ≥ umbral de CI 80%)
- [ ] 5.3 Verificar con Postgres real que el `UNIQUE` y los índices quedaron creados correctamente sobre `attendances`