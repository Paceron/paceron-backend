# Tasks

## Tasks

### 1. Modelos de DB y migración

- [x] Agregar `photo_key string` (nullable) y `photo_updated_at *time.Time` (nullable) a `cmd/api/domains/dbs/user.go`.
- [x] Agregar `icon_key string` (nullable) y `icon_updated_at *time.Time` (nullable) a `cmd/api/domains/dbs/team.go`.
- [x] Validar que las columnas quedan registradas en AutoMigrate (`cmd/api/infrastructure/postgresdb/postgres.go`) — son aditivas sobre tablas existentes, no requieren SQL crudo. `dbs.User{}`/`dbs.Team{}` ya estaban en la lista.
- [x] Validar con `go build ./...` — verde. Test de DAO con Postgres real: pendiente, requiere `TEST_DB_HOST` (se corre en CI / local con `make test-db-up`).

### 2. RestClient de storage

- [x] Crear `cmd/api/restclients/storageclient/client.go`: interfaz `StorageClientInterface` con `Upload(ctx, key string, content []byte, contentType string) error` y `Delete(ctx, key string) error` — usa `aws-sdk-go-v2/service/s3` (path-style, `BaseEndpoint` custom) contra el storage S3-compatible de Supabase. `content []byte` en vez de `io.Reader` (desvío menor del diseño original): el service ya necesita el archivo completo en memoria para validar magic bytes (D4), y acotado a 5MB evita cualquier caso borde de firmado SigV4 con streams no-seekable.
- [x] Sin `client_test.go` dedicado — mismo criterio que `mercadopagoclient` (envuelve un SDK oficial ya testeado; la cobertura real llega vía mocks de `StorageClientInterface` en los tests de service/controller, sección 6).

### 3. Config

- [x] Agregar `SUPABASE_TESTING_S3_ENDPOINT`, `_REGION`, `_ACCESS_ID`, `_SECRET_KEY`, `_BUCKET` y el mismo set con `_PRODUCTION_` a `.env.example` — nombres ajustados durante la prueba manual (sección 9) para matchear las credenciales que el usuario ya tenía provisionadas (`_ACCESS_ID`/`_SECRET_KEY`, no `_ACCESS_KEY_ID`/`_SECRET_ACCESS_KEY`). Sin `_PUBLIC_BASE_URL`: se deriva del endpoint + bucket (`storageclient.PublicBaseURL`), no hace falta env var aparte.
- [x] Crear `StorageConfig`/`MyStorage` en `cmd/api/config/config.go`, resuelto por `config.IsProductionStage()` (mismo mecanismo que `MyMP`/`MyMailer`, sin flag nuevo).

### 4. DAOs

- [x] Extender `cmd/api/daos/user_dao.go`: `UpdatePhoto(ctx, userID int64, key string, updatedAt time.Time) error`, `ClearPhoto(ctx, userID int64) error`.
- [x] Extender `cmd/api/daos/team_dao.go`: `UpdateIcon(ctx, teamID int64, key string, updatedAt time.Time) error`, `ClearIcon(ctx, teamID int64) error`.
- [x] Mocks de `UserDaoInterface`/`TeamDaoInterface` actualizados en todos los test files que los implementan (`opt_service_test.go`, `team_service_test.go`, `permissions_query_service_test.go`, `invitation_service_test.go`, `user_role_service_test.go`) — `go build ./...`/`go vet ./...`/`go test ./...` verdes en todo el repo.
- [x] Tests de DAO con Postgres real vía `testutils.SetupTestDB` para los 4 métodos nuevos (`user_dao_test.go`, `team_dao_test.go`) — se skipean local sin `TEST_DB_HOST`, corren en CI.

### 5. DTOs y dominios

- [x] Agregar `photo_url *string` a `auth.UserResponse` (`domains/auth/register_response.go` — es la que devuelve `GET /api/v1/auth/user`).
- [x] Agregar `icon_url *string` a `team.TeamResponse` (`domains/team/team_response.go`).
- [x] Custom codes de error: siguiendo la convención existente del repo (`apierror.APIError.Code` como string literal, sin constantes centralizadas — ver `payment_controller.go`), se usan los literales `"PHOTO_TOO_LARGE"`/`"PHOTO_INVALID_TYPE"` directamente en los controllers (sección 7), no un paquete de constantes nuevo.
- [x] Helper compartido `buildMediaURL(key *string, updatedAt *time.Time) *string` (`services/media_url.go`) — nil-safe, arma `{public_base_url}/{key}?v={unix(updated_at)}`. Conectado en `auth_service.go`/`team_service.go` (`toResponse`).

### 6. Servicios

- [x] `services/user_service.go`: `UploadPhoto(ctx, userID, content []byte) (*string, error)` (valida tamaño/magic bytes vía `validatePhotoContent`, arma key determinística, sube, actualiza DAO) y `DeletePhoto(ctx, userID) error`.
- [x] `services/team_service.go`: `UploadIcon(ctx, id, callerID, content []byte) (*string, error)` (valida vía `isEntrenadorOfTeam` existente que `callerID` sea el entrenador dueño del equipo antes de tocar nada) y `DeleteIcon(ctx, id, callerID) error`.
- [x] Errores de validación como sentinels (`ErrPhotoTooLarge`, `ErrPhotoInvalidType` en `services/media_url.go`) — controllers los distinguen con `errors.Is` en vez de matchear el mensaje (sección 7).
- [x] Tests unitarios (testify + `mockStorageClient` nuevo en `opt_service_test.go` + mocks de DAOs existentes): tamaño excedido, tipo inválido, upload exitoso, fallo de storage (no toca DB), fallo de DB post-upload (loguea, no rompe la respuesta — verificado indirectamente, no bloquea), borrado exitoso, borrado sin foto previa (idempotente), caller no autorizado en equipo. `NewUserService`/`NewTeamService` ganaron un 5°/9° parámetro (`storageClient`) — actualizados los ~77 call sites en tests + `app.go` (cliente real vía `storageclient.New`).

### 7. Controllers y rutas

- [x] Extender `controllers/user_controller.go`: `UploadPhoto`, `DeletePhoto` (self-only vía `utils.GetAuthUserID`, `http.MaxBytesReader` antes de parsear el multipart).
- [x] Extender `controllers/team_controller.go`: `UploadIcon`, `DeleteIcon` (autorización delegada al service vía `isEntrenadorOfTeam`, controller solo mapea el string de error a 403).
- [x] Registrar rutas en `cmd/api/app/url_mappings.go`: `PUT`/`DELETE /api/v1/users/:id/photo`, `PUT`/`DELETE /api/v1/teams/:id/icon`.
- [x] Tests de controller (httptest, multipart vía helper `newMultipartPhotoRequest` compartido) — tamaño/tipo/forbidden/success/idempotente. Nota de hallazgo: `c.Status(204)` con `gin.CreateTestContext` no flushea el código a menos que se llame `c.Writer.WriteHeaderNow()` explícitamente en el test (gin real lo hace en `engine.handleHTTPRequest`, `gin.CreateTestContext` no pasa por ahí) — no es un bug del controller, confirmado leyendo `response_writer.go`/`gin.go` del SDK.

### 8. Swagger y documentación

- [x] Regenerar `cmd/api/docs` con `swag init --parseDependency -g cmd/api/docs.go --output cmd/api/docs` — confirmados `/api/v1/users/{id}/photo` y `/api/v1/teams/{id}/icon` en `swagger.json`.
- [x] Actualizar tabla de endpoints en `README.md`.

### 9. Verificación final

- [x] `go build ./...`, `go vet ./...`, `go test ./...` en verde (repo completo).
- [x] Validado el camino de escritura (`Upload`/`Delete`) contra el bucket real de testing (`testing_stage_bucket`) con las credenciales reales del `.env` local — confirmado que conecta, sube y borra correctamente (vía test descartable `zz_manual_test.go`, borrado al terminar, mismo patrón que las pruebas manuales de mailer).
- [ ] **Hallazgo pendiente de resolver (fuera de código):** la URL pública derivada (`https://<project-ref>.supabase.co/storage/v1/object/public/<bucket>/<key>`) devuelve `404 Bucket not found` — el bucket `testing_stage_bucket` no está reconocido por la API pública de Storage, aunque el gateway S3 sí lo acepta para `PutObject`/`DeleteObject`. Hay que revisar en el dashboard de Supabase (proyecto testing) que el bucket exista como bucket de Storage real y esté marcado público — posible que solo exista implícitamente del lado S3 y nunca se haya creado como bucket desde la UI/API de Storage. No bloquea mergear (el código está correcto y probado), pero las fotos no se van a poder *ver* hasta resolverlo.
