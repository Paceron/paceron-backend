# Tasks

## Tasks

### 1. Modelos de DB y migración

- [ ] Agregar `photo_key string` (nullable) y `photo_updated_at *time.Time` (nullable) a `cmd/api/domains/dbs/user.go`.
- [ ] Agregar `icon_key string` (nullable) y `icon_updated_at *time.Time` (nullable) a `cmd/api/domains/dbs/team.go`.
- [ ] Validar que las columnas quedan registradas en AutoMigrate (`cmd/api/infrastructure/postgresdb/postgres.go`) — son aditivas sobre tablas existentes, no requieren SQL crudo.
- [ ] Validar con `go build ./...` y test de DAO con Postgres real.

### 2. RestClient de storage

- [ ] Crear `cmd/api/restclients/storageclient/client.go`: interfaz `StorageClientInterface` con `Upload(ctx, key string, content io.Reader, contentType string) error` y `Delete(ctx, key string) error`, implementación contra el storage S3-compatible de Supabase.
- [ ] Crear `cmd/api/restclients/storageclient/options.go` si aplica (endpoint, región, credenciales, bucket, public base URL configurables).
- [ ] Tests unitarios con servidor `httptest` simulando las respuestas del storage (sin llamadas reales a Supabase en CI), mismo patrón que `mercadopagoclient`/`mailer`.

### 3. Config

- [ ] Agregar `SUPABASE_TESTING_S3_ENDPOINT`, `_REGION`, `_ACCESS_KEY_ID`, `_SECRET_ACCESS_KEY`, `_BUCKET`, `_PUBLIC_BASE_URL` y el mismo set con `_PRODUCTION_` a `.env.example`.
- [ ] Crear `StorageConfig`/`MyStorage` en `cmd/api/config/config.go`, resuelto por `config.IsProductionStage()` (mismo mecanismo que `MyMP`/`MyMailer`, sin flag nuevo).

### 4. DAOs

- [ ] Extender `cmd/api/daos/user_dao.go`: `UpdatePhoto(ctx, userID int64, key string, updatedAt time.Time) error`, `ClearPhoto(ctx, userID int64) error`.
- [ ] Extender `cmd/api/daos/team_dao.go`: `UpdateIcon(ctx, teamID int64, key string, updatedAt time.Time) error`, `ClearIcon(ctx, teamID int64) error`.
- [ ] Tests unitarios (mocks) y de DAO con Postgres real vía `testutils.SetupTestDB`.

### 5. DTOs y dominios

- [ ] Agregar `photo_url *string` a la respuesta de usuario que corresponda (`domains/user/` — la que devuelve `GET /api/v1/auth/user`).
- [ ] Agregar `icon_url *string` a `domains/team/` (respuesta de `GET /api/v1/teams/:id`).
- [ ] Custom codes de error en `domains/apierror`: `PHOTO_TOO_LARGE`, `PHOTO_INVALID_TYPE`.
- [ ] Helper compartido de armado de URL (`{public_base_url}/{key}?v={unix(updated_at)}`), nil-safe si `key` es nulo.

### 6. Servicios

- [ ] `services/user_service.go`: `UploadPhoto(ctx, userID, file)` (valida tamaño/magic bytes, arma key, sube, actualiza DAO) y `DeletePhoto(ctx, userID)`.
- [ ] `services/team_service.go`: `UploadIcon(ctx, callerID, teamID, file)` (valida que `callerID` sea el entrenador dueño del equipo antes de tocar nada) y `DeleteIcon(ctx, callerID, teamID)`.
- [ ] Tests unitarios (testify + mocks de `storageclient`/DAOs): tamaño excedido, tipo inválido, upload exitoso, fallo de storage (no toca DB), fallo de DB post-upload (loguea, no rompe la respuesta), borrado exitoso, borrado sin foto previa (204 idempotente), caller no autorizado en equipo.

### 7. Controllers y rutas

- [ ] Extender `controllers/user_controller.go`: `UploadPhoto`, `DeletePhoto`.
- [ ] Extender `controllers/team_controller.go`: `UploadIcon`, `DeleteIcon`.
- [ ] Registrar rutas en `cmd/api/app/url_mappings.go`: `PUT`/`DELETE /api/v1/users/:id/photo`, `PUT`/`DELETE /api/v1/teams/:id/icon`.
- [ ] Tests de controller (httptest, multipart) — validación de tamaño/tipo antes de llegar al service.

### 8. Swagger y documentación

- [ ] Regenerar `cmd/api/docs` con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs`.
- [ ] Actualizar tabla de endpoints en `README.md`.

### 9. Verificación final

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` en verde.
- [ ] Validar manualmente contra el bucket de testing de Supabase: upload de foto de usuario, reemplazo (misma key, URL cambia por el `?v=`), borrado, upload de ícono de equipo como entrenador dueño, rechazo como no-dueño, rechazo por tamaño/tipo inválido.
