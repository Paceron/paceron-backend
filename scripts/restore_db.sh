#!/usr/bin/env bash
# Restaura un dump de Supabase en el Postgres del docker-compose.
#
#   scripts/restore_db.sh                     # el dump más reciente de backup/
#   scripts/restore_db.sh --file backup/x.dump
#   scripts/restore_db.sh --reset             # borra el volumen antes (base vacía)
#   scripts/restore_db.sh --yes               # no pide confirmación
#
# Todo corre dentro del contenedor `db`: no hace falta postgresql-client en el
# host. Y el pg_restore es el del contenedor, así que su versión es la del server
# local — por eso la versión del contenedor tiene que coincidir con la del
# proyecto de Supabase (ver el comentario de `db:` en docker-compose.yml).
#
# Lo que va a fallar, y está permitido a propósito:
#
#   El dump arrastra `CREATE EXTENSION supabase_vault`, que no existe en un
#   Postgres vanilla. Es el error esperado y prácticamente el único: medido sobre
#   el dump de este proyecto, NO hay policies RLS, NO hay
#   ENABLE ROW LEVEL SECURITY y NO hay referencias a auth.uid() — el backend
#   tiene su propio auth (JWT + tabla users propia) y nunca usó Supabase Auth,
#   así que no queda nada de Supabase en el schema de la app. Por eso el
#   restore no aborta por ese error (--exit-on-error apagado): pg_restore sigue.
#
#   Lo que NO se negocia es que al final se verifique que las tablas de la app
#   existen y tienen filas. Un restore a medias se reporta como falla, para que
#   no parezca un entorno sano.
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly BACKUP_DIR="$REPO_ROOT/backup"
readonly COMPOSE_SERVICE="db"

# Tablas que la app necesita encontrar pobladas. Es la lista mínima para que el
# backend arranque con sentido; no hace falta que sea la lista completa de
# modelos, con que falle si el restore no restauró nada alcanza.
readonly REQUIRED_TABLES=(
  users
  teams
  team_users
  roles
  permissions
)

in_file=""
do_reset="false"
assume_yes="false"
verbose_log=""

die() {
  echo "error: $*" >&2
  exit 1
}

usage() {
  sed -n '2,25p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

compose() {
  (cd "$REPO_ROOT" && docker compose "$@")
}

# db_exec corre un comando psql dentro del contenedor y devuelve stdout.
db_exec() {
  compose exec -T "$COMPOSE_SERVICE" \
    psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-paceron_local}" \
    -v ON_ERROR_STOP=1 "$@"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -f | --file)
      [[ $# -ge 2 ]] || die "$1 necesita un valor"
      in_file="$2"
      shift 2
      ;;
    --reset)
      do_reset="true"
      shift
      ;;
    -y | --yes)
      assume_yes="true"
      shift
      ;;
    --verbose-log)
      [[ $# -ge 2 ]] || die "$1 necesita un valor"
      verbose_log="$2"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *) die "argumento desconocido: $1 (probá --help)" ;;
  esac
done

command -v docker >/dev/null 2>&1 || die "no se encontré el binario 'docker'"
docker info >/dev/null 2>&1 || die "el daemon de Docker no está corriendo (¿abriste Docker Desktop?)"

# --- elegir dump -----------------------------------------------------------
if [[ -z "$in_file" ]]; then
  [[ -d "$BACKUP_DIR" ]] || die "no existe $BACKUP_DIR. Generá un dump primero: make local-dump"
  # El más reciente por fecha de modificación.
  in_file="$(ls -t "$BACKUP_DIR"/*.dump 2>/dev/null | head -n1 || true)"
  [[ -n "$in_file" ]] || die "no hay ningún .dump en $BACKUP_DIR. Generá uno: make local-dump"
else
  [[ -f "$in_file" ]] || die "no existe el dump: $in_file"
fi

in_file="$(cd "$(dirname "$in_file")" && pwd)/$(basename "$in_file")"
[[ -s "$in_file" ]] || die "el dump está vacío: $in_file"

readonly in_file_abs="$in_file"

# --- contenedores arriba ---------------------------------------------------
if $do_reset; then
  echo "→ --reset: bajando contenedores y borrando volúmenes"
  compose down -v >/dev/null
fi

echo "→ levantando el servicio db"
compose up -d "$COMPOSE_SERVICE" >/dev/null

echo "→ esperando a que Postgres acepte conexiones…"
ready="false"
for _ in $(seq 1 60); do
  if compose exec -T "$COMPOSE_SERVICE" pg_isready -q -U "${POSTGRES_USER:-postgres}"; then
    ready="true"
    break
  fi
  sleep 1
done
$ready || die "Postgres no respondió al healthcheck. Mirá los logs: make local-logs"

# --- chequeo de versión (evita el 'unsupported version (1.16)' de adentro) ---
readonly local_major="$(compose exec -T "$COMPOSE_SERVICE" psql -U "${POSTGRES_USER:-postgres}" -d postgres -tAc 'SHOW server_version_num' | tr -d '[:space:]' | cut -c1-2)"
readonly dump_major="$(docker run --rm -v "$in_file_abs:/d.dump:ro" "postgres:${local_major}-alpine" pg_restore -l /d.dump >/dev/null 2>&1 && echo "$local_major" || echo "")"

if [[ -z "$dump_major" ]]; then
  echo "ATENCIÓN: pg_restore de postgres:${local_major}-alpine no pudo leer el dump."
  echo "          Casi seguro el dump es de una versión mayor que el contenedor."
  echo "          Chequeá con: docker run --rm -v \"\$PWD/$(basename "$in_file_abs"):/d.dump:ro\" postgres:${local_major}-alpine pg_restore -l /d.dump"
  echo "          Si es eso, levantá con POSTGRES_IMAGE=postgres:<mayor>-alpine y make local-reset."
  die "dump incompatible con el contenedor db (que es postgres:${local_major})"
fi
echo "→ dump y contenedor compatibles (postgres:${local_major})"

# --- confirmación -----------------------------------------------------------
if [[ $assume_yes != true ]]; then
  cat <<EOF

  Se va a restaurar sobre el Postgres LOCAL, pisando lo que haya ahora:
    contenedor : $(compose ps -q "$COMPOSE_SERVICE" | cut -c1-12)… ($COMPOSE_SERVICE)
    base       : ${POSTGRES_DB:-paceron_local}
    dump       : $(basename "$in_file_abs") ($(du -h "$in_file_abs" | cut -f1 | tr -d ' '))

  Esto NO toca Supabase. Para continuar:
EOF
  read -r -p "  ¿Seguimos? [s/N] " answer
  [[ "$answer" =~ ^[sSyY]$ ]] || die "cancelado por el usuario"
fi

# --- schemas que el dump espera ---------------------------------------------
# El dump pide `CREATE EXTENSION ... WITH SCHEMA extensions` / `WITH SCHEMA
# vault`, y esos schemas van excluidos del dump. Sin recrearlos acá, las tres
# extensiones que sí existen en vanilla (pgcrypto, uuid-ossp, pg_stat_statements)
# también fallan, y la lista de errores queda ruidosa y menos entendible.
#
# pgcrypto y uuid-ossp hoy no las usa ninguna columna del schema de la app (no
# hay columnas uuid ni defaults con funciones de extensión), pero conviene
# dejarlas instaladas para que una migración futura no tenga que pensarlo.
echo "→ precreando schemas para las extensiones del dump"
db_exec -q -c 'CREATE SCHEMA IF NOT EXISTS extensions' >/dev/null
db_exec -q -c 'CREATE SCHEMA IF NOT EXISTS vault' >/dev/null

# --- pg_restore ------------------------------------------------------------
# `docker cp` y no un pipe por stdin: el formato custom de pg_dump necesita
# input seekable, y no vale la pena depender de que pg_restore lo tolere.
readonly container_dump="/tmp/$(basename "$in_file_abs")"
echo "→ copiando el dump al contenedor"
docker cp "$in_file_abs" "$(compose ps -q "$COMPOSE_SERVICE"):$container_dump"

readonly restore_log="${verbose_log:-$REPO_ROOT/backup/restore.log}"
mkdir -p "$(dirname "$restore_log")"

echo "→ restaurando (los errores de objetos de Supabase se ignoran a propósito)"
set +e
compose exec -T "$COMPOSE_SERVICE" \
  pg_restore \
  --username="${POSTGRES_USER:-postgres}" \
  --dbname="${POSTGRES_DB:-paceron_local}" \
  --no-owner \
  --no-privileges \
  --verbose \
  "$container_dump" >"$restore_log" 2>&1
restore_status=$?
set -e

# Con --verbose, pg_restore Prefija TODAS sus líneas con "pg_restore:" —las de
# progreso inclusive— así que contar con `grep -c '^pg_restore:'` da hundreds en
# vez de errores reales. Hay que buscar el texto del error.
ignored_errors="$(grep -c 'pg_restore: error:' "$restore_log" 2>/dev/null || true)"
ignored_errors="${ignored_errors:-0}"

echo
echo "→ restore terminado."
echo "  errores: $ignored_errors  (log completo: $restore_log)"
if (( ignored_errors > 0 )); then
  echo "  esperados (objetos de Supabase que no existen en vanilla Postgres):"
  grep 'pg_restore: error:' "$restore_log" \
    | sed -E 's/^pg_restore: error: could not execute query: ERROR:  //; s/^pg_restore: error: //' \
    | sort -u | sed 's/^/    /'
fi
echo "  (pg_restore exit=$restore_status; no es criterio de falla — ver verificación abajo)"

# --- cleanup: RLS off -------------------------------------------------------
# Red de seguridad, no un fix. En el dump de este proyecto NO viene nada de RLS
# (ni ENABLE ROW LEVEL SECURITY ni policies), así que hoy esto no cambia una
# fila. Se deja igual porque es idempotente y barato, y porque un restore
# tenga RLS, sin este paso el backend conecta, AutoMigrate corre bien, y todas
# las consultas devuelven cero filas — parece un bug de la app y se pierde el
# rato.
echo
echo "→ desactivando Row Level Security (red de seguridad, hoy no aplica)"
db_exec -q <<'SQL' >/dev/null
DO $$
DECLARE r record;
BEGIN
  FOR r IN
    SELECT schemaname, tablename FROM pg_tables
    WHERE schemaname IN ('public', 'auth', 'storage')
  LOOP
    EXECUTE format('ALTER TABLE %I.%I DISABLE ROW LEVEL SECURITY', r.schemaname, r.tablename);
  END LOOP;
END $$;
SQL
echo "  RLS desactivado"

# --- verificación -----------------------------------------------------------
echo
echo "→ verificando que las tablas de la app quedaron pobladas"
missing=0
printf '  %-16s %10s  %s\n' "tabla" "filas" "estado"
printf '  %-16s %10s  %s\n' "----------------" "----------" "--------"
for table in "${REQUIRED_TABLES[@]}"; do
  if ! exists="$(db_exec -tAc "SELECT to_regclass('public.${table}') IS NOT NULL")"; then
    exists="f"
  fi
  if [[ "$exists" != "t" ]]; then
    printf '  %-16s %10s  %s\n' "$table" "-" "NO EXISTE"
    missing=$((missing + 1))
    continue
  fi
  count="$(db_exec -tAc "SELECT count(*) FROM public.${table}" | tr -d '[:space:]')"
  if [[ "$count" == "0" ]]; then
    printf '  %-16s %10s  %s\n' "$table" "$count" "VACÍA"
    missing=$((missing + 1))
  else
    printf '  %-16s %10s  %s\n' "$table" "$count" "ok"
  fi
done

echo
echo "  totales por tabla (top 10):"
db_exec -tAc "
  SELECT relname || ' ' || n_live_tup
  FROM pg_stat_user_tables
  WHERE schemaname = 'public'
  ORDER BY n_live_tup DESC, relname
  LIMIT 10" | sed 's/^/    /'

if (( missing > 0 )); then
  echo
  die "$missing tablas de la app quedaron ausentes o vacías. El restore NO quedó usable — mirá $restore_log"
fi

cat <<EOF

Restore OK.

Siguiente paso:
  go run . --stage=local     # backend contra la base local
EOF