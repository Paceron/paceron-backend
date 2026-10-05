## ADDED Requirements

### Requirement: Entorno local orquestado por Docker Compose
El sistema SHALL proveer un `docker-compose.yml` en la raíz que levante, con un solo `docker compose up -d`, un Postgres y un storage S3-compatible locales, ambos con volumen persistente.

#### Scenario: Levantar el entorno completo
- **WHEN** el desarrollador ejecuta `docker compose up -d` (o `make local-up`) en un repo limpio
- **THEN** quedan disponibles Postgres en `localhost:5432` y la API S3 en `localhost:9000` con la consola web en `localhost:9001`, ambos con healthcheck, y el bucket de media `paceron-media` ya creado

#### Scenario: Los datos sobreviven a un reinicio
- **WHEN** el desarrollador corre `docker compose down` y después `docker compose up -d`
- **THEN** las filas de la base y los objetos subidos al storage siguen ahí, porque ambos servicios usan volúmenes nombrados

#### Scenario: El storage arranca con la consola en el puerto correcto
- **WHEN** el contenedor de storage arranca
- **THEN** su consola web responde en `http://localhost:9001` con las credenciales `RUSTFS_ACCESS_KEY`/`RUSTFS_SECRET_KEY`

#### Scenario: El bucket sirve lecturas anónimas
- **WHEN** termina de correr el servicio `storage-init`
- **THEN** el bucket `paceron-media` existe con una policy que permite `GetObject` público, para que el frontend pueda cargar avatares e íconos de equipo sin credenciales; las escrituras y borrados siguen requiriendo autenticación

### Requirement: Clasificación de datos desde Supabase sin dependencias en el host
El sistema SHALL proveer un script que ejecute `pg_dump` en formato custom (`-F c`) dentro de un contenedor, sin requerir PostgreSQL cliente instalado en el host.

#### Scenario: El host no tiene `pg_dump`
- **WHEN** el desarrollador corre `scripts/dump_db.sh` en una máquina donde no existe el binario `pg_dump` y sí existe Docker
- **THEN** el dump se genera igual, porque el `pg_dump` corre dentro de un contenedor efímero

#### Scenario: El cliente de `pg_dump` nunca es más viejo que el server
- **WHEN** el proyecto de Supabase corre una versión de Postgres distinta a la de la imagen local
- **THEN** el script detecta la versión mayor del server remoto y dumpea con un contenedor `postgres:<esa-major>-alpine`, en lugar de usar la versión local hardcodeada

#### Scenario: La imagen local puede leer el formato del dump
- **WHEN** el dump fue generado contra un servidor con una versión mayor a la de la imagen local configurada
- **THEN** el restore falla con un mensaje que nombra las dos versiones, en lugar de un error de header incomprensible

#### Scenario: El dump no arrastra objetos administrados por Supabase
- **WHEN** se genera el dump
- **THEN** excluye los schemas gestionados por Supabase y usa `--no-owner --no-privileges`, porque los roles y extensiones de Supabase no existen en un Postgres vanilla y harían fallar el restore

#### Scenario: Sin credenciales en el `.env`
- **WHEN** el script no encuentra una URL de base de datos en el archivo `.env` que se le indique
- **THEN** falla con un mensaje que dice qué variable falta y en qué archivo la esperó, sin volcar la URL (que contiene la password) al output

### Requirement: Restauración del dump en el contenedor local
El sistema SHALL proveer un script que restaure el dump en el Postgres local y deje la base utilizable por el backend, sin requerir herramientas de PostgreSQL en el host.

#### Scenario: Restaurar contra el contenedor recién creado
- **WHEN** el desarrollador corre `scripts/restore_db.sh` con el servicio `db` del compose arriba
- **THEN** espera a que Postgres esté listo, copia el dump al contenedor y ejecuta `pg_restore` con `--no-owner --no-privileges`

#### Scenario: Objetos de Supabase que no se pueden recrear
- **WHEN** el restore encuentra objetos que dependen de infraestructura de Supabase ausente en vanilla Postgres (extensiones como `vault`, `pgsodium` o `pg_graphql`, y sus event triggers)
- **THEN** el restore no aborta por esos errores, los lista como no fatales y sigue adelante

#### Scenario: El backend puede leer las filas restauradas
- **WHEN** terminó el restore
- **THEN** el script desactiva Row Level Security en las tablas de `public` como paso defensivo idempotente, para que el backend conectado como owner nunca quede filtrando cero filas de forma silenciosa

#### Scenario: Un restore a medias se reporta como falla
- **WHEN** al terminar el restore alguna tabla de la aplicación no existe o quedó vacía
- **THEN** el script sale con código de error e informa cuál, en vez de dejar un entorno aparentemente sano

### Requirement: Stage de configuración `local`
El sistema SHALL permitir resolver base de datos y storage contra el entorno local mediante un flag explícito, sin alterar la resolución de los stages existentes.

#### Scenario: Correr con el stage local
- **WHEN** el proceso arranca con `--stage=local`
- **THEN** la base se resuelve desde `DATABASE_URL` y el storage desde `S3_ENDPOINT`/`S3_REGION`/`S3_ACCESS_ID`/`S3_SECRET_KEY`/`S3_BUCKET`

#### Scenario: Correr sin flag sigue igual que hoy
- **WHEN** el proceso arranca sin `--stage=local` y sin `--stage=production`
- **THEN** la resolución es idéntica a la actual: base y storage del proyecto de Supabase testing

#### Scenario: El stage resuelto es visible al arrancar
- **WHEN** el proceso termina de cargar la configuración
- **THEN** loguea cuál de los tres ambientes resolvió, para que un dev que se olvidó de `--stage=local` lo note en la consola

#### Scenario: Las dos etapas no se pisan
- **WHEN** se pasan los dos flags (`--stage=local` y `--stage=production`)
- **THEN** el proceso no entra en un estado ambiguo: `local` gana, por ser el ambiente con menos consecuencias si se resuelve mal

### Requirement: Variables de path-style y URL pública del storage
El sistema SHALL exponer la configuración de path-style y de URL pública del bucket como variables de entorno, con defaults que preserven el comportamiento actual contra Supabase.

#### Scenario: Path-style por default
- **WHEN** `S3_FORCE_PATH_STYLE` no está seteada
- **THEN** el cliente S3 usa path-style, que es lo que requieren tanto Supabase Storage como el storage local

#### Scenario: Path-style se lee igual en los tres ambientes
- **WHEN** el proceso arranca en cualquier stage (local, testing o producción)
- **THEN** `S3_FORCE_PATH_STYLE` se aplica igual, porque no es específica de un ambiente

#### Scenario: Path-style se puede desactivar
- **WHEN** `S3_FORCE_PATH_STYLE` está seteada a `false`/`0`/`no`
- **THEN** el cliente S3 usa virtual-host-style

#### Scenario: URL pública con override
- **WHEN** `S3_PUBLIC_BASE_URL` está seteada
- **THEN** las URLs públicas de avatar e ícono de equipo se arman con ese valor como base

#### Scenario: URL pública sin override contra Supabase
- **WHEN** `S3_PUBLIC_BASE_URL` no está seteada
- **THEN** se deriva igual que hoy desde el endpoint, con la forma `https://<project-ref>.supabase.co/storage/v1/object/public/<bucket>`