# Paceron Backend — Guía de trabajo

Convenciones de workflow, git y decisiones para trabajar en este repo — humanos y agentes de IA por igual. **Si tomás una decisión relevante para el equipo (workflow, arquitectura, configuración de proyecto), reflejala acá** para que aplique a todos, no solo a la sesión donde se decidió.

Esto incluye, sin limitarse a: cambios de contexto relevantes en backend o frontend (ej. el frontend depende de un modelo de roles que hoy no existe acá), incorporar una tecnología/librería nueva de peso, o configuración a nivel proyecto que afecte a todo el equipo. Si en el momento no amerita su propia sección, al menos dejar una línea en "Quirks conocidos" — mejor una nota corta que nada.

## Stack

Go 1.26 + Gin (HTTP) + GORM (ORM sobre PostgreSQL/Supabase) + JWT (`golang-jwt/jwt`) + Swagger (`swaggo/swag`). Arquitectura en capas: Controllers → Delegates → Services → DAOs/RestClients → Infrastructure (ver diagrama completo en [`README.md`](README.md)). Frontend separado (Expo/React Native, otro repo), ver sección Frontend.

Documentación técnica detallada ya existe en [`.agentics/`](.agentics/) (en inglés) y en `cmd/api/docs/documentationdetail/` (en español) — este archivo **no la duplica**, es la capa de convenciones de trabajo/git sobre esa base:

- [`.agentics/CONVENTIONS.md`](.agentics/CONVENTIONS.md) — convenciones de código, capas, qué no está permitido (ej. service-to-service imports, DAO directo desde controller).
- [`.agentics/STRUCTURE.md`](.agentics/STRUCTURE.md) — estructura de carpetas.
- [`.agentics/WORKFLOW.md`](.agentics/WORKFLOW.md) — cómo correr, testear, buildear, regenerar swagger, agregar una feature paso a paso.
- [`.agentics/GLOSSARY.md`](.agentics/GLOSSARY.md) — glosario de términos del dominio.
- [`openspec/`](openspec/) — spec-driven development ya configurado en el repo (usar este esquema, no inventar uno nuevo tipo `docs/superpowers/`).

## Workflow de branches y PRs

- **Rama base:** `develop`. Producción es `master`, se llega ahí vía release (`release/<version>` → PR manual a `master`).
- **Nomenclatura de rama:** `feature/<kebab-case>`, `fix/<kebab-case>`, `chore/<kebab-case>` — siempre creada desde `develop` actualizado. Un cambio = una rama = un tema. No mezclar en la misma rama cosas sin relación (ej. no meter un fix de CORS en la misma rama que un setup de CI).
- **Ciclo de PR:** al hacer push de una rama `feature/*`, `fix/*` o `chore/*`, un workflow de GitHub Actions (`auto-pr.yml`) crea automáticamente una PR en draft hacia `develop` con título/descripción placeholder. Se actualiza título y descripción (ver formato abajo), se marca como lista (`gh pr ready`), se espera CI verde (`ci.yml`), y se mergea. Después: `git checkout develop && git pull && git branch -d <rama> && git remote prune origin`.
- **Quién corre los comandos git:** a elección de cada dev. El agente de IA puede ejecutarlos directamente, o armar el bloque de comandos para que el desarrollador lo corra él mismo. Ninguna de las dos es "la forma correcta" — se acuerda con quien esté trabajando.
- Hoy en la práctica solo se usa `feature/*` → `develop`. El modelo completo (`release/*`, `hotfix/*`, `backport/*`) está soportado por los workflows pero todavía no se usó — si empieza a usarse, documentar acá lo que se aprenda del flujo real.

### Mensajes de commit

Formato [Conventional Commits](https://www.conventionalcommits.org/): `tipo(alcance): resumen corto en imperativo`. Tipos usados: `feat`, `fix`, `docs`, `refactor`, `chore`.

Preferir simple: **el subject alcanza en la mayoría de los casos.** Agregar cuerpo (1-2 oraciones) solo cuando el "por qué" no sea obvio desde el diff — no narrar el "qué" (el diff ya lo muestra).

```
# Bien — subject solo, cambio mecánico y autoexplicativo
feat(cors): sumar dominios de Vercel a origenes permitidos

# Bien — cuerpo corto, explica un "por qué" no obvio
fix(auth): invalidar refresh token en logout, no solo el access token

El refresh token sobrevivía al logout y permitía reautenticar sin login.

# Evitar — cuerpo largo narrando el "qué", eso ya está en el diff
feat(user): nuevo endpoint de actualización de estado

Agrega PATCH /users/:id/status que permite cambiar el estado del
usuario entre active/inactive/pause/blocked/suspended...
[3 párrafos más]
```

### Formato de PR

**Título:** mismo estilo que el commit principal — `tipo(alcance): resumen corto`.

**Descripción — corta y concreta:**

```markdown
## Qué cambió
- Bullet corto por cambio, no párrafos.
- Otro bullet.

Spec: `openspec/changes/<nombre>/` ← SOLO si esta rama usó una spec de OpenSpec

## Cómo probarlo
Pasos mínimos para confirmar que funciona.

`go test ./...` → todo verde.
```

## Cuándo usar OpenSpec

El repo ya tiene `openspec/` configurado para spec-driven development (ver [`SETUP.md`](SETUP.md) sección OpenSpec). Regla práctica:

| Tamaño del cambio | Spec de OpenSpec |
|---|---|
| Retoque/corrección chica (1-3 archivos, sin decisión de arquitectura) | No — alcanza con charlarlo y aprobar en el chat |
| Feature nueva, endpoint nuevo, cambio de modelo/schema | Sí |
| Cambio grande o que afecta varias capas con decisiones no triviales | Sí |

Sea cual sea el tamaño: **siempre crear la rama dedicada antes de tocar código**, incluso si se saltea la spec — no quedan commits sueltos en `develop`.

## Testing

`go test ./...` corre sobre archivos `*_test.go` co-ubicados con el código que testean (convención `testify`). La mayoría de las capas (`services`/`controllers`/`delegates`) siguen usando mocks. `daos` es la excepción: desde que hay Postgres real disponible en CI (ver debajo), sus tests corren contra una base real vía `testutils.SetupTestDB(t)` — se skipean solos si no hay `TEST_DB_HOST` seteada, así que `go test ./...` sigue funcionando sin Docker en cualquier máquina. Detalle completo, cómo correrlos localmente contra Postgres, y por qué se descartó sqlmock/SQLite: [`docs/TESTING.md`](docs/TESTING.md). Antes de mergear, la suite completa debe estar en verde (`ci.yml` la corre automáticamente en cada push/PR, con el service de Postgres levantado).

### Coverage

`make coverage` corre la suite con `-coverprofile` y muestra el resumen por paquete (`go tool cover -func`) — sin DB, `daos` skipea y el número es parcial. `make coverage-with-db` (requiere `make test-db-up` antes) corre lo mismo con Postgres real, mismo comando que usa CI. `make coverage-html`/`coverage-html` con DB generan `ci/test_coverage/coverage.html` navegable.

El comando usa `-coverpkg=./...` (no solo `-coverprofile` sobre los paquetes testeados) para que el % refleje código ejercitado indirectamente por otros tests (ej. `customlogger`/`httpclient` se llaman desde controllers/services aunque no tengan su propio `_test.go`) — sin esto, el total queda inflado porque ignora por completo cualquier paquete sin tests propios, ocultando deuda real.

`ci.yml` mide coverage en cada push/PR y además escribe un resumen (`go tool cover -func`) al Job Summary del run — visible directo en la pestaña de checks de GitHub, sin abrir logs. El gate lo hace la action `vladopajic/go-test-coverage`, configurada en `.testcoverage.yml` — **`threshold.total: 80`, bloquea el merge si el coverage cae por debajo.** Alcanzado agregando Postgres real en CI para testear `daos` (antes ~4%, ahora >85% ese paquete; total del proyecto ~85%). No bajar el número real del proyecto para pasar el gate — si baja, agregar tests, no ajustar el umbral hacia abajo.

**Qué cuenta y qué no:** `.testcoverage.yml` tiene una sección `exclude.paths` para paquetes sin lógica real (structs/DTOs puros como `domains/{dbs,user,auth,apierror,exampleweather}`, constantes, swagger autogenerado en `cmd/api/docs`, helpers de test en `testutils`, el `main.go` de arranque) — se sacan del cálculo porque no tiene sentido exigirles tests. **Un paquete con lógica real y 0% de cobertura nunca entra ahí** — paquetes como `infrastructure/httpclient` (donde no se ejercita indirectamente), `restclients/exampleweatherclient`, `delegates` o `metrics` cuentan en el total tal cual están, a propósito, para que la deuda sea visible y no quede maquillada.

## CORS

`CORSMiddleware()` en [`cmd/api/app/middleware.go`](cmd/api/app/middleware.go) lee `CORS_ALLOWED_ORIGINS` (env var, orígenes separados por coma). Si no está seteada, cae a una lista default hardcodeada en el código (hoy incluye localhost de desarrollo + los dominios de Vercel del frontend). En producción (Render, ver [`render.yaml`](render.yaml)) se configura explícitamente vía esa env var — al agregar un nuevo dominio de frontend, actualizar **ambos** lugares (el fallback en código y `render.yaml`) para que quede documentado en el repo, no solo en el dashboard de Render.

## Frontend

- Repo separado (Expo/React Native + React Native Web), no vive en este working directory, lo mantiene otro miembro del equipo.
- Se comunica vía REST, ver tabla de endpoints en [`README.md`](README.md).
- Deploy: producción en Vercel (`https://paceron-frontend.vercel.app`), preview de `develop` en `https://paceron-frontend-git-develop-paceron.vercel.app` — ambos dominios deben estar en `CORS_ALLOWED_ORIGINS`.

## Quirks conocidos

- `render.yaml` tiene `branch: main`, pero el repo no tiene rama `main` (usa `master`/`develop`) — revisar si esto es intencional o un desalineamiento antes de tocar el deploy config.
- El deploy en Render tiene cold-start de ~20-25s en la primera request tras inactividad (plan free) — no es un error real si el backend "no responde" al toque.
- El toolchain de Go 1.26 descargado automáticamente por `GOTOOLCHAIN=auto` (módulo `golang.org/toolchain@...go1.26.0...` en el mod cache) no trae el binario `covdata` — falla con `go: no such tool "covdata"` al correr `go test -coverprofile` sobre paquetes sin ningún `_test.go`. Confirmado que no es caché corrupto (persiste tras redescarga limpia). Por eso `make coverage`/`ci.yml` corren coverage solo sobre paquetes con `TestGoFiles` (`go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs go test ...`), no sobre `./...` directo — no tocar ese patrón sin motivo, evita el bug.
