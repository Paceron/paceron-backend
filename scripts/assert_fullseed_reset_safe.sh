#!/usr/bin/env bash
# Evita que `fullseed-reset` borre datos locales que no pertenecen al perfil full.
#
# `fullseed-reset` hace `local-reset` (docker compose down -v): borra el volumen
# completo, no solo lo sembrado. Este guard aborta si la base tiene usuarios o
# equipos ajenos al seed; para forzar el reset destructivo:
#
#   FULLSEED_RESET_FORCE=1 make fullseed-reset
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
readonly SCRIPT_DIR REPO_ROOT
readonly ENV_FILE="${ENV_FILE:-.env.local}"
readonly COMPOSE_SERVICE="db"

compose() {
  (cd "$REPO_ROOT" && docker compose --env-file "$ENV_FILE" "$@")
}

db_query() {
  compose exec -T "$COMPOSE_SERVICE" sh -c \
    'psql -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-paceron_local}" -tAc "$1"' _ "$1"
}

die() {
  echo "error: $*" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || die "no se encontró el binario 'docker'"
docker info >/dev/null 2>&1 || die "el daemon de Docker no está corriendo"

if [[ -z "$(compose ps -q "$COMPOSE_SERVICE" 2>/dev/null || true)" ]]; then
  compose up -d "$COMPOSE_SERVICE" >/dev/null
fi

ready="false"
for _ in $(seq 1 60); do
  if compose exec -T "$COMPOSE_SERVICE" sh -c \
    'pg_isready -q -U "${POSTGRES_USER:-postgres}" -d "${POSTGRES_DB:-paceron_local}"' \
    >/dev/null 2>&1; then
    ready="true"
    break
  fi
  sleep 1
done
[[ "$ready" == "true" ]] || die "Postgres no respondió al healthcheck"

exists="$(db_query "SELECT to_regclass('public.users') IS NOT NULL" | tr -d '[:space:]')"
if [[ "$exists" != "t" ]]; then
  exit 0
fi

users="$(db_query "SELECT count(*) FROM users WHERE email NOT LIKE '%@seed-full.local'" | tr -d '[:space:]')"
teams="$(db_query "SELECT count(*) FROM teams WHERE COALESCE(description, '') NOT LIKE '%[seed_full]%'" | tr -d '[:space:]')"

if [[ "$users" == "0" && "$teams" == "0" ]]; then
  exit 0
fi

if [[ "${FULLSEED_RESET_FORCE:-}" == "1" ]]; then
  echo "aviso: fullseed-reset forzado; se van a borrar $users usuarios y $teams equipos ajenos al perfil full." >&2
  exit 0
fi

cat >&2 <<EOF
error: fullseed-reset borra TODO el volumen local y la base tiene datos ajenos al perfil full:
  usuarios ajenos: $users
  equipos ajenos:  $teams

Para conservarlos, usá el seed aditivo:
  make fullseed-seed

Para resetear igual (destructivo):
  FULLSEED_RESET_FORCE=1 make fullseed-reset

Para guardar la base antes del reset:
  make local-dump        # clona Supabase
  make demo-baseline     # o congelá la base local como baseline
EOF
exit 1
