#!/usr/bin/env bash
# Clona la base de datos de Supabase a un dump local, para después restaurarla
# en el Postgres del docker-compose (scripts/restore_db.sh).
#
#   scripts/dump_db.sh                      # lee .env, usa SUPABASE_TESTING_DATABASE_URL
#   scripts/dump_db.sh --url postgresql://…  # URL explícita
#   scripts/dump_db.sh --env-file .env.prod  # otro archivo de credenciales
#   scripts/dump_db.sh --out backup/dump.dump
#
# Detalles que hacen que funcione sin instalar nada en el host:
#
#   - `pg_dump` corre DENTRO de un contenedor postgres:<major>-alpine, así que
#     no hace falta postgresql-client en la máquina. Y la versión del cliente
#     tiene que coincidir con la del server: un pg_dump más viejo que el server
#     aborta con "server version mismatch". Por eso el script primero pregunta la
#     versión al server remoto y después baja la imagen que la matchea.
#
#   - Se excluyen los schemas que administra Supabase (auth, storage, vault,
#     graphql, pgsodium, …) y se usa --no-owner/--no-privileges: ninguno de
#     esos existe en un Postgres vanilla y hacen fallar el restore.
#
#   - La URL se le pasa a libpq tal cual (él la parsea y la URL-decodea), así que
#     este script no la descompone. Solo la redacciona para poder imprimirla.
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Imagen para preguntar la versión del server. Cualquiera moderna sirve: un
# cliente más nuevo contra un server más viejo siempre funciona (al revés no).
readonly PROBE_IMAGE="${DUMP_PROBE_IMAGE:-postgres:17-alpine}"

# Schemas que administra Supabase. No aparecen en una base creada desde cero con
# los modelos GORM del proyecto, y varios extensiones (pgsodium, supabase_vault)
# ni siquiera existen en postgres:15-alpine.
readonly EXCLUDED_SCHEMAS=(
  _supabase
  auth
  storage
  extensions
  vault
  pgsodium
  pgsodium_masks
  realtime
  supabase_functions
  graphql_public
  graphql
  pgtle
  pgbouncer
  net
  cron
)

# Orden en que se buscan las variables de conexión, para no tener que adivinar
# cuál está seteada.
readonly URL_VARS=(
  SUPABASE_TESTING_DATABASE_URL
  SUPABASE_PRODUCTION_DATABASE_URL
  DATABASE_URL
)

env_file="$REPO_ROOT/.env"
db_url=""
out_file=""

usage() {
  sed -n '2,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

die() {
  echo "error: $*" >&2
  exit 1
}

# redact_url deja la URL imprimible: la password nunca se muestra.
redact_url() {
  sed -E 's#://([^:@/]+):[^@]*@#://\1:***@#' <<<"$1"
}

# read_env_var busca KEY en el archivo y devuelve su valor (vacío si no está).
#
# Tolera `KEY=value`, `export KEY=value`, espacios alrededor del `=`, comillas
# simples o dobles, y comentarios al final de la línea. Todo en un único awk a
# propósito: un `sed | head -n1` con `set -o pipefail` puede salir con 141
# (SIGPIPE) cuando head corta antes de que sed termine, y eso mata el script.
read_env_var() {
  local file="$1" key="$2"
  [[ -f "$file" ]] || return 0
  awk -v key="$key" '
    /^[[:space:]]*($|#)/ { next }
    {
      line = $0
      sub(/^[[:space:]]*export[[:space:]]+/, "", line)
      sub(/^[[:space:]]+/, "", line)
      pat = "^" key "[[:space:]]*="
      if (match(line, pat) != 1) next

      v = substr(line, RLENGTH + 1)
      gsub(/^[[:space:]]+/, "", v)
      gsub(/[[:space:]]+$/, "", v)

      # Valor entrecomillado: no se toca nada adentro (un # puede ser dato).
      if (v ~ /^".*"$/ || v ~ /^'"'"'.*'"'"'$/) {
        v = substr(v, 2, length(v) - 2)
      } else {
        sub(/[[:space:]]+#.*$/, "", v)
        gsub(/[[:space:]]+$/, "", v)
      }
      print v
      exit
    }
  ' "$file"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -f | --env-file)
      [[ $# -ge 2 ]] || die "$1 necesita un valor"
      env_file="$2"
      shift 2
      ;;
    -u | --url)
      [[ $# -ge 2 ]] || die "$1 necesita un valor"
      db_url="$2"
      shift 2
      ;;
    -o | --out)
      [[ $# -ge 2 ]] || die "$1 necesita un valor"
      out_file="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *) die "argumento desconocido: $1 (probá --help)" ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "no se encontró el binario 'docker'"
docker info >/dev/null 2>&1 || die "el daemon de Docker no está corriendo (¿abriste Docker Desktop?)"

if [[ -z "$db_url" ]]; then
  [[ -f "$env_file" ]] || die "no existe el archivo de env '$env_file'. Pasalo con --env-file, o la URL con --url."
  for var in "${URL_VARS[@]}"; do
    candidate="$(read_env_var "$env_file" "$var")"
    if [[ -n "$candidate" ]]; then
      db_url="$candidate"
      echo "→ conexión tomada de $var en $(basename "$env_file")"
      break
    fi
  done
  [[ -n "$db_url" ]] || die "no encontré ninguna de ${URL_VARS[*]} en $env_file. Pasala con --url."
fi

[[ -n "$out_file" ]] || out_file="$REPO_ROOT/backup/paceron-$(date +%Y%m%d-%H%M%S).dump"

# Rutas absolutas antes de mkdir y antes de montar: `docker run -v backup:/out` con
# una ruta relativa NO es un bind mount, Docker lo interpreta como un volumen
# nombrado y el dump se pierde adentro de un volumen anónimo. El síntoma es
# "terminó sin error pero el archivo no está".
mkdir -p "$(dirname "$out_file")"
out_dir="$(cd "$(dirname "$out_file")" && pwd)"
out_file="$out_dir/$(basename "$out_file")"

# Nombre de archivo interno dentro del contenedor (solo es válido como path
# POSIX, el de arriba puede tener espacios).
readonly remote_dump="/out/$(basename "$out_file")"

echo "→ origen: $(redact_url "$db_url")"

# ---------------------------------------------------------------------------
# 1. ¿Qué versión de Postgres corre el server remoto?
# ---------------------------------------------------------------------------
echo "→ detectando versión del server remoto…"
server_version_num="$(
  docker run --rm --network host "$PROBE_IMAGE" \
    psql "$db_url" -w -tAc 'SHOW server_version_num' 2>/dev/null | tr -d '[:space:]'
)" || die "no pude conectarme al server. Chequeá la URL, la red y que el proyecto de Supabase esté accesible."

[[ -n "$server_version_num" ]] || die "el server no devolvió server_version_num. Chequeá la URL y las credenciales."

# server_version_num codifica major, minor y patch con ceros a la izquierda:
# 170006 es 17.6, 150019 es 15.19. La major son los primeros dos dígitos si el
# número es de 5+ (Postgres 10+), y el primero si es de 4 (9.x). El minor NO son
# los dos dígitos siguientes: hay que leer el resto como entero, o 170006 se
# muestra como 17.00. La major es lo único que decide la imagen del cliente.
if (( ${#server_version_num} >= 5 )); then
  server_major="${server_version_num:0:2}"
  server_minor=$(( 10#${server_version_num:2} ))
else
  server_major="${server_version_num:0:1}"
  server_minor=$(( 10#${server_version_num:1:2} ))
fi
readonly dump_image="${DUMP_IMAGE:-postgres:${server_major}-alpine}"
echo "→ server remoto: Postgres ${server_major}.${server_minor} | cliente: ${dump_image}"

# ---------------------------------------------------------------------------
# 2. El dump
# ---------------------------------------------------------------------------
exclude_args=()
for schema in "${EXCLUDED_SCHEMAS[@]}"; do
  exclude_args+=("--exclude-schema=${schema}")
done

echo "→ dumping a $(basename "$out_file")…"
docker run --rm --network host \
  -v "$(dirname "$out_file"):/out" \
  "$dump_image" \
  pg_dump "$db_url" \
  -w \
  -F c \
  --no-owner \
  --no-privileges \
  "${exclude_args[@]}" \
  -f "$remote_dump"

[[ -f "$out_file" ]] || die "pg_dump terminó sin error pero no escribió nada en $out_file"
[[ -s "$out_file" ]] || die "pg_dump terminó sin error pero $out_file quedó vacío (0 bytes)"

size="$(du -h "$out_file" | cut -f1 | tr -d '[:space:]')"
echo "→ listo: $out_file ($size)"

cat <<EOF

Siguiente paso:
  make local-restore      # restaura el dump en el contenedor db local

El dump contiene datos reales (emails, nombres). backup/ está en .gitignore:
no lo commitees ni lo compartas.
EOF