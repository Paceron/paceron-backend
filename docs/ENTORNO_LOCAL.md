# Entorno local (Docker Compose)

Desarrollo 100% local: Postgres + storage S3-compatible, con los datos reales clonados de Supabase. Para trabajar y romper cosas sin depender de la nube.

- **Qué es y qué no**: la tabla de ambientes está en [`ENVIRONMENTS.md`](ENVIRONMENTS.md).
- **Spec**: `openspec/changes/entorno-local-docker/`.

> El `docker compose` de acá **no** es lo mismo que los targets `test-db-*` del `Makefile`. Esos levantan una base de test descartable en `:5433` para los tests de `daos/`; este es el entorno de trabajo, con datos reales, en `:5432`.

## Requisitos

Solo **Docker**. No hace falta instalar Postgres ni `pg_dump`/`pg_restore` en el host: los scripts corren todo dentro de contenedores.

Si `docker` no está en el PATH pero Docker Desktop sí está instalado (macOS), los symlinks se crean así:

```bash
for b in docker kubectl helm; do
  sudo ln -sf "/Applications/Docker.app/Contents/Resources/bin/$b" "/usr/local/bin/$b"
done
# credenciales de Docker Hub, si da error de auth:
sudo ln -sf "/Applications/Docker.app/Contents/Resources/bin/docker-credential-desktop" "/usr/local/bin/docker-credential-desktop"
```

## Quickstart

```bash
cp .env.local.example .env.local    # opcional: los defaults del compose ya funcionan
make local-up                        # Postgres :5432 + storage :9000/:9001 + bucket
make local-restore                   # clona los datos de Supabase (pide confirmación)
make local-logs                      # ver logs; Ctrl-C para salir
go run ./cmd/api --stage=local       # el backend contra todo lo anterior
```

Sin `--stage=local` el backend resuelve contra Supabase testing, exactamente como siempre. El flag es lo único que activa este entorno.

Para servirse el backend hot-reload: `air` (o el que uses) pasando los flags, `go run ./cmd/api --stage=local`.

## Las 3 partes del flujo

### 1. Levantar / bajar el stack

```bash
make local-up        # db + storage + storage-init
make local-ps        # estado y health
make local-logs      # logs de db y storage
make local-down      # baja, CONSERVA los volúmenes (los datos quedan)
make local-reset     # baja y BORRA los volúmenes (base y bucket vacíos)
make local-shell     # psql contra la base local
```

Después de `local-reset` hay que volver a hacer `local-restore` (y el bucket se recrea solo al próximo `local-up`).

### 2. Clonar la base desde Supabase

```bash
make local-dump      # o ./scripts/dump_db.sh
```

Corre `pg_dump -F c` dentro de un contenedor `postgres:<major>-alpine`, con la versión **detectada del server remoto** (no asumida), y deja el dump en `backup/paceron-<timestamp>.dump`.

Detalle que importa: **`pg_dump` no puede ser más viejo que el server**. El script primero levanta un contenedor efímero, pregunta `SHOW server_version_num` y recién ahí dumpea. Lo mismo al revés: un `pg_restore` no puede leer un archivo de una versión mayor, así que la imagen local tiene que coincidir con la del dump.

El dump excluye los schemas que administra Supabase (`auth`, `storage`, `_supabase`, `vault`, `pgsodium*`, `realtime`, `graphql_public`, `extensions`, `supabase_functions`) y usa `--no-owner --no-privileges`, porque los roles `anon`/`authenticated`/`service_role`/`supabase_admin` no existen en un Postgres vanilla.

Las credenciales salen de `.env` (`.env.local` no se usa para esto: el dump va contra la nube). Para otro ambiente:

```bash
./scripts/dump_db.sh --env-file .env.prod --out backup/prod.dump
./scripts/dump_db.sh --url "$SUPABASE_PRODUCTION_DATABASE_URL" --out backup/prod.dump
```

Por default busca, en orden, `SUPABASE_TESTING_DATABASE_URL`, `SUPABASE_PRODUCTION_DATABASE_URL` y `DATABASE_URL`.

### 3. Restaurar en el Postgres local

```bash
make local-restore   # o ./scripts/restore_db.sh
./scripts/restore_db.sh --file backup/paceron.dump --reset   # wipe + restore
./scripts/restore_db.sh --yes                              # sin prompt
```

Imprime los conteos de filas de las tablas de la app y **falla con exit != 0** si alguna quedó vacía, para que un restore a medias no parezca sano.

`backup/` tiene datos reales de usuarios (emails, nombres). Está en `.gitignore` y no se comparte.

## Configuración

Todo el stage local se configura en **`.env.local`** (gitignored). El backend lo carga solo cuando corre con `--stage=local`, y el compose lo lee con `--env-file`.

| Variable | Para qué | Default |
|---|---|---|
| `DATABASE_URL` | Base del stage local | `postgresql://postgres:postgres@localhost:5432/paceron_local` |
| `S3_ENDPOINT` | API S3 del storage local | `http://localhost:9000` |
| `S3_REGION` | Región S3 | `us-east-1` |
| `S3_ACCESS_ID` / `S3_SECRET_KEY` | Credenciales | `minioadmin` / `minioadmin` |
| `S3_BUCKET` | Bucket de media | `paceron-media` |
| `S3_FORCE_PATH_STYLE` | Path-style (host/bucket/key) | `true` |
| `S3_PUBLIC_BASE_URL` | Base de las URLs públicas de media | `http://localhost:9000/paceron-media` |
| `POSTGRES_*` / `RUSTFS_*` | Credenciales de los servicios del compose | ver `.env.local.example` |

Las dos últimas se leen **igual en los tres ambientes** (no son específicas de un stage): `S3_FORCE_PATH_STYLE` con default `true` preserva el comportamiento de siempre contra Supabase, y `S3_PUBLIC_BASE_URL` vacío hace que las URLs se deriven del endpoint con la forma de Supabase.

`S3_PUBLIC_BASE_URL` no es opcional en local: sin ella el backend deriva `https://localhost.supabase.co/storage/v1/object/public/...` y todas las fotos dan 404. Además el bucket tiene que permitir lectura anónima (lo hace `storage-init`).

## Frontend

Las URLs de avatar e ícono de equipo salen de `localhost:9000`. El frontend tiene que permitir ese origin en las imágenes (CORS de `Image` no aplica igual que el de `fetch`, pero los hosts permitidos en plataforma sí importan: Android bloquea HTTP cleartext por defecto en `release`, y Expo necesita `localhost:9000` en la config de red).

Coordinar con quien mantiene `paceron-frontend` (ver `AGENTS.md` §8).

## Troubleshooting

**`docker: command not found`** → symlinks de la sección Requisitos.

**El backend no ve las variables de `.env.local`** → falta `--stage=local`. El log de arranque dice `stage resolved: LOCAL`; si dice `testing`, no lo pasó.

**`pg_restore: unsupported version (1.16) in file header`** → la imagen local es más vieja que el dump. Correr `POSTGRES_IMAGE=postgres:17-alpine make local-up`. El script de restore también lo detecta y lo dice con las dos versiones.

**Fotos dan 404 en local** → falta `S3_PUBLIC_BASE_URL`, o el bucket no tiene lectura pública (revisar `make local-logs` del `storage-init`).

**El bucket no tiene las credenciales esperadas** → RustFS lee `RUSTFS_ACCESS_KEY`/`RUSTFS_SECRET_KEY`. **No** lee `RUSTFS_ROOT_USER` ni `MINIO_ROOT_USER`: probados los dos, ambos caen a las credenciales por defecto con un warning en el log del storage.

**Las fotos existentes de los usuarios no aparecen** → esperado. Solo se clonó la base, no los objetos de Supabase Storage. Subí las que necesites por la API o contra el bucket.

**`Error initializing mailer`** → falta `RESEND_API_KEY` en `.env`. No bloquea el arranque del server, solo el envío de emails.

**Puerto 5432 ocupado** → hay un Postgres del sistema. Matarlo, o mover el compose con `POSTGRES_PORT=5433` (y actualizar `DATABASE_URL` en `.env.local`).