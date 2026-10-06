#!/usr/bin/env bash
# Congela la base local como "baseline" y la vuelve a dejar en ese estado, para
# demos y pruebas manuales donde los datos se ensucian.
#
#   scripts/demo_db.sh baseline      # congela el estado actual (una vez, antes de la demo)
#   scripts/demo_db.sh restore       # vuelve al baseline — rápido, entre rondas
#   scripts/demo_db.sh reset         # vuelve al baseline — en frío, borrando volúmenes
#   scripts/demo_db.sh verify        # ¿la base está igual que el baseline?
#   scripts/demo_db.sh status        # en qué estado está todo
#
# Flags (válidos para todos): --force (solo baseline), --yes (sin confirmar),
# -h/--help.
#
# Por qué dos comandos de restore y no uno:
#
#   `restore` usa `pg_restore --clean --if-exists` sobre el contenedor que ya está
#   arriba: borra y recrea cada objeto del dump antes de crearlo, así que la
#   base vuelve al baseline en segundos sin bajar el contenedor. Es el que se usa
#   entre rondas de una demo, cuando importa que el reset sea rápido.
#
#   Su límite, y por qué existe igual `reset`: --clean sólo dropea los objetos que
#   están EN el dump. Si una ronda dejó una tabla nueva (por ejemplo creada por un
#   AutoMigrate posterior al baseline), esa tabla sobrevive. `reset` no tiene ese
#   límite porque baja los contenedores con `down -v` (base y bucket vacíos) y
#   levanta el compose completo de cero. Es más lento y es el que hay que correr
#   antes de empezar y al terminar.
#
# El baseline se congela con `pg_dump` de la base LOCAL (no de Supabase): así no
# depende de tener credenciales ni red, y congela exactamente la foto que se está
# por mostrar en la demo. `backup/` está en .gitignore porque el dump trae datos
# reales (emails, nombres, DNI).
#
# `--single-transaction` en el restore: si algo falla a mitad, no queda una base
# a medias, queda la que había antes del intento. Por eso el script no aborta
# por errores sueltos de pg_restore y en cambio confía en `verify`, que compara
# los conteos de todas las tablas contra la huella del baseline y sale con
# código 1 si difieren.
#
# Lo que verify NO garantiza: compara conteos de filas, no el contenido. Una ronda que
# solo edite filas (un UPDATE, sin alta ni baja) pasa la verificación aunque la
# base esté distinta. El restore sí revierte esas ediciones — el dump va con los
# valores — así que es una limitación de la verificación, no del restore. Para
# ese caso, `demo-status` más una mirada a la fila.
set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly BACKUP_DIR="$REPO_ROOT/backup"
readonly COMPOSE_SERVICE="db"

readonly BASELINE_DUMP="$BACKUP_DIR/baseline.dump"
readonly BASELINE_COUNTS_FILE="$BACKUP_DIR/baseline.counts"
readonly BASELINE_SHA_FILE="$BACKUP_DIR/baseline.sha256"

# Mismo criterio que el Makefile: si existe, se lo pasa a docker compose.
readonly ENV_FILE="${ENV_FILE:-.env.local}"

force="false"
assume_yes="false"

# El path del archivo temporal de conteos vive en scope de script, no en el de
# cmd_verify: si fuera local, el trap de EXIT lo buscaría cuando la variable ya
# salió de scope y, con `set -u`, abortaría con "unbound variable" — justo en el
# comando que se acaba de ejecutar bien.
counts_tmp=""
cleanup() {
  if [[ -n "$counts_tmp" ]]; then
    rm -f "$counts_tmp"
  fi
  return 0
}
trap cleanup EXIT

die() {
  echo "error: $*" >&2
  exit 1
}

usage() {
  # Todo el bloque de comentario inicial menos la línea del shebang, sin depender
  # de un rango de líneas fijo (que se desalinea cada vez que se edita el header).
  awk 'NR > 1 && /^set -e/ { exit } NR > 1 { sub(/^# ?/, ""); print }' "${BASH_SOURCE[0]}"
}

compose() {
  local -a env_args=()
  if [[ -f "$REPO_ROOT/$ENV_FILE" ]]; then
    env_args+=(--env-file "$REPO_ROOT/$ENV_FILE")
  fi
  (cd "$REPO_ROOT" && docker compose "${env_args[@]}" "$@")
}

pg_user() {
  printf '%s' "${POSTGRES_USER:-postgres}"
}

pg_db() {
  printf '%s' "${POSTGRES_DB:-paceron_local}"
}

# db_exec corre un comando psql dentro del contenedor y devuelve stdout.
db_exec() {
  compose exec -T "$COMPOSE_SERVICE" \
    psql -U "$(pg_user)" -d "$(pg_db)" -v ON_ERROR_STOP=1 "$@"
}

require_docker() {
  command -v docker >/dev/null 2>&1 || die "no encontré el binario 'docker'"
  docker info >/dev/null 2>&1 || die "el daemon de Docker no está corriendo (¿abriste Docker Desktop?)"
}

wait_for_db() {
  local ready="false"
  for _ in $(seq 1 60); do
    if db_exec -tAc 'SELECT 1' >/dev/null 2>&1; then
      ready="true"
      break
    fi
    sleep 1
  done
  $ready || die "Postgres no respondió al healthcheck. Mirá los logs: make local-logs"
}

ensure_db_up() {
  require_docker
  if ! db_exec -tAc 'SELECT 1' >/dev/null 2>&1; then
    echo "→ el servicio db no responde, lo levanto"
    compose up -d "$COMPOSE_SERVICE" >/dev/null
    wait_for_db
  fi
}

sha256_of() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    sha256sum "$1" | awk '{print $1}'
  fi
}

human_size() {
  du -h "$1" | cut -f1 | tr -d ' '
}

# `date -r` es de BSD (macOS); en Linux `-r` significa otra cosa y falla. Por eso
# el mtime va por stat con las dos variantes.
file_mtime() {
  if stat -f '%Sm' -t '%Y-%m-%d %H:%M' "$1" >/dev/null 2>&1; then
    stat -f '%Sm' -t '%Y-%m-%d %H:%M' "$1"
  else
    stat -c '%y' "$1" | cut -c1-16
  fi
}

# ---------------------------------------------------------------------------
# La huella de conteos: qué es lo que define "la misma foto".
#
# Un solo round-trip. Se genera un UNION ALL con un count(*) por tabla y se
# ejecuta; psql lo devuelve como `tabla|conteo`. Comparar eso contra la huella
# del baseline es lo que hace que `verify` sirva: la verificación de
# restore_db.sh ("la tabla existe y tiene filas") no detecta que haya 40
# usuarios en vez de 34, esto sí.
# ---------------------------------------------------------------------------
write_counts() {
  local dest="$1"
  local query
  query="$(db_exec -tAc "
    SELECT COALESCE(string_agg(
      format('SELECT %L AS table_name, count(*)::bigint AS row_count FROM public.%I', c.relname, c.relname),
      ' UNION ALL ' ORDER BY c.relname), '')
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relkind = 'r'")"
  [[ -n "$query" ]] || die "no encontré ninguna tabla en el schema public"
  db_exec -tAc "$query" | sed '/^[[:space:]]*$/d' >"$dest"
}

require_baseline() {
  [[ -s "$BASELINE_DUMP" ]] || die "no hay baseline todavía. Congelá el estado actual con: make demo-baseline"
  [[ -s "$BASELINE_COUNTS_FILE" ]] ||
    die "falta la huella de conteos ($BASELINE_COUNTS_FILE). Regenerá el baseline con: make demo-baseline-force"
}

# El baseline tiene que seguir siendo el mismo entre medio: si alguien lo
# pisa a mano o el archivo se corrompe, restaurar "el baseline" restauraría otra
# cosa en silencio.
verify_baseline_integrity() {
  [[ -s "$BASELINE_SHA_FILE" ]] || return 0
  local expected actual
  expected="$(awk '{print $1}' "$BASELINE_SHA_FILE")"
  actual="$(sha256_of "$BASELINE_DUMP")"
  [[ "$expected" == "$actual" ]] ||
    die "el baseline no coincide con su checksum (se modificó o se corrompió).
     esperaba: $expected
     encontré:  $actual
     Si el cambio fue a propósito: scripts/demo_db.sh baseline --force"
}

confirm_or_exit() {
  local message="$1"
  $assume_yes && return 0
  echo
  echo "$message"
  read -r -p "  ¿Seguimos? [s/N] " answer
  [[ "$answer" =~ ^[sSyY]$ ]] || die "cancelado por el usuario"
}

# ---------------------------------------------------------------------------
# restore: el dump entra al contenedor, pg_restore lo aplica.
# ---------------------------------------------------------------------------
restore_into_db() {
  local label="$1"
  shift
  local cid in_container
  cid="$(compose ps -q "$COMPOSE_SERVICE")"
  [[ -n "$cid" ]] || die "el servicio db no está levantado: make local-up"
  in_container="/tmp/baseline.dump"

  echo "→ copio el baseline al contenedor"
  docker cp "$BASELINE_DUMP" "$cid:$in_container"

  echo "→ pg_restore ($label)"
  compose exec -T "$COMPOSE_SERVICE" \
    pg_restore -U "$(pg_user)" -d "$(pg_db)" \
    --no-owner --no-privileges --single-transaction \
    "$@" "$in_container" || true

  compose exec -T "$COMPOSE_SERVICE" rm -f "$in_container" >/dev/null
}

cmd_verify() {
  require_baseline
  ensure_db_up
  counts_tmp="$(mktemp)"
  write_counts "$counts_tmp"
  if diff -u "$BASELINE_COUNTS_FILE" "$counts_tmp"; then
    echo "✓ la base está igual que el baseline"
  else
    echo "✗ la base difiere del baseline (diff arriba)" >&2
    rm -f "$counts_tmp"
    counts_tmp=""
    # `return`, no `exit`: cmd_status la llama dentro de un `if`, y un exit
    # cortaría el status mostrando "✓ igual que el baseline" que nunca se
    # llegaría a imprimir. Como return, el set -e de arriba igual hace que el
    # comando `verify` de verdad salga con código 1.
    return 1
  fi
  rm -f "$counts_tmp"
  counts_tmp=""
}

# ------------------------------------------------------------------ baseline
cmd_baseline() {
  ensure_db_up
  mkdir -p "$BACKUP_DIR"
  if [[ -s "$BASELINE_DUMP" && "$force" != true ]]; then
    die "ya existe un baseline ($(human_size "$BASELINE_DUMP"), del $(file_mtime "$BASELINE_DUMP")).
     Reemplazarlo pisa la foto contra la que se restauraría después.
     Si es lo que querés: scripts/demo_db.sh baseline --force"
  fi

  echo "→ las tablas que la app necesita encontrar pobladas"
  local table rows
  for table in users teams team_users roles permissions; do
    rows="$(db_exec -tAc "SELECT count(*) FROM public.$table" 2>/dev/null || true)"
    [[ -n "$rows" ]] ||
      die "la tabla public.$table no existe o no se puede leer. Si la base está rota: make local-reset && make local-restore"
    echo "   $table = $rows"
  done

  echo "→ pg_dump de public → backup/baseline.dump"
  local cid in_container
  cid="$(compose ps -q "$COMPOSE_SERVICE")"
  in_container="/tmp/baseline-new.dump"
  compose exec -T "$COMPOSE_SERVICE" \
    pg_dump -U "$(pg_user)" -d "$(pg_db)" \
    -n public -F c --no-owner --no-privileges \
    -f "$in_container" >/dev/null
  compose exec -T "$COMPOSE_SERVICE" pg_restore -l "$in_container" >/dev/null ||
    die "el dump generado no se puede leer. No lo guardé."
  docker cp "$cid:$in_container" "$BASELINE_DUMP"
  compose exec -T "$COMPOSE_SERVICE" rm -f "$in_container" >/dev/null
  [[ -s "$BASELINE_DUMP" ]] || die "pg_dump terminó sin error pero no escribió nada en $BASELINE_DUMP"

  echo "→ huella de conteos → backup/baseline.counts"
  write_counts "$BASELINE_COUNTS_FILE"
  sha256_of "$BASELINE_DUMP" >"$BASELINE_SHA_FILE"

  echo
  echo "baseline congelado:"
  echo "  dump     $(human_size "$BASELINE_DUMP")  (backup/baseline.dump)"
  echo "  tablas   $(wc -l <"$BASELINE_COUNTS_FILE" | tr -d ' ')  (backup/baseline.counts)"
  echo "  sha256   $(cut -c1-16 "$BASELINE_SHA_FILE")…"
  echo
  echo "Para volver a este estado:"
  echo "  make demo-restore    # entre rondas (segundos)"
  echo "  make demo-reset      # en frío, borra volúmenes (incluido el bucket)"
}

# ------------------------------------------------------------------- restore
cmd_restore() {
  require_baseline
  verify_baseline_integrity
  ensure_db_up
  confirm_or_exit "Se va a dejar la base local IGUAL que el baseline, pisando lo que haya ahora.
  dump: $(basename "$BASELINE_DUMP") ($(human_size "$BASELINE_DUMP"))

  Esto NO toca Supabase. Para continuar:"
  restore_into_db "restore en caliente" --clean --if-exists
  cmd_verify
}

# --------------------------------------------------------------------- reset
cmd_reset() {
  require_baseline
  verify_baseline_integrity
  require_docker
  confirm_or_exit "Se va a BORRAR el volumen de la base y el del bucket, y volver a levantarlos desde el baseline.
  dump: $(basename "$BASELINE_DUMP") ($(human_size "$BASELINE_DUMP"))

  Esto NO toca Supabase. Para continuar:"

  echo "→ down -v (borra paceron-db-data y paceron-s3-data)"
  compose down -v >/dev/null

  echo "→ compose completo (db + storage + bucket)"
  # OJO: levantar solo `db` deja el bucket sin existir y las fotos de perfil dan
  # 404 — por eso acá va el compose entero y no `compose up -d db`.
  compose up -d >/dev/null
  wait_for_db

  restore_into_db "restore en frío" --clean --if-exists
  cmd_verify

  cat <<EOF

  Para volver a levantar el backend contra esta base:
    go run ./cmd/api --stage=local
EOF
}

# -------------------------------------------------------------------- status
cmd_status() {
  echo "baseline"
  if [[ -s "$BASELINE_DUMP" ]]; then
    echo "  dump     $(human_size "$BASELINE_DUMP")  (tomado el $(file_mtime "$BASELINE_DUMP"))"
    if [[ -s "$BASELINE_SHA_FILE" ]]; then
      echo "  sha256   $(cut -c1-16 "$BASELINE_SHA_FILE")…"
    fi
    if [[ -s "$BASELINE_COUNTS_FILE" ]]; then
      echo "  filas    $(awk -F'|' '{s+=$2} END {print s+0}' "$BASELINE_COUNTS_FILE") en $(wc -l <"$BASELINE_COUNTS_FILE" | tr -d ' ') tablas"
    fi
    verify_baseline_integrity || echo "  ⚠ el checksum no coincide (se modificó o se corrompió)"
  else
    echo "  no existe todavía — make demo-baseline"
  fi

  echo
  echo "contenedores"
  compose ps -a 2>/dev/null || echo "  (docker compose no pudo leer el estado)"

  echo
  echo "estado actual de la base"
  if db_exec -tAc 'SELECT 1' >/dev/null 2>&1; then
    if [[ -s "$BASELINE_DUMP" && -s "$BASELINE_COUNTS_FILE" ]]; then
      if cmd_verify >/dev/null 2>&1; then
        echo "  ✓ igual que el baseline"
      else
        echo "  ✗ difiere del baseline — make demo-verify para ver el detalle"
      fi
    else
      echo "  (sin baseline, no hay contra qué comparar)"
    fi
  else
    echo "  el servicio db no está levantado — make local-up"
  fi
}

# ---------------------------------------------------------------------- main
[[ $# -ge 1 ]] || {
  usage
  exit 1
}
command="$1"
shift

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force)
      force="true"
      shift
      ;;
    -y | --yes)
      assume_yes="true"
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *) die "argumento desconocido: $1 (probá --help)" ;;
  esac
done

case "$command" in
  baseline) cmd_baseline ;;
  restore) cmd_restore ;;
  reset) cmd_reset ;;
  verify) cmd_verify ;;
  status) cmd_status ;;
  -h | --help)
    usage
    ;;
  *) die "comando desconocido: $command (probá --help)" ;;
esac