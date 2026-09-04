## Why

Hoy `users` y `teams` no tienen forma de guardar una imagen: no hay columna de foto, ni endpoint de carga, ni integración con storage. El storage S3-compatible de Supabase (proyectos de testing/producción, mismo split que la DB) ya tiene credenciales provistas por el usuario pero **cero código en el repo lo usa** — quedó deprioritizado hasta tener una feature concreta que lo consuma. Foto de perfil de usuario y foto/ícono de equipo son esa feature.

## What Changes

- **Nuevo cliente de storage** (`restclients/storageclient`, mismo patrón que `restclients/mercadopagoclient`): wrapper sobre el storage S3-compatible de Supabase, `Upload`/`Delete` por key, sin lógica de negocio.
- **Config nueva**: credenciales de storage de testing/producción (`SUPABASE_TESTING_S3_*` / `SUPABASE_PRODUCTION_S3_*`), resueltas por `config.IsProductionStage()` — mismo mecanismo que ya separa la DB de test/prod, sin flag nuevo.
- **Columnas nuevas**: `users.photo_key` / `users.photo_updated_at` (nullable); `teams.icon_key` / `teams.icon_updated_at` (nullable). Se guarda la **key** del objeto, no la URL completa — la URL pública se arma al servir la respuesta (`base_url + key + "?v=" + timestamp`), para no tener que backfillear filas si cambia el bucket/dominio.
- **Endpoints nuevos** (autenticados): `PUT`/`DELETE /api/v1/users/:id/photo` (self only), `PUT`/`DELETE /api/v1/teams/:id/icon` (solo el entrenador dueño del equipo). Subida por **proxy del backend** (multipart, no URL presignada — ver Non-Goals).
- **Campos nuevos en respuestas existentes**: `photo_url` en `GET /api/v1/auth/user`, `icon_url` en `GET /api/v1/teams/:id` — no hace falta un endpoint de consulta dedicado.
- **Key fija por entidad** (`avatars/user-{id}.{ext}`, `teams/team-{id}-icon.{ext}`): resubir pisa el archivo anterior, no hay que rastrear/limpiar archivos huérfanos.
- **Validación**: máx 5MB, tipos `image/jpeg`/`image/png`/`image/webp`, verificados por magic bytes (no por extensión del nombre de archivo).

## Capabilities

### New Capabilities

- `user-team-photos`: carga/reemplazo/borrado de foto de perfil de usuario y de ícono de equipo, servidas desde bucket público de Supabase Storage vía proxy del backend.

### Modified Capabilities

- Ninguna: no se toca lógica de pagos, tiers, ni el resto de usuarios/equipos.

## Non-Goals

- **Documentos** (certificados médicos, diplomas, certificaciones): quedan fuera de este change. La arquitectura (`storageclient` agnóstico al contenido, key + content-type genéricos) queda lista para extenderlos después sin rediseño, pero no se crean endpoints ni columnas para eso ahora.
- **URL presignada** (upload directo frontend → Supabase sin pasar por el backend): evaluada y descartada por ahora — requiere configurar CORS en el bucket, y el volumen/tamaño de archivo actual (fotos chicas, tráfico bajo) no justifica el ahorro de banda de Render. Queda documentada en `design.md` como alternativa a migrar si el caso de uso crece (archivos grandes, mucho tráfico).
- **Edición de imagen** (recortar, rotar, etc.): "editar" una foto es resubirla (pisa la key existente); cualquier edición real se resuelve del lado del frontend antes de subir.
- **Historial de fotos**: no se guardan versiones anteriores, solo la vigente.

## Impact

- **Schema/DB**: `users.photo_key` (nullable), `users.photo_updated_at` (nullable), `teams.icon_key` (nullable), `teams.icon_updated_at` (nullable). Columnas aditivas sobre tablas existentes, no rompen nada. AutoMigrate en `cmd/api/infrastructure/postgresdb/postgres.go`.
- **Config**: `config/config.go` — `StorageConfig`/`MyStorage`, nuevas env vars en `.env.example`.
- **RestClients**: nuevo `cmd/api/restclients/storageclient/client.go`.
- **DAOs**: `daos/user_dao.go` y `daos/team_dao.go` extienden con métodos de actualizar/limpiar key+timestamp.
- **Servicios**: `services/user_service.go` (UploadPhoto/DeletePhoto) y `services/team_service.go` (UploadIcon/DeleteIcon, con check de entrenador dueño) extienden — no se crean servicios nuevos, el alcance no lo justifica.
- **Controllers/rutas**: `controllers/user_controller.go` y `controllers/team_controller.go` extienden; rutas nuevas en `cmd/api/app/url_mappings.go`.
- **Dominios/DTOs**: `photo_url` en `UserResponse`, `icon_url` en `TeamResponse` (o el DTO que corresponda).
- **Swagger**: regenerar `cmd/api/docs` con los endpoints nuevos.
- **Tests**: unitarios (services/controllers con mocks del `storageclient`, DAOs con Postgres real vía `testutils.SetupTestDB`); test de integración real contra Supabase Storage gateado por env var (skip si no está seteada), mismo patrón que `TestSendEmail_RealEmail_Integration` en `mailer_test.go`.
