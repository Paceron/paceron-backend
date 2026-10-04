## Context

El repo tiene hoy 2 ambientes de infra (testing/producción) resueltos por `--stage=production` (`config.go:49`), y las dos instancias son de Supabase. El pedido original era "levantar Postgres + MinIO y clonar la BD", pero al leer el código aparecieron cuatro supuestos del plan que no se sostienen contra esta base de código:

| El plan asumía | La realidad |
|---|---|
| `DATABASE_URL=localhost:5432` | `DATABASE_URL` **no se lee**. `stagedDatabaseURL()` (`config.go:246`) solo mira `SUPABASE_TESTING_DATABASE_URL` / `SUPABASE_PRODUCTION_DATABASE_URL` |
| `S3_ENDPOINT` + `S3_FORCE_PATH_STYLE` | No existen esas vars. `loadStorageConfig()` (`config.go:325`) arma el prefijo `SUPABASE_TESTING_S3_` / `SUPABASE_PRODUCTION_S3_` |
| "refactorizar para respetar `forcePathStyle`" | El cliente ya tenía `o.UsePathStyle = true` **hardcodeado**. No había nada que refactorizar, solo dejarlo configurable |
| El storage local puede servir las descargas | `PublicBaseURL()` **derivaba** `https://<ref>.supabase.co/storage/v1/object/public/<bucket>`. Con un storage local, todo avatar/ícono de equipo apuntaría a `https://localhost.supabase.co/...` → roto |

Sin resolver eso, `.env.local.example` con `DATABASE_URL`/`S3_ENDPOINT` habría sido un archivo inerte: el backend ni lo miraría.

Verificado además que la dependencia de Supabase Auth ya es evitable: los modelos en `cmd/api/domains/dbs/` no tienen ninguna FK a `auth.users` ni referencias a `auth.uid()`. El login es JWT propio + tabla `users` propia (ver `docs/AUTH_MIGRATION.md`).

### Desvíos del pedido original, y por qué

Dos partes del pedido no se pudieron cumplir tal cual. Ambas se نعgocian acá porque son cambios de herramienta, no de alcance:

| Pedido | Qué se hizo | Por qué |
|---|---|---|
| MinIO | **RustFS** (`rustfs/rustfs:latest`) | Las imágenes oficiales de MinIO ya no se pueden pulls: `minio/minio` da 404 en Docker Hub y el registry de quay responde unauthorized. RustFS es S3-compatible y maintained |
| `postgres:15-alpine` | **`postgres:17-alpine`** | La base real de Supabase corre **PG 17.6**, no 15. DumpVerified: un `pg_restore` de la imagen 15 no puede ni leer el archivo, con `unsupported version (1.16) in file header` |

Ninguna de las dos diferencias afecta el resultado buscado: un S3-compatible y un Postgres son un S3-compatible y un Postgres. Lo que cambia es la versión de la imagen, que queda parametrizada (`POSTGRES_IMAGE`, `RUSTFS_IMAGE`).

## Goals / Non-Goals

**Goals:**
- Levantar Postgres + storage con `docker compose up -d` y correr la app contra ellos, con los datos reales de Supabase clonados.
- Que funcione sin instalar PostgreSQL cliente ni nada más que Docker en el host.
- Cero riesgo de regresión en Render/CI: el stage local es opt-in explícito.
- Que el restore sea idempotente y re-corrible (un dev va a resetear la BD muchas veces).

**Non-Goals:**
- Reemplazar la base por Supabase completo (con su Auth, Storage y Realtime). El backend no usa ninguno de los tres.
- Tocar los targets `test-db-*` del `Makefile` ni `ci.yml` (`postgres:16-alpine` en :5433). Son otra cosa: base de test descartable, no entorno de desarrollo. Solo se evita que choquen puertos.
- Clonar los objetos ya subidos a Supabase Storage. Solo la base.
- Pinear la imagen de RustFS a una versión fija. Hoy va con `latest`, que es un tradeoff consciente (ver Risks).

## Decisions

### 1. Stage `local` explícito con `--stage=local`, no default

El stage se activa **solo** con el flag `--stage=local`, nunca por default.

La alternativa obvia —"local es el default, cloud es el override"— es la que da el nombre al patrón Supabase, pero acá es exactamente la decisión equivocada: el default actual (`testing`) es lo que evita que un `go run` suelto toque producción, y los dos servicios de `render.yaml` (que corren sin flag, uno desde `master`) resuelven a `testing`/`production` por ese default. Invertirlo cambia el comportamiento de un deploy existente. Con flag opt-in, un dev que se olvide del flag no rompe nada: cae en testing cloud y el log de arranque (`router.go:36`) dice `stage resolved: testing`, que es la señal visible de que faltaba el flag. Ese log ahora distingue los tres casos.

Cuando vienen los dos flags (`--stage=production --stage=local`), gana `local`. Conectar a la base local es reversible; haber tocado producción no.

### 2. Variables con prefijo por stage, reutilizando `stagedDatabaseURL`

Se sigue el patrón que ya existe en el archivo en vez de inventar un segundo mecanismo: una función `stagedStorageEnvPrefix()` (`config.go:315`) que devuelve `S3_` para local y `SUPABASE_{TESTING,PRODUCTION}_S3_` para cloud. El resultado es que bajo `--stage=local` las variables se llaman exactamente `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_ID`, `S3_SECRET_KEY`, `S3_BUCKET` — los nombres que pedía el plan original.

Dos variables quedan **fuera** del esquema de prefijo porque son genéricas de S3 y no tienen nada de Supabase:

- `S3_FORCE_PATH_STYLE` — leída en todos los stages, **default `true`**. Es el valor que el código ya usaba hardcodeado, así que el default preserva el comportamiento actual contra Supabase. `false` queda disponible para un S3 real con virtual-host-style.
- `S3_PUBLIC_BASE_URL` — override de la URL pública. Sin default: si está vacía, se usa el derivado de Supabase de siempre.

`DATABASE_URL` revive como variable del stage local (no se leía desde la migración a los prefijos, según `docs/ENVIRONMENTS.md`).

### 3. `pg_dump` y `pg_restore` corren dentro de contenedores

El host no necesita `postgresql-client`. Además no es un detalle de comfort: `pg_dump` **no puede ser más viejo que el server**, y `pg_restore` **no puede ser más viejo que el formato del archivo**. Por eso `dump_db.sh` primero levanta un contenedor efímero, pregunta `SHOW server_version_num`, y después corre el `pg_dump` real en un `postgres:<major>-alpine` con la misma versión. La versión del dump nunca se adivisa — y como el dump hoy es de un PG 17, el restore local tiene que correr en 17 también.

El dump excluye los schemas que administra Supabase (`auth`, `storage`, `_supabase`, `graphql_public`, `vault`, `pgsodium*`, `realtime`, `supabase_functions`, `extensions`) y usa `--no-owner --no-privileges`, porque los roles `anon` / `authenticated` / `service_role` / `supabase_admin` no existen en un Postgres vanilla.

### 4. `pg_restore` no-fatal + verificación

El dump trae objetos que no restauran limpio en vanilla Postgres. Al inspeccionar el dump de verdad, el ruido real resultó ser de extensiones de Supabase (`vault`, `pgsodium`, `pg_graphql`) y event triggers, **no** de RLS: las tablas de `public` del dump no traen policies ni dependencias de `auth.uid()` (el backend no usa Supabase Auth, ver Context). El design original daba por hecho lo contrario y estaba equivocado.

Por eso el restore corre con `--exit-on-error` apagado, captura el log, y después verifica que las tablas de la app existan y tengan filas, imprimiendo los conteos. El script falla si alguna quedó vacía: así un restore "verde" no puede enmascarar uno a medias.

El paso de `ALTER TABLE ... DISABLE ROW LEVEL SECURITY` sobre `public` se conserva, pero como **defensa idempotente**, no porque hoy haga falta: si mañana alguien dump-ea una base con RLS activo, el backend no va a ver cero filas con el mismo aspecto de un bug de aplicación. Cuesta una línea y no estorba.

El dump se copia con `docker cp` en lugar de piped por stdin: el formato custom de `pg_dump` necesita input seekable y no vale la pena depender de ese comportamiento.

### 5. Bucket listo sin pasos manuales, con `aws-cli`

Un storage recién arrancado tiene el bucket vacío. `paceron-media` podría crearse a mano una vez y persistir en el volumen, pero eso hace que `local-reset` (que borra volúmenes) deje el entorno inconsistente y que el DoD dependa de un paso manual no documentado. Un servicio `storage-init` de un solo uso hace el compose auto-suficiente: `docker compose up -d` y listo.

Se usa `amazon/aws-cli` en vez de `minio/mc`. Ninguna de las dos opciones depende de que MinIO siga publicando imágenes (`mc` es del mismo proyecto que dejó de hacerlo), pero `aws-cli` habla S3 estándar y no arrastra el acoplamiento. El script crea el bucket con `--ignore-existing` y después aplica la policy de lectura pública, que es lo que permite que el frontend cargue avatares sin credenciales.

Las credenciales de RustFS son `RUSTFS_ACCESS_KEY` / `RUSTFS_SECRET_KEY`. Verificado contra el servidor: **no** lee `RUSTFS_ROOT_USER` ni `MINIO_ROOT_USER` (probados los dos; ambos caen a las credenciales por defecto con un warning en el log).

### 6. `postgres:17-alpine`, versión parametrizable

El pedido original decía 15, pero el dump es de un PG 17.6 real y la imagen 15 no puede restaurarlo. La versión queda en `POSTGRES_IMAGE` (default `postgres:17-alpine`) para ajustarla sin editar el compose.

La serie `alpine` se mantiene por ser la imagen más liviana y por traer `contrib`, necesario para las extensiones que existen en vanilla (`pgcrypto`, `uuid-ossp`) y que el dump pueda pedir.

### 7. Puerto 5432, verificado libre

El plan pedía 5432 y `lsof` confirmó que está libre en esta máquina. No choca con `make test-db-up`, que usa 5433 a propósito.

## Risks / Trade-offs

- **Restore con errores no fatales**: la corrida imprime errores de objetos de Supabase. Es esperado y el script los distingue de un fallo real por el paso de verificación final. El riesgo es que alguien lo lea como "funciona a medias" — mitigado con output explícito de qué se ignoró y por qué.
- **Un dev se olvida de `--stage=local` y pega contra Supabase testing**: mitigado por el log de stage al arrancar (decisión 1), no por prevención. Una mejora futura sería pedir confirmación interactiva si el entorno dice local y el stage resuelto es cloud.
- **El dump contiene datos reales de usuarios** (emails, nombres): `backup/` tiene que quedar fuera de git. Se agrega a `.gitignore`, y `docs/ENTORNO_LOCAL.md` avisa que no se comparta.
- **Fotos ya subidas no se clonan**: el DoD pide "subir/descargar", no "clonar el bucket". Migrar los objetos existentes de Supabase Storage es un script aparte y no se hizo acá. Un dev que necesite fixtures de media los sube por la API.
- **`rustfs/rustfs:latest` sin pin**: si upstream publica un breaking change, un `local-up` futuro puede fallar sin que haya cambiado nada del repo. Se acepta hoy porque no hay un tag estable publicado que valga la pena fijar; cuando lo haya, `RUSTFS_IMAGE` es el lugar.