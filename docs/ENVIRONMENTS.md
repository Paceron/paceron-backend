# Stages de Supabase: testing vs. production

Dos proyectos de Supabase separados (DB por ahora — storage S3 queda para cuando arranque esa iniciativa, ver `openspec/changes/postgres-ci-dao-tests/design.md` y memoria de sesión). **Default siempre testing, producción exige un flag explícito** — a propósito, para que un deploy mal configurado nunca termine pegándole a datos reales por accidente.

## Cómo se elige el stage

```bash
./main                    # testing stage (default)
./main --stage=production # production stage — el único disparador válido
```

`config.IsProductionStage()` escanea `os.Args` buscando exactamente `--stage=production`. Cualquier otra cosa (nada, un typo, otro flag) cae en testing. Se loguea al arrancar (`"supabase stage resolved"`), revisar los logs de Render/local para confirmar a qué stage pegó un arranque dado.

**Importante — esto NO es lo mismo que `ENVIRONMENT`:** `ENVIRONMENT` (env var, `config.GetEnvironment()`) gobierna cómo se carga la config (local/test/production — hoy los tres hacen lo mismo, es un hook para diferenciarlos a futuro si hace falta). `--stage` gobierna a **cuál proyecto de Supabase** apunta. Son ejes independientes: los dos services de Render corren con `ENVIRONMENT=production` (son deploys reales, no local/test), pero se diferencian por el flag `--stage`.

## Variables de entorno

| Variable | Stage | Reemplaza a |
|---|---|---|
| `SUPABASE_TESTING_DATABASE_URL` | testing (default) | el viejo `DATABASE_URL` genérico |
| `SUPABASE_PRODUCTION_DATABASE_URL` | production (`--stage=production`) | — |

`DATABASE_URL` genérico ya no se lee — `loadDBConfig()` elige entre las dos de arriba según `IsProductionStage()`. Si ninguna está seteada, cae al fallback histórico de `db_host`/`db_port`/`db_user`/`db_password`/`db_name` (sin distinción de stage, legacy).

## Deploys en Render

Dos services (`render.yaml`), mismo repo:

| Service | Branch | Flag | DB |
|---|---|---|---|
| `paceron-backend` | `master` | `--stage=production` | `SUPABASE_PRODUCTION_DATABASE_URL` |
| `paceron-backend-develop` | `develop` | *(ninguno)* | `SUPABASE_TESTING_DATABASE_URL` |

**Antes de este cambio**, el único service existente en Render desplegaba `develop` pero estaba etiquetado/tratado como "producción" — mismo quirk que ya estaba documentado en `CLAUDE.md`. Esto lo corrige: `master` pasa a ser la producción real, `develop` queda como preview separado.

### Checklist manual en el dashboard de Render (esto no lo puede hacer `render.yaml` solo — las secrets no viven en git)

1. **Service existente** (el que hoy despliega `develop` y se muestra como prod):
   - Cambiar su branch a `master`.
   - Agregar `--stage=production` al Start Command (o confirmar que tome el de `render.yaml` en el próximo sync).
   - Renombrar su variable `DATABASE_URL` a `SUPABASE_PRODUCTION_DATABASE_URL`, con el valor de producción (el `.env.backend` que ya tenés armado).
   - Confirmar que están seteadas: `JWT_SECRET`, `SMTP_HOST`, `GMAIL_USER`, `GMAIL_APP_PASSWORD` (no se tocan, pero verificar que sigan ahí después del cambio).
2. **Service nuevo** (`paceron-backend-develop`, desde `render.yaml` o creado a mano):
   - Branch `develop`, sin flag de stage.
   - `SUPABASE_TESTING_DATABASE_URL` con el valor de testing.
   - Mismas `JWT_SECRET`/`SMTP_*`/`GMAIL_*` que el de producción (o propias, si se quiere aislar el envío de mail también — no resuelto acá, mismo criterio que se use hoy).

## Local / CI

- **Local**: `.env` con `SUPABASE_TESTING_DATABASE_URL` (nunca `SUPABASE_PRODUCTION_DATABASE_URL` salvo necesidad puntual explícita — y ahí sí, correr con `go run cmd/api/main.go --stage=production` a sabiendas).
- **CI** (`ci.yml`): no usa ninguna de las dos — los tests de `daos` pegan contra el Postgres efímero del service container (`TEST_DB_*`, ver `docs/TESTING.md`), completamente separado de los stages de Supabase.
