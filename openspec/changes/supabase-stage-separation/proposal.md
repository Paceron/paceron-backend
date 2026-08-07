## Why

Hoy hay dos proyectos de Supabase (testing y production) creados y probados, pero solo se usa uno — el `.env` local y (según confirmó el usuario) el único service de Render apuntan al mismo `DATABASE_URL`, que resultó ser el proyecto de **testing**, aunque Render lo mostraba/trataba como si fuera producción. Además Render desplegaba `develop`, no `master`. Sin separación real, cualquier trabajo en curso puede terminar escribiendo en la base "de producción" sin que nadie lo haya decidido explícitamente.

## What Changes

- `config.go`: nuevo `IsProductionStage()` — escanea `os.Args` buscando exactamente `--stage=production`. Sin ese flag, siempre testing. `loadDBConfig()` elige `SUPABASE_PRODUCTION_DATABASE_URL` o `SUPABASE_TESTING_DATABASE_URL` según el flag (reemplaza el `DATABASE_URL` genérico anterior).
- `app/router.go`: loguea el stage resuelto al arrancar (`"supabase stage resolved"`), visible en los logs de Render/local sin ambigüedad.
- `render.yaml`: pasa de 1 a 2 services — `paceron-backend` (branch `master`, `--stage=production`, `SUPABASE_PRODUCTION_DATABASE_URL`) y `paceron-backend-develop` (branch `develop`, sin flag, `SUPABASE_TESTING_DATABASE_URL`). Corrige de paso el quirk ya documentado (`branch: main` no existía).
- `docs/ENVIRONMENTS.md` (nuevo): mecanismo completo + checklist manual para el dashboard de Render (las secrets no viven en `render.yaml`, hay que setearlas a mano).
- `CLAUDE.md`: sección nueva, quirk de `render.yaml`/`main` resuelto (ya no aplica).
- Storage S3 **explícitamente fuera de alcance** — el usuario lo priorizó para después, no hay cliente de storage en este cambio, solo la separación de DB.

## Design decision explícita: default siempre testing

El usuario pidió que el modo por default (sin flag, cualquier `go run`/deploy mal configurado) caiga siempre en testing stage — producción exige el flag exacto `--stage=production`. Preferencia explícita del usuario: arg/flag por sobre env var (aunque un env var también hubiera funcionado). Fail-safe: un Render mal configurado, un typo, o correr el binario sin pensarlo dos veces nunca termina pegándole a datos reales.

## Capabilities

### New Capabilities
- `supabase-stage-separation`: selección explícita de proyecto Supabase (testing/production) vía flag, con testing como default seguro.

## Impact

- **Modificado**: `config.go` (+test), `app/router.go`, `render.yaml`, `CLAUDE.md`
- **Nuevo**: `docs/ENVIRONMENTS.md`
- **Rotos intencionalmente**: `DATABASE_URL` genérico ya no se lee — quien lo tuviera seteado (local o Render) necesita migrar a `SUPABASE_TESTING_DATABASE_URL`/`SUPABASE_PRODUCTION_DATABASE_URL`. `.env` local ya migrado en esta sesión.
- **Manual, no automatizable**: reconfigurar el dashboard de Render (branch, nombre de env var, valores) — checklist en `docs/ENVIRONMENTS.md`, lo hace el usuario.
- **Sin cambios**: storage/S3 (deferred), `ENVIRONMENT`/`GetEnvironment()` (eje independiente, sin tocar)
