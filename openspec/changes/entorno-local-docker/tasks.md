## 1. Infra como código

- [x] 1.1 `docker-compose.yml`: servicio `db` (`postgres:17-alpine`, `POSTGRES_IMAGE` parametrizable, puerto 5432, volumen `paceron-db-data`, healthcheck `pg_isready`)
- [x] 1.2 `docker-compose.yml`: servicio `storage` (RustFS, 9000/9001, `RUSTFS_ACCESS_KEY`/`RUSTFS_SECRET_KEY` parametrizables, volumen `paceron-s3-data`, healthcheck)
- [x] 1.3 `docker-compose.yml`: servicio `storage-init` (`amazon/aws-cli`) + `scripts/storage_init.sh`: espera a storage, crea `paceron-media` con `--ignore-existing` y le aplica la policy de lectura pública
- [x] 1.4 `.gitignore`: `backup/` fuera de git (contiene datos reales de usuarios)

## 2. Scripts de dump y restore

- [x] 2.1 `scripts/dump_db.sh`: parseo de `.env` (soporta `export KEY=val`, comillas y ` #` comments), selección de URL por `--url`, errores sin exponer la password
- [x] 2.2 `scripts/dump_db.sh`: detección de la versión mayor del server remoto vía contenedor efímero, y `pg_dump -F c` con esa misma versión de imagen
- [x] 2.3 `scripts/dump_db.sh`: exclusiones de schemas Supabase + `--no-owner --no-privileges`
- [x] 2.4 `scripts/restore_db.sh`: espera el healthcheck de `db`, `docker cp` del dump, `pg_restore --no-owner --no-privileges` con log a archivo
- [x] 2.5 `scripts/restore_db.sh`: cleanup defensivo de RLS sobre tablas de `public` + verificación de tablas de la app con conteo de filas (exit != 0 si alguna queda vacía)

## 3. Stage local en config

- [x] 3.1 `config.go`: `--stage=local` (`IsLocalStage`, `stageEnvPrefix`), `local` gana sobre `production` si llegan ambos
- [x] 3.2 `config.go`: `stagedDatabaseURL` lee `DATABASE_URL` en local; `stagedStorageEnvPrefix` devuelve `S3_`
- [x] 3.3 `config.go`: `StorageConfig.PublicBaseURL` + `ForcePathStyle` (default true), leídos en todos los stages
- [x] 3.4 `router.go`: log de arranque distingue los 3 ambientes

## 4. Storage client

- [x] 4.1 `storageclient.Options`: `ForcePathStyle` + `PublicBaseURL`, `New` los aplica
- [x] 4.2 `PublicBaseURL()`: override como primer branch, derivado Supabase intacto como fallback
- [x] 4.3 `app.go`: pasar `config.MyStorage.ForcePathStyle` y `PublicBaseURL`; `media_url.go` usar el override
- [x] 4.4 Comentarios del paquete actualizados (ya no dicen "S3-compatible de Supabase" como si fuera el único backend)

## 5. Configuración de ejemplo y Makefile

- [x] 5.1 `.env.local.example`: `DATABASE_URL`, `S3_*`, `S3_FORCE_PATH_STYLE`, `S3_PUBLIC_BASE_URL`, `RUSTFS_*`, `POSTGRES_*`
- [x] 5.2 `.env.example`: sección nueva de stage local, referenciando `.env.local.example`
- [x] 5.3 `Makefile`: `local-up`/`local-down`/`local-ps`/`local-logs`/`local-dump`/`local-restore`/`local-reset`/`local-shell`

## 6. Tests

- [x] 6.1 `config_test.go`: `IsLocalStage`, precedencia local>production, URL de DB y prefijo de storage por stage, `ForcePathStyle` default y parseo de false, `PublicBaseURL` leído
- [x] 6.2 `storageclient/client_test.go` (nuevo): `PublicBaseURL` con override, sin override, con path en el endpoint; `ForcePathStyle` propagado
- [x] 6.3 `go build ./...`, `go vet ./...`, `go test ./...` en verde

## 7. Docs

- [x] 7.1 `docs/ENTORNO_LOCAL.md` (nuevo): requisitos, quickstart, los 3 targets del flujo, troubleshooting
- [x] 7.2 `docs/ENVIRONMENTS.md`: de 2 ambientes a 3 (local/testing/production)
- [x] 7.3 `AGENTS.md` + `CLAUDE.md`: sección del entorno local + nota de coordinación con frontend (URLs de media served desde `localhost:9000`)

## 8. Verificación (requiere Docker corriendo)

- [x] 8.1 `docker compose up -d` levanta `db`, `storage`, `storage-init` sano
- [x] 8.2 `make local-dump` genera el dump sin errores contra Supabase real
- [x] 8.3 `make local-restore` restaura y verifica tablas de la app pobladas
- [x] 8.4 El backend con `--stage=local` levanta contra la BD local y `AutoMigrate` no rompe nada
- [x] 8.5 Upload y download de avatar/ícono de equipo contra el storage local vía la API del backend, con la URL pública sirviendo el objeto
- [x] 8.6 `go test ./...` sigue verde sin Docker (el stage local no debe filtrarse a la suite)