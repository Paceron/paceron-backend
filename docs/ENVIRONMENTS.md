# Stages de Supabase: testing vs. production

Dos proyectos de Supabase separados — DB y storage S3, mismo split. **Default siempre testing, producción exige un flag explícito** — a propósito, para que un deploy mal configurado nunca termine pegándole a datos reales por accidente.

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

### ¿Cómo se activan los dos services en Render?

`render.yaml` es un Blueprint — solo se aplica solo si el service está linkeado a un Blueprint en Render. Dos casos:

- **Si el service actual ya es Blueprint** (dashboard → pestaña Blueprints): sync manual, Render muestra el diff (branch nueva, service `paceron-backend-develop` nuevo, env vars nuevas declaradas) y se aplica con un click. Sigue pidiendo los valores de las vars con `sync: false` que no tengan valor todavía.
- **Si el service actual se creó a mano** (probable, dado que hoy despliega `develop` sin que `render.yaml` lo dijera nunca): `render.yaml` no se autoaplica. Hay que crear el Blueprint desde cero (`New +` → `Blueprint`, apuntar al repo) — Render va a detectar que ya existe un service con el mismo `name: paceron-backend` y ofrece adoptarlo en vez de duplicarlo; o si se prefiere no arriesgar eso, editar el service actual a mano y crear el segundo también a mano, usando `render.yaml` como referencia de qué valores va cada uno.

`render.yaml` ahora declara **todas** las keys con `sync: false` que necesita el binario para arrancar (`SUPABASE_*_DATABASE_URL`, `JWT_SECRET`, `RESEND_API_KEY`, `RESEND_FROM_ADDRESS`) — el valor nunca viaja por git (Render no puede leer un `.env` local, ni debería), pero declarar la clave hace que Render la pida al sincronizar en vez de depender de acordarse a mano.

### Checklist de valores a cargar (una vez por service, vía dashboard)

1. **`paceron-backend`** (master, producción): `SUPABASE_PRODUCTION_DATABASE_URL` con el valor de producción, más `JWT_SECRET`/`RESEND_API_KEY`/`RESEND_FROM_ADDRESS`.
2. **`paceron-backend-develop`** (develop, testing): `SUPABASE_TESTING_DATABASE_URL` con el valor de testing, más los mismos `JWT_SECRET`/`RESEND_*` (compartidos, no dependen del stage — un proveedor de mail no maneja datos que requieran aislamiento como la DB) o propios si más adelante se prefiere aislar también el envío de mail — no resuelto acá, mismo criterio que se use hoy.

## Storage (Supabase Buckets)

Mismo split testing/producción que la DB — **buckets en proyectos separados** (`testing_stage_bucket` / `production_stage_bucket`), no un solo bucket con dos prefijos.

| Variable | Stage |
|---|---|
| `SUPABASE_TESTING_S3_*` | testing (default) — bucket `testing_stage_bucket` |
| `SUPABASE_PRODUCTION_S3_*` | production (`--stage=production`) — bucket `production_stage_bucket` |

### Bucket público (avatares e íconos de equipo)

Ambos buckets están marcados **`public = true`** (toggle "Public bucket" del dashboard de Supabase, aplicado en los dos proyectos). Sirven foto de perfil de usuario (`avatars/user-{id}.{ext}`, `cmd/api/services/user_service.go`) e ícono de equipo (`teams/team-{id}-icon.{ext}`, `cmd/api/services/team_service.go`) vía URL pública directa, sin pasar por el backend.

**Por qué no RLS scopeada por prefijo** (evaluado y descartado): la URL pública de Supabase Storage (`/storage/v1/object/public/<bucket>/<path>`, la que usa un `<img src>` sin headers) **ignora RLS por completo** — depende únicamente del flag `public` del bucket, todo-o-nada. Confirmado por un colaborador de Supabase: ["You have to decide if you want the entire bucket public or not. It can't be done on a folder basis."](https://github.com/orgs/supabase/discussions/18415) No hay término medio en ningún sentido — ni RLS scopeada sobre bucket privado, ni bucket público con RLS "reprivatizando" una carpeta.

**Aceptado como suficiente para este proyecto** (tesis, no producto de mercado con datos de terceros en juego): las fotos no son información sensible. Documentos privados (certificados médicos, diplomas, certificaciones) quedan pendientes — cuando se necesiten, van a un **bucket nuevo y separado**, privado, con RLS real (ahí sí cumple su función, porque nunca se sirve por la URL pública). Nunca comparte bucket con las fotos.

El `PUT`/upload no depende de este flag — corre con la `service_role` key del backend, que bypasea cualquier RLS/ACL igual.

### Lectura: directo a Supabase, no proxy por backend

El frontend consume las URLs públicas (`photo_url`/`icon_url`) directo contra Supabase, no a través del backend — decisión ya tomada en el diseño de la feature (`openspec/changes/fotos-perfil-equipo/design.md`, D9). Solo el upload pasa por el backend (proxy, ver mismo doc D1). Razón: una lectura ocurre muchas más veces que un upload (cada render de un avatar en el front), proxyear eso metería ese volumen por Render sin necesidad y perdería el cacheo de navegador/CDN que da servir directo desde Supabase.

## Local / CI

- **Local**: `.env` con `SUPABASE_TESTING_DATABASE_URL` (nunca `SUPABASE_PRODUCTION_DATABASE_URL` salvo necesidad puntual explícita — y ahí sí, correr con `go run cmd/api/main.go --stage=production` a sabiendas).
- **CI** (`ci.yml`): no usa ninguna de las dos — los tests de `daos` pegan contra el Postgres efímero del service container (`TEST_DB_*`, ver `docs/TESTING.md`), completamente separado de los stages de Supabase.
