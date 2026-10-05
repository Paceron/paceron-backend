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

### `.env` y `.env.local`: por qué dos archivos

`config.LoadValues()` hace, en este orden:

```go
godotenv.Load()                        // lee .env
if IsLocalStage() {                    // --stage=local
    godotenv.Overload(".env.local")    // .env.local PISA a .env
}
```

`Load` no sobreescribe lo que ya está en el entorno; `Overload` sí. Por eso el orden importa: primero `.env`, después `.env.local` encima.

| Comando | Lee | Gana |
|---|---|---|
| `go run ./cmd/api --stage=local` | `.env` + `.env.local` | `.env.local` → infra Docker |
| `go run ./cmd/api` | `.env` | Supabase testing |
| `go run ./cmd/api --stage=production` | `.env` | Supabase producción |

`.env` conserva los valores reales de Supabase; `.env.local` los pisa solo cuando se pide el flag. **No renombres uno con el nombre del otro**: perderías el cloud para siempre y los dos se pisarían entre sí.

Generar `.env.local` a partir de `.env` (copia literal + solo la infra reemplazada):

```bash
# .env.local.example es el template limpio; la variante "copia de .env" se hace a mano:
cp .env .env.local
# reemplazar en .env.local:
#   DATABASE_URL=postgresql://postgres:postgres@localhost:5432/paceron_local
#   y AGREGAR el bloque S3_* (ver abajo), que .env no trae
```

Ojo: `.env` solo tiene `SUPABASE_TESTING_S3_*` / `SUPABASE_PRODUCTION_S3_*`, que el backend lee con esos prefijos en cloud. Bajo `--stage=local` el prefijo es `S3_`, así que **hay que agregar** `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_ID`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_FORCE_PATH_STYLE` y `S3_PUBLIC_BASE_URL` — si no, el storage sigue yendo a Supabase.

**Decisión de seguridad:** si en `.env.local` quedan los valores reales de `MERCADOPAGO_*`, los endpoints de MercadoPago desde local pegan contra la cuenta real y pueden generar pagos reales. Para probar pagos en local, poné credenciales de test. `.env.local` es gitignored, pero sigue siendo una copia más de los secretos: no lo comitees ni lo compartas.

### Debug en VS Code (`.vscode/launch.json`)

El flag va en `args`. Dos configuraciones para no editar el JSON al cambiar de ambiente:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "paceron backend (local: Docker)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/api",
      "cwd": "${workspaceFolder}",
      "envFile": "${workspaceFolder}/.env",
      "args": ["--stage=local"],
      "console": "integratedTerminal"
    },
    {
      "name": "paceron backend (testing: Supabase)",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/api",
      "cwd": "${workspaceFolder}",
      "envFile": "${workspaceFolder}/.env",
      "args": [],
      "console": "integratedTerminal"
    }
  ]
}
```

**`envFile` se queda en `.env`, no en `.env.local`.** El debugger de Go lo inyecta en el entorno del proceso *antes* de que arranque el programa; después `Load()` no opa nada (ya está todo seteado) y `Overload(".env.local")` gana igual. Si lo apuntás a `.env.local`, también funciona, pero perdés poder debuggear contra testing con el mismo archivo y el comportamiento pasa a depender del orden de carga en vez de ser explícito por el flag.

Los tests de `daos/` corren aparte (`make test-with-db`), así que el debug local no estorba.

Si el log de arranque dice `stage resolved testing` en vez de `LOCAL`, falta el flag. Ese log es la única señal visible de a dónde pegó el proceso.

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

## Demo: congelar la base y volver a ella

Para cuando los datos se ensucian y hay que volver a la foto exacta: una demo con
varias rondas donde los participantes tocan todo, pruebas manuales, etc.

`make local-restore` **no sirve para esto**. Dos motivos concretos:

- Sin `--file` elige el `.dump` más reciente de `backup/` por fecha. Si alguien
  corre `make local-dump` durante la demo, restaura el estado roto y reporta éxito
  igual, porque el chequeo de versión y la verificación dan el visto bueno igual.
- Su verificación ("la tabla existe y tiene filas") no distingue 34 usuarios de
  40. Un restore a medias se ve sano.

`scripts/demo_db.sh` resuelve las dos cosas con un **baseline con nombre fijo**:

| Comando | Qué hace | Cuándo |
|---|---|---|
| `make demo-baseline` | Congela el estado actual: `backup/baseline.dump` + `baseline.counts` + `baseline.sha256`. | **Una vez**, antes de empezar. |
| `make demo-restore` | Vuelve al baseline en caliente, en ~3 s. | **Entre rondas.** |
| `make demo-reset` | Vuelve al baseline en frío: `down -v` + compose completo. ~15 s. | Antes de empezar y al terminar. |
| `make demo-verify` | Compara los conteos de las 33 tablas contra la huella. Sale ≠ 0 si difieren. | Cuando quieras saber en qué estado está. |
| `make demo-status` | Estado del baseline, contenedores, y si la base difiere. | Diagnóstico. |

```bash
# una vez, antes de la demo
make demo-baseline

# entre rondas
make demo-restore

# al terminar
make demo-reset
go run ./cmd/api --stage=local      # el backend hay que relanzarlo después de reset
```

Lo que hace que sea confiable:

- **El baseline no se pisa por accidente.** `demo-baseline` se niega a
  sobrescribir el dump existente; hace falta `make demo-baseline-force`. Y
  `restore`/`reset` chequean el sha256 antes de restaurar, así que un baseline
  pisado a mano o corrupto aborta en vez de "restaurar otra cosa en silencio".
- **`restore` es rápido porque no recrea nada.** Usa
  `pg_restore --clean --if-exists` sobre el contenedor que ya está arriba: borra y
  recrea cada objeto del dump. Su límite es que `--clean` sólo dropea lo que está
  **en** el dump, así que una tabla que la demo haya creado sobreviviría. Para eso
  está `demo-reset`, que borra los volúmenes y no tiene ese límite.
- **`reset` levanta el compose completo, no sólo `db`.** Es la diferencia con
  `restore_db.sh --reset`, que hace `down -v` (y `down -v` borra también
  `paceron-s3-data`) pero después sólo levanta `up -d db`: el bucket no vuelve a
  existir y **las fotos de perfil dan 404**. `demo-reset` levanta `db` + `storage`
  + `storage-init`.
- **El restore es atómico.** Va con `--single-transaction`: si falla a mitad,
  queda la base que había antes del intento, no una a medias.
- **`restore` se autoverifica.** Termina corriendo `demo-verify`, así que no
  depende de que alguien se acuerde de mirarlo.

Dos límites que conviene tener claros:

- **`verify` compara conteos de filas, no el contenido.** Una ronda que sólo edite
  filas (un `UPDATE`, sin alta ni baja) pasa la verificación aunque la base esté
  distinta. El restore sí revierte esas ediciones — el dump va con los valores —
  así que es una limitación de la verificación, no del restore.
- **El backend hay que relanzarlo después de `demo-reset`** (`--stage=local`). Con
  `demo-restore` no hace falta: la conexión sigue viva.

El instructivo corto para quien opera la demo está en [`INSTRUCTIVO-DEMO.html`](../INSTRUCTIVO-DEMO.html).

`backup/baseline.*` tiene datos reales de usuarios, como el resto de `backup/`.

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

## Llevar el entorno a otra máquina

La **infraestructura** es portable: `docker-compose.yml`, los scripts y el `.env.local.example` están en el repo, así que en otra notebook alcanza con clonar, `make local-up` y `make local-restore`. El compose fija `name: paceron` y las imágenes son las mismas para todos, así que los volúmenes se crean con los mismos nombres.

Lo que **no** se transfiere:

- **La VM de Docker Desktop** (en macOS no es exportable).
- **Los volúmenes con datos**: `paceron_paceron-db-data` y `paceron_paceron-s3-data` atan a plataforma, major de Postgres y arquitectura. Se pueden respaldar con un `tar` del volumen, pero eso sirve para recuperar *esa* máquina, no para compartir el entorno.
- **Los datos**: `backup/` está gitignored a propósito, porque son datos reales de usuarios. No viaja por git y no se comparte.
- **`.env` y `.env.local`**: los secretos no viajan por git. Hay que pasarlos por un canal seguro.

### Procedimiento en otra notebook

1. Clonar el repo.
2. Copiar `.env` (credenciales de Supabase) y armar `.env.local` — ver la sección de arriba.
3. `make local-up`.
4. Conseguir un dump y restaurarlo:

```bash
# En la máquina que ya tiene los datos:
./scripts/dump_db.sh --out backup/paceron.dump     # ~260K
```

```bash
# En la nueva notebook:
cp /ruta/al/dump backup/paceron.dump
make local-restore
```

El dump pesa ~260K, o sea que pasarlo por Drive/Slack/email es trivial. **Solo si es a alguien de confianza**: contiene emails, nombres y DNI de usuarios reales.

Alternativa preferible si el destino es otra persona del equipo: que cada uno saque su propio dump con `make local-dump` en lugar de compartir el de otro. Los datos de Supabase testing son los mismos para todos, y nadie tiene que recibir datos reales de otra máquina.

Si lo que se necesita es un entorno compartido sin datos reales, el camino es un **seed script con datos ficticios** (más `make local-reset` + seed en vez de restore). No existe todavía; es la deuda conocida de este setup.

Ojo con un detalle de versión: la imagen de Postgres tiene que poder leer el formato del dump (hoy `postgres:17-alpine`). Si mañana Supabase sube de major, el restore local falla con `unsupported version (1.16) in file header` hasta que se cambie `POSTGRES_IMAGE`.

## Troubleshooting

**`docker: command not found`** → symlinks de la sección Requisitos.

**El backend no ve las variables de `.env.local`** → falta `--stage=local`. El log de arranque dice `stage resolved: LOCAL`; si dice `testing`, no lo pasó.

**`pg_restore: unsupported version (1.16) in file header`** → la imagen local es más vieja que el dump. Correr `POSTGRES_IMAGE=postgres:17-alpine make local-up`. El script de restore también lo detecta y lo dice con las dos versiones.

**Fotos dan 404 en local** → falta `S3_PUBLIC_BASE_URL`, o el bucket no tiene lectura pública (revisar `make local-logs` del `storage-init`).

**El bucket no tiene las credenciales esperadas** → RustFS lee `RUSTFS_ACCESS_KEY`/`RUSTFS_SECRET_KEY`. **No** lee `RUSTFS_ROOT_USER` ni `MINIO_ROOT_USER`: probados los dos, ambos caen a las credenciales por defecto con un warning en el log del storage.

**Las fotos existentes de los usuarios no aparecen** → esperado. Solo se clonó la base, no los objetos de Supabase Storage. Subí las que necesites por la API o contra el bucket.

**`Error initializing mailer`** → falta `RESEND_API_KEY` en `.env`. No bloquea el arranque del server, solo el envío de emails.

**Puerto 5432 ocupado** → hay un Postgres del sistema. Matarlo, o mover el compose con `POSTGRES_PORT=5433` (y actualizar `DATABASE_URL` en `.env.local`).