## Why

Hoy, trabajar contra Supabase (testing) + Supabase Storage significa que *toda* iteración depende de la nube: sin internet no se levanta la app, cada `go run` muta la base real del proyecto de testing (tests, filas basura, migraciones de `AutoMigrate` aplicadas en cloud), y probar features destructivas exige restaurar un snapshot a mano.

Además, el repo ya pagaba el costo de esa dependencia en el código: `config.go` tiene un mecanismo de *stages* (`--stage=production`) porque la única alternativa a "producción" es "testing cloud". No hay un tercer escenario. Y el auth ya es 100% propio (JWT propio + tabla `users` propia, cero FK a `auth.users`), así que la dependencia de Supabase **ya es evitable** — lo que falta es la infraestructura para hacerlo.

El objetivo es un entorno 100% local orquestado por Docker Compose (Postgres + storage S3-compatible) con los datos reales clonados desde Supabase, para poder desarrollar y romper cosas sin miedo.

## What Changes

- **`docker-compose.yml`** (nuevo, raíz): servicio `db` (`postgres:17-alpine`, puerto 5432, volumen nombrado `paceron-db-data`) y servicio `storage` (RustFS, puertos 9000 API / 9001 consola, volumen `paceron-s3-data`) más `storage-init` (`amazon/aws-cli`), que crea el bucket `paceron-media` y le aplica la policy de lectura pública de forma idempotente al arrancar.
- **`scripts/dump_db.sh`** (nuevo): corre `pg_dump -F c` **dentro de un contenedor** `postgres:<major>-alpine`, detectando la versión mayor del server remoto primero (un `pg_dump` más viejo que el server aborta). Lee las credenciales de `.env`, excluye los schemas/roles que administra Supabase y deja el dump en `backup/`.
- **`scripts/restore_db.sh`** (nuevo): `pg_restore` del dump contra el contenedor `db` local, no-fatal ante errores de objetos gestionados por Supabase, seguido de una verificación de que las tablas de la app quedaron pobladas.
- **`cmd/api/config/config.go`**: nuevo stage `local` vía `--stage=local`, con su propio juego de variables — `DATABASE_URL`, `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_ID`, `S3_SECRET_KEY`, `S3_BUCKET` — siguiendo el patrón de prefijo por stage ya existente. `S3_FORCE_PATH_STYLE` (default `true`) y `S3_PUBLIC_BASE_URL` se leen igual en todos los stages (defaults retrocompatibles).
- **`cmd/api/restclients/storageclient/client.go`**: `Options.ForcePathStyle` (hoy `UsePathStyle` está hardcodeado en `true`) y override de `PublicBaseURL` — la función hoy deriva sí o sí `https://<ref>.supabase.co/storage/v1/object/public/...`, que en local apuntaría a `https://localhost.supabase.co/...` y rompería avatar/ícono de equipo.
- **`Makefile`**: targets `local-up`/`local-down`/`local-logs`/`local-dump`/`local-restore`/`local-reset`/`local-ps`/`local-shell`.
- **`.env.local.example`** (nuevo) + `.gitignore` ignorando `.env.local` y `backup/`.
- **`docs/ENTORNO_LOCAL.md`** (nuevo): el manual completo (requisitos, dump, restore, troubleshooting).
- **`docs/ENVIRONMENTS.md`**: pasa a ser la tabla de 3 ambientes (local / testing / production) en vez de 2.
- **`AGENTS.md`** / **`CLAUDE.md`**: sección del entorno local + nota de coordinación con el frontend.

## Capabilities

### New Capabilities
- `local-dev-environment`: entorno de desarrollo 100% local orquestado por Docker Compose (Postgres + storage S3-compatible), con clonación de los datos de Supabase vía `pg_dump`/`pg_restore` y un stage de configuración propio.

### Modified Capabilities
- `storage-client`: el cliente S3 pasa a tener path-style configurable y a permitir override de la URL pública base, hoy derivada hardcodeando la forma de URL de Supabase.

## Impact

- **Nuevo**: `docker-compose.yml`, `scripts/dump_db.sh`, `scripts/restore_db.sh`, `scripts/storage_init.sh`, `.env.local.example`, `docs/ENTORNO_LOCAL.md`, `cmd/api/restclients/storageclient/client_test.go`, tests de `config`.
- **Modificado**: `cmd/api/config/config.go`, `cmd/api/restclients/storageclient/client.go`, `cmd/api/services/media_url.go`, `cmd/api/app/app.go` (wiring), `cmd/api/app/router.go` (log de stage), `Makefile`, `AGENTS.md`, `CLAUDE.md`.
- **Sin impacto en Render/CI**: `--stage=local` es opt-in explícito. Sin ese flag, todo sigue resolviendo a testing exactamente como hoy — ni `render.yaml` ni `ci.yml` cambian.
- **Frontend**: la forma de la URL de avatar/ícono cambia solo bajo `--stage=local` (de la derivada de Supabase a `S3_PUBLIC_BASE_URL/<key>`). Contra Supabase no cambia nada. Coordinar con `paceron-frontend` (ver AGENTS.md §8) que las fotos servidas en local van a salir de `localhost:9000`.
- **No clonea objetos**: la base sí se clona, pero las fotos ya subidas a Supabase Storage no. Migrar los objetos existentes es un script aparte y queda fuera de este change.